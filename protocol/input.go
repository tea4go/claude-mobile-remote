package protocol

import "errors"

// MaxPromptBytes is the hard upper bound on a single normalized prompt. Transport
// and the CLI derive their read/send budgets from it so they stay in lockstep.
const MaxPromptBytes = 1024 * 1024

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrQueueFull          = errors.New("input queue full")
	ErrSessionMismatch    = errors.New("session ID mismatch")
	ErrSessionUnavailable = errors.New("session unavailable")
	ErrInternal           = errors.New("internal error")
)

type InputSource string

const (
	InputTerminal InputSource = "terminal"
	InputMobile   InputSource = "mobile"
)

type Input struct {
	SessionID string      `json:"sessionId"`
	Source    InputSource `json:"source"`
	Text      string      `json:"text"`
}
