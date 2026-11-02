package protocol

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxSessionTitleRunes = 80

// SessionTitleFromInput keeps one bounded plain-text line of the first input.
// It does not retain the full prompt or emit control characters.
func SessionTitleFromInput(input string) string {
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var title []rune
		space := false
		for _, char := range line {
			if unicode.IsControl(char) || unicode.IsSpace(char) {
				space = len(title) > 0
				continue
			}
			if space {
				if len(title)+1 >= MaxSessionTitleRunes {
					break
				}
				title = append(title, ' ')
			}
			space = false
			if len(title) == MaxSessionTitleRunes {
				break
			}
			title = append(title, char)
		}
		if len(title) > 0 {
			return string(title)
		}
	}
	return ""
}

// CurrentInputText bounds a user-visible input without exposing execution paths.
func CurrentInputText(text string) string {
	if !utf8.ValidString(text) {
		return ""
	}
	if len(text) <= MaxPromptBytes {
		return text
	}
	const suffix = "\n[提问过长，余下内容请在电脑端查看]"
	text = text[:MaxPromptBytes-len(suffix)]
	for !utf8.ValidString(text) {
		text = text[:len(text)-1]
	}
	return text + suffix
}
