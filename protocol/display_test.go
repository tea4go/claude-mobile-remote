package protocol

import (
	"strings"
	"testing"
)

func TestSessionTitleFromInput(t *testing.T) {
	if got := SessionTitleFromInput("\n \t\n  修复   登录\t错误\n第二行"); got != "修复 登录 错误" {
		t.Fatalf("title = %q", got)
	}
	if got := SessionTitleFromInput(strings.Repeat("界", 100)); len([]rune(got)) != MaxSessionTitleRunes {
		t.Fatalf("title length = %d", len([]rune(got)))
	}
	if got := SessionTitleFromInput("  \n\t"); got != "" {
		t.Fatalf("empty title = %q", got)
	}
}
