package transport

import (
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestNetworkProbeKeepsResponsivePeerAndRejectsUnrelatedPong(t *testing.T) {
	changes := make(chan struct{}, 64)
	fixture := newServerFixture(t, func(config *Config) {
		config.OnConnectionChange = func() {
			select {
			case changes <- struct{}{}:
			default:
			}
		}
	})
	healthy := fixture.dial(t)
	authenticateAndSubscribe(t, fixture, healthy)
	silent := fixture.dial(t)
	authenticateAndSubscribe(t, fixture, silent)
	// This client still reads, but responds with an unrelated ordinary heartbeat.
	silent.SetPingHandler(func(string) error {
		return silent.WriteControl(websocket.PongMessage, nil, time.Now().Add(time.Second))
	})
	var readers sync.WaitGroup
	for _, socket := range []*websocket.Conn{healthy, silent} {
		readers.Add(1)
		go func(socket *websocket.Conn) {
			defer readers.Done()
			for {
				if _, _, err := socket.ReadMessage(); err != nil {
					return
				}
			}
		}(socket)
	}
	defer func() { healthy.Close(); silent.Close(); readers.Wait() }()
	// A callback can read status: notification must run outside the server lock.
	select {
	case <-changes:
	case <-time.After(time.Second):
		t.Fatal("missing connection notification")
	}
	fixture.server.mu.Lock()
	clients := make([]*connection, 0, 2)
	for c := range fixture.server.subscribed {
		clients = append(clients, c)
	}
	fixture.server.mu.Unlock()
	for _, c := range clients {
		c.startProbe(100 * time.Millisecond)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fixture.server.ConnectionStatus("session-1").OnlineConnections == 1 {
			// Probe again through the public transport entry point; the live peer remains.
			fixture.server.RecheckConnections()
			time.Sleep(150 * time.Millisecond)
			if got := fixture.server.ConnectionStatus("session-1").OnlineConnections; got != 1 {
				t.Fatalf("responsive peer lost: %d", got)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("unrelated pong kept an unresponsive probe alive")
}

func TestConnectionNotificationOutsideLock(t *testing.T) {
	var server *Server
	called := make(chan struct{}, 16)
	fixture := newServerFixture(t, func(config *Config) {
		config.OnConnectionChange = func() {
			server.ConnectionStatus("session-1")
			select {
			case called <- struct{}{}:
			default:
			}
		}
	})
	server = fixture.server
	socket := fixture.dial(t)
	authenticateAndSubscribe(t, fixture, socket)
	socket.Close()
	deadline := time.After(time.Second)
	for {
		select {
		case <-called:
			if server.ConnectionStatus("session-1").OnlineConnections == 0 {
				return
			}
		case <-deadline:
			t.Fatal("disconnect callback missing or locked")
		}
	}
}
