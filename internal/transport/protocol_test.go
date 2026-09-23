package transport

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tea4go/claude-mobile-remote/protocol"

	projectsession "github.com/tea4go/claude-mobile-remote/internal/session"
)

// TestProtocolVersionIsTwo pins the B2 §4.1 bump so A4/B1 clients are refused
// instead of silently mis-parsing structured payloads.
func TestProtocolVersionIsTwo(t *testing.T) {
	if Version != 2 {
		t.Fatalf("Version = %d, want 2", Version)
	}
}

func TestDecodeEnvelope(t *testing.T) {
	input := []byte(`{"version":2,"type":"command","sessionId":"session-1","commandId":"command-1","requestId":"","seq":0,"payload":{"kind":"send_text","text":"hello"}}`)
	envelope, err := DecodeEnvelope(input)
	if err != nil {
		t.Fatalf("DecodeEnvelope() error = %v", err)
	}
	if envelope.Version != Version || envelope.Type != TypeCommand || envelope.SessionID != "session-1" || envelope.CommandID != "command-1" {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
	var command CommandPayload
	if err := DecodePayload(envelope, &command); err != nil {
		t.Fatalf("DecodePayload() error = %v", err)
	}
	if command.Kind != CommandSendText || command.Text != "hello" {
		t.Fatalf("unexpected command: %#v", command)
	}
}

func TestValidateEnvelope(t *testing.T) {
	tests := []struct {
		name     string
		envelope Envelope
		wantCode string
	}{
		{
			name:     "unsupported version",
			envelope: Envelope{Version: Version + 1, Type: TypeHello, SessionID: "s", Payload: json.RawMessage(`{"pairToken":"fixture"}`)},
			wantCode: CodeUnsupportedVersion,
		},
		{
			name:     "previous protocol version",
			envelope: Envelope{Version: 1, Type: TypeHello, SessionID: "s", Payload: json.RawMessage(`{"pairToken":"fixture"}`)},
			wantCode: CodeUnsupportedVersion,
		},
		{
			name:     "missing session",
			envelope: Envelope{Version: Version, Type: TypeHello, Payload: json.RawMessage(`{"pairToken":"fixture"}`)},
			wantCode: CodeInvalidMessage,
		},
		{
			name:     "command missing id",
			envelope: Envelope{Version: Version, Type: TypeCommand, SessionID: "s", Payload: json.RawMessage(`{"kind":"send_text","text":"x"}`)},
			wantCode: CodeInvalidMessage,
		},
		{
			name:     "command carries request id",
			envelope: Envelope{Version: Version, Type: TypeCommand, SessionID: "s", CommandID: "c", RequestID: "r", Payload: json.RawMessage(`{"kind":"send_text","text":"x"}`)},
			wantCode: CodeInvalidMessage,
		},
		{
			name:     "unknown message type",
			envelope: Envelope{Version: Version, Type: "other", SessionID: "s", Payload: json.RawMessage(`{}`)},
			wantCode: CodeInvalidMessage,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateEnvelope(test.envelope)
			var protocolErr *ProtocolError
			if !errors.As(err, &protocolErr) || protocolErr.Code != test.wantCode {
				t.Fatalf("ValidateEnvelope() error = %v, want code %q", err, test.wantCode)
			}
		})
	}
}

func TestValidateEnvelopeAcceptsCommand(t *testing.T) {
	envelope := Envelope{
		Version:   Version,
		Type:      TypeCommand,
		SessionID: "session-1",
		CommandID: "command-1",
		Payload:   json.RawMessage(`{"kind":"send_text","text":"hello"}`),
	}
	if err := ValidateEnvelope(envelope); err != nil {
		t.Fatalf("ValidateEnvelope() error = %v", err)
	}
}

func TestManagementEnvelopeContract(t *testing.T) {
	created := protocol.Pairing{
		Version: protocol.PayloadVersion, WSURL: "ws://127.0.0.1:8765/ws",
		SessionID: "new-session", PairToken: "fixture-token",
		ExpiresAt:    time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC),
		Capabilities: []string{"create_session", "stop_session"},
	}
	valid := []struct {
		typeName MessageType
		payload  any
	}{
		{TypeCreateSession, struct{}{}},
		{TypeStopSession, struct{}{}},
		{TypeCreateSessionResult, ManagementResultPayload{Accepted: true, Pairing: &created}},
		{TypeStopSessionResult, ManagementResultPayload{Accepted: true}},
		{TypeCreateSessionResult, ManagementResultPayload{Code: CodeSessionUnavailable, Message: "unavailable"}},
	}
	for _, test := range valid {
		envelope, err := NewEnvelope(test.typeName, "source-session", "cmd-1", "", 0, test.payload)
		if err != nil {
			t.Fatal(err)
		}
		if err := ValidateEnvelope(envelope); err != nil {
			t.Fatalf("type %s: %v", test.typeName, err)
		}
	}

	invalid := []struct {
		typeName MessageType
		payload  any
	}{
		{TypeCreateSession, map[string]any{"cwd": "/tmp"}},
		{TypeStopSession, map[string]any{"sessionId": "other"}},
		{TypeCreateSessionResult, ManagementResultPayload{Accepted: true}},
		{TypeCreateSessionResult, ManagementResultPayload{Accepted: true, Pairing: &protocol.Pairing{SessionID: "source-session"}}},
		{TypeStopSessionResult, ManagementResultPayload{Accepted: true, Pairing: &created}},
		{TypeStopSessionResult, ManagementResultPayload{Code: CodeSessionUnavailable, Pairing: &created}},
	}
	for _, test := range invalid {
		envelope, err := NewEnvelope(test.typeName, "source-session", "cmd-1", "", 0, test.payload)
		if err != nil {
			t.Fatal(err)
		}
		if err := ValidateEnvelope(envelope); err == nil {
			t.Fatalf("type %s unexpectedly valid", test.typeName)
		}
	}
}

func TestNewEnvelopeRoundTrip(t *testing.T) {
	envelope, err := NewEnvelope(TypeCommand, "session-1", "command-1", "", 0, CommandPayload{
		Kind: CommandSendText,
		Text: "hello",
	})
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}
	data, err := EncodeEnvelope(envelope)
	if err != nil {
		t.Fatalf("EncodeEnvelope() error = %v", err)
	}
	decoded, err := DecodeEnvelope(data)
	if err != nil {
		t.Fatalf("DecodeEnvelope() error = %v", err)
	}
	if err := ValidateEnvelope(decoded); err != nil {
		t.Fatalf("ValidateEnvelope() error = %v", err)
	}
}

func TestValidateEnvelopeAcceptsServiceMessages(t *testing.T) {
	snapshot := projectsession.Snapshot{SessionID: "session-1", SessionStatus: projectsession.SessionReady, Seq: 7}
	event := projectsession.Event{
		SessionID: "session-1", RequestID: "request-1", Seq: 8,
		Kind: projectsession.EventTurnStarted, Source: projectsession.InputTerminal,
		Text: "check changes", TurnStatus: projectsession.TurnRunning,
	}
	tests := []struct {
		name      string
		typeName  MessageType
		commandID string
		requestID string
		seq       uint64
		payload   any
	}{
		{name: "hello ack", typeName: TypeHelloAck, payload: struct{}{}},
		{name: "snapshot", typeName: TypeSnapshot, seq: 7, payload: SnapshotPayload{ConnectionStatus: "subscribed", Snapshot: snapshot}},
		{name: "event", typeName: TypeEvent, requestID: "request-1", seq: 8, payload: EventPayload{Event: event}},
		{name: "accepted command", typeName: TypeCommandResult, commandID: "command-1", requestID: "request-1", payload: CommandResultPayload{Accepted: true}},
		{name: "rejected command", typeName: TypeCommandResult, commandID: "command-1", payload: CommandResultPayload{Code: CodeQueueFull, Message: "input queue full"}},
		{name: "error", typeName: TypeError, payload: ErrorPayload{Code: CodeInvalidState, Message: "invalid connection state"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			envelope, err := NewEnvelope(test.typeName, "session-1", test.commandID, test.requestID, test.seq, test.payload)
			if err != nil {
				t.Fatalf("NewEnvelope() error = %v", err)
			}
			if err := ValidateEnvelope(envelope); err != nil {
				t.Fatalf("ValidateEnvelope() error = %v", err)
			}
		})
	}
}

func TestValidateEnvelopeRejectsInvalidEventSemantics(t *testing.T) {
	baseEvent := func() projectsession.Event {
		return projectsession.Event{
			SessionID: "session-1", RequestID: "request-1", Seq: 8,
			Kind: projectsession.EventTurnStarted, Source: projectsession.InputTerminal,
			Text: "check changes", TurnStatus: projectsession.TurnRunning,
		}
	}
	tests := []struct {
		name  string
		event projectsession.Event
	}{
		{name: "turn started without request id", event: func() projectsession.Event {
			event := baseEvent()
			event.RequestID = ""
			return event
		}()},
		{name: "turn started without source", event: func() projectsession.Event {
			event := baseEvent()
			event.Source = ""
			return event
		}()},
		{name: "turn started with unknown source", event: func() projectsession.Event {
			event := baseEvent()
			event.Source = "watch"
			return event
		}()},
		{name: "turn started without text", event: func() projectsession.Event {
			event := baseEvent()
			event.Text = ""
			return event
		}()},
		{name: "turn started with oversized text", event: func() projectsession.Event {
			event := baseEvent()
			event.Text = string(make([]byte, projectsession.MaxPromptBytes+1))
			return event
		}()},
		{name: "turn started with wrong turn status", event: func() projectsession.Event {
			event := baseEvent()
			event.TurnStatus = projectsession.TurnCompleted
			return event
		}()},
		{name: "text delta without text", event: projectsession.Event{
			SessionID: "session-1", RequestID: "request-1", Seq: 8,
			Kind: projectsession.EventTextDelta, Source: projectsession.InputTerminal,
		}},
		{name: "turn completed with text", event: projectsession.Event{
			SessionID: "session-1", RequestID: "request-1", Seq: 8,
			Kind: projectsession.EventTurnCompleted,
			Text: "unexpected",
		}},
		{name: "session started with text", event: projectsession.Event{
			SessionID: "session-1", Seq: 8,
			Kind: projectsession.EventSessionStarted,
			Text: "unexpected",
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			envelope, err := NewEnvelope(TypeEvent, "session-1", "", test.event.RequestID, test.event.Seq, EventPayload{Event: test.event})
			if err != nil {
				t.Fatalf("NewEnvelope() error = %v", err)
			}
			err = ValidateEnvelope(envelope)
			var protocolErr *ProtocolError
			if !errors.As(err, &protocolErr) || protocolErr.Code != CodeInvalidMessage {
				t.Fatalf("ValidateEnvelope() error = %v, want code %q", err, CodeInvalidMessage)
			}
			if protocolErr != nil && test.event.Text != "" && protocolErr.Message == test.event.Text {
				t.Fatal("error message must not echo event text")
			}
		})
	}
}

func TestValidateEnvelopeRejectsMismatchedServiceSequence(t *testing.T) {
	envelope, err := NewEnvelope(TypeSnapshot, "session-1", "", "", 8, SnapshotPayload{
		ConnectionStatus: "subscribed",
		Snapshot:         projectsession.Snapshot{SessionID: "session-1", Seq: 7},
	})
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}
	if err := ValidateEnvelope(envelope); err == nil {
		t.Fatal("ValidateEnvelope() accepted mismatched snapshot seq")
	}
}

func TestDecodeEnvelopeRejectsUnknownAndTrailingData(t *testing.T) {
	inputs := [][]byte{
		[]byte(`{"version":2,"type":"hello","sessionId":"s","commandId":"","requestId":"","seq":0,"payload":{},"extra":true}`),
		[]byte(`{"version":2,"type":"hello","sessionId":"s","commandId":"","requestId":"","seq":0,"payload":{}} {}`),
	}
	for _, input := range inputs {
		if _, err := DecodeEnvelope(input); !errors.Is(err, ErrInvalidMessage) {
			t.Fatalf("DecodeEnvelope(%q) error = %v, want ErrInvalidMessage", input, err)
		}
	}
}

// TestUnsupportedVersionErrorPayloadCarriesExpectedVersion covers §4.1: the
// refusal tells the client which version to upgrade to and leaks no Session data.
func TestUnsupportedVersionErrorPayloadCarriesExpectedVersion(t *testing.T) {
	payload := newErrorPayload(CodeUnsupportedVersion, "unsupported protocol version")
	if payload.ExpectedVersion != Version {
		t.Fatalf("ExpectedVersion = %d, want %d", payload.ExpectedVersion, Version)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	for key := range fields {
		switch key {
		case "code", "message", "expectedVersion":
		default:
			t.Fatalf("version error payload carries unexpected field %q: %s", key, encoded)
		}
	}

	other := newErrorPayload(CodeInvalidState, "invalid connection state")
	if other.ExpectedVersion != 0 {
		t.Fatalf("ExpectedVersion = %d for %s, want 0", other.ExpectedVersion, CodeInvalidState)
	}
	otherEncoded, err := json.Marshal(other)
	if err != nil {
		t.Fatalf("json.Marshal(other) error = %v", err)
	}
	if strings.Contains(string(otherEncoded), "expectedVersion") {
		t.Fatalf("unrelated error must omit expectedVersion: %s", otherEncoded)
	}
}

// TestProtocolErrorCodeConstants asserts the §4.5 codes exist verbatim so the
// three clients can branch on stable strings.
func TestProtocolErrorCodeConstants(t *testing.T) {
	if CodeInvalidAction != "invalid_action" || CodeActionNotPending != "action_not_pending" ||
		CodeInvalidAnswer != "invalid_answer" || CodeTurnNotCancellable != "turn_not_cancellable" ||
		CodeRequestNotQueued != "request_not_queued" || CodeRequestAlreadyFinished != "request_already_finished" ||
		CodeActionTimeout != "action_timeout" {
		t.Fatal("unexpected B2 error code values")
	}
	if CommandResolveAction != "resolve_action" || CommandCancelTurn != "cancel_turn" || CommandCancelQueued != "cancel_queued" {
		t.Fatal("unexpected B2 command kind values")
	}
	if DecisionAllowOnce != "allow_once" || DecisionAllowSession != "allow_session" || DecisionDeny != "deny" {
		t.Fatal("unexpected tool approval decision values")
	}
}

func structuredEvent(kind projectsession.EventKind) projectsession.Event {
	return projectsession.Event{SessionID: "session-1", RequestID: "request-1", Seq: 8, Kind: kind}
}

func validateEventEnvelope(t *testing.T, event projectsession.Event) error {
	t.Helper()
	envelope, err := NewEnvelope(TypeEvent, "session-1", "", event.RequestID, event.Seq, EventPayload{Event: event})
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}
	return ValidateEnvelope(envelope)
}

func contentBlockEvent(block projectsession.ContentBlock) projectsession.Event {
	event := structuredEvent(projectsession.EventContentBlock)
	event.Block = &block
	return event
}

// TestValidateEnvelopeAcceptsContentBlocks covers the three §4.2 kinds, including
// Task being structurally identical to any other builtin tool.
func TestValidateEnvelopeAcceptsContentBlocks(t *testing.T) {
	blocks := map[string]projectsession.ContentBlock{
		"thinking":     {ID: "block-1", Kind: projectsession.BlockThinking, Thinking: "weighing options"},
		"builtin":      {ID: "block-2", Kind: projectsession.BlockToolUse, ToolUseID: "tool-1", ToolClass: projectsession.ToolBuiltin, ToolName: "Read"},
		"task as tool": {ID: "block-3", Kind: projectsession.BlockToolUse, ToolUseID: "tool-2", ToolClass: projectsession.ToolBuiltin, ToolName: "Task"},
		"mcp":          {ID: "block-4", Kind: projectsession.BlockToolUse, ToolUseID: "tool-3", ToolClass: projectsession.ToolMCP, MCPServer: "filesystem", ToolName: "read_file"},
		"result ok":    {ID: "block-5", Kind: projectsession.BlockToolResult, ToolUseID: "tool-1", IsError: projectsession.BoolPointer(false)},
		"result fail":  {ID: "block-6", Kind: projectsession.BlockToolResult, ToolUseID: "tool-1", IsError: projectsession.BoolPointer(true)},
	}
	for name, block := range blocks {
		t.Run(name, func(t *testing.T) {
			if err := validateEventEnvelope(t, contentBlockEvent(block)); err != nil {
				t.Fatalf("ValidateEnvelope() error = %v", err)
			}
		})
	}
}

// TestValidateEnvelopeRejectsContentBlockFieldsOutsideWhitelist enforces the
// per-kind field whitelist from §4.2, including that unsupported is never a
// fourth content_block kind.
func TestValidateEnvelopeRejectsContentBlockFieldsOutsideWhitelist(t *testing.T) {
	events := map[string]projectsession.Event{
		"thinking without text":      contentBlockEvent(projectsession.ContentBlock{ID: "b", Kind: projectsession.BlockThinking}),
		"thinking with tool name":    contentBlockEvent(projectsession.ContentBlock{ID: "b", Kind: projectsession.BlockThinking, Thinking: "x", ToolName: "Read"}),
		"thinking with tool use id":  contentBlockEvent(projectsession.ContentBlock{ID: "b", Kind: projectsession.BlockThinking, Thinking: "x", ToolUseID: "tool-1"}),
		"thinking with is error":     contentBlockEvent(projectsession.ContentBlock{ID: "b", Kind: projectsession.BlockThinking, Thinking: "x", IsError: projectsession.BoolPointer(false)}),
		"tool use without name":      contentBlockEvent(projectsession.ContentBlock{ID: "b", Kind: projectsession.BlockToolUse, ToolUseID: "tool-1", ToolClass: projectsession.ToolBuiltin}),
		"tool use without class":     contentBlockEvent(projectsession.ContentBlock{ID: "b", Kind: projectsession.BlockToolUse, ToolUseID: "tool-1", ToolName: "Read"}),
		"builtin with mcp server":    contentBlockEvent(projectsession.ContentBlock{ID: "b", Kind: projectsession.BlockToolUse, ToolUseID: "tool-1", ToolClass: projectsession.ToolBuiltin, ToolName: "Read", MCPServer: "filesystem"}),
		"mcp without server":         contentBlockEvent(projectsession.ContentBlock{ID: "b", Kind: projectsession.BlockToolUse, ToolUseID: "tool-1", ToolClass: projectsession.ToolMCP, ToolName: "read_file"}),
		"tool use with thinking":     contentBlockEvent(projectsession.ContentBlock{ID: "b", Kind: projectsession.BlockToolUse, ToolUseID: "tool-1", ToolClass: projectsession.ToolBuiltin, ToolName: "Read", Thinking: "x"}),
		"tool use with is error":     contentBlockEvent(projectsession.ContentBlock{ID: "b", Kind: projectsession.BlockToolUse, ToolUseID: "tool-1", ToolClass: projectsession.ToolBuiltin, ToolName: "Read", IsError: projectsession.BoolPointer(true)}),
		"result without tool use id": contentBlockEvent(projectsession.ContentBlock{ID: "b", Kind: projectsession.BlockToolResult, IsError: projectsession.BoolPointer(false)}),
		"result without is error":    contentBlockEvent(projectsession.ContentBlock{ID: "b", Kind: projectsession.BlockToolResult, ToolUseID: "tool-1"}),
		"result with tool name":      contentBlockEvent(projectsession.ContentBlock{ID: "b", Kind: projectsession.BlockToolResult, ToolUseID: "tool-1", IsError: projectsession.BoolPointer(false), ToolName: "Read"}),
		"block without id":           contentBlockEvent(projectsession.ContentBlock{Kind: projectsession.BlockThinking, Thinking: "x"}),
		"unsupported as block kind":  contentBlockEvent(projectsession.ContentBlock{ID: "b", Kind: "unsupported"}),
		"unknown block kind":         contentBlockEvent(projectsession.ContentBlock{ID: "b", Kind: "other"}),
	}
	events["content block without block"] = structuredEvent(projectsession.EventContentBlock)
	blockWithText := contentBlockEvent(projectsession.ContentBlock{ID: "b", Kind: projectsession.BlockThinking, Thinking: "x"})
	blockWithText.Text = "unexpected"
	events["content block with text"] = blockWithText
	textDeltaWithBlock := structuredEvent(projectsession.EventTextDelta)
	textDeltaWithBlock.Text = "hi"
	textDeltaWithBlock.Block = &projectsession.ContentBlock{ID: "b", Kind: projectsession.BlockThinking, Thinking: "x"}
	events["unrelated kind carrying a block"] = textDeltaWithBlock

	for name, event := range events {
		t.Run(name, func(t *testing.T) {
			err := validateEventEnvelope(t, event)
			var protocolErr *ProtocolError
			if !errors.As(err, &protocolErr) || protocolErr.Code != CodeInvalidMessage {
				t.Fatalf("ValidateEnvelope() error = %v, want code %q", err, CodeInvalidMessage)
			}
		})
	}
}

// TestValidateEnvelopeAcceptsPendingActionEvents covers the action_required and
// action_resolved pair that carries §4.3 pending actions into the event stream.
func TestValidateEnvelopeAcceptsPendingActionEvents(t *testing.T) {
	required := structuredEvent(projectsession.EventActionRequired)
	required.TurnStatus = projectsession.TurnWaiting
	required.PendingAction = &projectsession.PendingAction{
		ActionID: "action-1", RequestID: "request-1", Kind: projectsession.ActionToolApproval,
		ToolName: "Bash", Preview: "go test ./...", AllowSessionAvailable: true,
	}
	if err := validateEventEnvelope(t, required); err != nil {
		t.Fatalf("action_required error = %v", err)
	}

	resolved := structuredEvent(projectsession.EventActionResolved)
	resolved.TurnStatus = projectsession.TurnRunning
	if err := validateEventEnvelope(t, resolved); err != nil {
		t.Fatalf("action_resolved error = %v", err)
	}

	answered := structuredEvent(projectsession.EventActionResolved)
	answered.TurnStatus = projectsession.TurnRunning
	answered.Answers = []projectsession.ResolvedAnswer{
		{QuestionID: "q1", Header: "h", Question: "pick one", Values: []string{"yes"}},
		{QuestionID: "q2", Question: "pick many", Values: []string{"a", "b"}},
	}
	if err := validateEventEnvelope(t, answered); err != nil {
		t.Fatalf("answered action_resolved error = %v", err)
	}
}

// TestValidateEnvelopeRejectsInvalidResolvedAnswers pins the D52 echo: only
// action_resolved may carry answers, and each answer must name its question and
// carry real values.
func TestValidateEnvelopeRejectsInvalidResolvedAnswers(t *testing.T) {
	resolvedWith := func(answers ...projectsession.ResolvedAnswer) projectsession.Event {
		event := structuredEvent(projectsession.EventActionResolved)
		event.TurnStatus = projectsession.TurnRunning
		event.Answers = answers
		return event
	}
	answer := projectsession.ResolvedAnswer{QuestionID: "q1", Header: "h", Question: "pick one", Values: []string{"yes"}}

	blockWithAnswers := contentBlockEvent(projectsession.ContentBlock{ID: "b", Kind: projectsession.BlockThinking, Thinking: "x"})
	blockWithAnswers.Answers = []projectsession.ResolvedAnswer{answer}
	requiredWithAnswers := structuredEvent(projectsession.EventActionRequired)
	requiredWithAnswers.TurnStatus = projectsession.TurnWaiting
	requiredWithAnswers.PendingAction = &projectsession.PendingAction{
		ActionID: "action-1", RequestID: "request-1", Kind: projectsession.ActionToolApproval, ToolName: "Bash",
	}
	requiredWithAnswers.Answers = []projectsession.ResolvedAnswer{answer}
	completedWithAnswers := structuredEvent(projectsession.EventTurnCompleted)
	completedWithAnswers.Answers = []projectsession.ResolvedAnswer{answer}

	events := map[string]projectsession.Event{
		"answer without question id":   resolvedWith(projectsession.ResolvedAnswer{Question: "pick one", Values: []string{"yes"}}),
		"answer without question":      resolvedWith(projectsession.ResolvedAnswer{QuestionID: "q1", Values: []string{"yes"}}),
		"answer without values":        resolvedWith(projectsession.ResolvedAnswer{QuestionID: "q1", Question: "pick one"}),
		"blank answer value":           resolvedWith(projectsession.ResolvedAnswer{QuestionID: "q1", Question: "pick one", Values: []string{"  "}}),
		"duplicate question id":        resolvedWith(answer, answer),
		"content_block with answers":   blockWithAnswers,
		"action_required with answers": requiredWithAnswers,
		"unrelated kind with answers":  completedWithAnswers,
	}

	for name, event := range events {
		t.Run(name, func(t *testing.T) {
			err := validateEventEnvelope(t, event)
			var protocolErr *ProtocolError
			if !errors.As(err, &protocolErr) || protocolErr.Code != CodeInvalidMessage {
				t.Fatalf("ValidateEnvelope() error = %v, want code %q", err, CodeInvalidMessage)
			}
		})
	}
}

func TestValidateEnvelopeRejectsInvalidPendingActions(t *testing.T) {
	withAction := func(turnStatus projectsession.TurnStatus, action *projectsession.PendingAction) projectsession.Event {
		event := structuredEvent(projectsession.EventActionRequired)
		event.TurnStatus = turnStatus
		event.PendingAction = action
		return event
	}
	approval := func() *projectsession.PendingAction {
		return &projectsession.PendingAction{
			ActionID: "action-1", RequestID: "request-1", Kind: projectsession.ActionToolApproval, ToolName: "Bash",
		}
	}
	question := func() *projectsession.PendingAction {
		return &projectsession.PendingAction{
			ActionID: "action-1", RequestID: "request-1", Kind: projectsession.ActionAskUserQuestion,
			ToolName: projectsession.AskUserQuestionToolName,
			Questions: []projectsession.Question{{
				QuestionID: "q1", Header: "h", Question: "q", InputType: projectsession.QuestionChoice,
				Options: []projectsession.QuestionOption{{Label: "yes"}},
			}},
		}
	}

	missingToolName := approval()
	missingToolName.ToolName = ""
	approvalWithQuestions := approval()
	approvalWithQuestions.Questions = question().Questions
	questionWithPreview := question()
	questionWithPreview.Preview = "leaked"
	questionWithoutQuestions := question()
	questionWithoutQuestions.Questions = nil
	choiceWithoutOptions := question()
	choiceWithoutOptions.Questions[0].Options = nil
	blankQuestionText := question()
	blankQuestionText.Questions[0].Header = ""
	blankQuestionText.Questions[0].Question = ""
	unknownInputType := question()
	unknownInputType.Questions[0].InputType = "voice"
	textWithOptions := question()
	textWithOptions.Questions[0].InputType = projectsession.QuestionText
	duplicateQuestionID := question()
	duplicateQuestionID.Questions = append(duplicateQuestionID.Questions, duplicateQuestionID.Questions[0])
	mismatchedRequest := approval()
	mismatchedRequest.RequestID = "request-2"
	missingActionID := approval()
	missingActionID.ActionID = ""
	unknownKind := approval()
	unknownKind.Kind = "other"

	events := map[string]projectsession.Event{
		"missing action":                withAction(projectsession.TurnWaiting, nil),
		"turn not waiting":              withAction(projectsession.TurnRunning, approval()),
		"approval without tool name":    withAction(projectsession.TurnWaiting, missingToolName),
		"approval carrying questions":   withAction(projectsession.TurnWaiting, approvalWithQuestions),
		"question carrying preview":     withAction(projectsession.TurnWaiting, questionWithPreview),
		"question without questions":    withAction(projectsession.TurnWaiting, questionWithoutQuestions),
		"choice without options":        withAction(projectsession.TurnWaiting, choiceWithoutOptions),
		"question without any text":     withAction(projectsession.TurnWaiting, blankQuestionText),
		"unknown input type":            withAction(projectsession.TurnWaiting, unknownInputType),
		"text question with options":    withAction(projectsession.TurnWaiting, textWithOptions),
		"duplicate question id":         withAction(projectsession.TurnWaiting, duplicateQuestionID),
		"action bound to other request": withAction(projectsession.TurnWaiting, mismatchedRequest),
		"action without id":             withAction(projectsession.TurnWaiting, missingActionID),
		"unknown action kind":           withAction(projectsession.TurnWaiting, unknownKind),
	}
	resolvedWithAction := structuredEvent(projectsession.EventActionResolved)
	resolvedWithAction.TurnStatus = projectsession.TurnRunning
	resolvedWithAction.PendingAction = approval()
	events["action_resolved still carrying an action"] = resolvedWithAction
	turnStartedWithAction := structuredEvent(projectsession.EventTurnStarted)
	turnStartedWithAction.Source = projectsession.InputTerminal
	turnStartedWithAction.Text = "hello"
	turnStartedWithAction.TurnStatus = projectsession.TurnRunning
	turnStartedWithAction.PendingAction = approval()
	events["unrelated kind carrying an action"] = turnStartedWithAction

	for name, event := range events {
		t.Run(name, func(t *testing.T) {
			err := validateEventEnvelope(t, event)
			var protocolErr *ProtocolError
			if !errors.As(err, &protocolErr) || protocolErr.Code != CodeInvalidMessage {
				t.Fatalf("ValidateEnvelope() error = %v, want code %q", err, CodeInvalidMessage)
			}
		})
	}
}

// TestValidateEnvelopeUnsupportedMessage covers §4.2: unsupported travels as its
// own envelope type carrying nothing but a fixed code.
func TestValidateEnvelopeUnsupportedMessage(t *testing.T) {
	codes := []projectsession.UnsupportedCode{
		projectsession.UnsupportedUnknownBlock,
		projectsession.UnsupportedUnknownSubtype,
		projectsession.UnsupportedConcurrentAction,
		projectsession.UnsupportedTextDisabled,
	}
	for _, code := range codes {
		t.Run(string(code), func(t *testing.T) {
			envelope, err := NewEnvelope(TypeUnsupported, "session-1", "", "request-1", 8, UnsupportedPayload{Code: code})
			if err != nil {
				t.Fatalf("NewEnvelope() error = %v", err)
			}
			if err := ValidateEnvelope(envelope); err != nil {
				t.Fatalf("ValidateEnvelope() error = %v", err)
			}
			if strings.Contains(string(envelope.Payload), "message") {
				t.Fatalf("unsupported payload must only carry a code: %s", envelope.Payload)
			}
		})
	}

	rejected := map[string]Envelope{}
	unknownCode, err := NewEnvelope(TypeUnsupported, "session-1", "", "", 0, UnsupportedPayload{Code: "whatever"})
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}
	rejected["unknown code"] = unknownCode
	withCommandID, err := NewEnvelope(TypeUnsupported, "session-1", "command-1", "", 0, UnsupportedPayload{Code: projectsession.UnsupportedUnknownBlock})
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}
	rejected["carrying a command id"] = withCommandID
	for name, envelope := range rejected {
		t.Run(name, func(t *testing.T) {
			err := ValidateEnvelope(envelope)
			var protocolErr *ProtocolError
			if !errors.As(err, &protocolErr) || protocolErr.Code != CodeInvalidMessage {
				t.Fatalf("ValidateEnvelope() error = %v, want code %q", err, CodeInvalidMessage)
			}
		})
	}
}

// TestValidateEnvelopeRejectsUnsupportedInsideEvent keeps the safety notice out
// of the ordinary event type so clients only handle it in one place.
func TestValidateEnvelopeRejectsUnsupportedInsideEvent(t *testing.T) {
	event := structuredEvent(projectsession.EventUnsupported)
	event.Code = projectsession.UnsupportedUnknownBlock
	err := validateEventEnvelope(t, event)
	var protocolErr *ProtocolError
	if !errors.As(err, &protocolErr) || protocolErr.Code != CodeInvalidMessage {
		t.Fatalf("ValidateEnvelope() error = %v, want code %q", err, CodeInvalidMessage)
	}
}

func commandEnvelope(t *testing.T, payload CommandPayload) Envelope {
	t.Helper()
	envelope, err := NewEnvelope(TypeCommand, "session-1", "command-1", "", 0, payload)
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}
	return envelope
}

// TestValidateEnvelopeAcceptsControlCommands covers the §4.5 command surface.
func TestValidateEnvelopeAcceptsControlCommands(t *testing.T) {
	payloads := map[string]CommandPayload{
		"allow once":    {Kind: CommandResolveAction, ActionID: "action-1", Decision: DecisionAllowOnce},
		"allow session": {Kind: CommandResolveAction, ActionID: "action-1", Decision: DecisionAllowSession},
		"deny":          {Kind: CommandResolveAction, ActionID: "action-1", Decision: DecisionDeny},
		"answers":       {Kind: CommandResolveAction, ActionID: "action-1", Answers: []QuestionAnswer{{QuestionID: "q1", Values: []string{"yes"}}}},
		"cancel turn":   {Kind: CommandCancelTurn, RequestID: "request-1"},
		"cancel queued": {Kind: CommandCancelQueued, RequestID: "request-2"},
	}
	for name, payload := range payloads {
		t.Run(name, func(t *testing.T) {
			if err := ValidateEnvelope(commandEnvelope(t, payload)); err != nil {
				t.Fatalf("ValidateEnvelope() error = %v", err)
			}
		})
	}
}

func TestValidateEnvelopeRejectsMalformedControlCommands(t *testing.T) {
	payloads := map[string]CommandPayload{
		"unknown kind":                    {Kind: "resolve", ActionID: "action-1", Decision: DecisionDeny},
		"resolve without action id":       {Kind: CommandResolveAction, Decision: DecisionDeny},
		"resolve without decision":        {Kind: CommandResolveAction, ActionID: "action-1"},
		"resolve with unknown decision":   {Kind: CommandResolveAction, ActionID: "action-1", Decision: "maybe"},
		"resolve with both forms":         {Kind: CommandResolveAction, ActionID: "action-1", Decision: DecisionDeny, Answers: []QuestionAnswer{{QuestionID: "q1", Values: []string{"yes"}}}},
		"resolve carrying text":           {Kind: CommandResolveAction, ActionID: "action-1", Decision: DecisionDeny, Text: "hello"},
		"resolve carrying request id":     {Kind: CommandResolveAction, ActionID: "action-1", Decision: DecisionDeny, RequestID: "request-1"},
		"answer without question id":      {Kind: CommandResolveAction, ActionID: "action-1", Answers: []QuestionAnswer{{Values: []string{"yes"}}}},
		"answer without values":           {Kind: CommandResolveAction, ActionID: "action-1", Answers: []QuestionAnswer{{QuestionID: "q1"}}},
		"answer with blank value":         {Kind: CommandResolveAction, ActionID: "action-1", Answers: []QuestionAnswer{{QuestionID: "q1", Values: []string{"  "}}}},
		"answer with duplicate question":  {Kind: CommandResolveAction, ActionID: "action-1", Answers: []QuestionAnswer{{QuestionID: "q1", Values: []string{"yes"}}, {QuestionID: "q1", Values: []string{"no"}}}},
		"cancel turn without request":     {Kind: CommandCancelTurn},
		"cancel turn carrying action id":  {Kind: CommandCancelTurn, RequestID: "request-1", ActionID: "action-1"},
		"cancel queued without request":   {Kind: CommandCancelQueued},
		"cancel queued carrying decision": {Kind: CommandCancelQueued, RequestID: "request-2", Decision: DecisionDeny},
		"send text carrying action id":    {Kind: CommandSendText, Text: "hello", ActionID: "action-1"},
		"send text carrying request id":   {Kind: CommandSendText, Text: "hello", RequestID: "request-1"},
	}
	for name, payload := range payloads {
		t.Run(name, func(t *testing.T) {
			err := ValidateEnvelope(commandEnvelope(t, payload))
			var protocolErr *ProtocolError
			if !errors.As(err, &protocolErr) || protocolErr.Code != CodeInvalidMessage {
				t.Fatalf("ValidateEnvelope() error = %v, want code %q", err, CodeInvalidMessage)
			}
		})
	}
}

// TestValidateActionResolution checks a decoded resolve_action against the
// pending action it claims to answer, which is where §4.5 option matching and
// the allow_session precondition live.
func TestValidateActionResolution(t *testing.T) {
	approval := projectsession.PendingAction{
		ActionID: "action-1", RequestID: "request-1", Kind: projectsession.ActionToolApproval,
		ToolName: "Bash", Preview: "go test ./...",
	}
	approvalWithSession := approval
	approvalWithSession.AllowSessionAvailable = true
	questions := projectsession.PendingAction{
		ActionID: "action-1", RequestID: "request-1", Kind: projectsession.ActionAskUserQuestion,
		ToolName: projectsession.AskUserQuestionToolName,
		Questions: []projectsession.Question{
			{QuestionID: "q1", Header: "模式", Question: "选一个", InputType: projectsession.QuestionChoice, Options: []projectsession.QuestionOption{{Label: "安全"}, {Label: "完整"}}},
			{QuestionID: "q2", Header: "范围", Question: "可多选", InputType: projectsession.QuestionChoice, MultiSelect: true, Options: []projectsession.QuestionOption{{Label: "前端"}, {Label: "后端"}}},
		},
	}
	textQuestion := projectsession.PendingAction{
		ActionID: "action-1", RequestID: "request-1", Kind: projectsession.ActionAskUserQuestion,
		ToolName: projectsession.AskUserQuestionToolName,
		Questions: []projectsession.Question{
			{QuestionID: "q1", Header: "名称", Question: "输入名称", InputType: projectsession.QuestionText},
		},
	}
	defaultConfig := projectsession.DefaultServiceConfig()
	textEnabled := defaultConfig
	textEnabled.AskUserTextEnabled = true

	tests := []struct {
		name     string
		action   projectsession.PendingAction
		config   projectsession.ServiceConfig
		payload  CommandPayload
		wantCode string
	}{
		{name: "allow once", action: approval, config: defaultConfig, payload: CommandPayload{Kind: CommandResolveAction, ActionID: "action-1", Decision: DecisionAllowOnce}},
		{name: "deny", action: approval, config: defaultConfig, payload: CommandPayload{Kind: CommandResolveAction, ActionID: "action-1", Decision: DecisionDeny}},
		{name: "allow session when suggested", action: approvalWithSession, config: defaultConfig, payload: CommandPayload{Kind: CommandResolveAction, ActionID: "action-1", Decision: DecisionAllowSession}},
		{
			name: "complete answers", action: questions, config: defaultConfig,
			payload: CommandPayload{Kind: CommandResolveAction, ActionID: "action-1", Answers: []QuestionAnswer{
				{QuestionID: "q1", Values: []string{"安全"}},
				{QuestionID: "q2", Values: []string{"前端", "后端"}},
			}},
		},
		{name: "free text when enabled", action: textQuestion, config: textEnabled, payload: CommandPayload{Kind: CommandResolveAction, ActionID: "action-1", Answers: []QuestionAnswer{{QuestionID: "q1", Values: []string{"my-service"}}}}},
		{name: "stale action id", action: approval, config: defaultConfig, payload: CommandPayload{Kind: CommandResolveAction, ActionID: "action-9", Decision: DecisionDeny}, wantCode: CodeActionNotPending},
		{name: "allow session without suggestions", action: approval, config: defaultConfig, payload: CommandPayload{Kind: CommandResolveAction, ActionID: "action-1", Decision: DecisionAllowSession}, wantCode: CodeInvalidAction},
		{name: "answers for an approval", action: approval, config: defaultConfig, payload: CommandPayload{Kind: CommandResolveAction, ActionID: "action-1", Answers: []QuestionAnswer{{QuestionID: "q1", Values: []string{"安全"}}}}, wantCode: CodeInvalidAction},
		{name: "decision for a question", action: questions, config: defaultConfig, payload: CommandPayload{Kind: CommandResolveAction, ActionID: "action-1", Decision: DecisionAllowOnce}, wantCode: CodeInvalidAction},
		{
			name: "partial answers", action: questions, config: defaultConfig,
			payload:  CommandPayload{Kind: CommandResolveAction, ActionID: "action-1", Answers: []QuestionAnswer{{QuestionID: "q1", Values: []string{"安全"}}}},
			wantCode: CodeInvalidAnswer,
		},
		{
			name: "unknown question id", action: questions, config: defaultConfig,
			payload: CommandPayload{Kind: CommandResolveAction, ActionID: "action-1", Answers: []QuestionAnswer{
				{QuestionID: "q1", Values: []string{"安全"}},
				{QuestionID: "q3", Values: []string{"前端"}},
			}},
			wantCode: CodeInvalidAnswer,
		},
		{
			name: "single select with two values", action: questions, config: defaultConfig,
			payload: CommandPayload{Kind: CommandResolveAction, ActionID: "action-1", Answers: []QuestionAnswer{
				{QuestionID: "q1", Values: []string{"安全", "完整"}},
				{QuestionID: "q2", Values: []string{"前端"}},
			}},
			wantCode: CodeInvalidAnswer,
		},
		{
			name: "value outside options", action: questions, config: defaultConfig,
			payload: CommandPayload{Kind: CommandResolveAction, ActionID: "action-1", Answers: []QuestionAnswer{
				{QuestionID: "q1", Values: []string{"自编选项"}},
				{QuestionID: "q2", Values: []string{"前端"}},
			}},
			wantCode: CodeInvalidAnswer,
		},
		{
			name: "free text while disabled", action: textQuestion, config: defaultConfig,
			payload:  CommandPayload{Kind: CommandResolveAction, ActionID: "action-1", Answers: []QuestionAnswer{{QuestionID: "q1", Values: []string{"my-service"}}}},
			wantCode: CodeInvalidAnswer,
		},
		{
			name: "free text with two values", action: textQuestion, config: textEnabled,
			payload:  CommandPayload{Kind: CommandResolveAction, ActionID: "action-1", Answers: []QuestionAnswer{{QuestionID: "q1", Values: []string{"a", "b"}}}},
			wantCode: CodeInvalidAnswer,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateActionResolution(test.action, test.config, test.payload)
			if test.wantCode == "" {
				if err != nil {
					t.Fatalf("ValidateActionResolution() error = %v, want nil", err)
				}
				return
			}
			var protocolErr *ProtocolError
			if !errors.As(err, &protocolErr) || protocolErr.Code != test.wantCode {
				t.Fatalf("ValidateActionResolution() error = %v, want code %q", err, test.wantCode)
			}
			for _, question := range test.action.Questions {
				if question.Question != "" && strings.Contains(protocolErr.Message, question.Question) {
					t.Fatalf("rejection must not echo question text: %s", protocolErr.Message)
				}
			}
			if test.action.Preview != "" && strings.Contains(protocolErr.Message, test.action.Preview) {
				t.Fatalf("rejection must not echo the tool preview: %s", protocolErr.Message)
			}
		})
	}
}

func FuzzDecodeEnvelope(f *testing.F) {
	f.Add([]byte(`{"version":2,"type":"hello","sessionId":"s","commandId":"","requestId":"","seq":0,"payload":{"pairToken":"fixture"}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = DecodeEnvelope(data)
	})
}
