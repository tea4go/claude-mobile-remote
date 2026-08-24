package transport

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tea4go/claude-mobile-remote/internal/channel"
	"github.com/tea4go/claude-mobile-remote/internal/pairing"
	projectsession "github.com/tea4go/claude-mobile-remote/internal/session"
)

type Authenticator interface {
	Authenticate(sessionID, token string) (*pairing.Lease, error)
}

type Config struct {
	ListenAddress  string
	AllowedOrigins []string
	AuthTimeout    time.Duration
	CloseTimeout   time.Duration
	ReadLimit      int64
}

func DefaultConfig() Config {
	return Config{
		ListenAddress:  "127.0.0.1:8765",
		AllowedOrigins: []string{"http://127.0.0.1:5173", "http://localhost:5173"},
		AuthTimeout:    5 * time.Second,
		CloseTimeout:   time.Second,
		ReadLimit:      int64(projectsession.MaxPromptBytes + 64*1024),
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
		_, allowed := allowedOrigins[origin]
		return allowed
	}}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", server.handleWebSocket)
	server.handler = mux
	return server, nil
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
