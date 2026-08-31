package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestSnapshotJSONShape locks the wire field names and omission behavior that B2
// clients already depend on. Moving these types into protocol must not change a
// single field name, enum value or omitempty rule.
func TestSnapshotJSONShape(t *testing.T) {
	snapshot := Snapshot{
		SessionID:     "session-1",
		SessionStatus: SessionReady,
		Seq:           7,
		Turn: TurnSnapshot{
			Status:    TurnRunning,
			RequestID: "req-1",
			Source:    InputMobile,
			Message:   "in progress",
			PendingAction: &PendingAction{
				ActionID:  "act-1",
				RequestID: "req-1",
				Kind:      ActionToolApproval,
				ToolName:  "Bash",
				Preview:   "ls -la",
			},
		},
		QueueDepth:     1,
		QueuedRequests: []QueuedRequest{{RequestID: "req-2", Source: InputTerminal, Preview: "pwd"}},
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("round-trip unmarshal error = %v", err)
	}

	for _, field := range []string{"sessionId", "sessionStatus", "seq", "turn", "queueDepth", "queuedRequests"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("snapshot JSON missing field %q", field)
		}
	}
	if raw["sessionId"] != "session-1" || raw["seq"] != float64(7) {
		t.Errorf("snapshot scalar mismatch: %v", raw)
	}
	turn, ok := raw["turn"].(map[string]any)
	if !ok {
		t.Fatalf("turn not an object: %T", raw["turn"])
	}
	pending, ok := turn["pendingAction"].(map[string]any)
	if !ok {
		t.Fatalf("pendingAction not an object: %T", turn["pendingAction"])
	}
	if pending["actionId"] != "act-1" || pending["kind"] != "tool_approval" {
		t.Errorf("pendingAction mismatch: %v", pending)
	}
}

// TestEventJSONShape locks the structured content_block fields and the isError
// pointer semantics that tell "explicit false" from "absent".
func TestEventJSONShape(t *testing.T) {
	isError := false
	event := Event{
		Seq:       9,
		Kind:      EventContentBlock,
		SessionID: "session-1",
		RequestID: "req-1",
		Block: &ContentBlock{
			ID:        "blk-1",
			Kind:      BlockToolResult,
			ToolUseID: "tool-1",
			IsError:   &isError,
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("round-trip unmarshal error = %v", err)
	}
	block, ok := raw["block"].(map[string]any)
	if !ok {
		t.Fatalf("block not an object: %T", raw["block"])
	}
	if block["toolUseId"] != "tool-1" || block["isError"] != false {
		t.Errorf("block mismatch: %v", block)
	}
	if _, ok := block["thinking"]; ok {
		t.Errorf("empty thinking must be omitted, got %v", block)
	}
}

// TestInputJSONShape locks the source enum and sessionId field naming.
func TestInputJSONShape(t *testing.T) {
	input := Input{SessionID: "session-1", Source: InputMobile, Text: "hello"}
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if !strings.Contains(string(data), `"sessionId":"session-1"`) {
		t.Errorf("input JSON missing sessionId: %s", data)
	}
	if !strings.Contains(string(data), `"source":"mobile"`) {
		t.Errorf("input JSON missing mobile source: %s", data)
	}
}

// TestPairingJSONShape locks the pairing payload contract shared with the mobile
// client and the QR fragment.
func TestPairingJSONShape(t *testing.T) {
	payload := Pairing{
		Version:   PayloadVersion,
		WSURL:     "ws://127.0.0.1:8765/ws",
		SessionID: "session-1",
		PairToken: "token",
	}
	data, err := payload.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("round-trip unmarshal error = %v", err)
	}
	for _, field := range []string{"version", "wsUrl", "sessionId", "pairToken", "expiresAt"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("pairing JSON missing field %q", field)
		}
	}
}

// TestErrorSentinelsAreDistinct guards the command-result mapping: each sentinel
// must be a distinct comparable error so Transport can route it deterministically.
func TestErrorSentinelsAreDistinct(t *testing.T) {
	sentinels := []error{
		ErrInvalidInput, ErrQueueFull, ErrSessionMismatch, ErrSessionUnavailable,
		ErrInternal, ErrActionNotPending, ErrInvalidAction, ErrInvalidAnswer,
		ErrTurnNotCancellable, ErrRequestNotQueued,
	}
	seen := make(map[error]struct{}, len(sentinels))
	for _, sentinel := range sentinels {
		if sentinel == nil {
			t.Fatalf("nil sentinel in list")
		}
		if _, ok := seen[sentinel]; ok {
			t.Errorf("duplicate sentinel %v", sentinel)
		}
		seen[sentinel] = struct{}{}
	}
}
