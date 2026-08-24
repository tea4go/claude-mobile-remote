package session

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

type PendingAction struct{}

type TurnSnapshot struct {
	Status        TurnStatus     `json:"status"`
	RequestID     string         `json:"requestId"`
	Source        InputSource    `json:"source"`
	Message       string         `json:"message"`
	Error         *ErrorInfo     `json:"error"`
	PendingAction *PendingAction `json:"pendingAction"`
}

type Snapshot struct {
	SessionID     string        `json:"sessionId"`
	SessionStatus SessionStatus `json:"sessionStatus"`
	SessionError  *ErrorInfo    `json:"sessionError"`
	Seq           uint64        `json:"seq"`
	Turn          TurnSnapshot  `json:"turn"`
	QueueDepth    int           `json:"queueDepth"`
}
