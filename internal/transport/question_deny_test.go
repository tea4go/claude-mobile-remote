package transport

import (
	"github.com/tea4go/claude-mobile-remote/protocol"
	"testing"
)

func TestQuestionDenyResolution(t *testing.T) {
	action := protocol.PendingAction{ActionID: "a", Kind: protocol.ActionAskUserQuestion}
	payload := CommandPayload{Kind: CommandResolveAction, ActionID: "a", Decision: protocol.DecisionDeny}
	if err := ValidateActionResolution(action, protocol.ServiceConfig{}, payload); err != nil {
		t.Fatal(err)
	}
	payload.Decision = protocol.DecisionAllowOnce
	if err := ValidateActionResolution(action, protocol.ServiceConfig{}, payload); err == nil {
		t.Fatal("accepted question allow without answers")
	}
}
