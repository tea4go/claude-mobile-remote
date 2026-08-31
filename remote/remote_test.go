package remote

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tea4go/claude-mobile-remote/internal/channel"
	"github.com/tea4go/claude-mobile-remote/internal/transport"
	"github.com/tea4go/claude-mobile-remote/protocol"
)

// fakeController implements the public Controller contract plus Start/Stop
// counters, so tests can prove the Server never reaches the host lifecycle.
type fakeController struct {
	startCalls int
	stopCalls  int
}

func (f *fakeController) Submit(context.Context, protocol.Input) (string, error) { return "", nil }
func (f *fakeController) Subscribe(context.Context, int) (protocol.Snapshot, <-chan protocol.Event, error) {
	return protocol.Snapshot{SessionID: "session-1"}, nil, nil
}
func (f *fakeController) Snapshot(context.Context) (protocol.Snapshot, error) {
	return protocol.Snapshot{SessionID: "session-1"}, nil
}
func (f *fakeController) ResolveAction(context.Context, string, protocol.ActionDecision, []protocol.QuestionAnswer) error {
	return nil
}
func (f *fakeController) CancelTurn(context.Context, string) error { return nil }
func (f *fakeController) CancelQueued(context.Context, string) error {
	return nil
}
func (f *fakeController) Start(context.Context) error { f.startCalls++; return nil }
func (f *fakeController) Stop(context.Context) error  { f.stopCalls++; return nil }

type fakeTransport struct {
	startErr error
	config   transport.Config
	starts   int
	started  bool
	shutdown bool
	closed   bool
}

func (f *fakeTransport) Start() error {
	f.starts++
	f.started = true
	return f.startErr
}
func (f *fakeTransport) BeginShutdown() {
	f.shutdown = true
}
func (f *fakeTransport) Close(context.Context) error { f.closed = true; return nil }

func fakeFactory(ft *fakeTransport) transportFactory {
	return func(config transport.Config, _ transport.Authenticator, _ *channel.Registry) (transportServer, error) {
		ft.config = config
		return ft, nil
	}
}

func testConfig(controller Controller) Config {
	return Config{
		SessionID:     "session-1",
		Controller:    controller,
		ListenAddress: "127.0.0.1:0",
		AdvertiseURL:  "ws://127.0.0.1:8765/ws",
		PairTokenTTL:  time.Hour,
	}
}

func TestServerStartReturnsPairing(t *testing.T) {
	ft := &fakeTransport{}
	srv, err := newServer(testConfig(&fakeController{}), fakeFactory(ft))
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}
	payload, err := srv.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if payload.SessionID != "session-1" {
		t.Errorf("payload session ID = %q, want session-1", payload.SessionID)
	}
	if !ft.started {
		t.Errorf("transport not started")
	}
	if _, ok := srv.registry.Get("session-1"); !ok {
		t.Errorf("channel not registered after Start")
	}
	_ = srv.Close(context.Background())
}

func TestServerDoesNotCallControllerStartStop(t *testing.T) {
	controller := &fakeController{}
	srv, err := newServer(testConfig(controller), fakeFactory(&fakeTransport{}))
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}
	if _, err := srv.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := srv.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if controller.startCalls != 0 || controller.stopCalls != 0 {
		t.Errorf("Server called host lifecycle: start=%d stop=%d, want both 0",
			controller.startCalls, controller.stopCalls)
	}
}

func TestServerStartRollbackOnTransportStartError(t *testing.T) {
	ft := &fakeTransport{startErr: errors.New("listen failed")}
	srv, err := newServer(testConfig(&fakeController{}), fakeFactory(ft))
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}
	if _, err := srv.Start(); err == nil {
		t.Fatalf("Start() error = nil, want transport error")
	}
	if _, ok := srv.registry.Get("session-1"); ok {
		t.Errorf("channel registered despite transport failure")
	}
}

func TestServerStartRollbackOnChannelRegistrationError(t *testing.T) {
	controller := &fakeController{}
	ft := &fakeTransport{}
	srv, err := newServer(testConfig(controller), fakeFactory(ft))
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}
	// Pre-register the same session ID so Start's Register fails after transport
	// has already started, exercising the rollback path.
	if err := srv.registry.Register(channel.New("session-1", controller)); err != nil {
		t.Fatalf("pre-register error = %v", err)
	}
	if _, err := srv.Start(); err == nil {
		t.Fatalf("Start() error = nil, want registration error")
	}
	if !ft.closed {
		t.Errorf("transport not closed on registration rollback")
	}
}

func TestServerRejectsSecondStartWithoutTouchingTransport(t *testing.T) {
	ft := &fakeTransport{}
	srv, err := newServer(testConfig(&fakeController{}), fakeFactory(ft))
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}
	if _, err := srv.Start(); err != nil {
		t.Fatalf("first Start() error = %v", err)
	}
	if _, err := srv.Start(); err == nil {
		t.Fatalf("second Start() error = nil, want rejection")
	}
	if ft.starts != 1 {
		t.Errorf("transport Start calls = %d, want 1", ft.starts)
	}
	_ = srv.Close(context.Background())
}

func TestNewPreservesAskUserTextEnabledWhenDefaultingPreviewLimit(t *testing.T) {
	ft := &fakeTransport{}
	config := testConfig(&fakeController{})
	config.ServiceConfig = protocol.ServiceConfig{AskUserTextEnabled: true}
	srv, err := newServer(config, fakeFactory(ft))
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}
	if ft.config.ServiceConfig.ToolPreviewMaxRunes != protocol.DefaultToolPreviewMaxRunes {
		t.Errorf("preview limit = %d, want default %d", ft.config.ServiceConfig.ToolPreviewMaxRunes, protocol.DefaultToolPreviewMaxRunes)
	}
	if !ft.config.ServiceConfig.AskUserTextEnabled {
		t.Errorf("AskUserTextEnabled = false, want true")
	}
	_ = srv.Close(context.Background())
}

func TestServerCloseUnregistersChannel(t *testing.T) {
	srv, err := newServer(testConfig(&fakeController{}), fakeFactory(&fakeTransport{}))
	if err != nil {
		t.Fatalf("newServer() error = %v", err)
	}
	if _, err := srv.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := srv.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, ok := srv.registry.Get("session-1"); ok {
		t.Errorf("channel still registered after Close")
	}
}

func TestNewRejectsInvalidConfig(t *testing.T) {
	if _, err := New(Config{Controller: &fakeController{}, AdvertiseURL: "ws://127.0.0.1:8765/ws", PairTokenTTL: time.Hour}); err == nil {
		t.Errorf("New() accepted empty session ID")
	}
	if _, err := New(Config{SessionID: "session-1", AdvertiseURL: "ws://127.0.0.1:8765/ws", PairTokenTTL: time.Hour}); err == nil {
		t.Errorf("New() accepted nil controller")
	}
}
