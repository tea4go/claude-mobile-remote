package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	projectsession "github.com/tea4go/claude-mobile-remote/internal/session"
	"github.com/tea4go/claude-mobile-remote/internal/transport"
)

var (
	errUnknownLineCommand = errors.New("unknown line command")
	errLineCommandUsage   = errors.New("invalid line command usage")
	errNoPendingAction    = errors.New("no action is pending")
	errNoTargetRequest    = errors.New("no target request")
)

// clientState 只保存行命令需要的隐式目标：当前待处理 action 和最近看到的
// requestId。完整上下文留在 PC，客户端不重建历史。
type clientState struct {
	actionID  string
	requestID string
}

func (s *clientState) observeSnapshot(snapshot projectsession.Snapshot) {
	s.requestID = snapshot.Turn.RequestID
	s.actionID = ""
	if snapshot.Turn.PendingAction != nil {
		s.actionID = snapshot.Turn.PendingAction.ActionID
	}
}

func (s *clientState) observeEvent(event projectsession.Event) {
	if event.RequestID != "" {
		s.requestID = event.RequestID
	}
	switch event.Kind {
	case projectsession.EventActionRequired:
		if event.PendingAction != nil {
			s.actionID = event.PendingAction.ActionID
		}
	case projectsession.EventActionResolved:
		// action 已被解决后立刻失效，避免后续行命令引用一个陈旧 actionId。
		s.actionID = ""
	}
}

// printPendingAction 回显答复一个待处理 action 所需的标识符与可选项，让命令行
// 能拼出合法的 :allow / :answer。
func printPendingAction(output io.Writer, action projectsession.PendingAction) {
	fmt.Fprintf(output, "action pending actionId=%s kind=%s allowSession=%t\n",
		action.ActionID, action.Kind, action.AllowSessionAvailable)
	for _, question := range action.Questions {
		labels := make([]string, 0, len(question.Options))
		for _, option := range question.Options {
			labels = append(labels, option.Label)
		}
		fmt.Fprintf(output, "  question %s multiSelect=%t options=%s\n",
			question.QuestionID, question.MultiSelect, strings.Join(labels, "|"))
	}
}

// printContentBlock 回显结构化块的标识与标题字段。联调靠它确认 block ID 由
// Controller 分配且真的落到线缆上：出站事件目前不经过 ValidateEnvelope。
func printContentBlock(output io.Writer, block projectsession.ContentBlock) {
	switch block.Kind {
	case projectsession.BlockThinking:
		fmt.Fprintf(output, "  block %s thinking=%d runes\n", block.ID, len([]rune(block.Thinking)))
	case projectsession.BlockToolUse:
		fmt.Fprintf(output, "  block %s tool_use=%s class=%s toolUseId=%s\n",
			block.ID, block.ToolName, block.ToolClass, block.ToolUseID)
	case projectsession.BlockToolResult:
		isError := block.IsError != nil && *block.IsError
		fmt.Fprintf(output, "  block %s tool_result toolUseId=%s isError=%t\n", block.ID, block.ToolUseID, isError)
	default:
		fmt.Fprintf(output, "  block %s unknown kind=%s\n", block.ID, block.Kind)
	}
}

// buildCommand 把一行输入翻译成一条命令：以 ':' 开头的是控制命令，其余按文本
// 输入发送。它只做本地形状检查，最终判定仍由服务端与 Controller 负责。
func buildCommand(state clientState, line string) (transport.CommandPayload, error) {
	if !strings.HasPrefix(line, ":") {
		return transport.CommandPayload{Kind: transport.CommandSendText, Text: line}, nil
	}
	fields := strings.Fields(line)
	switch fields[0] {
	case ":allow", ":allow-session", ":deny":
		if len(fields) != 1 {
			return transport.CommandPayload{}, errLineCommandUsage
		}
		if state.actionID == "" {
			return transport.CommandPayload{}, errNoPendingAction
		}
		decisions := map[string]transport.ActionDecision{
			":allow":         transport.DecisionAllowOnce,
			":allow-session": transport.DecisionAllowSession,
			":deny":          transport.DecisionDeny,
		}
		return transport.CommandPayload{
			Kind:     transport.CommandResolveAction,
			ActionID: state.actionID,
			Decision: decisions[fields[0]],
		}, nil
	case ":answer":
		if state.actionID == "" {
			return transport.CommandPayload{}, errNoPendingAction
		}
		answers, err := parseAnswers(strings.TrimPrefix(line, fields[0]))
		if err != nil {
			return transport.CommandPayload{}, err
		}
		return transport.CommandPayload{
			Kind:     transport.CommandResolveAction,
			ActionID: state.actionID,
			Answers:  answers,
		}, nil
	case ":cancel-turn":
		if len(fields) > 2 {
			return transport.CommandPayload{}, errLineCommandUsage
		}
		requestID := state.requestID
		if len(fields) == 2 {
			requestID = fields[1]
		}
		if requestID == "" {
			return transport.CommandPayload{}, errNoTargetRequest
		}
		return transport.CommandPayload{Kind: transport.CommandCancelTurn, RequestID: requestID}, nil
	case ":cancel-queued":
		if len(fields) != 2 {
			return transport.CommandPayload{}, errLineCommandUsage
		}
		return transport.CommandPayload{Kind: transport.CommandCancelQueued, RequestID: fields[1]}, nil
	default:
		return transport.CommandPayload{}, errUnknownLineCommand
	}
}

// parseAnswers 解析 `questionId=value[,value]` 形式的答案，对之间用 ';' 分隔。
// 选项标签本身可能含空格（如「方案 A」），所以不能按空白拆分。
func parseAnswers(raw string) ([]transport.QuestionAnswer, error) {
	pairs := strings.Split(raw, ";")
	answers := make([]transport.QuestionAnswer, 0, len(pairs))
	seen := make(map[string]struct{}, len(pairs))
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			return nil, errLineCommandUsage
		}
		questionID, rawValues, found := strings.Cut(pair, "=")
		questionID = strings.TrimSpace(questionID)
		if !found || questionID == "" {
			return nil, errLineCommandUsage
		}
		if _, duplicate := seen[questionID]; duplicate {
			return nil, errLineCommandUsage
		}
		seen[questionID] = struct{}{}
		rawParts := strings.Split(rawValues, ",")
		values := make([]string, 0, len(rawParts))
		for _, value := range rawParts {
			value = strings.TrimSpace(value)
			if value == "" {
				return nil, errLineCommandUsage
			}
			values = append(values, value)
		}
		answers = append(answers, transport.QuestionAnswer{QuestionID: questionID, Values: values})
	}
	return answers, nil
}
