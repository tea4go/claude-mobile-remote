package session

import projectclaude "github.com/tea4go/claude-mobile-remote/internal/claude"

// AskUserQuestionToolName is the SDK tool that carries interactive questions.
// It is also the only tool whose is_error result is corrected to false (B2 §3.1).
const AskUserQuestionToolName = "AskUserQuestion"

// ServiceConfig holds the PC service knobs that shape what B2 puts on the wire.
// Both fields are deliberately conservative defaults, see B2 §3.1 and §4.3.
type ServiceConfig struct {
	// ToolPreviewMaxRunes is the single source of truth for truncating every
	// display preview, both tool approvals and queued inputs.
	ToolPreviewMaxRunes int
	// AskUserTextEnabled gates the conditional free-text question form. It stays
	// false until the CLI probe proves the SDK accepts non-option answers, and
	// even then it must be turned on explicitly.
	AskUserTextEnabled bool
}

func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		ToolPreviewMaxRunes: projectclaude.DefaultToolPreviewMaxRunes,
		AskUserTextEnabled:  false,
	}
}

// BoolPointer builds the optional booleans the protocol needs to tell "absent"
// apart from "explicitly false", which the content block whitelist relies on.
func BoolPointer(value bool) *bool {
	return &value
}

type SessionStatus string

const (
	SessionStarting SessionStatus = "starting"
	SessionReady    SessionStatus = "ready"
	SessionStopped  SessionStatus = "stopped"
	SessionFailed   SessionStatus = "failed"
)

type TurnStatus string

const (
	TurnIdle      TurnStatus = "idle"
	TurnRunning   TurnStatus = "running"
	TurnWaiting   TurnStatus = "waiting"
	TurnCompleted TurnStatus = "completed"
	TurnCancelled TurnStatus = "cancelled"
	TurnFailed    TurnStatus = "failed"
)

type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// BlockKind enumerates the three structured content kinds of B2 §4.2. Unknown or
// incomplete SDK blocks never become a fourth kind; they degrade to the separate
// unsupported notice instead.
type BlockKind string

const (
	BlockThinking   BlockKind = "thinking"
	BlockToolUse    BlockKind = "tool_use"
	BlockToolResult BlockKind = "tool_result"
)

type ToolClass string

const (
	ToolBuiltin ToolClass = "builtin"
	ToolMCP     ToolClass = "mcp"
)

// ContentBlock is the title-only view of Claude's structured output. It never
// carries tool input, tool result bodies or server-side summaries.
type ContentBlock struct {
	ID        string    `json:"id"`
	Kind      BlockKind `json:"kind"`
	Thinking  string    `json:"thinking,omitempty"`
	ToolUseID string    `json:"toolUseId,omitempty"`
	ToolClass ToolClass `json:"toolClass,omitempty"`
	MCPServer string    `json:"mcpServer,omitempty"`
	ToolName  string    `json:"toolName,omitempty"`
	Preview   string    `json:"preview,omitempty"`
	// IsError is a pointer so tool_result can report an explicit false while the
	// other kinds must leave it absent.
	IsError *bool `json:"isError,omitempty"`
}

type ActionKind string

const (
	ActionToolApproval    ActionKind = "tool_approval"
	ActionAskUserQuestion ActionKind = "ask_user_question"
)

// QuestionInputType belongs to a single question rather than to the action, so a
// future free-text question can coexist with choice questions in one action.
type QuestionInputType string

const (
	QuestionChoice QuestionInputType = "choice"
	QuestionText   QuestionInputType = "text"
)

type QuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

type Question struct {
	QuestionID  string            `json:"questionId"`
	Header      string            `json:"header"`
	Question    string            `json:"question"`
	InputType   QuestionInputType `json:"inputType"`
	MultiSelect bool              `json:"multiSelect"`
	Options     []QuestionOption  `json:"options,omitempty"`
}

// PendingAction is the single interaction the Session is currently blocked on.
// The raw tool input, the SDK permission request ID, toolClass and toolUseId are
// deliberately absent: clients only ever address an action by its actionId.
type PendingAction struct {
	ActionID              string     `json:"actionId"`
	RequestID             string     `json:"requestId"`
	Kind                  ActionKind `json:"kind"`
	ToolName              string     `json:"toolName"`
	Preview               string     `json:"preview,omitempty"`
	AllowSessionAvailable bool       `json:"allowSessionAvailable"`
	Questions             []Question `json:"questions,omitempty"`
}

// ResolvedAnswer is one answered question exactly as the server accepted it, so
// any client can show what was answered even if another client answered it
// (D52). Values are the same strings the server hands to the SDK; no part of the
// tool result is involved. Answers ride on action_resolved only and never enter
// the snapshot.
type ResolvedAnswer struct {
	QuestionID string   `json:"questionId"`
	Header     string   `json:"header,omitempty"`
	Question   string   `json:"question"`
	Values     []string `json:"values"`
}

// QueuedRequest exposes one not-yet-started FIFO entry so any client can name a
// cancellation target, including inputs submitted from the PC or another phone.
type QueuedRequest struct {
	RequestID string      `json:"requestId"`
	Source    InputSource `json:"source"`
	Preview   string      `json:"preview"`
}

func newQueuedRequest(config ServiceConfig, requestID string, input Input) QueuedRequest {
	return QueuedRequest{
		RequestID: requestID,
		Source:    input.Source,
		Preview:   projectclaude.TruncatePreview(input.Text, config.ToolPreviewMaxRunes),
	}
}

type TurnSnapshot struct {
	Status        TurnStatus     `json:"status"`
	RequestID     string         `json:"requestId"`
	Source        InputSource    `json:"source"`
	Message       string         `json:"message"`
	Error         *ErrorInfo     `json:"error"`
	PendingAction *PendingAction `json:"pendingAction"`
}

type Snapshot struct {
	SessionID      string          `json:"sessionId"`
	SessionStatus  SessionStatus   `json:"sessionStatus"`
	SessionError   *ErrorInfo      `json:"sessionError"`
	Seq            uint64          `json:"seq"`
	Turn           TurnSnapshot    `json:"turn"`
	QueueDepth     int             `json:"queueDepth"`
	QueuedRequests []QueuedRequest `json:"queuedRequests"`
}

// setQueuedRequests is the only writer of both FIFO views, so queueDepth can
// never drift from queuedRequests (B2 §4.4).
func (s *Snapshot) setQueuedRequests(items []QueuedRequest) {
	s.QueuedRequests = items
	s.QueueDepth = len(items)
}
