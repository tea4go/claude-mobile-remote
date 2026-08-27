package claude

import (
	"errors"
	"testing"
)

func TestScopeOf(t *testing.T) {
	root := errors.New("root")
	turnErr := wrapError(ScopeTurn, "run turn", root)
	if got := ScopeOf(turnErr); got != ScopeTurn {
		t.Fatalf("ScopeOf(turnErr) = %q, want %q", got, ScopeTurn)
	}
	if !errors.Is(turnErr, root) {
		t.Fatal("wrapped error must preserve its cause")
	}
	if got := ScopeOf(root); got != ScopeSession {
		t.Fatalf("unknown errors must default to session scope, got %q", got)
	}
}

// TestStructuredEventValues 固定 B2 §4.2 结构化事件的字符串取值。session 包依赖
// claude 包，claude 不能反向 import，因此两侧同名枚举只能靠各自的取值测试保持
// 一致；任何一侧改动都会在这里或 session/model_test.go 暴露。
func TestStructuredEventValues(t *testing.T) {
	if EventContentBlock != "content_block" || EventUnsupported != "unsupported" {
		t.Fatalf("unexpected structured event kinds")
	}
	if BlockThinking != "thinking" || BlockToolUse != "tool_use" || BlockToolResult != "tool_result" {
		t.Fatalf("unexpected block kind values")
	}
	if ToolBuiltin != "builtin" || ToolMCP != "mcp" {
		t.Fatalf("unexpected tool class values")
	}
	if UnsupportedUnknownBlock != "unknown_block" || UnsupportedUnknownSubtype != "unknown_subtype" {
		t.Fatalf("unexpected unsupported code values")
	}
	if askUserQuestionToolName != "AskUserQuestion" {
		t.Fatalf("unexpected AskUserQuestion tool name")
	}
}
