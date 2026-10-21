package protocol

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestToolOutputTextOnlyAndBounded(t *testing.T) {
	output, clipped := ToolOutput([]any{map[string]any{"type": "text", "text": "hello"}, map[string]any{"type": "image", "data": "private-binary"}})
	if output != "hello" || clipped {
		t.Fatalf("text = %q, clipped %v", output, clipped)
	}
	output, clipped = ToolOutput(strings.Repeat("中", MaxToolOutputBytes))
	if !clipped || len(output) > MaxToolOutputBytes || !utf8.ValidString(output) {
		t.Fatal("bad UTF-8 truncation")
	}
	output, _ = ToolOutput(map[string]any{"secret": "not a text block"})
	if output != "" {
		t.Fatal("exposed unsupported content")
	}
}
