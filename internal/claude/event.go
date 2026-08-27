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
	// EventContentBlock 携带一个结构化内容块（B2 §4.2）。
	EventContentBlock EventKind = "content_block"
	// EventUnsupported 报告一个未知或不完整的 SDK 输出，只带固定 code，
	// 不透传任何原始字段。
	EventUnsupported   EventKind = "unsupported"
	EventTurnCompleted EventKind = "turn_completed"
	EventTurnFailed    EventKind = "turn_failed"
)

// askUserQuestionToolName 是 CLI 交互提问工具的固定名称。它的 tool_result 会以
// is_error=true 返回，需要在映射层修正（设计 §3.1「结果修正」）。
const askUserQuestionToolName = "AskUserQuestion"

// BlockKind 枚举 B2 §4.2 的三类结构化内容。未知或不完整的 SDK block 不会成为
// 第四种取值，而是降级为 EventUnsupported。
type BlockKind string

const (
	BlockThinking   BlockKind = "thinking"
	BlockToolUse    BlockKind = "tool_use"
	BlockToolResult BlockKind = "tool_result"
)

// ToolClass 区分内置工具与 MCP 工具。Task 按普通内置工具处理，不单独建类。
type ToolClass string

const (
	ToolBuiltin ToolClass = "builtin"
	ToolMCP     ToolClass = "mcp"
)

// UnsupportedCode 是降级事件的固定原因码。Adapter 只会产生下面两个，其余
// code（并发 action、自由文本未开启）由上层在自己的语义下产生。
type UnsupportedCode string

const (
	// UnsupportedUnknownBlock 报告未知或不完整的 SDK 内容块。
	UnsupportedUnknownBlock UnsupportedCode = "unknown_block"
	// UnsupportedUnknownSubtype 报告未知的 SystemMessage subtype。
	UnsupportedUnknownSubtype UnsupportedCode = "unknown_subtype"
)

// ContentBlock 是结构化输出的标题视图：只带展示和关联所需的字段，不带工具
// 输入、工具结果正文或任何服务端摘要。展示用的 ID 由 Controller 分配（它是
// seq 的唯一持有者），Adapter 不生成 ID。
type ContentBlock struct {
	Kind      BlockKind
	Thinking  string
	ToolUseID string
	ToolClass ToolClass
	MCPServer string
	ToolName  string
	// Preview is a bounded, schema-selected display string. It never contains
	// the raw tool input and is safe to show beside the tool title.
	Preview string
	// IsError 是指针：只有 tool_result 带该字段，且必须显式区分 true/false。
	IsError *bool
}

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
	// Block 仅在 Kind 为 EventContentBlock 时非空。
	Block *ContentBlock
	// Code 仅在 Kind 为 EventUnsupported 时非空。
	Code UnsupportedCode
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
