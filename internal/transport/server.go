package transport

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tea4go/claude-mobile-remote/internal/channel"
	"github.com/tea4go/claude-mobile-remote/internal/pairing"
	"github.com/tea4go/claude-mobile-remote/protocol"
)

type Authenticator interface {
	Authenticate(sessionID, token string) (*pairing.Lease, error)
}

// ManagementHandler is installed only by the Standalone CLI. Embedded
// single-Session remote.Server instances leave it nil and remain unchanged.
type ManagementHandler interface {
	CreateSession(context.Context, string, string) (protocol.Pairing, error)
	StopSession(context.Context, string) error
}

type Config struct {
	ListenAddress  string
	AllowedOrigins []string
	// AdvertiseURL is the address the phone is told to dial. Its host is trusted
	// as an Origin host: the user has already declared that clients reach this
	// server through it, and a real-device live reload shell is served from that
	// same host. Empty contributes no host.
	AdvertiseURL string
	AuthTimeout  time.Duration
	CloseTimeout time.Duration
	ReadLimit    int64
	// ServiceConfig gates which resolutions a remote client may even express.
	// It must stay identical to the value handed to session.WithServiceConfig:
	// the transport only pre-validates, the Controller still owns the verdict,
	// so a divergence would surface as a rejection the UI cannot explain.
	ServiceConfig protocol.ServiceConfig
	Management    ManagementHandler
}

func DefaultConfig() Config {
	return Config{
		ListenAddress:  "127.0.0.1:8765",
		AllowedOrigins: []string{"http://127.0.0.1:5173", "http://localhost:5173"},
		AuthTimeout:    5 * time.Second,
		CloseTimeout:   time.Second,
		ReadLimit:      int64(protocol.MaxPromptBytes + 64*1024),
		ServiceConfig:  protocol.DefaultServiceConfig(),
	}
}

type Server struct {
	mu            sync.Mutex
	config        Config
	authenticator Authenticator
	registry      *channel.Registry
	upgrader      websocket.Upgrader
	handler       http.Handler
	httpServer    *http.Server
	listener      net.Listener
	connections   map[*connection]struct{}
	shuttingDown  bool
}

func NewServer(config Config, authenticator Authenticator, registry *channel.Registry) (*Server, error) {
	if authenticator == nil || registry == nil {
		return nil, errors.New("transport dependencies are required")
	}
	if config.ListenAddress == "" || config.AuthTimeout <= 0 || config.CloseTimeout <= 0 || config.ReadLimit <= 0 {
		return nil, errors.New("invalid transport configuration")
	}
	allowedOrigins := make(map[string]struct{}, len(config.AllowedOrigins))
	for _, origin := range config.AllowedOrigins {
		allowedOrigins[origin] = struct{}{}
	}
	advertisedHost := advertiseHost(config.AdvertiseURL)
	server := &Server{
		config:        config,
		authenticator: authenticator,
		registry:      registry,
		connections:   make(map[*connection]struct{}),
	}
	server.upgrader = websocket.Upgrader{CheckOrigin: func(request *http.Request) bool {
		origin := request.Header.Get("Origin")
		if origin == "" {
			return true
		}
		if _, allowed := allowedOrigins[origin]; allowed {
			return true
		}
		return isTrustedOriginHost(origin, advertisedHost)
	}}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", server.handleWebSocket)
	server.handler = mux
	return server, nil
}

// advertiseHost extracts the host clients are told to dial. An empty or
// unparseable AdvertiseURL simply contributes no host.
func advertiseHost(advertiseURL string) string {
	parsed, err := url.Parse(advertiseURL)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

// isTrustedOriginHost decides by rule rather than by enumeration. An enumerated
// list cannot keep up with the Origins a browser shell actually sends — the
// Android shell is http://localhost when it loads its bundle and
// http://10.0.2.2:5173 under live reload, a real device uses the LAN IP, and the
// port follows whatever Vite picked — and every miss is an HTTP 403 that a
// browser can only observe as an undiagnosable 1006. Ports are deliberately
// ignored: the page and the WebSocket come from different processes with
// unrelated ports.
//
// The DNS-rebinding defense is unchanged. An attacker page keeps its own Origin
// even after its hostname resolves to 127.0.0.1, so "resolves locally" is never
// treated as trusted, and the advertised host is compared for equality only —
// never by suffix, so 192.168.1.10.evil.com stays rejected.
func isTrustedOriginHost(origin string, advertisedHost string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}
	// Android emulator's NAT alias for the host machine.
	if host == "10.0.2.2" {
		return true
	}
	return advertisedHost != "" && host == advertisedHost
}

func (s *Server) Handler() http.Handler {
	return s.handler
}

func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.config.ListenAddress)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if s.shuttingDown {
		s.mu.Unlock()
		_ = listener.Close()
		return errors.New("transport is shutting down")
	}
	s.listener = listener
	s.httpServer = &http.Server{Handler: s.handler}
	httpServer := s.httpServer
	s.mu.Unlock()
	go func() { _ = httpServer.Serve(listener) }()
	return nil
}

func (s *Server) BeginShutdown() {
	s.mu.Lock()
	if s.shuttingDown {
		s.mu.Unlock()
		return
	}
	s.shuttingDown = true
	listener := s.listener
	s.mu.Unlock()
	if listener != nil {
		_ = listener.Close()
	}
}

func (s *Server) Close(ctx context.Context) error {
	s.BeginShutdown()
	s.mu.Lock()
	connections := make([]*connection, 0, len(s.connections))
	for connection := range s.connections {
		connections = append(connections, connection)
	}
	s.mu.Unlock()
	var closeErr error
	for _, connection := range connections {
		if err := connection.gracefulShutdown(ctx); err != nil {
			closeErr = err
			break
		}
	}
	if closeErr != nil {
		for _, connection := range connections {
			connection.shutdown()
		}
		return closeErr
	}
	for _, connection := range connections {
		select {
		case <-connection.done:
		case <-ctx.Done():
			for _, remaining := range connections {
				remaining.shutdown()
			}
			return ctx.Err()
		}
	}
	return nil
}

func (s *Server) isShuttingDown() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.shuttingDown
}

func (s *Server) handleWebSocket(response http.ResponseWriter, request *http.Request) {
	s.mu.Lock()
	shuttingDown := s.shuttingDown
	s.mu.Unlock()
	if shuttingDown {
		http.Error(response, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	socket, err := s.upgrader.Upgrade(response, request, nil)
	if err != nil {
		return
	}
	connection := newConnection(s, socket)
	s.mu.Lock()
	if s.shuttingDown {
		s.mu.Unlock()
		_ = socket.Close()
		return
	}
	s.connections[connection] = struct{}{}
	s.mu.Unlock()
	connection.run()
	s.mu.Lock()
	delete(s.connections, connection)
	s.mu.Unlock()
}
