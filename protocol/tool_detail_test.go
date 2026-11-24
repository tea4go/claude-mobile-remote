package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestToolDetailsMergeAndDetach(t *testing.T) {
	var store ToolDetails
	input := ToolInputDetail(map[string]any{
		"file_path": "/project/main.go", "old_string": "old", "new_string": "new", "token": "ignored-fixture",
	})
	call := &ContentBlock{Kind: BlockToolUse, ToolUseID: "tool", Detail: &input}
	summary := store.Record("turn", call)
	if !summary.DetailAvailable || summary.Detail != nil || call.Detail == nil {
		t.Fatalf("wrong summary: %#v", summary)
	}
	wire, err := json.Marshal(summary)
	if err != nil || strings.Contains(string(wire), "/project/main.go") {
		t.Fatalf("detail leaked into summary: %s, %v", wire, err)
	}
	legacy, legacyTruncated, result := ToolResultDisplay(strings.Repeat("文", MaxToolResultBytes/3+30))
	if len(legacy) > MaxToolOutputBytes || !legacyTruncated || !result.OutputTruncated || len(result.Output) != MaxToolResultBytes || !utf8.ValidString(result.Output) {
		t.Fatalf("unexpected preview/detail bounds: %d, %d", len(legacy), len(result.Output))
	}
	store.Record("turn", &ContentBlock{Kind: BlockToolResult, ToolUseID: "tool", Detail: &result})
	detail, ok := store.Get("turn", "tool")
	if !ok || detail.Input != "file_path:\n/project/main.go\n\nold_string:\nold\n\nnew_string:\nnew" || detail.Output != result.Output || !detail.OutputTruncated {
		t.Fatal("call and result were not preserved together")
	}
	for _, other := range []struct{ request, tool string }{{"other-turn", "tool"}, {"turn", "/project/main.go"}} {
		if _, ok := store.Get(other.request, other.tool); ok {
			t.Fatalf("unexpected reference matched: %+v", other)
		}
	}
}

func TestToolDetailsEvictsOldestWithinBudget(t *testing.T) {
	var store ToolDetails
	detail := ToolDetail{Input: strings.Repeat("x", MaxToolInputBytes), Output: strings.Repeat("y", MaxToolResultBytes)}
	last := maxToolDetailBytes / (MaxToolInputBytes + MaxToolResultBytes)
	for i := 0; i <= last; i++ {
		store.Record("turn", &ContentBlock{Kind: BlockToolUse, ToolUseID: fmt.Sprint(i), Detail: &detail})
	}
	if _, ok := store.Get("turn", "0"); ok || store.bytes > maxToolDetailBytes {
		t.Fatal("oldest detail survived byte limit")
	}
	if _, ok := store.Get("turn", fmt.Sprint(last)); !ok {
		t.Fatal("latest detail was evicted")
	}
	var small ToolDetails
	for i := 0; i <= maxToolDetails; i++ {
		small.Record("turn", &ContentBlock{Kind: BlockToolUse, ToolUseID: fmt.Sprint(i), Detail: &ToolDetail{Input: "sample"}})
	}
	if _, ok := small.Get("turn", "0"); ok || len(small.items) != maxToolDetails {
		t.Fatal("oldest detail survived entry limit")
	}
}
