// Package remote exposes the single-session Remote Server as an importable public
// component. It composes the existing Pairing, Session Channel and WebSocket
// Transport into one Server that a host (the Standalone CLI today, WhaleTerm
// later) starts around a Session it already owns. It never creates an SDK Client
// and never starts or stops the host Session.
package remote

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/tea4go/claude-mobile-remote/internal/channel"
	"github.com/tea4go/claude-mobile-remote/internal/pairing"
	"github.com/tea4go/claude-mobile-remote/internal/transport"
	"github.com/tea4go/claude-mobile-remote/protocol"
)

// Controller is the public single-session control boundary a host provides. It
// uses only protocol types so an external host can implement it without
// importing any internal package. The Session Host owns Start/Stop; the Remote
// Server deliberately does not call them.
type Controller interface {
	Submit(context.Context, protocol.Input) (string, error)
	Subscribe(context.Context, int) (protocol.Snapshot, <-chan protocol.Event, error)
	Snapshot(context.Context) (protocol.Snapshot, error)
	ResolveAction(context.Context, string, protocol.ActionDecision, []protocol.QuestionAnswer) error
	CancelTurn(context.Context, string) error
	CancelQueued(context.Context, string) error
}

// ErrAlreadyStarted reports that Start was called after a successful start.
var ErrAlreadyStarted = errors.New("remote server already started")

// DefaultPairTokenTTL is shared by every host that wants the product default.
// Hosts may still provide a shorter positive TTL through Config.PairTokenTTL.
const DefaultPairTokenTTL = 30 * 24 * time.Hour

// Config is the public Server configuration. Zero-valued optional fields fall
// back to transport defaults; PairingPersistenceFile empty disables restart
// recovery, which the CLI uses for fake-host sessions.
type Config struct {
	SessionID              string
	Controller             Controller
	ListenAddress          string
	AdvertiseURL           string
	PairTokenTTL           time.Duration
	PairingPersistenceFile string
	PCID                   string
	PCName                 string
	AllowedOrigins         []string
	AuthTimeout            time.Duration
	CloseTimeout           time.Duration
	ReadLimit              int64
	ServiceConfig          protocol.ServiceConfig
}

// transportServer is the minimal seam the Server needs from Transport, so tests
// can inject a deterministic fake for rollback and shutdown paths.
type transportServer interface {
	Start() error
	BeginShutdown()
	Close(context.Context) error
}

type transportFactory func(transport.Config, transport.Authenticator, *channel.Registry) (transportServer, error)

func defaultTransportFactory(cfg transport.Config, auth transport.Authenticator, reg *channel.Registry) (transportServer, error) {
	return transport.NewServer(cfg, auth, reg)
}

// Server owns the pairing credential, the single Session Channel and the
// WebSocket transport for one session. It is not safe for concurrent Start.
type Server struct {
	sessionID  string
	controller channel.Controller
	manager    *pairing.Manager
	registry   *channel.Registry
	server     transportServer

	mu          sync.Mutex
	started     bool
	rotationErr error
	ticks       <-chan time.Time
	ticker      *time.Ticker
	pairings    chan protocol.Pairing
	stop        chan struct{}
	done        chan struct{}
	shutdown    sync.Once
	doneOnce    sync.Once
}

// New validates the stable configuration and constructs an unstarted Server.
func New(config Config) (*Server, error) {
	return newServer(config, defaultTransportFactory)
}

func newServer(config Config, factory transportFactory) (*Server, error) {
	if strings.TrimSpace(config.SessionID) == "" {
		return nil, errors.New("session ID is required")
	}
	if config.Controller == nil {
		return nil, errors.New("controller is required")
	}

	var managerOptions []pairing.Option
	if config.PairingPersistenceFile != "" {
		managerOptions = append(managerOptions, pairing.WithPersistenceFile(config.PairingPersistenceFile))
	}
	if config.PCID != "" {
		managerOptions = append(managerOptions, pairing.WithPCID(config.PCID))
	}
	if config.PCName != "" {
		managerOptions = append(managerOptions, pairing.WithPCName(config.PCName))
	}
	manager, err := pairing.NewManager(config.AdvertiseURL, config.PairTokenTTL, managerOptions...)
	if err != nil {
		return nil, err
	}
	registry := channel.NewRegistry()

	tcfg := transport.DefaultConfig()
	tcfg.ListenAddress = config.ListenAddress
	tcfg.AdvertiseURL = config.AdvertiseURL
	if len(config.AllowedOrigins) > 0 {
		tcfg.AllowedOrigins = append([]string(nil), config.AllowedOrigins...)
	}
	if config.AuthTimeout > 0 {
		tcfg.AuthTimeout = config.AuthTimeout
	}
	if config.CloseTimeout > 0 {
		tcfg.CloseTimeout = config.CloseTimeout
	}
	if config.ReadLimit > 0 {
		tcfg.ReadLimit = config.ReadLimit
	}
	serviceConfig := config.ServiceConfig
	if serviceConfig.ToolPreviewMaxRunes <= 0 {
		serviceConfig.ToolPreviewMaxRunes = protocol.DefaultToolPreviewMaxRunes
	}
	tcfg.ServiceConfig = serviceConfig

	server, err := factory(tcfg, manager, registry)
	if err != nil {
		return nil, err
	}
	return &Server{
		sessionID:  config.SessionID,
		controller: config.Controller,
		manager:    manager,
		registry:   registry,
		server:     server,
		pairings:   make(chan protocol.Pairing, 1),
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
	}, nil
}

// Start starts the transport, registers the single Session Channel, issues the
// first pairing payload and begins credential rotation. It does not touch the
// host Controller. Any step that fails rolls back the resources already set up.
func (s *Server) Start() (protocol.Pairing, error) {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return protocol.Pairing{}, ErrAlreadyStarted
	}
	s.mu.Unlock()

	if err := s.server.Start(); err != nil {
		return protocol.Pairing{}, err
	}
	sessionChannel := channel.New(s.sessionID, s.controller)
	if err := s.registry.Register(sessionChannel); err != nil {
		_ = s.server.Close(context.Background())
		return protocol.Pairing{}, err
	}
	payload, err := s.manager.Issue(s.sessionID)
	if err != nil {
		s.registry.Unregister(s.sessionID)
		_ = s.server.Close(context.Background())
		return protocol.Pairing{}, err
	}

	s.mu.Lock()
	if s.ticks == nil {
		s.ticker = time.NewTicker(time.Second)
		s.ticks = s.ticker.C
	}
	s.started = true
	s.mu.Unlock()
	go s.rotateLoop()
	return payload, nil
}

// Pairings returns the bounded channel of rotated pairing payloads. A slow
// consumer only ever sees the latest value.
func (s *Server) Pairings() <-chan protocol.Pairing {
	return s.pairings
}

// BeginShutdown stops new connections and new authentication immediately. It is
// idempotent.
func (s *Server) BeginShutdown() {
	s.shutdown.Do(func() {
		s.manager.Invalidate()
		s.server.BeginShutdown()
		close(s.stop)
		s.mu.Lock()
		started := s.started
		s.mu.Unlock()
		if !started {
			s.doneOnce.Do(func() { close(s.done) })
		}
	})
}

// Close waits for remote connections to close, revokes the credential and
// unregisters the Session Channel. It never stops the host Session. Close may be
// called more than once.
func (s *Server) Close(ctx context.Context) error {
	s.BeginShutdown()
	var waitErr error
	select {
	case <-s.done:
	case <-ctx.Done():
		waitErr = ctx.Err()
	}
	serverErr := s.server.Close(ctx)
	s.manager.Revoke()
	s.registry.Unregister(s.sessionID)
	s.mu.Lock()
	rotationErr := s.rotationErr
	s.mu.Unlock()
	return errors.Join(waitErr, serverErr, rotationErr)
}

func (s *Server) rotateLoop() {
	defer s.doneOnce.Do(func() { close(s.done) })
	defer func() {
		if s.ticker != nil {
			s.ticker.Stop()
		}
	}()
	for {
		select {
		case <-s.stop:
			return
		case <-s.ticks:
			payload, rotated, err := s.manager.RotateIfExpired()
			if errors.Is(err, pairing.ErrNoCredential) {
				continue
			}
			if err != nil {
				s.mu.Lock()
				s.rotationErr = err
				s.mu.Unlock()
				continue
			}
			if rotated {
				s.publishLatest(payload)
			}
		}
	}
}

func (s *Server) publishLatest(payload protocol.Pairing) {
	select {
	case s.pairings <- payload:
		return
	default:
	}
	select {
	case <-s.pairings:
	default:
	}
	select {
	case s.pairings <- payload:
	case <-s.stop:
	}
}
