package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/tea4go/claude-mobile-remote/internal/channel"
	projectclaude "github.com/tea4go/claude-mobile-remote/internal/claude"
	"github.com/tea4go/claude-mobile-remote/internal/pairing"
	projectsession "github.com/tea4go/claude-mobile-remote/internal/session"
	"github.com/tea4go/claude-mobile-remote/internal/transport"
	"github.com/tea4go/claude-mobile-remote/protocol"
	"github.com/tea4go/claude-mobile-remote/push"
)

const maxRetainedSessions = 128

var cliCapabilities = []string{"create_session", "stop_session"}

type managedSession struct {
	id         string
	controller *projectsession.Controller
	pairing    *pairing.Manager
	payload    protocol.Pairing
	state      string // active, stopping, or terminal; protected by managedCLI.mu
}

type creationResult struct {
	done    chan struct{}
	payload protocol.Pairing
	err     error
}

// managedCLI is a Standalone-only multiplexer. remote.Server remains the public
// single-Session component used by embedded hosts.
type managedCLI struct {
	mu          sync.Mutex
	config      cliConfig
	cwd         string
	ctx         context.Context
	cancel      context.CancelFunc
	registry    *channel.Registry
	transport   *transport.Server
	sessions    map[string]*managedSession
	active      int
	started     bool
	closing     bool
	creates     map[string]*creationResult
	createOrder []string
	lastCreate  map[string]time.Time
	pairings    chan protocol.Pairing
	closeOnce   sync.Once
	closeErr    error
	pushSender  push.Sender
}

func newManagedCLI(config cliConfig) (*managedCLI, error) {
	if config.pcID == "" {
		var err error
		if config.fakeHostScenario == "" {
			config.pcID, err = defaultPCID()
		} else {
			config.pcID, err = randomPCID()
		}
		if err != nil {
			return nil, err
		}
	}
	cwd := config.cwd
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("resolve CLI working directory: %w", err)
		}
	}
	absolute, err := filepath.Abs(cwd)
	if err != nil {
		return nil, fmt.Errorf("resolve CLI working directory: %w", err)
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.IsDir() {
		return nil, errors.New("CLI working directory is unavailable")
	}
	var sender push.Sender
	if config.fakeHostScenario == "" {
		configured, e := push.LoadDefault()
		if e != nil {
			return nil, e
		}
		if configured != nil {
			sender = configured
		}
	}
	config.cwd = absolute
	ctx, cancel := context.WithCancel(context.Background())
	m := &managedCLI{
		config: config, cwd: absolute, pushSender: sender, ctx: ctx, cancel: cancel,
		registry: channel.NewRegistry(), sessions: make(map[string]*managedSession),
		creates: make(map[string]*creationResult), lastCreate: make(map[string]time.Time),
		pairings: make(chan protocol.Pairing, 32),
	}
	tcfg := transport.DefaultConfig()
	tcfg.ListenAddress = config.listen
	tcfg.AdvertiseURL = config.advertiseURL
	tcfg.AllowedOrigins = append([]string(nil), config.allowedOrigins...)
	tcfg.Management = m
	server, err := transport.NewServer(tcfg, m, m.registry)
	if err != nil {
		cancel()
		return nil, err
	}
	m.transport = server
	return m, nil
}

func (m *managedCLI) Authenticate(sessionID, token string) (*pairing.Lease, error) {
	m.mu.Lock()
	session := m.sessions[sessionID]
	closing := m.closing
	m.mu.Unlock()
	if session == nil || closing {
		return nil, pairing.ErrUnauthorized
	}
	return session.pairing.Authenticate(sessionID, token)
}

func (m *managedCLI) Start(ctx context.Context) (protocol.Pairing, error) {
	m.mu.Lock()
	if m.started || m.closing {
		m.mu.Unlock()
		return protocol.Pairing{}, errors.New("CLI manager already started or closing")
	}
	m.started = true
	m.active++
	m.mu.Unlock()
	go func() {
		select {
		case <-ctx.Done():
			m.cancel()
		case <-m.ctx.Done():
		}
	}()

	session, err := m.startRuntime(true)
	if err != nil {
		m.mu.Lock()
		m.active--
		m.mu.Unlock()
		return protocol.Pairing{}, err
	}
	m.mu.Lock()
	m.sessions[session.id] = session
	m.mu.Unlock()
	if err := m.transport.Start(); err != nil {
		m.registry.Unregister(session.id)
		session.pairing.Revoke()
		_ = session.controller.Stop(context.Background())
		m.mu.Lock()
		delete(m.sessions, session.id)
		m.active--
		m.mu.Unlock()
		return protocol.Pairing{}, err
	}
	m.monitor(session)
	go m.rotateLoop()
	return session.payload, nil
}

func (m *managedCLI) startRuntime(initial bool) (*managedSession, error) {
	config := m.config
	if !initial {
		config.resumeSessionID = ""
	}
	var host projectsession.Host
	var id string
	var err error
	if !initial && config.fakeHostScenario != "" {
		id, err = randomSessionID()
		if err == nil {
			host, err = projectclaude.NewFakeHostForSession(config.fakeHostScenario, id)
		}
	} else {
		host, id, err = buildHost(config)
	}
	if err != nil {
		return nil, err
	}
	options := []projectsession.Option{projectsession.WithReservedSessionID(id)}
	createdAt := time.Time{}
	if config.resumeSessionID == "" {
		createdAt = time.Now()
	}
	options = append(options, projectsession.WithDisplayMetadata(m.cwd, createdAt))
	if config.permissionMode != "" {
		serviceConfig := protocol.DefaultServiceConfig()
		serviceConfig.PermissionMode = protocol.PermissionMode(config.permissionMode)
		options = append(options, projectsession.WithServiceConfig(serviceConfig))
	}
	controller := projectsession.NewController(host, options...)
	if err := controller.Start(m.ctx); err != nil {
		_ = controller.Stop(context.Background())
		return nil, err
	}

	var pairingFile string
	if config.fakeHostScenario == "" {
		pairingFile, err = defaultPairingFile(id)
		if err != nil {
			_ = controller.Stop(context.Background())
			return nil, err
		}
		_ = pairing.PruneExpired(filepath.Dir(pairingFile), time.Now())
	}
	var pairingOptions []pairing.Option
	if pairingFile != "" {
		pairingOptions = append(pairingOptions, pairing.WithPersistenceFile(pairingFile))
	}
	pairingOptions = append(pairingOptions, pairing.WithPCID(m.config.pcID))
	if hostName := cliHostName(); hostName != "" {
		pairingOptions = append(pairingOptions, pairing.WithPCName(hostName))
	}
	credential, err := pairing.NewManager(config.advertiseURL, config.pairTokenTTL, pairingOptions...)
	if err != nil {
		_ = controller.Stop(context.Background())
		return nil, err
	}
	if err := m.registry.Register(channel.New(id, &retainedController{Controller: controller})); err != nil {
		_ = controller.Stop(context.Background())
		return nil, err
	}
	payload, err := credential.Issue(id)
	if err != nil {
		m.registry.Unregister(id)
		_ = controller.Stop(context.Background())
		return nil, err
	}
	payload.Capabilities = append([]string(nil), cliCapabilities...)
	return &managedSession{id: id, controller: controller, pairing: credential, payload: payload, state: "active"}, nil
}

func randomSessionID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16]), nil
}

func (m *managedCLI) CreateSession(ctx context.Context, sourceSessionID, commandID string) (protocol.Pairing, error) {
	key := sourceSessionID + ":" + commandID
	m.mu.Lock()
	if m.closing || !m.started || m.sessions[sourceSessionID] == nil {
		m.mu.Unlock()
		return protocol.Pairing{}, &transport.ProtocolError{Code: transport.CodeSessionUnavailable, Message: "session unavailable"}
	}
	if result := m.creates[key]; result != nil {
		m.mu.Unlock()
		select {
		case <-result.done:
			return result.payload, result.err
		case <-ctx.Done():
			return protocol.Pairing{}, ctx.Err()
		}
	}
	if m.active >= m.config.maxSessions || len(m.sessions)+m.active >= maxRetainedSessions {
		m.mu.Unlock()
		return protocol.Pairing{}, &transport.ProtocolError{Code: "session_capacity", Message: "session capacity reached"}
	}
	if time.Since(m.lastCreate[sourceSessionID]) < time.Second {
		m.mu.Unlock()
		return protocol.Pairing{}, &transport.ProtocolError{Code: "create_rate_limited", Message: "create requests are too frequent"}
	}
	m.lastCreate[sourceSessionID] = time.Now()
	m.active++
	result := &creationResult{done: make(chan struct{})}
	m.creates[key] = result
	m.createOrder = append(m.createOrder, key)
	m.mu.Unlock()

	session, err := m.startRuntime(false)
	m.mu.Lock()
	if err != nil {
		m.active--
		result.err = &transport.ProtocolError{Code: transport.CodeInternal, Message: "session creation failed"}
	} else if m.closing {
		m.active--
		result.err = &transport.ProtocolError{Code: transport.CodeSessionUnavailable, Message: "session unavailable"}
	} else {
		m.sessions[session.id] = session
		result.payload = session.payload
	}
	close(result.done)
	m.trimCreateCacheLocked()
	m.mu.Unlock()
	if err != nil || result.err != nil {
		if session != nil {
			m.registry.Unregister(session.id)
			session.pairing.Revoke()
			_ = session.controller.Stop(context.Background())
		}
		return protocol.Pairing{}, result.err
	}
	m.monitor(session)
	m.publishPairing(session.payload)
	return session.payload, nil
}

func (m *managedCLI) trimCreateCacheLocked() {
	for len(m.createOrder) > 128 {
		key := m.createOrder[0]
		if result := m.creates[key]; result != nil {
			select {
			case <-result.done:
				delete(m.creates, key)
			default:
				return
			}
		}
		m.createOrder = m.createOrder[1:]
	}
}

func (m *managedCLI) StopSession(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	session := m.sessions[sessionID]
	if m.closing || session == nil {
		m.mu.Unlock()
		return &transport.ProtocolError{Code: transport.CodeSessionUnavailable, Message: "session unavailable"}
	}
	if session.state == "terminal" {
		m.mu.Unlock()
		return nil
	}
	if session.state == "stopping" {
		m.mu.Unlock()
		return &transport.ProtocolError{Code: transport.CodeSessionUnavailable, Message: "session is stopping"}
	}
	session.state = "stopping"
	m.mu.Unlock()
	err := session.controller.Stop(ctx)
	if err == nil {
		m.markTerminal(sessionID)
	} else {
		// A caller timeout does not prove the actor accepted Stop. Let a later
		// request retry; the monitor will still mark a completed stop terminal.
		m.mu.Lock()
		if session.state == "stopping" {
			session.state = "active"
		}
		m.mu.Unlock()
	}
	return err
}

func (m *managedCLI) markTerminal(sessionID string) {
	m.mu.Lock()
	if session := m.sessions[sessionID]; session != nil && session.state != "terminal" {
		session.state = "terminal"
		m.active--
	}
	m.mu.Unlock()
}

func (m *managedCLI) monitor(session *managedSession) {
	if m.pushSender != nil {
		go push.Watch(m.ctx, session.controller, m.pushSender, func() (protocol.Pairing, bool) {
			m.mu.Lock()
			defer m.mu.Unlock()
			return session.payload, !m.closing
		})
	}
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-ticker.C:
			}
			snapshot, err := session.controller.Snapshot(m.ctx)
			if err == nil && (snapshot.SessionStatus == projectsession.SessionStopped || snapshot.SessionStatus == projectsession.SessionFailed) {
				m.markTerminal(session.id)
				return
			}
		}
	}()
}

func (m *managedCLI) rotateLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
		}
		m.mu.Lock()
		sessions := make([]*managedSession, 0, len(m.sessions))
		for _, session := range m.sessions {
			sessions = append(sessions, session)
		}
		m.mu.Unlock()
		for _, session := range sessions {
			payload, rotated, err := session.pairing.RotateIfExpired()
			if err != nil || !rotated {
				continue
			}
			payload.Capabilities = append([]string(nil), cliCapabilities...)
			m.mu.Lock()
			session.payload = payload
			m.mu.Unlock()
			m.publishPairing(payload)
		}
	}
}

func (m *managedCLI) publishPairing(payload protocol.Pairing) {
	select {
	case m.pairings <- payload:
	default:
	}
}

func (m *managedCLI) Pairings() <-chan protocol.Pairing { return m.pairings }

func (m *managedCLI) SessionIDs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (m *managedCLI) Session(sessionID string) (*projectsession.Controller, protocol.Pairing, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session := m.sessions[sessionID]
	if session == nil {
		return nil, protocol.Pairing{}, false
	}
	return session.controller, session.payload, true
}

func (m *managedCLI) Close(ctx context.Context) error {
	m.closeOnce.Do(func() {
		m.mu.Lock()
		m.closing = true
		sessions := make([]*managedSession, 0, len(m.sessions))
		for _, session := range m.sessions {
			sessions = append(sessions, session)
			session.pairing.Invalidate()
		}
		m.mu.Unlock()
		m.transport.BeginShutdown()
		m.cancel()
		var failures []error
		for _, session := range sessions {
			if err := session.controller.Stop(ctx); err != nil {
				failures = append(failures, err)
			}
		}
		if err := m.transport.Close(ctx); err != nil {
			failures = append(failures, err)
		}
		for _, session := range sessions {
			session.pairing.Revoke()
			m.registry.Unregister(session.id)
		}
		m.closeErr = errors.Join(failures...)
	})
	return m.closeErr
}

// retainedController lets a stopped Session's credential obtain its final
// Snapshot. Its closed Runtime cannot accept execution commands.
type retainedController struct{ *projectsession.Controller }

func (r *retainedController) Subscribe(ctx context.Context, capacity int) (protocol.Snapshot, <-chan protocol.Event, error) {
	snapshot, events, err := r.Controller.Subscribe(ctx, capacity)
	if err == nil {
		retainedEvents := make(chan protocol.Event, capacity)
		go func() {
			defer close(retainedEvents)
			terminal := false
			for event := range events {
				if event.Kind == projectsession.EventSessionStopped || event.Kind == projectsession.EventSessionFailed {
					terminal = true
				}
				select {
				case retainedEvents <- event:
				case <-ctx.Done():
					return
				}
			}
			if terminal {
				<-ctx.Done()
			}
		}()
		return snapshot, retainedEvents, nil
	}
	if !errors.Is(err, projectsession.ErrSessionUnavailable) {
		return snapshot, events, err
	}
	snapshot, err = r.Controller.Snapshot(ctx)
	if err != nil {
		return protocol.Snapshot{}, nil, err
	}
	if snapshot.SessionStatus != projectsession.SessionStopped && snapshot.SessionStatus != projectsession.SessionFailed {
		return protocol.Snapshot{}, nil, projectsession.ErrSessionUnavailable
	}
	terminalEvents := make(chan protocol.Event)
	go func() {
		<-ctx.Done()
		close(terminalEvents)
	}()
	return snapshot, terminalEvents, nil
}
