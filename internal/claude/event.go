package claude

import (
	"errors"
	"fmt"
)

// EventKind 标识 SDK Adapter 向上层发出的 A2 内部事件。
type EventKind string

const (
	EventSessionStarted EventKind = "session_started"
	EventText           EventKind = "text"
	EventTurnCompleted  EventKind = "turn_completed"
	EventTurnFailed     EventKind = "turn_failed"
)

// SessionIDSource 标识真实 session ID 的 SDK 消息来源。
type SessionIDSource string

const (
	SessionIDFromInit   SessionIDSource = "init"
	SessionIDFromResult SessionIDSource = "result"
)

// Event 是 A2 SDK Adapter 的内部事件，不属于三端通信协议。
type Event struct {
	Kind      EventKind
	SessionID string
	Text      string
	Source    SessionIDSource
}

// ErrorScope 表示错误仅影响当前 Turn，或使整个 Session 流不再可信。
type ErrorScope string

const (
	ScopeTurn    ErrorScope = "turn"
	ScopeSession ErrorScope = "session"
)

type scopedError struct {
	scope ErrorScope
	op    string
	err   error
}

func (e *scopedError) Error() string     { return fmt.Sprintf("%s: %v", e.op, e.err) }
func (e *scopedError) Unwrap() error     { return e.err }
func (e *scopedError) Scope() ErrorScope { return e.scope }

func wrapError(scope ErrorScope, op string, err error) error {
	if err == nil {
		return nil
	}
	return &scopedError{scope: scope, op: op, err: err}
}

// ScopeOf 返回项目错误的影响范围。未知错误按保守策略视为 Session 级。
func ScopeOf(err error) ErrorScope {
	var target interface{ Scope() ErrorScope }
	if errors.As(err, &target) {
		return target.Scope()
	}
	return ScopeSession
}
