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
	var text string
	if value, ok := content.(string); ok {
		text = value
	} else {
		data, err := json.Marshal(content)
		if err != nil {
			return "", false
		}
		var blocks []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal(data, &blocks) != nil {
			return "", false
		}
		var parts []string
		for _, block := range blocks {
			if block.Type == "text" && block.Text != "" {
				parts = append(parts, block.Text)
			}
		}
		text = strings.Join(parts, "\n")
	}
	if len(text) <= MaxToolOutputBytes {
		return text, false
	}
	text = text[:MaxToolOutputBytes]
	for !utf8.ValidString(text) {
		text = text[:len(text)-1]
	}
	return text, true
}
