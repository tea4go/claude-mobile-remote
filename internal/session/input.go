package session

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	QueueCapacity  = 32
	MaxPromptBytes = 1024 * 1024
)

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

type requestIDGenerator func() (string, error)

func randomRequestID() (string, error) {
	return requestIDFrom(rand.Reader)
}

func requestIDFrom(reader io.Reader) (string, error) {
	var raw [16]byte
	if _, err := io.ReadFull(reader, raw[:]); err != nil {
		return "", fmt.Errorf("%w: generate request ID: %v", ErrInternal, err)
	}
	return hex.EncodeToString(raw[:]), nil
}

func normalizeInput(currentID string, input Input) (Input, error) {
	if input.Source != InputTerminal && input.Source != InputMobile {
		return Input{}, ErrInvalidInput
	}

	input.Text = strings.TrimSpace(input.Text)
	if input.Text == "" || len([]byte(input.Text)) > MaxPromptBytes {
		return Input{}, ErrInvalidInput
	}

	// A5.2：预留的 session ID 即真实 ID，手机输入在确认前即可携带它提交。
	if input.Source == InputMobile && currentID == "" {
		return Input{}, ErrSessionUnavailable
	}
	if currentID == "" && input.SessionID != "" {
		return Input{}, ErrSessionMismatch
	}
	if currentID != "" && input.SessionID != "" && input.SessionID != currentID {
		return Input{}, ErrSessionMismatch
	}
	if input.Source == InputMobile && currentID != "" && input.SessionID == "" {
		return Input{}, ErrSessionMismatch
	}

	return input, nil
}
