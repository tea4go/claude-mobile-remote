package protocol

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestToolTitleSummary(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input map[string]any
		want  string
	}{
		{"Bash", map[string]any{"description": "run\n tests", "command": "go test"}, "run tests"},
		{"Bash", map[string]any{"description": " ", "command": "go test"}, "go test"},
		{"NotebookEdit", map[string]any{"notebook_path": "demo.ipynb"}, "demo.ipynb"},
		{"TaskCreate", map[string]any{"subject": "检查"}, "检查"},
		{"Read", map[string]any{"file_path": "文件", "content": "body"}, "文件"},
		{"TodoWrite", map[string]any{"todos": []any{"one", "two"}}, "(2 items)"},
		{"unknown", map[string]any{"description": "hidden"}, ""},
	} {
		if got := ToolTitleSummary(tt.name, tt.input); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
	long := ToolTitleSummary("Agent", map[string]any{"description": strings.Repeat("字", 200)})
	if !ValidToolTitleSummary(long) || utf8.RuneCountInString(long) != MaxToolTitleRunes || !strings.HasSuffix(long, "…") {
		t.Fatal("invalid truncation")
	}
}
