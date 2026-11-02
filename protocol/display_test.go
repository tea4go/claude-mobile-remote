package protocol

import (
	"strings"
	"testing"
	"unicode/utf8"
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

func TestCurrentInputTextIsBoundedAndUTF8Safe(t *testing.T) {
	if got := CurrentInputText("普通提问"); got != "普通提问" {
		t.Fatalf("input = %q", got)
	}
	if got := CurrentInputText(string([]byte{0xff})); got != "" {
		t.Fatalf("invalid UTF-8 = %q", got)
	}
	got := CurrentInputText(strings.Repeat("界", MaxPromptBytes))
	if len(got) > MaxPromptBytes || !utf8.ValidString(got) || !strings.Contains(got, "余下内容请在电脑端查看") {
		t.Fatal("long input was not safely bounded")
	}
}
