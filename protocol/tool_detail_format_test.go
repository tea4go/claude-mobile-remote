package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestObservedToolDetails(t *testing.T) {
	tests := []struct {
		name, raw, args, metadata string
		input, output             string
	}{
		{"Read", "read successfully", `{"file_path":"/work/test.go"}`, `{"type":"text","file":{"filePath":"/work/test.go","content":"package main\n\t// keep whitespace"}}`, "file_path:\n/work/test.go", "package main\n\t// keep whitespace\n\nread successfully"},
		{"Edit", "updated successfully", `{"file_path":"/work/test.go"}`, `{"structuredPatch":[{"oldStart":1,"oldLines":1,"newStart":1,"newLines":1,"lines":["-old","+new"]}]}`, "file_path:\n/work/test.go", "@@ -1,1 +1,1 @@\n-old\n+new\n\n\nupdated successfully"},
		{"Write", "created successfully", `{"file_path":"/work/test.go"}`, `{"type":"create","content":"written text"}`, "file_path:\n/work/test.go", "written text\n\ncreated successfully"},
		{"Bash", "", `{"command":"printf '%s\\n' ok","description":"print test"}`, `{"stdout":"ok","stderr":"example warning"}`, `printf '%s\n' ok`, "ok\n\nexample warning"},
		{AskUserQuestionToolName, "", `{"questions":[{"question":"Pick a color","options":[{"label":"Blue","description":"Ocean"}]}]}`, `{"answers":{"Pick a color":"Blue"}}`, "Pick a color\n• Blue — Ocean", "Pick a color\nBlue\n\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var args, metadata map[string]any
			if err := json.Unmarshal([]byte(test.args), &args); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(test.metadata), &metadata); err != nil {
				t.Fatal(err)
			}
			if got := ToolCallDetail(test.name, args); got.Input != test.input || !got.Valid() {
				t.Fatalf("tool input = %#v", got)
			}
			preview, truncated, result := ToolResultWithMetadata(test.name, test.raw, metadata)
			if preview != test.raw || truncated || result.Output != test.output || !result.Valid() {
				t.Fatalf("tool result = %q, %v, %#v", preview, truncated, result)
			}
		})
	}
}

func TestObservedToolDetailBoundsAndUnselectedFields(t *testing.T) {
	metadata := map[string]any{"file": map[string]any{"content": strings.Repeat("正文", MaxToolResultBytes)}, "opaque": "excluded"}
	_, _, detail := ToolResultWithMetadata("Read", "execution completed", metadata)
	if !detail.Valid() || !detail.OutputTruncated || strings.Contains(detail.Output, "excluded") {
		t.Fatalf("unexpected bounded result: %d bytes, truncated = %v", len(detail.Output), detail.OutputTruncated)
	}
	_, _, detail = ToolResultWithMetadata("mcp__demo__read", nil, metadata)
	if detail.Valid() {
		t.Fatal("an arbitrary tool unexpectedly gained metadata detail")
	}
}
