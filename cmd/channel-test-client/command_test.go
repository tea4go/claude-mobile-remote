package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	projectsession "github.com/tea4go/claude-mobile-remote/internal/session"
	"github.com/tea4go/claude-mobile-remote/internal/transport"
)

// TestClientStateTracksActionAndRequest 验证行命令的隐式目标只来自 snapshot 与
// 事件流：待处理 action 出现后可被引用，被解决后立即失效。
func TestClientStateTracksActionAndRequest(t *testing.T) {
	var state clientState
	state.observeSnapshot(projectsession.Snapshot{
		SessionID: "session-1",
		Turn: projectsession.TurnSnapshot{
			RequestID: "request-1",
			Status:    projectsession.TurnWaiting,
			PendingAction: &projectsession.PendingAction{
				ActionID: "request-1-action-1", RequestID: "request-1",
				Kind: projectsession.ActionToolApproval,
			},
		},
	})
	if state.actionID != "request-1-action-1" || state.requestID != "request-1" {
		t.Fatalf("state after snapshot = %#v", state)
	}

	state.observeEvent(projectsession.Event{
		Seq: 8, Kind: projectsession.EventActionResolved,
		SessionID: "session-1", RequestID: "request-1",
	})
	if state.actionID != "" {
		t.Fatalf("actionID after action_resolved = %q, want it cleared", state.actionID)
	}

	state.observeEvent(projectsession.Event{
		Seq: 9, Kind: projectsession.EventActionRequired,
		SessionID: "session-1", RequestID: "request-2",
		PendingAction: &projectsession.PendingAction{
			ActionID: "request-2-action-1", RequestID: "request-2",
			Kind: projectsession.ActionAskUserQuestion,
		},
	})
	if state.actionID != "request-2-action-1" || state.requestID != "request-2" {
		t.Fatalf("state after action_required = %#v", state)
	}
}

func TestBuildCommandTranslatesLineCommands(t *testing.T) {
	state := clientState{actionID: "request-1-action-1", requestID: "request-1"}
	testCases := []struct {
		name string
		line string
		want transport.CommandPayload
	}{
		{
			name: "plain text",
			line: "hello there",
			want: transport.CommandPayload{Kind: transport.CommandSendText, Text: "hello there"},
		},
		{
			name: "allow once",
			line: ":allow",
			want: transport.CommandPayload{
				Kind: transport.CommandResolveAction, ActionID: "request-1-action-1",
				Decision: transport.DecisionAllowOnce,
			},
		},
		{
			name: "allow session",
			line: ":allow-session",
			want: transport.CommandPayload{
				Kind: transport.CommandResolveAction, ActionID: "request-1-action-1",
				Decision: transport.DecisionAllowSession,
			},
		},
		{
			name: "deny",
			line: ":deny",
			want: transport.CommandPayload{
				Kind: transport.CommandResolveAction, ActionID: "request-1-action-1",
				Decision: transport.DecisionDeny,
			},
		},
		{
			name: "answers",
			line: ":answer q1=方案 A; q2=单元测试,竞态检测",
			want: transport.CommandPayload{
				Kind: transport.CommandResolveAction, ActionID: "request-1-action-1",
				Answers: []transport.QuestionAnswer{
					{QuestionID: "q1", Values: []string{"方案 A"}},
					{QuestionID: "q2", Values: []string{"单元测试", "竞态检测"}},
				},
			},
		},
		{
			name: "cancel the observed turn",
			line: ":cancel-turn",
			want: transport.CommandPayload{Kind: transport.CommandCancelTurn, RequestID: "request-1"},
		},
		{
			name: "cancel an explicit turn",
			line: ":cancel-turn request-9",
			want: transport.CommandPayload{Kind: transport.CommandCancelTurn, RequestID: "request-9"},
		},
		{
			name: "cancel a queued request",
			line: ":cancel-queued request-2",
			want: transport.CommandPayload{Kind: transport.CommandCancelQueued, RequestID: "request-2"},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			payload, err := buildCommand(state, testCase.line)
			if err != nil {
				t.Fatalf("buildCommand() error = %v", err)
			}
			if !reflect.DeepEqual(payload, testCase.want) {
				t.Fatalf("payload = %#v, want %#v", payload, testCase.want)
			}
			// 客户端只发协议已接受的命令，避免把校验负担推给服务端。
			envelope, err := transport.NewEnvelope(transport.TypeCommand, "session-1", "command-1", "", 0, payload)
			if err != nil {
				t.Fatalf("NewEnvelope() error = %v", err)
			}
			if err := transport.ValidateEnvelope(envelope); err != nil {
				t.Fatalf("ValidateEnvelope() error = %v", err)
			}
		})
	}
}

func TestBuildCommandRejectsUnusableLines(t *testing.T) {
	withAction := clientState{actionID: "request-1-action-1", requestID: "request-1"}
	testCases := []struct {
		name  string
		state clientState
		line  string
		want  error
	}{
		{name: "unknown line command", state: withAction, line: ":nope", want: errUnknownLineCommand},
		{name: "allow without a pending action", line: ":allow", want: errNoPendingAction},
		{name: "answer without a pending action", line: ":answer q1=A", want: errNoPendingAction},
		{name: "allow with an argument", state: withAction, line: ":allow extra", want: errLineCommandUsage},
		{name: "answer without pairs", state: withAction, line: ":answer", want: errLineCommandUsage},
		{name: "answer without a value", state: withAction, line: ":answer q1=", want: errLineCommandUsage},
		{name: "answer without a question", state: withAction, line: ":answer =A", want: errLineCommandUsage},
		{name: "answer with a blank value", state: withAction, line: ":answer q1=A,,B", want: errLineCommandUsage},
		{name: "answer repeating a question", state: withAction, line: ":answer q1=A; q1=B", want: errLineCommandUsage},
		{name: "answer with an empty pair", state: withAction, line: ":answer q1=A;", want: errLineCommandUsage},
		{name: "cancel turn without a target", line: ":cancel-turn", want: errNoTargetRequest},
		{name: "cancel turn with extra words", state: withAction, line: ":cancel-turn a b", want: errLineCommandUsage},
		{name: "cancel queued without a target", state: withAction, line: ":cancel-queued", want: errLineCommandUsage},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			payload, err := buildCommand(testCase.state, testCase.line)
			if !errors.Is(err, testCase.want) {
				t.Fatalf("buildCommand() error = %v, want %v", err, testCase.want)
			}
			if payload.Kind != "" {
				t.Fatalf("payload = %#v, want no command", payload)
			}
		})
	}
}

// TestPrintPendingActionEchoesAnswerTargets 保证回显足以拼出一条合法 :answer：
// 缺少 questionId 或选项标签时联调就只能靠猜。
func TestPrintPendingActionEchoesAnswerTargets(t *testing.T) {
	var output strings.Builder
	printPendingAction(&output, projectsession.PendingAction{
		ActionID: "request-1-action-2", RequestID: "request-1",
		Kind: projectsession.ActionAskUserQuestion,
		Questions: []projectsession.Question{{
			QuestionID: "request-1-action-2-q1", Header: "方案", Question: "选哪个？",
			MultiSelect: true,
			Options: []projectsession.QuestionOption{
				{Label: "方案 A"}, {Label: "方案 B"},
			},
		}},
	})
	printed := output.String()
	for _, want := range []string{
		"actionId=request-1-action-2", "kind=ask_user_question", "allowSession=false",
		"question request-1-action-2-q1", "multiSelect=true", "options=方案 A|方案 B",
	} {
		if !strings.Contains(printed, want) {
			t.Fatalf("printed %q does not contain %q", printed, want)
		}
	}
}

// TestPrintContentBlockEchoesIDAndTitleFields 断言回显带上 Controller 分配的
// block ID，且不打印工具输入或结果正文。
func TestPrintContentBlockEchoesIDAndTitleFields(t *testing.T) {
	failed := true
	cases := []struct {
		name   string
		block  projectsession.ContentBlock
		want   []string
		absent string
	}{
		{
			name: "thinking",
			block: projectsession.ContentBlock{
				ID: "request-1-block-1", Kind: projectsession.BlockThinking, Thinking: "三个字",
			},
			want:   []string{"block request-1-block-1", "thinking=3 runes"},
			absent: "三个字",
		},
		{
			name: "tool use",
			block: projectsession.ContentBlock{
				ID: "request-1-block-2", Kind: projectsession.BlockToolUse, ToolUseID: "tool-1",
				ToolClass: projectsession.ToolMCP, MCPServer: "filesystem", ToolName: "read_file",
			},
			want: []string{"block request-1-block-2", "tool_use=read_file", "class=mcp", "toolUseId=tool-1"},
		},
		{
			name: "tool result",
			block: projectsession.ContentBlock{
				ID: "request-1-block-3", Kind: projectsession.BlockToolResult,
				ToolUseID: "tool-1", IsError: &failed,
			},
			want: []string{"block request-1-block-3", "tool_result", "isError=true"},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var output strings.Builder
			printContentBlock(&output, testCase.block)
			printed := output.String()
			for _, want := range testCase.want {
				if !strings.Contains(printed, want) {
					t.Fatalf("printed %q does not contain %q", printed, want)
				}
			}
			if testCase.absent != "" && strings.Contains(printed, testCase.absent) {
				t.Fatalf("printed %q must not echo the block body", printed)
			}
		})
	}
}
