package session

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"unicode/utf8"
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
	if EventContentBlock != "content_block" || EventActionRequired != "action_required" || EventActionResolved != "action_resolved" || EventUnsupported != "unsupported" {
		t.Fatalf("unexpected B2 event kind values")
	}
	if BlockThinking != "thinking" || BlockToolUse != "tool_use" || BlockToolResult != "tool_result" {
		t.Fatalf("unexpected block kind values")
	}
	if ToolBuiltin != "builtin" || ToolMCP != "mcp" {
		t.Fatalf("unexpected tool class values")
	}
	if ActionToolApproval != "tool_approval" || ActionAskUserQuestion != "ask_user_question" {
		t.Fatalf("unexpected action kind values")
	}
	if QuestionChoice != "choice" || QuestionText != "text" {
		t.Fatalf("unexpected question input type values")
	}
	if UnsupportedUnknownBlock != "unknown_block" || UnsupportedUnknownSubtype != "unknown_subtype" ||
		UnsupportedConcurrentAction != "concurrent_action" || UnsupportedTextDisabled != "text_input_disabled" {
		t.Fatalf("unexpected unsupported code values")
	}
}

// TestDefaultServiceConfig pins the two B2 service defaults so the preview
// limit and the conditional free-text capability never drift silently.
func TestDefaultServiceConfig(t *testing.T) {
	config := DefaultServiceConfig()
	if config.ToolPreviewMaxRunes != 200 {
		t.Fatalf("ToolPreviewMaxRunes = %d, want 200", config.ToolPreviewMaxRunes)
	}
	if config.AskUserTextEnabled {
		t.Fatal("AskUserTextEnabled must default to false until the CLI probe passes")
	}
}

func TestSnapshotJSONShape(t *testing.T) {
	want := Snapshot{
		SessionID:     "session-1",
		SessionStatus: SessionReady,
		SessionError:  &ErrorInfo{Code: "oops", Message: "failed"},
		Seq:           7,
		Turn: TurnSnapshot{Status: TurnRunning, RequestID: "request-1", Source: InputMobile, Message: "working", Error: &ErrorInfo{Code: "turn", Message: "waiting"}, PendingAction: &PendingAction{
			ActionID: "action-1", RequestID: "request-1", Kind: ActionToolApproval, ToolName: "Bash", Preview: "go test ./...", AllowSessionAvailable: true,
		}},
		QueueDepth:     2,
		QueuedRequests: []QueuedRequest{{RequestID: "request-2", Source: InputTerminal, Preview: "refactor login"}},
	}
	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	jsonText := string(encoded)
	for _, field := range []string{`"sessionId":"session-1"`, `"sessionStatus":"ready"`, `"requestId":"request-1"`, `"source":"mobile"`, `"turn":{"status":"running"`, `"sessionError":{"code":"oops","message":"failed"}`, `"error":{"code":"turn","message":"waiting"}`, `"pendingAction":{"actionId":"action-1"`, `"queuedRequests":[{"requestId":"request-2","source":"terminal","preview":"refactor login"}]`} {
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
	for _, field := range []string{`"sessionError":null`, `"error":null`, `"pendingAction":null`, `"queuedRequests":null`} {
		if !strings.Contains(zeroText, field) {
			t.Fatalf("zero JSON missing explicit null %s: %s", field, zeroText)
		}
	}
}

// TestPendingActionJSONShape asserts the B2 §4.3 wire contract: the client only
// ever sees the project-generated actionId, and the raw tool input, toolClass
// and toolUseId never leave the service.
func TestPendingActionJSONShape(t *testing.T) {
	question := PendingAction{
		ActionID:  "action-1",
		RequestID: "request-1",
		Kind:      ActionAskUserQuestion,
		ToolName:  AskUserQuestionToolName,
		Questions: []Question{{
			QuestionID:  "q1",
			Header:      "运行模式",
			Question:    "选择运行模式",
			InputType:   QuestionChoice,
			MultiSelect: true,
			Options:     []QuestionOption{{Label: "安全", Description: "只读检查"}, {Label: "完整"}},
		}},
	}
	encoded, err := json.Marshal(question)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	jsonText := string(encoded)
	for _, field := range []string{
		`"actionId":"action-1"`, `"requestId":"request-1"`, `"kind":"ask_user_question"`,
		`"toolName":"AskUserQuestion"`, `"allowSessionAvailable":false`, `"questionId":"q1"`,
		`"header":"运行模式"`, `"question":"选择运行模式"`, `"inputType":"choice"`, `"multiSelect":true`,
		`"label":"安全"`, `"description":"只读检查"`,
	} {
		if !strings.Contains(jsonText, field) {
			t.Fatalf("pending action JSON missing %s: %s", field, jsonText)
		}
	}
	for _, forbidden := range []string{`"input"`, `"toolClass"`, `"toolUseId"`, `"preview"`} {
		if strings.Contains(jsonText, forbidden) {
			t.Fatalf("pending action JSON must not carry %s: %s", forbidden, jsonText)
		}
	}

	approval := PendingAction{
		ActionID:              "action-2",
		RequestID:             "request-1",
		Kind:                  ActionToolApproval,
		ToolName:              "Bash",
		Preview:               "go test ./...",
		AllowSessionAvailable: true,
	}
	approvalEncoded, err := json.Marshal(approval)
	if err != nil {
		t.Fatalf("json.Marshal(approval) error = %v", err)
	}
	approvalText := string(approvalEncoded)
	for _, field := range []string{`"kind":"tool_approval"`, `"toolName":"Bash"`, `"preview":"go test ./..."`, `"allowSessionAvailable":true`} {
		if !strings.Contains(approvalText, field) {
			t.Fatalf("approval JSON missing %s: %s", field, approvalText)
		}
	}
	if strings.Contains(approvalText, `"questions"`) {
		t.Fatalf("tool_approval must omit questions: %s", approvalText)
	}
}

// TestContentBlockJSONShape covers the three §4.2 kinds and asserts isError can
// express an explicit false without leaking into the other kinds.
func TestContentBlockJSONShape(t *testing.T) {
	tests := []struct {
		name    string
		block   ContentBlock
		want    []string
		notWant []string
	}{
		{
			name:    "thinking",
			block:   ContentBlock{ID: "block-1", Kind: BlockThinking, Thinking: "weighing options"},
			want:    []string{`"id":"block-1"`, `"kind":"thinking"`, `"thinking":"weighing options"`},
			notWant: []string{`"toolUseId"`, `"toolClass"`, `"toolName"`, `"isError"`, `"mcpServer"`},
		},
		{
			name:    "builtin tool use",
			block:   ContentBlock{ID: "block-2", Kind: BlockToolUse, ToolUseID: "tool-1", ToolClass: ToolBuiltin, ToolName: "Read"},
			want:    []string{`"kind":"tool_use"`, `"toolUseId":"tool-1"`, `"toolClass":"builtin"`, `"toolName":"Read"`},
			notWant: []string{`"mcpServer"`, `"thinking"`, `"isError"`, `"input"`},
		},
		{
			name:    "mcp tool use",
			block:   ContentBlock{ID: "block-3", Kind: BlockToolUse, ToolUseID: "tool-2", ToolClass: ToolMCP, MCPServer: "filesystem", ToolName: "read_file"},
			want:    []string{`"toolClass":"mcp"`, `"mcpServer":"filesystem"`},
			notWant: []string{`"thinking"`, `"isError"`},
		},
		{
			name:    "successful tool result",
			block:   ContentBlock{ID: "block-4", Kind: BlockToolResult, ToolUseID: "tool-1", IsError: BoolPointer(false)},
			want:    []string{`"kind":"tool_result"`, `"toolUseId":"tool-1"`, `"isError":false`},
			notWant: []string{`"toolName"`, `"toolClass"`, `"thinking"`},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := json.Marshal(test.block)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			jsonText := string(encoded)
			for _, field := range test.want {
				if !strings.Contains(jsonText, field) {
					t.Fatalf("block JSON missing %s: %s", field, jsonText)
				}
			}
			for _, forbidden := range test.notWant {
				if strings.Contains(jsonText, forbidden) {
					t.Fatalf("block JSON must not carry %s: %s", forbidden, jsonText)
				}
			}
		})
	}
}

// TestSetQueuedRequestsKeepsDepthConsistent locks the §4.4 invariant that both
// views of the FIFO are written from a single place.
func TestSetQueuedRequestsKeepsDepthConsistent(t *testing.T) {
	var snapshot Snapshot
	setQueuedRequests(&snapshot, []QueuedRequest{
		{RequestID: "request-2", Source: InputTerminal, Preview: "one"},
		{RequestID: "request-3", Source: InputMobile, Preview: "two"},
	})
	if snapshot.QueueDepth != len(snapshot.QueuedRequests) || snapshot.QueueDepth != 2 {
		t.Fatalf("queueDepth = %d, queuedRequests = %d, want both 2", snapshot.QueueDepth, len(snapshot.QueuedRequests))
	}
	setQueuedRequests(&snapshot, nil)
	if snapshot.QueueDepth != 0 || len(snapshot.QueuedRequests) != 0 {
		t.Fatalf("cleared queue = depth %d, list %d, want both 0", snapshot.QueueDepth, len(snapshot.QueuedRequests))
	}
}

// TestNewQueuedRequestTruncatesPreview asserts a queued entry never carries the
// full submitted text, on a rune boundary.
func TestNewQueuedRequestTruncatesPreview(t *testing.T) {
	config := DefaultServiceConfig()
	long := strings.Repeat("界", config.ToolPreviewMaxRunes+50)
	queued := newQueuedRequest(config, "request-9", Input{Source: InputMobile, Text: long})
	if queued.RequestID != "request-9" || queued.Source != InputMobile {
		t.Fatalf("unexpected queued request: %#v", queued)
	}
	runes := []rune(queued.Preview)
	if len(runes) != config.ToolPreviewMaxRunes+1 {
		t.Fatalf("preview rune count = %d, want %d including the ellipsis", len(runes), config.ToolPreviewMaxRunes+1)
	}
	if runes[len(runes)-1] != '…' {
		t.Fatalf("truncated preview must end with an ellipsis: %q", queued.Preview)
	}
	if !utf8.ValidString(queued.Preview) {
		t.Fatalf("truncated preview is not valid UTF-8: %q", queued.Preview)
	}
	if strings.Contains(long, queued.Preview) && len([]rune(long)) == len(runes) {
		t.Fatal("preview must be shorter than the submitted text")
	}
}

// TestTerminalEventSemanticsUnchanged guards the A3/A4 contract that B2 reuses
// the existing Session and Turn terminal events instead of inventing new ones.
func TestTerminalEventSemanticsUnchanged(t *testing.T) {
	for _, kind := range []EventKind{EventSessionFailed, EventSessionStopped, EventTurnCompleted, EventTurnFailed} {
		event := Event{Kind: kind, SessionID: "session-1"}
		if event.Block != nil || event.PendingAction != nil || event.Code != "" {
			t.Fatalf("%s must not carry B2 structured payloads: %#v", kind, event)
		}
	}
}
