package remote_test

import (
	"context"
	"testing"
	"time"

	"github.com/tea4go/claude-mobile-remote/protocol"
	"github.com/tea4go/claude-mobile-remote/remote"
)

// TestExternalControllerImportable proves an external host can satisfy the
// remote.Controller contract using only remote and protocol, with no internal
// import. It is a compile-time assertion: if a signature drifts, this stops
// compiling.
func TestExternalControllerImportable(t *testing.T) {
	var _ remote.Controller = externalController{}
	_ = t
}

type externalController struct{}

func (externalController) Submit(context.Context, protocol.Input) (string, error) {
	return "", nil
}
func (externalController) Subscribe(context.Context, int) (protocol.Snapshot, <-chan protocol.Event, error) {
	return protocol.Snapshot{}, nil, nil
}
func (externalController) Snapshot(context.Context) (protocol.Snapshot, error) {
	return protocol.Snapshot{}, nil
}
func (externalController) ResolveAction(context.Context, string, protocol.ActionDecision, []protocol.QuestionAnswer) error {
	return nil
}
func (externalController) CancelTurn(context.Context, string) error { return nil }
func (externalController) CancelQueued(context.Context, string) error {
	return nil
}

// TestNewPublicSurface exercises the public constructor from outside the package
// and confirms Start returns a protocol.Pairing for the configured session.
func TestNewPublicSurface(t *testing.T) {
	srv, err := remote.New(remote.Config{
		SessionID:     "session-1",
		Controller:    externalController{},
		ListenAddress: "127.0.0.1:0",
		AdvertiseURL:  "ws://127.0.0.1:8765/ws",
		PairTokenTTL:  time.Hour,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	payload, err := srv.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if payload.SessionID != "session-1" {
		t.Errorf("payload session ID = %q, want session-1", payload.SessionID)
	}
	if err := srv.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
