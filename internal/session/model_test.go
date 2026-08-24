package session

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestNormalizeInput(t *testing.T) {
	tests := []struct {
		name      string
		currentID string
		input     Input
		wantText  string
		wantErr   error
	}{
		{name: "first terminal input", input: Input{Source: InputTerminal, Text: "  hello  "}, wantText: "hello"},
		{name: "terminal after init may omit target", currentID: "session-1", input: Input{Source: InputTerminal, Text: "next"}, wantText: "next"},
		{name: "mobile exact target", currentID: "session-1", input: Input{SessionID: "session-1", Source: InputMobile, Text: "remote"}, wantText: "remote"},
		{name: "blank", input: Input{Source: InputTerminal, Text: "  "}, wantErr: ErrInvalidInput},
		{name: "too large", input: Input{Source: InputTerminal, Text: strings.Repeat("x", MaxPromptBytes+1)}, wantErr: ErrInvalidInput},
		{name: "unknown source", input: Input{Source: "other", Text: "x"}, wantErr: ErrInvalidInput},
		{name: "mobile before init", input: Input{Source: InputMobile, Text: "x"}, wantErr: ErrSessionUnavailable},
		{name: "mobile missing target", currentID: "session-1", input: Input{Source: InputMobile, Text: "x"}, wantErr: ErrSessionMismatch},
		{name: "wrong target", currentID: "session-1", input: Input{SessionID: "session-2", Source: InputMobile, Text: "x"}, wantErr: ErrSessionMismatch},
		{name: "exact byte limit", input: Input{Source: InputTerminal, Text: strings.Repeat("x", MaxPromptBytes)}, wantText: strings.Repeat("x", MaxPromptBytes)},
		{name: "outer whitespace does not count toward byte limit", input: Input{Source: InputTerminal, Text: " \t" + strings.Repeat("x", MaxPromptBytes) + "\n "}, wantText: strings.Repeat("x", MaxPromptBytes)},
		{name: "multibyte over byte limit", input: Input{Source: InputTerminal, Text: strings.Repeat("界", MaxPromptBytes/len("界")) + "界"}, wantErr: ErrInvalidInput},
		{name: "terminal target before init", input: Input{SessionID: "session-1", Source: InputTerminal, Text: "x"}, wantErr: ErrSessionMismatch},
		{name: "terminal exact target after init", currentID: "session-1", input: Input{SessionID: "session-1", Source: InputTerminal, Text: "x"}, wantText: "x"},
		{name: "terminal wrong target after init", currentID: "session-1", input: Input{SessionID: "session-2", Source: InputTerminal, Text: "x"}, wantErr: ErrSessionMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeInput(tt.currentID, tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("normalizeInput() error = %v, want %v", err, tt.wantErr)
			}
			if err == nil && got.Text != tt.wantText {
				t.Fatalf("normalized text = %q, want %q", got.Text, tt.wantText)
			}
		})
	}
}

func TestRandomRequestID(t *testing.T) {
	id, err := randomRequestID()
	if err != nil {
		t.Fatalf("randomRequestID() error = %v", err)
	}
	if len(id) != 32 {
		t.Fatalf("request ID length = %d, want 32", len(id))
	}
	for _, r := range id {
		if !strings.ContainsRune("0123456789abcdef", r) {
			t.Fatalf("request ID is not lowercase hex: %q", id)
		}
	}
}

func TestRequestIDFromReader(t *testing.T) {
	raw := []byte{0x00, 0x01, 0x02, 0x0a, 0x10, 0x1f, 0x20, 0x7f, 0x80, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	id, err := requestIDFrom(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("requestIDFrom() error = %v", err)
	}
	if want := "0001020a101f207f8099aabbccddeeff"; id != want {
		t.Fatalf("requestIDFrom() = %q, want %q", id, want)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestRequestIDFromReaderFailure(t *testing.T) {
	id, err := requestIDFrom(failingReader{})
	if id != "" {
		t.Fatalf("requestIDFrom() ID = %q, want empty", id)
	}
	if !errors.Is(err, ErrInternal) {
		t.Fatalf("requestIDFrom() error = %v, want ErrInternal", err)
	}
}

func TestPublicEnumValues(t *testing.T) {
	if SessionStarting != "starting" || SessionReady != "ready" || SessionStopped != "stopped" || SessionFailed != "failed" {
		t.Fatalf("unexpected session status values")
	}
	if TurnIdle != "idle" || TurnRunning != "running" || TurnWaiting != "waiting" || TurnCompleted != "completed" || TurnCancelled != "cancelled" || TurnFailed != "failed" {
		t.Fatalf("unexpected turn status values")
	}
	if InputTerminal != "terminal" || InputMobile != "mobile" {
		t.Fatalf("unexpected input source values")
	}
	if EventSessionStarted != "session_started" || EventTurnStarted != "turn_started" || EventTextDelta != "text_delta" || EventTurnCompleted != "turn_completed" || EventTurnFailed != "turn_failed" || EventSessionFailed != "session_failed" || EventSessionStopped != "session_stopped" {
		t.Fatalf("unexpected event kind values")
	}
}

func TestSnapshotJSONShape(t *testing.T) {
	want := Snapshot{
		SessionID:     "session-1",
		SessionStatus: SessionReady,
		SessionError:  &ErrorInfo{Code: "oops", Message: "failed"},
		Seq:           7,
		Turn:          TurnSnapshot{Status: TurnRunning, RequestID: "request-1", Source: InputMobile, Message: "working", Error: &ErrorInfo{Code: "turn", Message: "waiting"}, PendingAction: &PendingAction{}},
		QueueDepth:    2,
	}
	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	jsonText := string(encoded)
	for _, field := range []string{`"sessionId":"session-1"`, `"sessionStatus":"ready"`, `"requestId":"request-1"`, `"source":"mobile"`, `"turn":{"status":"running"`, `"sessionError":{"code":"oops","message":"failed"}`, `"error":{"code":"turn","message":"waiting"}`, `"pendingAction":{}`} {
		if !strings.Contains(jsonText, field) {
			t.Fatalf("JSON shape missing %s: %s", field, jsonText)
		}
	}
	var got Snapshot
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got.SessionID != want.SessionID || got.SessionStatus != want.SessionStatus || got.Turn.Status != want.Turn.Status || got.Turn.Source != want.Turn.Source || got.Turn.Error.Code != want.Turn.Error.Code || got.Turn.PendingAction == nil {
		t.Fatalf("JSON round trip changed snapshot: got=%#v want=%#v", got, want)
	}

	var zero Snapshot
	zeroJSON, err := json.Marshal(zero)
	if err != nil {
		t.Fatalf("json.Marshal(zero) error = %v", err)
	}
	zeroText := string(zeroJSON)
	for _, field := range []string{`"sessionError":null`, `"error":null`, `"pendingAction":null`} {
		if !strings.Contains(zeroText, field) {
			t.Fatalf("zero JSON missing explicit null %s: %s", field, zeroText)
		}
	}
}
