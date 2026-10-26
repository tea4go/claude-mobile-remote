package remote

import (
	"context"
	"github.com/tea4go/claude-mobile-remote/protocol"
	"testing"
)

func TestProgressSnapshotIsolationAndTerminalCleanup(t *testing.T) {
	s, err := NewEventStream(protocol.Snapshot{SessionID: "s"})
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Publish(protocol.Event{Kind: protocol.EventTurnStarted, RequestID: "r", Source: protocol.InputTerminal, TurnStatus: protocol.TurnRunning, Text: "input"})
	p := &protocol.TurnProgress{Phase: "writing", StartedAt: 100, UpdatedAt: 200, OutputTokens: 42}
	_ = s.Publish(protocol.Event{Kind: protocol.EventTurnProgress, RequestID: "r", Progress: p})
	p.OutputTokens = 999
	snapshot, _ := s.Snapshot(context.Background())
	if snapshot.Turn.Progress.OutputTokens != 42 {
		t.Fatal("mutable progress")
	}
	snapshot.Turn.Progress.OutputTokens = 888
	_ = s.Publish(protocol.Event{Kind: protocol.EventSessionDiagnostic, RequestID: "other", Diagnostic: &protocol.SessionDiagnostic{Code: "receive_failed"}})
	snapshot, _ = s.Snapshot(context.Background())
	if snapshot.Turn.RequestID != "r" || snapshot.Turn.Progress.OutputTokens != 42 {
		t.Fatal("diagnostic changed turn")
	}
	_ = s.Publish(protocol.Event{Kind: protocol.EventTurnCompleted, RequestID: "r", TurnStatus: protocol.TurnCompleted})
	_ = s.Publish(protocol.Event{Kind: protocol.EventTurnProgress, RequestID: "r", Progress: p})
	snapshot, _ = s.Snapshot(context.Background())
	if snapshot.Turn.Progress != nil {
		t.Fatal("late progress revived finished turn")
	}
	_ = s.Publish(protocol.Event{Kind: protocol.EventTurnStarted, RequestID: "new", TurnStatus: protocol.TurnRunning})
	before := snapshot.Seq + 1
	_ = s.Publish(protocol.Event{Kind: protocol.EventTurnProgress, RequestID: "r", Progress: p})
	snapshot, _ = s.Snapshot(context.Background())
	if snapshot.Turn.RequestID != "new" || snapshot.Seq != before {
		t.Fatal("late progress changed new turn")
	}
}
