package transport

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tea4go/claude-mobile-remote/protocol"
)

// Version remains the v2 default for callers constructing legacy envelopes.
const Version = 2
const LatestVersion = 3

var ErrInvalidMessage = errors.New("invalid protocol message")

const (
	CodeInvalidJSON        = "invalid_json"
	CodeUnsupportedVersion = "unsupported_version"
	CodeInvalidMessage     = "invalid_message"
	CodeInvalidState       = "invalid_state"
	CodeUnauthorized       = "unauthorized"
	CodeMessageTooLarge    = "message_too_large"
	CodeInvalidCommand     = "invalid_command"
	CodeDuplicateCommandID = "duplicate_command_id"
	CodeSessionUnavailable = "session_unavailable"
	CodeQueueFull          = "queue_full"
	CodeInternal           = "internal"
)

// B2 §4.5 rejection codes. They are business outcomes rather than malformed
// messages, so they travel in command results and stay stable across clients.
const (
	CodeInvalidAction          = "invalid_action"
	CodeActionNotPending       = "action_not_pending"
	CodeInvalidAnswer          = "invalid_answer"
	CodeTurnNotCancellable     = "turn_not_cancellable"
	CodeRequestNotQueued       = "request_not_queued"
	CodeRequestAlreadyFinished = "request_already_finished"
	CodeActionTimeout          = "action_timeout"
)

// ProtocolError contains a stable client-facing code and a credential-safe message.
type ProtocolError struct {
	Code    string
	Message string
}

func (e *ProtocolError) Error() string {
	return e.Code + ": " + e.Message
}

func (e *ProtocolError) Unwrap() error {
	return ErrInvalidMessage
}

type MessageType string

const (
	TypeHello               MessageType = "hello"
	TypeHelloAck            MessageType = "hello_ack"
	TypeSubscribe           MessageType = "subscribe"
	TypeSubscribed          MessageType = "subscribed"
	TypeUnsubscribe         MessageType = "unsubscribe"
	TypeUnsubscribed        MessageType = "unsubscribed"
	TypeSnapshot            MessageType = "snapshot"
	TypeEvent               MessageType = "event"
	TypeCommand             MessageType = "command"
	TypeCommandResult       MessageType = "command_result"
	TypeCreateSession       MessageType = "create_session"
	TypeCreateSessionResult MessageType = "create_session_result"
	TypeStopSession         MessageType = "stop_session"
	TypeStopSessionResult   MessageType = "stop_session_result"
	TypeError               MessageType = "error"
	// TypeUnsupported reports that something could not be represented safely.
	// It is its own type rather than a content block kind or an event kind, so a
	// client handles the notice in exactly one place and never has to guess at
	// partially mapped data (B2 §4.2).
	TypeUnsupported MessageType = "unsupported"
)

type Envelope struct {
	Version   int             `json:"version"`
	Type      MessageType     `json:"type"`
	SessionID string          `json:"sessionId"`
	CommandID string          `json:"commandId"`
	RequestID string          `json:"requestId"`
	Seq       uint64          `json:"seq"`
	Payload   json.RawMessage `json:"payload"`
}

type CommandKind string

const (
	CommandSendText          CommandKind = "send_text"
	CommandResolveAction     CommandKind = "resolve_action"
	CommandCancelTurn        CommandKind = "cancel_turn"
	CommandCancelQueued      CommandKind = "cancel_queued"
	CommandSetPermissionMode CommandKind = "set_permission_mode"
)

// The decision/answer wire vocabulary now lives in protocol. These aliases keep
// transport-internal call sites and tests on the old unqualified names without
// reintroducing a second definition of the types.
type (
	ActionDecision = protocol.ActionDecision
	QuestionAnswer = protocol.QuestionAnswer
)

const (
	DecisionAllowOnce    = protocol.DecisionAllowOnce
	DecisionAllowSession = protocol.DecisionAllowSession
	DecisionDeny         = protocol.DecisionDeny
)

// HelloPayload carries the credential a client submits in its first message.
type HelloPayload struct {
	PairToken string `json:"pairToken"`
}

// CommandPayload carries every client command. Each kind uses a disjoint subset
// of the fields, and ValidateEnvelope rejects any field outside that subset so a
// malformed control command can never be read as another kind. Decision and
// Answers use the protocol package's single wire vocabulary; there is no
// transport-local ActionDecision or QuestionAnswer copy.
type CommandPayload struct {
	Kind           CommandKind               `json:"kind"`
	Text           string                    `json:"text,omitempty"`
	ActionID       string                    `json:"actionId,omitempty"`
	RequestID      string                    `json:"requestId,omitempty"`
	Decision       protocol.ActionDecision   `json:"decision,omitempty"`
	Answers        []protocol.QuestionAnswer `json:"answers,omitempty"`
	PermissionMode protocol.PermissionMode   `json:"permissionMode,omitempty"`
}

type CommandResultPayload struct {
	Accepted bool   `json:"accepted"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

// ManagementResultPayload never enters a Session event stream. A successful
// create result carries the new Session's credential only to its requester.
type ManagementResultPayload struct {
	Accepted bool              `json:"accepted"`
	Code     string            `json:"code"`
	Message  string            `json:"message"`
	Pairing  *protocol.Pairing `json:"pairing,omitempty"`
}

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	// ExpectedVersion is only set for unsupported_version so a stale client can
	// report the version it must upgrade to. It carries no Session data.
	ExpectedVersion int `json:"expectedVersion,omitempty"`
}

// newErrorPayload is the only constructor for outbound errors, so the version
// hint is attached consistently and never leaks into unrelated failures.
func newErrorPayload(code, message string) ErrorPayload {
	payload := ErrorPayload{Code: code, Message: message}
	if code == CodeUnsupportedVersion {
		payload.ExpectedVersion = LatestVersion
	}
	return payload
}

// UnsupportedPayload carries nothing but a fixed code: the original SDK data is
// never forwarded, so an unmappable block cannot leak through this path.
type UnsupportedPayload struct {
	Code protocol.UnsupportedCode `json:"code"`
}

type SnapshotPayload struct {
	ConnectionStatus string            `json:"connectionStatus"`
	Snapshot         protocol.Snapshot `json:"snapshot"`
}

type EventPayload struct {
	Event protocol.Event `json:"event"`
}

func NewEnvelope(messageType MessageType, sessionID, commandID, requestID string, seq uint64, payload any) (Envelope, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, fmt.Errorf("encode protocol payload: %w", err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &object); err != nil || object == nil {
		return Envelope{}, ErrInvalidMessage
	}
	return Envelope{Version: Version, Type: messageType, SessionID: sessionID, CommandID: commandID, RequestID: requestID, Seq: seq, Payload: encoded}, nil
}

func EncodeEnvelope(envelope Envelope) ([]byte, error) {
	return json.Marshal(envelope)
}

func DecodeEnvelope(data []byte) (Envelope, error) {
	var envelope Envelope
	if err := decodeStrict(data, &envelope); err != nil {
		return Envelope{}, &ProtocolError{Code: CodeInvalidJSON, Message: "invalid JSON message"}
	}
	return envelope, nil
}

// ValidateEnvelope enforces the shared v2/v3 field and payload invariants.
func ValidateEnvelope(envelope Envelope) error {
	if envelope.Version != Version && envelope.Version != LatestVersion {
		return protocolError(CodeUnsupportedVersion, "unsupported protocol version")
	}
	if envelope.Type == "" || envelope.SessionID == "" || len(envelope.Payload) == 0 {
		return protocolError(CodeInvalidMessage, "missing required message field")
	}

	switch envelope.Type {
	case TypeCreateSession, TypeStopSession:
		if envelope.CommandID == "" || envelope.RequestID != "" || envelope.Seq != 0 || !isEmptyPayload(envelope) {
			return protocolError(CodeInvalidMessage, "invalid management request")
		}
	case TypeCreateSessionResult, TypeStopSessionResult:
		var payload ManagementResultPayload
		if envelope.CommandID == "" || envelope.RequestID != "" || envelope.Seq != 0 || DecodePayload(envelope, &payload) != nil {
			return protocolError(CodeInvalidMessage, "invalid management result")
		}
		if payload.Accepted {
			if payload.Code != "" || payload.Message != "" ||
				(envelope.Type == TypeCreateSessionResult && !validCreatedPairing(payload.Pairing, envelope.SessionID)) ||
				(envelope.Type == TypeStopSessionResult && payload.Pairing != nil) {
				return protocolError(CodeInvalidMessage, "invalid accepted management result")
			}
		} else if payload.Code == "" || payload.Pairing != nil {
			return protocolError(CodeInvalidMessage, "invalid rejected management result")
		}
	case TypeCommand:
		if envelope.CommandID == "" || envelope.RequestID != "" || envelope.Seq != 0 {
			return protocolError(CodeInvalidMessage, "invalid command correlation fields")
		}
		var payload CommandPayload
		if err := DecodePayload(envelope, &payload); err != nil {
			return protocolError(CodeInvalidMessage, "invalid command payload")
		}
		if err := validateCommand(payload); err != nil {
			return err
		}
	case TypeHello:
		if envelope.CommandID != "" || envelope.RequestID != "" || envelope.Seq != 0 {
			return protocolError(CodeInvalidMessage, "invalid hello correlation fields")
		}
		var payload HelloPayload
		if err := DecodePayload(envelope, &payload); err != nil || payload.PairToken == "" {
			return protocolError(CodeInvalidMessage, "invalid hello payload")
		}
	case TypeHelloAck, TypeSubscribe, TypeSubscribed, TypeUnsubscribe, TypeUnsubscribed:
		if envelope.CommandID != "" || envelope.RequestID != "" || envelope.Seq != 0 || !isEmptyPayload(envelope) {
			return protocolError(CodeInvalidMessage, "invalid message fields")
		}
	case TypeSnapshot:
		var payload SnapshotPayload
		if envelope.CommandID != "" || envelope.RequestID != "" || DecodePayload(envelope, &payload) != nil ||
			payload.Snapshot.SessionID != envelope.SessionID || payload.Snapshot.Seq != envelope.Seq {
			return protocolError(CodeInvalidMessage, "invalid snapshot message")
		}
		if err := validateSnapshotDisplay(envelope.Version, payload.Snapshot); err != nil {
			return err
		}
	case TypeEvent:
		var payload EventPayload
		if envelope.CommandID != "" || DecodePayload(envelope, &payload) != nil || payload.Event.SessionID != envelope.SessionID ||
			payload.Event.RequestID != envelope.RequestID || payload.Event.Seq != envelope.Seq {
			return protocolError(CodeInvalidMessage, "invalid event message")
		}
		if err := validateEvent(payload.Event); err != nil {
			return err
		}
		if err := validateEventDisplay(envelope.Version, payload.Event); err != nil {
			return err
		}
	case TypeCommandResult:
		var payload CommandResultPayload
		if envelope.CommandID == "" || envelope.Seq != 0 || DecodePayload(envelope, &payload) != nil {
			return protocolError(CodeInvalidMessage, "invalid command result")
		}
		if payload.Accepted && (envelope.RequestID == "" || payload.Code != "") {
			return protocolError(CodeInvalidMessage, "invalid accepted command result")
		}
		if !payload.Accepted && (envelope.RequestID != "" || payload.Code == "") {
			return protocolError(CodeInvalidMessage, "invalid rejected command result")
		}
	case TypeError:
		var payload ErrorPayload
		if envelope.CommandID != "" || envelope.RequestID != "" || envelope.Seq != 0 || DecodePayload(envelope, &payload) != nil || payload.Code == "" {
			return protocolError(CodeInvalidMessage, "invalid error message")
		}
	case TypeUnsupported:
		var payload UnsupportedPayload
		if envelope.CommandID != "" || DecodePayload(envelope, &payload) != nil || !payload.Code.IsKnown() {
			return protocolError(CodeInvalidMessage, "invalid unsupported message")
		}
	default:
		return protocolError(CodeInvalidMessage, "unknown message type")
	}
	return nil
}

func validCreatedPairing(pairing *protocol.Pairing, sourceSessionID string) bool {
	return pairing != nil && pairing.Version == protocol.PayloadVersion && pairing.WSURL != "" &&
		pairing.SessionID != "" && pairing.SessionID != sourceSessionID && pairing.PairToken != "" &&
		!pairing.ExpiresAt.IsZero()
}

func validateDisplayTime(value string) bool {
	if value == "" {
		return true
	}
	if !strings.HasSuffix(value, "Z") {
		return false
	}
	_, err := time.Parse(time.RFC3339Nano, value)
	return err == nil
}

func validateDisplayTitle(value string) bool {
	return utf8.ValidString(value) && len([]rune(value)) <= protocol.MaxSessionTitleRunes && !strings.ContainsAny(value, "\r\n")
}

func validateSnapshotDisplay(version int, snapshot protocol.Snapshot) error {
	if version == Version {
		if snapshot.ProjectCwd != "" || snapshot.SessionTitle != "" || snapshot.CreatedAt != "" || snapshot.LastActivityAt != "" {
			return protocolError(CodeInvalidMessage, "v2 snapshot cannot carry display metadata")
		}
		return nil
	}
	if len(snapshot.ProjectCwd) > 4096 || !validateDisplayTitle(snapshot.SessionTitle) ||
		!validateDisplayTime(snapshot.CreatedAt) || !validateDisplayTime(snapshot.LastActivityAt) {
		return protocolError(CodeInvalidMessage, "invalid snapshot display metadata")
	}
	return nil
}

func validateEventDisplay(version int, event protocol.Event) error {
	if version == Version {
		if event.SessionTitle != "" || event.LastActivityAt != "" {
			return protocolError(CodeInvalidMessage, "v2 event cannot carry display metadata")
		}
		return nil
	}
	if (event.SessionTitle != "" && (event.Kind != protocol.EventTurnStarted || !validateDisplayTitle(event.SessionTitle))) ||
		!validateDisplayTime(event.LastActivityAt) {
		return protocolError(CodeInvalidMessage, "invalid event display metadata")
	}
	return nil
}

// validateCommand keeps each command kind to its own disjoint field subset, so
// an extra field is a hard rejection rather than a silently ignored hint.
func validateCommand(payload CommandPayload) error {
	switch payload.Kind {
	case CommandSendText:
		if payload.ActionID != "" || payload.RequestID != "" || payload.Decision != "" || len(payload.Answers) > 0 {
			return protocolError(CodeInvalidMessage, "invalid send_text command")
		}
	case CommandResolveAction:
		if payload.ActionID == "" || payload.Text != "" || payload.RequestID != "" {
			return protocolError(CodeInvalidMessage, "invalid resolve_action command")
		}
		hasDecision := payload.Decision != ""
		hasAnswers := len(payload.Answers) > 0
		if hasDecision == hasAnswers {
			return protocolError(CodeInvalidMessage, "resolve_action must carry exactly one resolution form")
		}
		if hasDecision && !payload.Decision.IsKnown() {
			return protocolError(CodeInvalidMessage, "unknown approval decision")
		}
		if hasAnswers {
			return validateAnswerShape(payload.Answers)
		}
	case CommandCancelTurn, CommandCancelQueued:
		if payload.RequestID == "" || payload.ActionID != "" || payload.Text != "" ||
			payload.Decision != "" || len(payload.Answers) > 0 {
			return protocolError(CodeInvalidMessage, "invalid cancel command")
		}
	case CommandSetPermissionMode:
		if payload.Text != "" || payload.ActionID != "" || payload.RequestID != "" || payload.Decision != "" || len(payload.Answers) > 0 || !payload.PermissionMode.IsKnown() {
			return protocolError(CodeInvalidMessage, "invalid permission mode command")
		}
	default:
		return protocolError(CodeInvalidMessage, "unknown command kind")
	}
	return nil
}

// validateAnswerShape checks what can be judged without the pending action.
// Matching answers against the offered questions happens in
// ValidateActionResolution, which owns the business-level rejection codes.
func validateAnswerShape(answers []protocol.QuestionAnswer) error {
	seen := make(map[string]struct{}, len(answers))
	for _, answer := range answers {
		if answer.QuestionID == "" || len(answer.Values) == 0 {
			return protocolError(CodeInvalidMessage, "invalid answer")
		}
		if _, duplicate := seen[answer.QuestionID]; duplicate {
			return protocolError(CodeInvalidMessage, "duplicate answer")
		}
		seen[answer.QuestionID] = struct{}{}
		for _, value := range answer.Values {
			if strings.TrimSpace(value) == "" {
				return protocolError(CodeInvalidMessage, "blank answer value")
			}
		}
	}
	return nil
}

// ValidateActionResolution checks a decoded resolve_action against the action it
// claims to answer. It is the single place where option matching, select arity
// and the allow_session precondition are decided, so the Controller and the
// protocol layer cannot drift apart. Rejection messages stay generic: they never
// echo question text or a tool preview.
func ValidateActionResolution(action protocol.PendingAction, config protocol.ServiceConfig, payload CommandPayload) error {
	if payload.Kind != CommandResolveAction {
		return protocolError(CodeInvalidAction, "command does not resolve an action")
	}
	if action.ActionID == "" || payload.ActionID != action.ActionID {
		return protocolError(CodeActionNotPending, "action is no longer pending")
	}
	hasDecision := payload.Decision != ""
	hasAnswers := len(payload.Answers) > 0
	if hasDecision == hasAnswers {
		return protocolError(CodeInvalidAction, "resolution must carry exactly one resolution form")
	}

	switch action.Kind {
	case protocol.ActionToolApproval:
		if !hasDecision {
			return protocolError(CodeInvalidAction, "tool approval expects a decision")
		}
		switch payload.Decision {
		case protocol.DecisionAllowOnce, protocol.DecisionDeny:
		case protocol.DecisionAllowSession:
			if !action.AllowSessionAvailable {
				return protocolError(CodeInvalidAction, "allow_session is not offered for this action")
			}
		default:
			return protocolError(CodeInvalidAction, "unknown approval decision")
		}
	case protocol.ActionAskUserQuestion:
		if !hasAnswers {
			return protocolError(CodeInvalidAction, "question expects answers")
		}
		return validateAnswersAgainstQuestions(action.Questions, config, payload.Answers)
	default:
		return protocolError(CodeInvalidAction, "unknown action kind")
	}
	return nil
}

func validateAnswersAgainstQuestions(questions []protocol.Question, config protocol.ServiceConfig, answers []protocol.QuestionAnswer) error {
	submitted := make(map[string][]string, len(answers))
	for _, answer := range answers {
		if answer.QuestionID == "" {
			return protocolError(CodeInvalidAnswer, "answer without a question id")
		}
		if _, duplicate := submitted[answer.QuestionID]; duplicate {
			return protocolError(CodeInvalidAnswer, "duplicate answer")
		}
		submitted[answer.QuestionID] = answer.Values
	}
	if len(submitted) != len(questions) {
		return protocolError(CodeInvalidAnswer, "answers must cover every question exactly once")
	}
	for _, question := range questions {
		values, answered := submitted[question.QuestionID]
		if !answered {
			return protocolError(CodeInvalidAnswer, "missing answer for an offered question")
		}
		if err := validateAnswerValues(question, config, values); err != nil {
			return err
		}
	}
	return nil
}

func validateAnswerValues(question protocol.Question, config protocol.ServiceConfig, values []string) error {
	switch question.InputType {
	case protocol.QuestionText:
		if !config.AskUserTextEnabled {
			return protocolError(CodeInvalidAnswer, "free-text answers are disabled")
		}
		if len(values) != 1 || strings.TrimSpace(values[0]) == "" {
			return protocolError(CodeInvalidAnswer, "free-text answer must be a single non-blank value")
		}
	case protocol.QuestionChoice:
		if len(values) == 0 || (!question.MultiSelect && len(values) != 1) {
			return protocolError(CodeInvalidAnswer, "unexpected number of selected options")
		}
		offered := make(map[string]struct{}, len(question.Options))
		for _, option := range question.Options {
			offered[option.Label] = struct{}{}
		}
		selected := make(map[string]struct{}, len(values))
		for _, value := range values {
			if _, ok := offered[value]; !ok {
				return protocolError(CodeInvalidAnswer, "selected option was not offered")
			}
			if _, duplicate := selected[value]; duplicate {
				return protocolError(CodeInvalidAnswer, "duplicate selected option")
			}
			selected[value] = struct{}{}
		}
	default:
		return protocolError(CodeInvalidAnswer, "unknown question input type")
	}
	return nil
}

func isEmptyPayload(envelope Envelope) bool {
	var payload struct{}
	return DecodePayload(envelope, &payload) == nil
}

// validateEvent enforces kind-level field semantics for outbound events:
// turn_started must carry the normalized input text, text_delta a non-empty
// increment, and all other kinds no text at all.
func validateEvent(event protocol.Event) error {
	if err := validateEventPayloadFields(event); err != nil {
		return err
	}
	switch event.Kind {
	case protocol.EventTurnStarted:
		if event.RequestID == "" ||
			(event.Source != protocol.InputTerminal && event.Source != protocol.InputMobile) ||
			event.Text == "" || len([]byte(event.Text)) > protocol.MaxPromptBytes ||
			event.TurnStatus != protocol.TurnRunning {
			return protocolError(CodeInvalidMessage, "invalid turn_started event")
		}
	case protocol.EventTextDelta:
		if event.Text == "" {
			return protocolError(CodeInvalidMessage, "invalid text_delta event")
		}
	default:
		if event.Text != "" {
			return protocolError(CodeInvalidMessage, "unexpected event text")
		}
	}
	return nil
}

// validateEventPayloadFields binds the B2 structured fields to the exact kinds
// that own them. Every other kind must leave them absent, so a client never has
// to guess whether a block or an action belongs to the event it is reading.
func validateEventPayloadFields(event protocol.Event) error {
	switch event.Kind {
	case protocol.EventContentBlock:
		if event.PendingAction != nil || event.Code != "" || len(event.Answers) > 0 {
			return protocolError(CodeInvalidMessage, "unexpected content_block fields")
		}
		if event.Block == nil {
			return protocolError(CodeInvalidMessage, "content_block event without a block")
		}
		return validateContentBlock(*event.Block)
	case protocol.EventActionRequired:
		if event.Block != nil || event.Code != "" || len(event.Answers) > 0 {
			return protocolError(CodeInvalidMessage, "unexpected action_required fields")
		}
		if event.PendingAction == nil {
			return protocolError(CodeInvalidMessage, "action_required event without an action")
		}
		if event.TurnStatus != protocol.TurnWaiting {
			return protocolError(CodeInvalidMessage, "action_required must report a waiting turn")
		}
		return validatePendingAction(*event.PendingAction, event.RequestID)
	case protocol.EventActionResolved:
		// The resolution drops the action itself (D47) but may echo the answers
		// the server just accepted, so every client can show what was answered
		// even when another client answered it (D52).
		if event.Block != nil || event.PendingAction != nil || event.Code != "" {
			return protocolError(CodeInvalidMessage, "unexpected action_resolved fields")
		}
		return validateResolvedAnswers(event.Answers)
	case protocol.EventPermissionModeChanged:
		if event.Block != nil || event.PendingAction != nil || event.Code != "" || len(event.Answers) > 0 || !event.PermissionMode.IsKnown() || event.TurnStatus == "" {
			return protocolError(CodeInvalidMessage, "invalid permission mode event")
		}
	case protocol.EventUnsupported:
		// The safety notice has its own envelope type, so it must never reach a
		// client as an ordinary event (B2 §4.2).
		return protocolError(CodeInvalidMessage, "unsupported must not travel as an event")
	default:
		if event.Block != nil || event.PendingAction != nil || event.Code != "" || len(event.Answers) > 0 {
			return protocolError(CodeInvalidMessage, "unexpected structured event fields")
		}
	}
	return nil
}

// validateResolvedAnswers keeps the D52 answer echo to the shape the server
// accepted: one entry per answered question, each naming its question and
// carrying at least one non-blank value. The echo is the answer, never any part
// of the tool result, so no other field may travel here.
func validateResolvedAnswers(answers []protocol.ResolvedAnswer) error {
	seen := make(map[string]struct{}, len(answers))
	for _, answer := range answers {
		if answer.QuestionID == "" || answer.Question == "" || len(answer.Values) == 0 {
			return protocolError(CodeInvalidMessage, "invalid resolved answer")
		}
		if _, duplicate := seen[answer.QuestionID]; duplicate {
			return protocolError(CodeInvalidMessage, "duplicate resolved answer")
		}
		seen[answer.QuestionID] = struct{}{}
		for _, value := range answer.Values {
			if strings.TrimSpace(value) == "" {
				return protocolError(CodeInvalidMessage, "blank resolved answer value")
			}
		}
	}
	return nil
}

// validateContentBlock applies the per-kind field whitelist of B2 §4.2. Anything
// unknown or incomplete is rejected here instead of being forwarded, because the
// Adapter is expected to degrade such input to the unsupported notice.
func validateContentBlock(block protocol.ContentBlock) error {
	if block.ID == "" {
		return protocolError(CodeInvalidMessage, "content block without an id")
	}
	switch block.Kind {
	case protocol.BlockThinking:
		if block.Thinking == "" || block.ToolUseID != "" || block.ToolClass != "" ||
			block.MCPServer != "" || block.ToolName != "" || block.IsError != nil {
			return protocolError(CodeInvalidMessage, "invalid thinking block")
		}
	case protocol.BlockToolUse:
		if block.ToolUseID == "" || block.ToolName == "" || block.Thinking != "" || block.IsError != nil {
			return protocolError(CodeInvalidMessage, "invalid tool_use block")
		}
		switch block.ToolClass {
		case protocol.ToolBuiltin:
			if block.MCPServer != "" {
				return protocolError(CodeInvalidMessage, "builtin tool must not name an MCP server")
			}
		case protocol.ToolMCP:
			if block.MCPServer == "" {
				return protocolError(CodeInvalidMessage, "MCP tool without a server")
			}
		default:
			return protocolError(CodeInvalidMessage, "unknown tool class")
		}
	case protocol.BlockToolResult:
		if block.ToolUseID == "" || block.IsError == nil || block.Thinking != "" ||
			block.ToolClass != "" || block.MCPServer != "" || block.ToolName != "" {
			return protocolError(CodeInvalidMessage, "invalid tool_result block")
		}
	default:
		return protocolError(CodeInvalidMessage, "unknown content block kind")
	}
	return nil
}

// validatePendingAction applies the B2 §4.3 shape, including that an approval
// never carries questions and a question never carries a tool preview.
func validatePendingAction(action protocol.PendingAction, requestID string) error {
	if action.ActionID == "" || action.RequestID == "" || action.RequestID != requestID {
		return protocolError(CodeInvalidMessage, "pending action correlation mismatch")
	}
	switch action.Kind {
	case protocol.ActionToolApproval:
		if action.ToolName == "" || len(action.Questions) > 0 {
			return protocolError(CodeInvalidMessage, "invalid tool_approval action")
		}
	case protocol.ActionAskUserQuestion:
		if action.ToolName != protocol.AskUserQuestionToolName || action.Preview != "" || len(action.Questions) == 0 {
			return protocolError(CodeInvalidMessage, "invalid ask_user_question action")
		}
		return validateQuestions(action.Questions)
	default:
		return protocolError(CodeInvalidMessage, "unknown action kind")
	}
	return nil
}

func validateQuestions(questions []protocol.Question) error {
	seen := make(map[string]struct{}, len(questions))
	for _, question := range questions {
		if question.QuestionID == "" || question.Header == "" || question.Question == "" {
			return protocolError(CodeInvalidMessage, "invalid question")
		}
		if _, duplicate := seen[question.QuestionID]; duplicate {
			return protocolError(CodeInvalidMessage, "duplicate question id")
		}
		seen[question.QuestionID] = struct{}{}
		switch question.InputType {
		case protocol.QuestionChoice:
			if len(question.Options) == 0 {
				return protocolError(CodeInvalidMessage, "choice question without options")
			}
			for _, option := range question.Options {
				if option.Label == "" {
					return protocolError(CodeInvalidMessage, "option without a label")
				}
			}
		case protocol.QuestionText:
			if len(question.Options) > 0 || question.MultiSelect {
				return protocolError(CodeInvalidMessage, "text question must not offer options")
			}
		default:
			return protocolError(CodeInvalidMessage, "unknown question input type")
		}
	}
	return nil
}

func protocolError(code, message string) error {
	return &ProtocolError{Code: code, Message: message}
}

func DecodePayload(envelope Envelope, destination any) error {
	if err := decodeStrict(envelope.Payload, destination); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidMessage, err)
	}
	return nil
}

func decodeStrict(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}
