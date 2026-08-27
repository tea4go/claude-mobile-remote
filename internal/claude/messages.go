package claude

import (
	"strings"

	claudecode "github.com/tea4go/claude-agent-sdk-go"
)

type resultFact struct {
	isError bool
	errors  []string
	text    string
}

// assistantErrorFact 记录带 Error 的 AssistantMessage：该消息终止当前流，
// 其 Content 中的文本即错误描述（可能为空，需回退到错误类型）。
type assistantErrorFact struct {
	errorType string
	text      string
}

// parsedItem 是一条 SDK 消息里的一个可上报条目。items 严格按 SDK 原始顺序保存，
// 不合并也不重排：thinking 在正文之前、tool_use 在正文之后对手机展示有意义。
type parsedItem struct {
	kind  EventKind
	text  string
	block *ContentBlock
	code  UnsupportedCode
}

type parsedMessage struct {
	sessionID    string
	source       SessionIDSource
	items        []parsedItem
	result       *resultFact
	assistantErr *assistantErrorFact
}

// mcpToolPrefix 是 MCP 工具名的固定前缀，完整形式为 mcp__<server>__<tool>。
const mcpToolPrefix = "mcp__"

// blockMapper 把 SDK 消息映射为上报条目。它必须跳转多条消息保留状态：
// tool_result 只带 toolUseId，要判定它是否属于 AskUserQuestion 必须回查先前
// tool_use 的工具名。同一个 Host 持有同一个 mapper。
type blockMapper struct {
	toolNames map[string]string
}

func newBlockMapper() *blockMapper {
	return &blockMapper{toolNames: make(map[string]string)}
}

func unsupportedItem(code UnsupportedCode) parsedItem {
	return parsedItem{kind: EventUnsupported, code: code}
}

func (m *blockMapper) parse(message claudecode.Message) parsedMessage {
	switch msg := message.(type) {
	case *claudecode.SystemMessage:
		if msg.Subtype != "init" {
			// 真实 Claude 每轮会发非 init 的 SystemMessage（如 compact_boundary/
			// api_retry），它们不是待处理事项；静默忽略，不产生降级通知，
			// 避免普通轮误弹“不支持待处理事项”（参考实现 whale-term
			// claude_code_cli_new_stream.go 对非 init subtype continue）。
			return parsedMessage{}
		}
		sessionID, _ := msg.Data["session_id"].(string)
		return parsedMessage{sessionID: sessionID, source: SessionIDFromInit}
	case *claudecode.AssistantMessage:
		if msg.ParentToolUseID != nil {
			return parsedMessage{}
		}
		if msg.HasError() {
			return parsedMessage{assistantErr: &assistantErrorFact{
				errorType: string(msg.GetError()),
				text:      assistantText(msg),
			}}
		}
		parsed := parsedMessage{}
		for _, block := range msg.Content {
			if item, ok := m.mapBlock(block); ok {
				parsed.items = append(parsed.items, item)
			}
		}
		return parsed
	case *claudecode.UserMessage:
		if msg.ParentToolUseID != nil {
			return parsedMessage{}
		}
		// UserMessage 的 Content 可能是纯文本（回显的 prompt），也可能是块
		// 数组。只有其中的 tool_result 对手机有意义，其余块全部忽略。
		blocks, ok := msg.Content.([]claudecode.ContentBlock)
		if !ok {
			return parsedMessage{}
		}
		parsed := parsedMessage{}
		for _, block := range blocks {
			result, isResult := block.(*claudecode.ToolResultBlock)
			if !isResult {
				continue
			}
			parsed.items = append(parsed.items, m.mapToolResult(result))
		}
		return parsed
	case *claudecode.ResultMessage:
		text := ""
		if msg.Result != nil {
			text = *msg.Result
		}
		return parsedMessage{
			sessionID: msg.SessionID,
			source:    SessionIDFromResult,
			result:    &resultFact{isError: msg.IsError, errors: msg.Errors, text: text},
		}
	default:
		return parsedMessage{}
	}
}

// mapBlock 映射单个内容块。第二个返回值为 false 表示该块不产生任何条目。
func (m *blockMapper) mapBlock(block claudecode.ContentBlock) (parsedItem, bool) {
	switch typed := block.(type) {
	case *claudecode.TextBlock:
		if typed.Text == "" {
			return parsedItem{}, false
		}
		return parsedItem{kind: EventText, text: typed.Text}, true
	case *claudecode.ThinkingBlock:
		if typed.Thinking == "" {
			return unsupportedItem(UnsupportedUnknownBlock), true
		}
		return parsedItem{kind: EventContentBlock, block: &ContentBlock{
			Kind:     BlockThinking,
			Thinking: typed.Thinking,
		}}, true
	case *claudecode.ToolUseBlock:
		return m.mapToolUse(typed), true
	case *claudecode.ToolResultBlock:
		// SDK 目前只在 UserMessage 中投递 tool_result，这里按同一规则映射，
		// 避免投递位置变化时把正常结果误报为未知块。
		return m.mapToolResult(typed), true
	default:
		return unsupportedItem(UnsupportedUnknownBlock), true
	}
}

// mapToolUse 映射工具调用并记住 toolUseId → toolName。缺关联 ID、缺工具名或
// MCP 名残缺时降级，且不记录映射。
func (m *blockMapper) mapToolUse(block *claudecode.ToolUseBlock) parsedItem {
	if block.ToolUseID == "" || block.Name == "" {
		return unsupportedItem(UnsupportedUnknownBlock)
	}
	class, server, name, ok := splitToolName(block.Name)
	if !ok {
		return unsupportedItem(UnsupportedUnknownBlock)
	}
	m.toolNames[block.ToolUseID] = block.Name
	return parsedItem{kind: EventContentBlock, block: &ContentBlock{
		Kind:      BlockToolUse,
		ToolUseID: block.ToolUseID,
		ToolClass: class,
		MCPServer: server,
		ToolName:  name,
		Preview:   previewForTool(block.Name, block.Input),
	}}
}

func previewForTool(toolName string, input map[string]any) string {
	preview, ok := ToolPreview(toolName, input, DefaultToolPreviewMaxRunes)
	if !ok {
		return ""
	}
	return preview
}

// mapToolResult 只下发关联 ID 与成败状态，结果正文不进协议。
func (m *blockMapper) mapToolResult(block *claudecode.ToolResultBlock) parsedItem {
	if block.ToolUseID == "" {
		return unsupportedItem(UnsupportedUnknownBlock)
	}
	// SDK 省略 is_error 表示成功，协议要求该字段显式存在，因此补 false。
	isError := block.IsError != nil && *block.IsError
	// 结果修正（设计 §3.1）：AskUserQuestion 的回答以 is_error=true 返回，
	// 改为 false，避免手机把正常问答展示成失败。未见过对应 tool_use 的结果
	// 不做修正，也不猜测工具名。
	if isError && m.toolNames[block.ToolUseID] == askUserQuestionToolName {
		isError = false
	}
	return parsedItem{kind: EventContentBlock, block: &ContentBlock{
		Kind:      BlockToolResult,
		ToolUseID: block.ToolUseID,
		IsError:   &isError,
	}}
}

// splitToolName 解析工具名。不带 mcp__ 前缀的是内置工具；带前缀但不满足
// mcp__<server>__<tool> 三段形式的按普通工具处理并保留原始全名（设计 §3.1）；
// 三段形式但 server 或 tool 为空时无法安全展示，返回 false 交由调用方降级。
func splitToolName(name string) (ToolClass, string, string, bool) {
	if !strings.HasPrefix(name, mcpToolPrefix) {
		return ToolBuiltin, "", name, true
	}
	parts := strings.SplitN(name, "__", 3)
	if len(parts) != 3 {
		return ToolBuiltin, "", name, true
	}
	if parts[1] == "" || parts[2] == "" {
		return "", "", "", false
	}
	return ToolMCP, parts[1], parts[2], true
}

// assistantText 拼接 AssistantMessage 顶层文本块，用于错误描述提取。
func assistantText(message *claudecode.AssistantMessage) string {
	var builder strings.Builder
	for _, block := range message.Content {
		if text, ok := block.(*claudecode.TextBlock); ok {
			builder.WriteString(text.Text)
		}
	}
	return builder.String()
}
