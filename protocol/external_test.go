package protocol_test

import (
	"context"
	"testing"

	"github.com/tea4go/claude-mobile-remote/protocol"
)

// TestExternalControllerShape proves an external host can express the public
// Controller contract using only protocol types, without importing any internal
// package. It is a compile-time assertion: if a signature drifts, this test stops
// compiling.
func TestExternalControllerShape(t *testing.T) {
	var _ protocolController = externalController{}
	_ = t
}

type protocolController interface {
	Submit(context.Context, protocol.Input) (string, error)
	Snapshot(context.Context) (protocol.Snapshot, error)
	Subscribe(context.Context, int) (protocol.Snapshot, <-chan protocol.Event, error)
	ResolveAction(context.Context, string, protocol.ActionDecision, []protocol.QuestionAnswer) error
	CancelTurn(context.Context, string) error
	CancelQueued(context.Context, string) error
}

type externalController struct{}

func (externalController) Submit(context.Context, protocol.Input) (string, error) {
	return "", nil
}
func (externalController) Snapshot(context.Context) (protocol.Snapshot, error) {
	return protocol.Snapshot{}, nil
}
func (externalController) Subscribe(context.Context, int) (protocol.Snapshot, <-chan protocol.Event, error) {
	return protocol.Snapshot{}, nil, nil
}
func (externalController) ResolveAction(context.Context, string, protocol.ActionDecision, []protocol.QuestionAnswer) error {
	return nil
}
func (externalController) CancelTurn(context.Context, string) error { return nil }
func (externalController) CancelQueued(context.Context, string) error {
	return nil
}
