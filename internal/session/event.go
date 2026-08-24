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
)

type Event struct {
	Seq           uint64        `json:"seq"`
	Kind          EventKind     `json:"kind"`
	SessionID     string        `json:"sessionId"`
	RequestID     string        `json:"requestId"`
	Source        InputSource   `json:"source"`
	Text          string        `json:"text"`
	SessionStatus SessionStatus `json:"sessionStatus"`
	TurnStatus    TurnStatus    `json:"turnStatus"`
	Error         *ErrorInfo    `json:"error"`
}
