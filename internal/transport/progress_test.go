package transport

import (
	"encoding/json"
	"github.com/tea4go/claude-mobile-remote/protocol"
	"testing"
)

func TestProgressWireCompatibility(t *testing.T) {
	p := &protocol.TurnProgress{Phase: "thinking", StartedAt: 100, UpdatedAt: 200, OutputTokens: 42, TokensKnown: true}
	events := []protocol.Event{
		{Kind: protocol.EventTurnProgress, SessionID: "s", RequestID: "r", Seq: 7, Progress: p},
		{Kind: protocol.EventSessionDiagnostic, SessionID: "s", RequestID: "r", Seq: 8, Diagnostic: &protocol.SessionDiagnostic{Code: "api_retry", HTTPStatus: 429, Attempt: 2, MaxAttempts: 5, RetryDelayMs: 1500}},
	}
	for _, version := range []int{2, 3, 4, 5} {
		for _, event := range events {
			c := &connection{version: version, sessionID: "s"}
			envelope, err := c.outboundEvent(event)
			if err != nil {
				t.Fatal(err)
			}
			data, err := EncodeEnvelope(envelope)
			if err != nil {
				t.Fatalf("v%d: %v", version, err)
			}
			if envelope.Seq != event.Seq {
				t.Fatal("sequence lost")
			}
			if version < 5 {
				if envelope.Type != TypeUnsupported {
					t.Fatal("old client received new event")
				}
				var payload UnsupportedPayload
				if json.Unmarshal(envelope.Payload, &payload) != nil || payload.Code != protocol.UnsupportedUnknownSubtype {
					t.Fatal("invalid fallback")
				}
			} else if envelope.Type != TypeEvent {
				t.Fatal("new client lost progress")
			}
			if _, err := DecodeEnvelope(data); err != nil {
				t.Fatal(err)
			}
		}
	}
}
func TestProgressRejectsUnsafeMetadata(t *testing.T) {
	p := &protocol.TurnProgress{Phase: "thinking", StartedAt: 100, UpdatedAt: 200}
	event := protocol.Event{Kind: protocol.EventTurnProgress, SessionID: "s", RequestID: "r", Progress: p}
	if validateEvent(event) != nil {
		t.Fatal("valid progress rejected")
	}
	for _, mutate := range []func(){func() { p.Phase = "raw arbitrary status" }, func() { p.StartedAt = 300 }, func() { p.OutputTokens = -1 }, func() { p.ToolName = "secret\ntext" }} {
		*p = protocol.TurnProgress{Phase: "thinking", StartedAt: 100, UpdatedAt: 200}
		mutate()
		if validateEvent(event) == nil {
			t.Fatal("unsafe progress accepted")
		}
	}
	if (protocol.SessionDiagnostic{Code: "arbitrary raw log"}).Valid() || (protocol.SessionDiagnostic{Code: "api_retry", HTTPStatus: 999}).Valid() {
		t.Fatal("unsafe diagnostics accepted")
	}
}
