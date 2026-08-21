package claude

import (
	"strings"

	claudecode "github.com/tea4go/claude-agent-sdk-go"
)

type resultFact struct {
	isError bool
	errors  []string
	text    string
}

// assistantErrorFact 记录带 Error 的 AssistantMessage：该消息终止当前流，
// 其 Content 中的文本即错误描述（可能为空，需回退到错误类型）。
type assistantErrorFact struct {
	errorType string
	text      string
}

type parsedMessage struct {
	sessionID    string
	source       SessionIDSource
	text         []string
	result       *resultFact
	assistantErr *assistantErrorFact
}

func parseMessage(message claudecode.Message) parsedMessage {
	switch msg := message.(type) {
	case *claudecode.SystemMessage:
		if msg.Subtype != "init" {
			return parsedMessage{}
		}
		sessionID, _ := msg.Data["session_id"].(string)
		return parsedMessage{sessionID: sessionID, source: SessionIDFromInit}
	case *claudecode.AssistantMessage:
		if msg.ParentToolUseID != nil {
			return parsedMessage{}
		}
		if msg.HasError() {
			return parsedMessage{assistantErr: &assistantErrorFact{
				errorType: string(msg.GetError()),
				text:      assistantText(msg),
			}}
		}
		parsed := parsedMessage{}
		for _, block := range msg.Content {
			if text, ok := block.(*claudecode.TextBlock); ok && text.Text != "" {
				parsed.text = append(parsed.text, text.Text)
			}
		}
		return parsed
	case *claudecode.ResultMessage:
		text := ""
		if msg.Result != nil {
			text = *msg.Result
		}
		return parsedMessage{
			sessionID: msg.SessionID,
			source:    SessionIDFromResult,
			result:    &resultFact{isError: msg.IsError, errors: msg.Errors, text: text},
		}
	default:
		return parsedMessage{}
	}
}

// assistantText 拼接 AssistantMessage 顶层文本块，用于错误描述提取。
func assistantText(message *claudecode.AssistantMessage) string {
	var builder strings.Builder
	for _, block := range message.Content {
		if text, ok := block.(*claudecode.TextBlock); ok {
			builder.WriteString(text.Text)
		}
	}
	return builder.String()
}
