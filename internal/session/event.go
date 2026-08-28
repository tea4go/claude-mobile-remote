package session

type EventKind string

const (
	EventSessionStarted EventKind = "session_started"
	EventTurnStarted    EventKind = "turn_started"
	EventTextDelta      EventKind = "text_delta"
	EventTurnCompleted  EventKind = "turn_completed"
	EventTurnFailed     EventKind = "turn_failed"
	EventSessionFailed  EventKind = "session_failed"
	EventSessionStopped EventKind = "session_stopped"
	// EventContentBlock carries one structured Thinking / tool / tool result
	// title (B2 §4.2).
	EventContentBlock EventKind = "content_block"
	// EventActionRequired and EventActionResolved bracket the pending action, so
	// a client sees the same waiting state through both the event stream and the
	// snapshot (B2 §3.2).
	EventActionRequired EventKind = "action_required"
	EventActionResolved EventKind = "action_resolved"
	// EventUnsupported reports that something could not be represented safely.
	// Transport carries it as its own envelope type rather than as an ordinary
	// event, so clients handle the notice in exactly one place (B2 §4.2).
	EventUnsupported EventKind = "unsupported"
)

// UnsupportedCode is the closed set of safe reasons an unsupported notice can
// state. It never carries a message, raw SDK data or user content.
type UnsupportedCode string

const (
	// UnsupportedUnknownBlock reports an unknown or incomplete SDK content block.
	UnsupportedUnknownBlock UnsupportedCode = "unknown_block"
	// UnsupportedUnknownSubtype reports an unknown SystemMessage subtype.
	UnsupportedUnknownSubtype UnsupportedCode = "unknown_subtype"
	// UnsupportedConcurrentAction reports that a second permission callback was
	// denied because one action was already pending.
	UnsupportedConcurrentAction UnsupportedCode = "concurrent_action"
	// UnsupportedTextDisabled reports that a free-text question was dropped
	// because ServiceConfig.AskUserTextEnabled is off.
	UnsupportedTextDisabled UnsupportedCode = "text_input_disabled"
)

// IsKnown reports whether the code belongs to the closed set above.
func (code UnsupportedCode) IsKnown() bool {
	switch code {
	case UnsupportedUnknownBlock, UnsupportedUnknownSubtype, UnsupportedConcurrentAction, UnsupportedTextDisabled:
		return true
	}
	return false
}

type Event struct {
	Seq           uint64          `json:"seq"`
	Kind          EventKind       `json:"kind"`
	SessionID     string          `json:"sessionId"`
	RequestID     string          `json:"requestId"`
	Source        InputSource     `json:"source"`
	Text          string          `json:"text"`
	SessionStatus SessionStatus   `json:"sessionStatus"`
	TurnStatus    TurnStatus      `json:"turnStatus"`
	Error         *ErrorInfo      `json:"error"`
	Block         *ContentBlock   `json:"block,omitempty"`
	PendingAction *PendingAction  `json:"pendingAction,omitempty"`
	Code          UnsupportedCode `json:"code,omitempty"`
	// Answers only ever accompanies EventActionResolved for an answered
	// AskUserQuestion (D52); permission decisions leave it empty.
	Answers []ResolvedAnswer `json:"answers,omitempty"`
}
