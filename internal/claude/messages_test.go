package claude

import (
	"reflect"
	"testing"

	claudecode "github.com/tea4go/claude-agent-sdk-go"
)

func stringPointer(value string) *string { return &value }

func errorPointer(value claudecode.AssistantMessageError) *claudecode.AssistantMessageError {
	return &value
}

func TestParseMessage(t *testing.T) {
	tests := []struct {
		name string
		msg  claudecode.Message
		want parsedMessage
	}{
		{
			name: "init session",
			msg:  &claudecode.SystemMessage{Subtype: "init", Data: map[string]any{"session_id": "session-1"}},
			want: parsedMessage{sessionID: "session-1", source: SessionIDFromInit},
		},
		{
			name: "non-init system ignored",
			msg:  &claudecode.SystemMessage{Subtype: "status", Data: map[string]any{"session_id": "wrong"}},
			want: parsedMessage{},
		},
		{
			name: "top-level assistant text only",
			msg: &claudecode.AssistantMessage{Content: []claudecode.ContentBlock{
				&claudecode.TextBlock{Text: "hello"},
				&claudecode.ThinkingBlock{},
			}},
			want: parsedMessage{text: []string{"hello"}},
		},
		{
			name: "subagent assistant ignored",
			msg: &claudecode.AssistantMessage{
				ParentToolUseID: stringPointer("tool-1"),
				Content:         []claudecode.ContentBlock{&claudecode.TextBlock{Text: "hidden"}},
			},
			want: parsedMessage{},
		},
		{
			name: "result",
			msg:  &claudecode.ResultMessage{SessionID: "session-1", Result: stringPointer("fallback")},
			want: parsedMessage{
				sessionID: "session-1",
				source:    SessionIDFromResult,
				result:    &resultFact{text: "fallback"},
			},
		},
		{
			name: "assistant error with text",
			msg: &claudecode.AssistantMessage{
				Error:   errorPointer(claudecode.AssistantMessageErrorServer),
				Content: []claudecode.ContentBlock{&claudecode.TextBlock{Text: "overloaded"}},
			},
			want: parsedMessage{assistantErr: &assistantErrorFact{
				errorType: "server_error",
				text:      "overloaded",
			}},
		},
		{
			name: "assistant error without text",
			msg: &claudecode.AssistantMessage{
				Error: errorPointer(claudecode.AssistantMessageErrorAuthFailed),
			},
			want: parsedMessage{assistantErr: &assistantErrorFact{
				errorType: "authentication_failed",
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseMessage(tt.msg); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseMessage() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
