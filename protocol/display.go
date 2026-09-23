package protocol

import (
	"strings"
	"unicode"
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
