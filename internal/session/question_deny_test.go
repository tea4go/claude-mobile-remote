package session

import (
	projectclaude "github.com/tea4go/claude-mobile-remote/internal/claude"
	"testing"
)

func TestQuestionDenyDoesNotInterruptTurn(t *testing.T) {
	decision, err := buildActionDecision(&PendingAction{Kind: ActionAskUserQuestion}, DecisionDeny, nil)
	if err != nil || decision.Kind != projectclaude.DecisionDeny || decision.Interrupt {
		t.Fatalf("question deny = %#v, %v", decision, err)
	}
}

func TestQuestionDenyResolvesPendingAndKeepsTurnRunning(t *testing.T) {
	s := startActiveSession(t)
	request, event := awaitActionRequired(t, s, questionSpec(choiceQuestion("H", "Q?", false, "A", "B")))
	if err := resolveAction(t, s.controller, event.PendingAction.ActionID, DecisionDeny, nil); err != nil {
		t.Fatal(err)
	}
	decision := awaitDecision(t, request)
	if decision.Kind != projectclaude.DecisionDeny || decision.Interrupt {
		t.Fatalf("decision = %#v", decision)
	}
	resolved := receiveDomainEvent(t, s.events)
	if resolved.Kind != EventActionResolved || resolved.TurnStatus != TurnRunning {
		t.Fatalf("event = %#v", resolved)
	}
	if snapshot := snapshotController(t, s.controller); snapshot.Turn.Status != TurnRunning || snapshot.Turn.PendingAction != nil {
		t.Fatalf("turn = %#v", snapshot.Turn)
	}
}
