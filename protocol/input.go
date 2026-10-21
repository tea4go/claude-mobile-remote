package protocol

import (
	"errors"
	"strings"
)

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

// InputDisplay is trusted upload presentation metadata, separate from the
// execution prompt. It is never read from a caller-supplied path or persisted.
type InputDisplay struct {
	Text        string       `json:"text"`
	Attachments []Attachment `json:"attachments"`
}

type Input struct {
	SessionID string        `json:"sessionId"`
	Source    InputSource   `json:"source"`
	Text      string        `json:"text"`
	Display   *InputDisplay `json:"-"`
}

// DisplayInputText keeps execution-only cache paths out of user-facing titles.
func DisplayInputText(input Input) string {
	if input.Display == nil {
		return input.Text
	}
	var parts []string
	if caption := strings.TrimSpace(input.Display.Text); caption != "" {
		parts = append(parts, caption)
	}
	for _, item := range input.Display.Attachments {
		parts = append(parts, "[附件: "+item.Name+"]")
	}
	return strings.Join(parts, "\n")
}
