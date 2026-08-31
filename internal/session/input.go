package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/tea4go/claude-mobile-remote/protocol"
)

// QueueCapacity is the Session-side FIFO bound. It is internal: no wire field
// carries it, so it stays here rather than in protocol.
const QueueCapacity = 32

// MaxPromptBytes, the input vocabulary and the input sentinels are shared with
// Transport and the CLI, so they live in protocol and are aliased here.
const MaxPromptBytes = protocol.MaxPromptBytes

type (
	InputSource = protocol.InputSource
	Input       = protocol.Input
)

const (
	InputTerminal = protocol.InputTerminal
	InputMobile   = protocol.InputMobile
)

var (
	ErrInvalidInput       = protocol.ErrInvalidInput
	ErrQueueFull          = protocol.ErrQueueFull
	ErrSessionMismatch    = protocol.ErrSessionMismatch
	ErrSessionUnavailable = protocol.ErrSessionUnavailable
	ErrInternal           = protocol.ErrInternal
)

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
