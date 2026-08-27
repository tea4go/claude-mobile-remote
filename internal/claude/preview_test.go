package claude

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestTruncatePreview 覆盖 B2 §3.1 截断的边界：按 rune 而非字节切分，恰好等于
// 上限时不加省略号，超出时结果长度恰为上限加一个省略号。
func TestTruncatePreview(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxRunes int
		want     string
	}{
		{name: "short ascii", text: "go test", maxRunes: 10, want: "go test"},
		{name: "exactly at limit", text: "abcde", maxRunes: 5, want: "abcde"},
		{name: "one rune over", text: "abcdef", maxRunes: 5, want: "abcde…"},
		{name: "multi byte runes", text: "查看当前改动", maxRunes: 3, want: "查看当…"},
		{name: "empty text", text: "", maxRunes: 5, want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := TruncatePreview(test.text, test.maxRunes); got != test.want {
				t.Fatalf("TruncatePreview(%q, %d) = %q, want %q", test.text, test.maxRunes, got, test.want)
			}
		})
	}
}

// TestTruncatePreviewFallsBackOnInvalidLimit 保证非法上限退回默认值，调用方
// 传入零值或负数时不会意外得到未截断的原文。
func TestTruncatePreviewFallsBackOnInvalidLimit(t *testing.T) {
	text := strings.Repeat("字", DefaultToolPreviewMaxRunes+50)
	for _, maxRunes := range []int{0, -1} {
		got := TruncatePreview(text, maxRunes)
		if utf8.RuneCountInString(got) != DefaultToolPreviewMaxRunes+1 {
			t.Fatalf("TruncatePreview(text, %d) kept %d runes, want %d plus an ellipsis",
				maxRunes, utf8.RuneCountInString(got), DefaultToolPreviewMaxRunes)
		}
		if !strings.HasSuffix(got, string(previewEllipsis)) {
			t.Fatalf("TruncatePreview(text, %d) must mark the truncation", maxRunes)
		}
	}
}

// TestTruncatePreviewNeverSplitsARune 保证截断结果始终是合法 UTF-8，避免端侧
// 解码到半个字符。
func TestTruncatePreviewNeverSplitsARune(t *testing.T) {
	text := "编辑 internal/session/model.go 并补齐字段"
	for maxRunes := 1; maxRunes <= utf8.RuneCountInString(text); maxRunes++ {
		if got := TruncatePreview(text, maxRunes); !utf8.ValidString(got) {
			t.Fatalf("TruncatePreview(text, %d) = %q is not valid UTF-8", maxRunes, got)
		}
	}
}

// TestToolPreview 覆盖 B2 §3.1 的工具审批预览白名单：每个工具各一例，Glob/Grep
// 带 path 时拼为 "<pattern> @ <path>"。抽取结果只是单个字符串，原始 input 不下发。
func TestToolPreview(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		input    map[string]any
		want     string
	}{
		{
			name:     "Bash uses command",
			toolName: "Bash",
			input:    map[string]any{"command": "go test ./...", "description": "run tests"},
			want:     "go test ./...",
		},
		{
			name:     "Read uses file_path",
			toolName: "Read",
			input:    map[string]any{"file_path": "/repo/main.go", "offset": 10},
			want:     "/repo/main.go",
		},
		{
			name:     "Write uses file_path",
			toolName: "Write",
			input:    map[string]any{"file_path": "/repo/new.go", "content": "package main"},
			want:     "/repo/new.go",
		},
		{
			name:     "Edit uses file_path",
			toolName: "Edit",
			input:    map[string]any{"file_path": "/repo/edit.go", "old_string": "a", "new_string": "b"},
			want:     "/repo/edit.go",
		},
		{
			name:     "NotebookEdit uses file_path",
			toolName: "NotebookEdit",
			input:    map[string]any{"file_path": "/repo/nb.ipynb"},
			want:     "/repo/nb.ipynb",
		},
		{
			name:     "Glob uses pattern",
			toolName: "Glob",
			input:    map[string]any{"pattern": "**/*.go"},
			want:     "**/*.go",
		},
		{
			name:     "Glob joins path",
			toolName: "Glob",
			input:    map[string]any{"pattern": "**/*.go", "path": "internal"},
			want:     "**/*.go @ internal",
		},
		{
			name:     "Grep uses pattern",
			toolName: "Grep",
			input:    map[string]any{"pattern": "TODO"},
			want:     "TODO",
		},
		{
			name:     "Grep joins path",
			toolName: "Grep",
			input:    map[string]any{"pattern": "TODO", "path": "cmd"},
			want:     "TODO @ cmd",
		},
		{
			name:     "Grep ignores empty path",
			toolName: "Grep",
			input:    map[string]any{"pattern": "TODO", "path": ""},
			want:     "TODO",
		},
		{
			name:     "Grep ignores non string path",
			toolName: "Grep",
			input:    map[string]any{"pattern": "TODO", "path": 7},
			want:     "TODO",
		},
		{
			name:     "WebFetch uses url",
			toolName: "WebFetch",
			input:    map[string]any{"url": "https://example.com/doc", "prompt": "summarise"},
			want:     "https://example.com/doc",
		},
		{
			name:     "WebSearch uses query",
			toolName: "WebSearch",
			input:    map[string]any{"query": "go generics"},
			want:     "go generics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ToolPreview(tt.toolName, tt.input, DefaultToolPreviewMaxRunes)
			if !ok || got != tt.want {
				t.Fatalf("ToolPreview(%q) = %q, %t, want %q, true", tt.toolName, got, ok, tt.want)
			}
		})
	}
}

// TestToolPreviewWithoutPreview 覆盖三种不下发 preview 的情况：工具名未命中白
// 名单、字段缺失、字段不是字符串。移动端此时只展示工具名，服务端不做通用序列化。
func TestToolPreviewWithoutPreview(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		input    map[string]any
	}{
		{name: "unknown tool", toolName: "TodoWrite", input: map[string]any{"todos": "x"}},
		{name: "mcp tool", toolName: "mcp__filesystem__read_file", input: map[string]any{"path": "/tmp"}},
		{name: "empty tool name", toolName: "", input: map[string]any{"command": "ls"}},
		{name: "missing field", toolName: "Bash", input: map[string]any{"description": "run"}},
		{name: "nil input", toolName: "Bash", input: nil},
		{name: "non string field", toolName: "Read", input: map[string]any{"file_path": 42}},
		{name: "nested object field", toolName: "WebFetch", input: map[string]any{"url": map[string]any{"raw": "x"}}},
		{name: "blank field", toolName: "Bash", input: map[string]any{"command": "   "}},
		{name: "glob without pattern", toolName: "Glob", input: map[string]any{"path": "internal"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, ok := ToolPreview(tt.toolName, tt.input, DefaultToolPreviewMaxRunes); ok || got != "" {
				t.Fatalf("ToolPreview(%q) = %q, %t, want \"\", false", tt.toolName, got, ok)
			}
		})
	}
}

// TestToolPreviewTruncatesByRune 保证超长输入按 rune 边界截断并补省略号，结果仍
// 是合法 UTF-8，且上限来自调用方传入的配置而非散落常量。
func TestToolPreviewTruncatesByRune(t *testing.T) {
	command := strings.Repeat("查", 40)
	got, ok := ToolPreview("Bash", map[string]any{"command": command}, 10)
	if !ok {
		t.Fatal("ToolPreview() must still return a preview for an over-long command")
	}
	if utf8.RuneCountInString(got) != 11 {
		t.Fatalf("ToolPreview() kept %d runes, want 10 plus an ellipsis", utf8.RuneCountInString(got))
	}
	if !utf8.ValidString(got) {
		t.Fatalf("ToolPreview() = %q is not valid UTF-8", got)
	}
	if !strings.HasSuffix(got, string(previewEllipsis)) {
		t.Fatalf("ToolPreview() = %q must mark the truncation", got)
	}

	// 拼接后的 Glob 预览同样按同一上限截断。
	joined, ok := ToolPreview("Glob", map[string]any{"pattern": "**/*.go", "path": strings.Repeat("d", 100)}, 12)
	if !ok || utf8.RuneCountInString(joined) != 13 {
		t.Fatalf("ToolPreview(Glob) = %q, %t, want a 12 rune preview plus an ellipsis", joined, ok)
	}
}
