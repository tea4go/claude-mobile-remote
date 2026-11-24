package protocol

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

const MaxToolOutputBytes = 16 * 1024

// ToolOutput exposes text only: image/resource payloads and arbitrary objects
// never reach the display contract. The SDK owns interpretation of tool output.
func ToolOutput(content any) (string, bool) {
	return clipToolText(toolResultText(content), MaxToolOutputBytes)
}

// ToolResultDisplay produces the legacy event preview and the independently
// bounded detail served to newer clients on demand.
func ToolResultDisplay(content any) (string, bool, ToolDetail) {
	text := toolResultText(content)
	preview, previewTruncated := clipToolText(text, MaxToolOutputBytes)
	output, outputTruncated := clipToolText(text, MaxToolResultBytes)
	return preview, previewTruncated, ToolDetail{Output: output, OutputTruncated: outputTruncated}
}

func toolResultText(content any) string {
	var text string
	if value, ok := content.(string); ok {
		text = value
	} else {
		data, err := json.Marshal(content)
		if err != nil {
			return ""
		}
		var blocks []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal(data, &blocks) != nil {
			return ""
		}
		var parts []string
		for _, block := range blocks {
			if block.Type == "text" && block.Text != "" {
				parts = append(parts, block.Text)
			}
		}
		text = strings.Join(parts, "\n")
	}
	return text
}

func clipToolText(text string, maxBytes int) (string, bool) {
	if len(text) <= maxBytes {
		return strings.ToValidUTF8(text, ""), false
	}
	for maxBytes > 0 && !utf8.RuneStart(text[maxBytes]) {
		maxBytes--
	}
	return strings.Clone(strings.ToValidUTF8(text[:maxBytes], "")), true
}
