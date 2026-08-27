package claude

import (
	"reflect"
	"testing"

	claudecode "github.com/tea4go/claude-agent-sdk-go"
)

func stringPointer(value string) *string { return &value }

func boolPointer(value bool) *bool { return &value }

func errorPointer(value claudecode.AssistantMessageError) *claudecode.AssistantMessageError {
	return &value
}

// unknownBlock 模拟 SDK 未来新增或本项目尚未验证的内容块类型，用于断言降级
// 路径不透传任何原始数据。
type unknownBlock struct{}

func (unknownBlock) BlockType() string { return "future_block" }

func TestParseMessage(t *testing.T) {
	tests := []struct {
		name string
		msg  claudecode.Message
		want parsedMessage
	}{
		{
			name: "init session",
			msg:  &claudecode.SystemMessage{Subtype: "init", Data: map[string]any{"session_id": "session-1"}},
			want: parsedMessage{sessionID: "session-1", source: SessionIDFromInit},
		},
		{
			// 真实 Claude 每轮会发非 init 的 SystemMessage（如 compact_boundary/
			// api_retry）；这些不是待处理事项，静默忽略，不产生任何上报条目
			// （参考实现 whale-term 对非 init subtype continue）。
			name: "non-init system is silently ignored",
			msg:  &claudecode.SystemMessage{Subtype: "status", Data: map[string]any{"session_id": "wrong"}},
			want: parsedMessage{},
		},
		{
			name: "top-level assistant text keeps sdk order",
			msg: &claudecode.AssistantMessage{Content: []claudecode.ContentBlock{
				&claudecode.ThinkingBlock{Thinking: "weighing options"},
				&claudecode.TextBlock{Text: "hello"},
			}},
			want: parsedMessage{items: []parsedItem{
				{kind: EventContentBlock, block: &ContentBlock{Kind: BlockThinking, Thinking: "weighing options"}},
				{kind: EventText, text: "hello"},
			}},
		},
		{
			name: "subagent assistant ignored",
			msg: &claudecode.AssistantMessage{
				ParentToolUseID: stringPointer("tool-1"),
				Content:         []claudecode.ContentBlock{&claudecode.TextBlock{Text: "hidden"}},
			},
			want: parsedMessage{},
		},
		{
			name: "result",
			msg:  &claudecode.ResultMessage{SessionID: "session-1", Result: stringPointer("fallback")},
			want: parsedMessage{
				sessionID: "session-1",
				source:    SessionIDFromResult,
				result:    &resultFact{text: "fallback"},
			},
		},
		{
			name: "assistant error with text",
			msg: &claudecode.AssistantMessage{
				Error:   errorPointer(claudecode.AssistantMessageErrorServer),
				Content: []claudecode.ContentBlock{&claudecode.TextBlock{Text: "overloaded"}},
			},
			want: parsedMessage{assistantErr: &assistantErrorFact{
				errorType: "server_error",
				text:      "overloaded",
			}},
		},
		{
			name: "assistant error without text",
			msg: &claudecode.AssistantMessage{
				Error: errorPointer(claudecode.AssistantMessageErrorAuthFailed),
			},
			want: parsedMessage{assistantErr: &assistantErrorFact{
				errorType: "authentication_failed",
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newBlockMapper().parse(tt.msg); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parse() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// TestBlockMapperMapsToolBlocks 覆盖 B2 §4.2 的三类结构化内容块：内置工具、MCP
// 工具（从 mcp__<server>__<tool> 解析出 toolClass 与 mcpServer）与工具结果。
// Task 工具刻意按普通内置工具输出，不带任何任务字段。
func TestBlockMapperMapsToolBlocks(t *testing.T) {
	tests := []struct {
		name string
		msg  claudecode.Message
		want []parsedItem
	}{
		{
			name: "builtin tool use",
			msg: &claudecode.AssistantMessage{Content: []claudecode.ContentBlock{
				&claudecode.ToolUseBlock{
					ToolUseID: "toolu_1",
					Name:      "Read",
					Input:     map[string]any{"file_path": "/tmp/a.go"},
				},
			}},
			want: []parsedItem{{kind: EventContentBlock, block: &ContentBlock{
				Kind:      BlockToolUse,
				ToolUseID: "toolu_1",
				ToolClass: ToolBuiltin,
				ToolName:  "Read",
				Preview:   "/tmp/a.go",
			}}},
		},
		{
			name: "task tool is an ordinary builtin tool",
			msg: &claudecode.AssistantMessage{Content: []claudecode.ContentBlock{
				&claudecode.ToolUseBlock{ToolUseID: "toolu_2", Name: "Task", Input: map[string]any{}},
			}},
			want: []parsedItem{{kind: EventContentBlock, block: &ContentBlock{
				Kind:      BlockToolUse,
				ToolUseID: "toolu_2",
				ToolClass: ToolBuiltin,
				ToolName:  "Task",
			}}},
		},
		{
			name: "mcp tool use",
			msg: &claudecode.AssistantMessage{Content: []claudecode.ContentBlock{
				&claudecode.ToolUseBlock{ToolUseID: "toolu_3", Name: "mcp__filesystem__read_file"},
			}},
			want: []parsedItem{{kind: EventContentBlock, block: &ContentBlock{
				Kind:      BlockToolUse,
				ToolUseID: "toolu_3",
				ToolClass: ToolMCP,
				MCPServer: "filesystem",
				ToolName:  "read_file",
			}}},
		},
		{
			// mcp__ 前缀存在但不满足 mcp__<server>__<tool> 格式时按普通工具处理
			// （设计 §3.1「前缀匹配失败时按普通工具走审批链路」）。
			name: "malformed mcp name falls back to builtin",
			msg: &claudecode.AssistantMessage{Content: []claudecode.ContentBlock{
				&claudecode.ToolUseBlock{ToolUseID: "toolu_4", Name: "mcp__onlyserver"},
			}},
			want: []parsedItem{{kind: EventContentBlock, block: &ContentBlock{
				Kind:      BlockToolUse,
				ToolUseID: "toolu_4",
				ToolClass: ToolBuiltin,
				ToolName:  "mcp__onlyserver",
			}}},
		},
		{
			// 工具结果只下发关联 ID 与成败状态，正文不进协议（§4.2）。
			name: "tool result keeps only id and status",
			msg: &claudecode.UserMessage{Content: []claudecode.ContentBlock{
				&claudecode.ToolResultBlock{
					ToolUseID: "toolu_1",
					Content:   "file contents that must not be forwarded",
					IsError:   boolPointer(true),
				},
			}},
			want: []parsedItem{{kind: EventContentBlock, block: &ContentBlock{
				Kind:      BlockToolResult,
				ToolUseID: "toolu_1",
				IsError:   boolPointer(true),
			}}},
		},
		{
			// SDK 省略 is_error 表示成功；协议要求该字段显式存在，因此补 false。
			name: "absent is_error becomes false",
			msg: &claudecode.UserMessage{Content: []claudecode.ContentBlock{
				&claudecode.ToolResultBlock{ToolUseID: "toolu_1", Content: "ok"},
			}},
			want: []parsedItem{{kind: EventContentBlock, block: &ContentBlock{
				Kind:      BlockToolResult,
				ToolUseID: "toolu_1",
				IsError:   boolPointer(false),
			}}},
		},
		{
			// UserMessage 中的其他块（含用户自己的文本）不产生任何事件。
			name: "user text blocks are ignored",
			msg: &claudecode.UserMessage{Content: []claudecode.ContentBlock{
				&claudecode.TextBlock{Text: "the prompt echoed back"},
			}},
			want: nil,
		},
		{
			name: "string user content is ignored",
			msg:  &claudecode.UserMessage{Content: "plain prompt"},
			want: nil,
		},
		{
			name: "subagent tool result ignored",
			msg: &claudecode.UserMessage{
				ParentToolUseID: stringPointer("toolu_task"),
				Content: []claudecode.ContentBlock{
					&claudecode.ToolResultBlock{ToolUseID: "toolu_child", Content: "hidden"},
				},
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newBlockMapper().parse(tt.msg).items; !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parse().items = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// TestBlockMapperCorrectsAskUserQuestionResult 覆盖设计 §3.1「结果修正」：交互类
// 工具的回答会以 is_error=true 返回，Adapter 按 toolUseId → toolName 判定后改为
// false，避免手机把正常问答显示成失败。
func TestBlockMapperCorrectsAskUserQuestionResult(t *testing.T) {
	mapper := newBlockMapper()
	mapper.parse(&claudecode.AssistantMessage{Content: []claudecode.ContentBlock{
		&claudecode.ToolUseBlock{ToolUseID: "toolu_ask", Name: askUserQuestionToolName},
		&claudecode.ToolUseBlock{ToolUseID: "toolu_bash", Name: "Bash"},
	}})

	corrected := mapper.parse(&claudecode.UserMessage{Content: []claudecode.ContentBlock{
		&claudecode.ToolResultBlock{ToolUseID: "toolu_ask", IsError: boolPointer(true)},
	}})
	want := []parsedItem{{kind: EventContentBlock, block: &ContentBlock{
		Kind:      BlockToolResult,
		ToolUseID: "toolu_ask",
		IsError:   boolPointer(false),
	}}}
	if !reflect.DeepEqual(corrected.items, want) {
		t.Fatalf("AskUserQuestion result = %#v, want is_error false", corrected.items)
	}

	// 其他工具的真实失败必须保留。
	kept := mapper.parse(&claudecode.UserMessage{Content: []claudecode.ContentBlock{
		&claudecode.ToolResultBlock{ToolUseID: "toolu_bash", IsError: boolPointer(true)},
	}})
	if len(kept.items) != 1 || kept.items[0].block.IsError == nil || !*kept.items[0].block.IsError {
		t.Fatalf("Bash failure = %#v, want is_error true", kept.items)
	}

	// 未见过对应 tool_use 的结果不做修正，也不猜测工具名。
	unknown := mapper.parse(&claudecode.UserMessage{Content: []claudecode.ContentBlock{
		&claudecode.ToolResultBlock{ToolUseID: "toolu_missing", IsError: boolPointer(true)},
	}})
	if len(unknown.items) != 1 || !*unknown.items[0].block.IsError {
		t.Fatalf("unknown tool result = %#v, want is_error preserved", unknown.items)
	}
}

// TestBlockMapperDegradesIncompleteBlocks 覆盖 §4.2 最后一条：未知或不完整的
// SDK block 转成固定 code 的安全事件，不把原始对象序列化给客户端。
func TestBlockMapperDegradesIncompleteBlocks(t *testing.T) {
	tests := []struct {
		name  string
		block claudecode.ContentBlock
	}{
		{name: "unknown block type", block: unknownBlock{}},
		{name: "empty thinking", block: &claudecode.ThinkingBlock{}},
		{name: "tool use without id", block: &claudecode.ToolUseBlock{Name: "Read"}},
		{name: "tool use without name", block: &claudecode.ToolUseBlock{ToolUseID: "toolu_1"}},
		{name: "mcp name without tool part", block: &claudecode.ToolUseBlock{ToolUseID: "toolu_1", Name: "mcp__server__"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newBlockMapper().parse(&claudecode.AssistantMessage{
				Content: []claudecode.ContentBlock{tt.block},
			}).items
			want := []parsedItem{{kind: EventUnsupported, code: UnsupportedUnknownBlock}}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("parse().items = %#v, want a single unknown_block event", got)
			}
		})
	}

	// tool_result 缺少关联 ID 同样降级，且降级事件不携带任何原始字段。
	got := newBlockMapper().parse(&claudecode.UserMessage{Content: []claudecode.ContentBlock{
		&claudecode.ToolResultBlock{Content: "orphan result"},
	}}).items
	want := []parsedItem{{kind: EventUnsupported, code: UnsupportedUnknownBlock}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("orphan tool_result = %#v, want a single unknown_block event", got)
	}
}
