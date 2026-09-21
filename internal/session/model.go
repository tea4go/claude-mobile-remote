package session

import "github.com/tea4go/claude-mobile-remote/protocol"

// The JSON-bearing types below are aliases to protocol, the single Go source of
// truth for the wire contract. This package keeps the actor-only helpers and the
// queue construction that are internal to the Session, but no second definition
// of any shared type exists here.

type (
	ServiceConfig     = protocol.ServiceConfig
	SessionStatus     = protocol.SessionStatus
	TurnStatus        = protocol.TurnStatus
	ErrorInfo         = protocol.ErrorInfo
	BlockKind         = protocol.BlockKind
	ToolClass         = protocol.ToolClass
	ContentBlock      = protocol.ContentBlock
	ActionKind        = protocol.ActionKind
	QuestionInputType = protocol.QuestionInputType
	QuestionOption    = protocol.QuestionOption
	Question          = protocol.Question
	PendingAction     = protocol.PendingAction
	ResolvedAnswer    = protocol.ResolvedAnswer
	QueuedRequest     = protocol.QueuedRequest
	TurnSnapshot      = protocol.TurnSnapshot
	Snapshot          = protocol.Snapshot
	PermissionMode    = protocol.PermissionMode
)

const (
	AskUserQuestionToolName = protocol.AskUserQuestionToolName

	SessionStarting = protocol.SessionStarting
	SessionReady    = protocol.SessionReady
	SessionStopped  = protocol.SessionStopped
	SessionFailed   = protocol.SessionFailed

	TurnIdle      = protocol.TurnIdle
	TurnRunning   = protocol.TurnRunning
	TurnWaiting   = protocol.TurnWaiting
	TurnCompleted = protocol.TurnCompleted
	TurnCancelled = protocol.TurnCancelled
	TurnFailed    = protocol.TurnFailed

	PermissionModeDefault           = protocol.PermissionModeDefault
	PermissionModeAcceptEdits       = protocol.PermissionModeAcceptEdits
	PermissionModePlan              = protocol.PermissionModePlan
	PermissionModeBypassPermissions = protocol.PermissionModeBypassPermissions

	BlockThinking   = protocol.BlockThinking
	BlockToolUse    = protocol.BlockToolUse
	BlockToolResult = protocol.BlockToolResult

	ToolBuiltin = protocol.ToolBuiltin
	ToolMCP     = protocol.ToolMCP

	ActionToolApproval    = protocol.ActionToolApproval
	ActionAskUserQuestion = protocol.ActionAskUserQuestion

	QuestionChoice = protocol.QuestionChoice
	QuestionText   = protocol.QuestionText
)

var DefaultServiceConfig = protocol.DefaultServiceConfig

var BoolPointer = protocol.BoolPointer

// newQueuedRequest builds one FIFO view entry with the same preview truncation
// shared by tool approvals.
func newQueuedRequest(config ServiceConfig, requestID string, input Input) QueuedRequest {
	return QueuedRequest{
		RequestID: requestID,
		Source:    input.Source,
		Preview:   protocol.TruncatePreview(input.Text, config.ToolPreviewMaxRunes),
	}
}

// setQueuedRequests is the only writer of both FIFO views, so queueDepth can
// never drift from queuedRequests (B2 §4.4). It lives here rather than as a
// Snapshot method because Go cannot define methods on an alias to a non-local
// type.
func setQueuedRequests(snapshot *Snapshot, items []QueuedRequest) {
	snapshot.QueuedRequests = items
	snapshot.QueueDepth = len(items)
}
