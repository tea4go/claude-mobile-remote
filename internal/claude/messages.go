package claude

import (
	"strings"

	claudecode "github.com/tea4go/claude-agent-sdk-go"
	"github.com/tea4go/claude-mobile-remote/protocol"
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
	model string
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
		if parent := msg.GetParentToolUseID(); parent != "" {
			return m.parseChildTools(parent, msg.Content, nil)
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
				if item.kind == EventText && protocol.ValidReplyModel(msg.Model) {
					item.model = msg.Model
				}
				parsed.items = append(parsed.items, item)
			}
		}
		return parsed
	case *claudecode.UserMessage:
		// UserMessage 的 Content 可能是纯文本（回显的 prompt），也可能是块
		// 数组。只有其中的 tool_result 对手机有意义，其余块全部忽略。
		blocks, ok := msg.Content.([]claudecode.ContentBlock)
		if !ok {
			return parsedMessage{}
		}
		var metadata map[string]any
		for _, block := range blocks {
			if _, ok := block.(*claudecode.ToolResultBlock); ok {
				if metadata != nil {
					metadata = nil
					break
				}
				metadata = msg.GetToolUseResult()
			}
		}
		if parent := msg.GetParentToolUseID(); parent != "" {
			return m.parseChildTools(parent, blocks, metadata)
		}
		parsed := parsedMessage{}
		for _, block := range blocks {
			result, isResult := block.(*claudecode.ToolResultBlock)
			if !isResult {
				continue
			}
			parsed.items = append(parsed.items, m.mapToolResult(result, metadata))
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

func (m *blockMapper) parseChildTools(parent string, blocks []claudecode.ContentBlock, metadata map[string]any) parsedMessage {
	var parsed parsedMessage
	if !protocol.ValidToolDetailID(parent) {
		return parsed
	}
	for _, block := range blocks {
		var item parsedItem
		switch tool := block.(type) {
		case *claudecode.ToolUseBlock:
			item = m.mapToolUse(tool)
		case *claudecode.ToolResultBlock:
			item = m.mapToolResult(tool, metadata)
		}
		if item.block != nil && protocol.ValidToolDetailID(item.block.ToolUseID) && item.block.ToolUseID != parent {
			item.block.ParentToolUseID = parent
			parsed.items = append(parsed.items, item)
		}
	}
	return parsed
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
		return m.mapToolResult(typed, nil), true
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
	detail := protocol.ToolCallDetail(block.Name, block.Input)
	var availableDetail *protocol.ToolDetail
	if detail.Valid() {
		availableDetail = &detail
	}
	preview := previewForTool(block.Name, block.Input)
	if block.Name == "Agent" || block.Name == "Task" {
		if description, _ := block.Input["description"].(string); description != "" {
			preview = protocol.TruncatePreview(description, DefaultToolPreviewMaxRunes)
		}
	}
	return parsedItem{kind: EventContentBlock, block: &ContentBlock{
		Kind:         BlockToolUse,
		ToolUseID:    block.ToolUseID,
		ToolClass:    class,
		MCPServer:    server,
		ToolName:     name,
		Preview:      preview,
		TitleSummary: protocol.ToolTitleSummary(block.Name, block.Input),
		Detail:       availableDetail,
	}}
}

func previewForTool(toolName string, input map[string]any) string {
	preview, ok := ToolPreview(toolName, input, DefaultToolPreviewMaxRunes)
	if !ok {
		return ""
	}
	return preview
}

// mapToolResult 下发关联 ID、成败状态和有界纯文本结果（v4）。
func (m *blockMapper) mapToolResult(block *claudecode.ToolResultBlock, metadata map[string]any) parsedItem {
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
	output, truncated, detail := protocol.ToolResultWithMetadata(m.toolNames[block.ToolUseID], block.Content, metadata)
	var availableDetail *protocol.ToolDetail
	if detail.Valid() {
		availableDetail = &detail
	}
	return parsedItem{kind: EventContentBlock, block: &ContentBlock{
		Output: output, Truncated: truncated,
		Kind:      BlockToolResult,
		ToolUseID: block.ToolUseID,
		IsError:   &isError,
		Detail:    availableDetail,
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
