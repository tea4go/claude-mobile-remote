package claude

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fakeTurnResult struct {
	events []Event
	err    error
}

// startFakeTurn 在后台执行一个桩 Turn。事件在 Turn 结束后一次性回传，主 goroutine
// 因此可以安全地同时消费旁路 action 通道。
func startFakeTurn(ctx context.Context, host *FakeHost, prompt string) <-chan fakeTurnResult {
	done := make(chan fakeTurnResult, 1)
	go func() {
		events := []Event{}
		err := host.RunTurn(ctx, prompt, func(event Event) { events = append(events, event) })
		done <- fakeTurnResult{events: events, err: err}
	}()
	return done
}

func awaitFakeTurn(t *testing.T, done <-chan fakeTurnResult) fakeTurnResult {
	t.Helper()
	select {
	case result := <-done:
		return result
	case <-time.After(hostTestTimeout):
		t.Fatal("timed out waiting for the fake Turn to finish")
		return fakeTurnResult{}
	}
}

func nextFakeAction(t *testing.T, host *FakeHost) *ActionRequest {
	t.Helper()
	select {
	case request := <-host.Actions():
		return request
	case <-time.After(hostTestTimeout):
		t.Fatal("timed out waiting for a fake pending action")
		return nil
	}
}

// describeEvents 把事件流压成可直接比较的短描述，使脚本顺序成为断言对象。
func describeEvents(events []Event) []string {
	described := make([]string, 0, len(events))
	for _, event := range events {
		switch event.Kind {
		case EventContentBlock:
			block := event.Block
			switch block.Kind {
			case BlockThinking:
				described = append(described, "content_block/thinking")
			case BlockToolUse:
				described = append(described, fmt.Sprintf("content_block/tool_use:%s", block.ToolName))
			case BlockToolResult:
				described = append(described, fmt.Sprintf("content_block/tool_result:isError=%t", *block.IsError))
			}
		case EventUnsupported:
			described = append(described, fmt.Sprintf("unsupported/%s", event.Code))
		default:
			described = append(described, string(event.Kind))
		}
	}
	return described
}

func newConnectedFakeHost(t *testing.T, scenario FakeScenario) *FakeHost {
	t.Helper()
	host, err := NewFakeHost(scenario)
	if err != nil {
		t.Fatalf("NewFakeHost(%q) error = %v", scenario, err)
	}
	t.Cleanup(func() { _ = host.Close() })
	if err := host.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	return host
}

func TestParseFakeScenario(t *testing.T) {
	for _, scenario := range FakeScenarios() {
		parsed, err := ParseFakeScenario(string(scenario))
		if err != nil || parsed != scenario {
			t.Fatalf("ParseFakeScenario(%q) = %q, %v; want the same scenario and nil", scenario, parsed, err)
		}
	}
	if len(FakeScenarios()) < 4 {
		t.Fatalf("FakeScenarios() = %#v, want at least the four scripted scenarios", FakeScenarios())
	}
	for _, invalid := range []string{"", "  ", "unknown", "Interactive", "interactive "} {
		if _, err := ParseFakeScenario(invalid); err == nil {
			t.Fatalf("ParseFakeScenario(%q) error = nil, want a rejection", invalid)
		}
	}
}

func TestFakeHostUsesAnObviousTestSessionID(t *testing.T) {
	host := newConnectedFakeHost(t, FakeScenarioText)
	if host.SessionID() != FakeSessionID {
		t.Fatalf("SessionID() = %q, want the fixed fake ID %q", host.SessionID(), FakeSessionID)
	}
	// 桩模式的 session ID 必须一眼可辨、且不能是随机 UUID，否则测试记录会与真实
	// 会话混淆。
	if !strings.HasPrefix(FakeSessionID, "00000000-0000-") {
		t.Fatalf("FakeSessionID = %q, want an obviously synthetic UUID", FakeSessionID)
	}
}

func TestFakeHostInteractiveScript(t *testing.T) {
	host := newConnectedFakeHost(t, FakeScenarioInteractive)
	done := startFakeTurn(context.Background(), host, "请改一下这个文件")

	approval := nextFakeAction(t, host)
	spec := approval.Spec()
	if spec.Kind != ActionToolApproval || spec.ToolName != "Bash" {
		t.Fatalf("first action = %#v, want a Bash tool approval", spec)
	}
	if spec.Preview == "" || !spec.AllowSessionAvailable {
		t.Fatalf("first action = %#v, want a whitelisted preview and the session-scope option", spec)
	}
	if len(spec.Questions) != 0 {
		t.Fatalf("a tool approval must not carry questions: %#v", spec.Questions)
	}
	approval.Resolve(ActionDecision{Kind: DecisionAllowOnce})

	question := nextFakeAction(t, host)
	questionSpec := question.Spec()
	if questionSpec.Kind != ActionAskUserQuestion || questionSpec.ToolName != askUserQuestionToolName {
		t.Fatalf("second action = %#v, want an AskUserQuestion action", questionSpec)
	}
	if len(questionSpec.Questions) != 2 {
		t.Fatalf("Questions = %#v, want one single-select and one multi-select question", questionSpec.Questions)
	}
	if questionSpec.Questions[0].MultiSelect || !questionSpec.Questions[1].MultiSelect {
		t.Fatalf("Questions = %#v, want the first single-select and the second multi-select", questionSpec.Questions)
	}
	for index, scripted := range questionSpec.Questions {
		if scripted.InputType != QuestionChoice || len(scripted.Options) < 2 {
			t.Fatalf("Questions[%d] = %#v, want a choice question with at least two options", index, scripted)
		}
		if scripted.Header == "" || scripted.Question == "" {
			t.Fatalf("Questions[%d] = %#v, want a header and a prompt", index, scripted)
		}
	}
	answers := [][]string{
		{questionSpec.Questions[0].Options[0].Label},
		{questionSpec.Questions[1].Options[0].Label, questionSpec.Questions[1].Options[1].Label},
	}
	question.Resolve(ActionDecision{Kind: DecisionAnswer, Answers: answers})

	result := awaitFakeTurn(t, done)
	if result.err != nil {
		t.Fatalf("RunTurn() error = %v, want nil", result.err)
	}
	want := []string{
		"session_started",
		"content_block/thinking",
		"content_block/tool_use:Bash",
		"content_block/tool_result:isError=false",
		"content_block/tool_use:" + askUserQuestionToolName,
		"content_block/tool_result:isError=false",
		"text",
		"turn_completed",
	}
	if got := describeEvents(result.events); !reflect.DeepEqual(got, want) {
		t.Fatalf("event script = %#v, want %#v", got, want)
	}
	for _, event := range result.events {
		if event.SessionID != FakeSessionID {
			t.Fatalf("event %#v carries session ID %q, want the fake ID", event, event.SessionID)
		}
	}
	if result.events[0].Source != SessionIDFromInit {
		t.Fatalf("session_started Source = %q, want init", result.events[0].Source)
	}
	// 回显答案是桩可用的关键：没有它就无法判断决议真的抵达了脚本。
	replied := result.events[len(result.events)-2].Text
	for _, values := range answers {
		for _, value := range values {
			if !strings.Contains(replied, value) {
				t.Fatalf("reply %q does not echo the answer %q", replied, value)
			}
		}
	}
}

func TestFakeHostSecondTurnSkipsSessionStarted(t *testing.T) {
	host := newConnectedFakeHost(t, FakeScenarioText)
	first := awaitFakeTurn(t, startFakeTurn(context.Background(), host, "第一轮"))
	if first.err != nil {
		t.Fatalf("first RunTurn() error = %v", first.err)
	}
	if got, want := describeEvents(first.events), []string{"session_started", "text", "turn_completed"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("first Turn = %#v, want %#v", got, want)
	}

	second := awaitFakeTurn(t, startFakeTurn(context.Background(), host, "第二轮"))
	if second.err != nil {
		t.Fatalf("second RunTurn() error = %v", second.err)
	}
	// Turn 完成只结束 Turn（硬约束 10）：Session 继续，且不再重复宣告已开始。
	if got, want := describeEvents(second.events), []string{"text", "turn_completed"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("second Turn = %#v, want %#v", got, want)
	}
}

func TestFakeHostDeniedToolReportsFailedResult(t *testing.T) {
	host := newConnectedFakeHost(t, FakeScenarioInteractive)
	done := startFakeTurn(context.Background(), host, "跑个命令")

	nextFakeAction(t, host).Resolve(ActionDecision{Kind: DecisionDeny, Reason: "用户拒绝"})
	question := nextFakeAction(t, host)
	question.Resolve(ActionDecision{Kind: DecisionAnswer, Answers: [][]string{
		{question.Spec().Questions[0].Options[0].Label},
		{question.Spec().Questions[1].Options[0].Label},
	}})

	result := awaitFakeTurn(t, done)
	if result.err != nil {
		t.Fatalf("RunTurn() error = %v, want nil: a denied tool only fails that tool", result.err)
	}
	// 被拒的工具产生失败结果，但 Turn 继续（与真实 Claude 行为一致）。
	want := []string{
		"session_started",
		"content_block/thinking",
		"content_block/tool_use:Bash",
		"content_block/tool_result:isError=true",
		"content_block/tool_use:" + askUserQuestionToolName,
		"content_block/tool_result:isError=false",
		"text",
		"turn_completed",
	}
	if got := describeEvents(result.events); !reflect.DeepEqual(got, want) {
		t.Fatalf("event script = %#v, want %#v", got, want)
	}
}

func TestFakeHostBlocksMatchTheAdapterShape(t *testing.T) {
	host := newConnectedFakeHost(t, FakeScenarioInteractive)
	done := startFakeTurn(context.Background(), host, "检查块形状")
	nextFakeAction(t, host).Resolve(ActionDecision{Kind: DecisionAllowOnce})
	question := nextFakeAction(t, host)
	question.Resolve(ActionDecision{Kind: DecisionAnswer, Answers: [][]string{
		{question.Spec().Questions[0].Options[0].Label},
		{question.Spec().Questions[1].Options[0].Label},
	}})
	result := awaitFakeTurn(t, done)
	if result.err != nil {
		t.Fatalf("RunTurn() error = %v, want nil", result.err)
	}

	// 桩必须产出与真实 Adapter 完全同构的块，否则它会掩盖下游按 B2 §4.2 白名单
	// 做的校验，把联调变成假通过。
	for index, event := range result.events {
		if event.Kind != EventContentBlock {
			continue
		}
		block := event.Block
		switch block.Kind {
		case BlockThinking:
			if block.Thinking == "" || block.ToolUseID != "" || block.ToolClass != "" ||
				block.MCPServer != "" || block.ToolName != "" || block.IsError != nil {
				t.Fatalf("events[%d] thinking block = %#v, want only the thinking text", index, block)
			}
		case BlockToolUse:
			if block.ToolUseID == "" || block.ToolName == "" || block.ToolClass != ToolBuiltin ||
				block.Thinking != "" || block.MCPServer != "" || block.IsError != nil {
				t.Fatalf("events[%d] tool_use block = %#v, want an identified builtin tool", index, block)
			}
		case BlockToolResult:
			if block.ToolUseID == "" || block.IsError == nil || block.Thinking != "" ||
				block.ToolClass != "" || block.MCPServer != "" || block.ToolName != "" {
				t.Fatalf("events[%d] tool_result block = %#v, want only toolUseId and isError", index, block)
			}
		default:
			t.Fatalf("events[%d] block kind = %q, want one of the three B2 kinds", index, block.Kind)
		}
	}
}

func TestFakeHostTurnFailureScenario(t *testing.T) {
	host := newConnectedFakeHost(t, FakeScenarioTurnFailure)
	result := awaitFakeTurn(t, startFakeTurn(context.Background(), host, "触发 Turn 失败"))
	if result.err == nil {
		t.Fatal("RunTurn() error = nil, want a scripted Turn failure")
	}
	if ScopeOf(result.err) != ScopeTurn {
		t.Fatalf("ScopeOf(err) = %q, want turn: the Session must survive", ScopeOf(result.err))
	}
	if got, want := describeEvents(result.events), []string{"session_started", "turn_failed"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("event script = %#v, want %#v", got, want)
	}
}

func TestFakeHostSessionFailureScenario(t *testing.T) {
	host := newConnectedFakeHost(t, FakeScenarioSessionFailure)
	result := awaitFakeTurn(t, startFakeTurn(context.Background(), host, "触发 Session 失败"))
	if result.err == nil {
		t.Fatal("RunTurn() error = nil, want a scripted Session failure")
	}
	if ScopeOf(result.err) != ScopeSession {
		t.Fatalf("ScopeOf(err) = %q, want session", ScopeOf(result.err))
	}
}

func TestFakeHostCancellationReleasesPendingAction(t *testing.T) {
	host := newConnectedFakeHost(t, FakeScenarioInteractive)
	ctx, cancel := context.WithCancel(context.Background())
	done := startFakeTurn(ctx, host, "然后取消")

	nextFakeAction(t, host)
	// 决议永不到来：取消必须自己解开等待，否则桩会和真实 Host 一样死锁。
	cancel()
	result := awaitFakeTurn(t, done)
	if !errors.Is(result.err, context.Canceled) {
		t.Fatalf("RunTurn() error = %v, want a context.Canceled chain", result.err)
	}
}

func TestFakeHostCloseReleasesPendingAction(t *testing.T) {
	host := newConnectedFakeHost(t, FakeScenarioInteractive)
	done := startFakeTurn(context.Background(), host, "然后关闭")

	nextFakeAction(t, host)
	if err := host.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if result := awaitFakeTurn(t, done); result.err == nil {
		t.Fatal("RunTurn() error = nil, want the Turn to end when the Host closes")
	}
	// Close 幂等：Controller 的停止路径可能重复调用。
	if err := host.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func TestFakeHostRejectsUseBeforeConnectAndAfterClose(t *testing.T) {
	host, err := NewFakeHost(FakeScenarioText)
	if err != nil {
		t.Fatalf("NewFakeHost() error = %v", err)
	}
	if err := host.RunTurn(context.Background(), "太早了", nil); err == nil {
		t.Fatal("RunTurn() before Connect error = nil, want a rejection")
	}
	if err := host.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if err := host.RunTurn(context.Background(), "   ", nil); err == nil {
		t.Fatal("RunTurn(blank prompt) error = nil, want a rejection")
	}
	if err := host.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := host.RunTurn(context.Background(), "太晚了", nil); err == nil {
		t.Fatal("RunTurn() after Close error = nil, want a rejection")
	}
}

func TestFakeHostInterruptEndsTheTurnWithoutPendingAction(t *testing.T) {
	host := newConnectedFakeHost(t, FakeScenarioInteractive)
	done := startFakeTurn(context.Background(), host, "中断我")

	nextFakeAction(t, host)
	if err := host.Interrupt(context.Background()); err != nil {
		t.Fatalf("Interrupt() error = %v", err)
	}
	if result := awaitFakeTurn(t, done); result.err == nil {
		t.Fatal("RunTurn() error = nil, want the interrupted Turn to end")
	}
	if host.InterruptCount() != 1 {
		t.Fatalf("InterruptCount() = %d, want 1", host.InterruptCount())
	}
}

// TestFakeHostNeedsNoClaudeEnvironment 是「桩不依赖真实 Claude」的可执行断言：
// 清空 PATH 后任何子进程都无法启动，把 HOME 指向不存在的目录后任何用户 Claude
// 配置都无法读取，脚本仍必须完整跑通。
func TestFakeHostNeedsNoClaudeEnvironment(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("HOME", "/nonexistent-fake-host-home")
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "")
	t.Setenv("ANTHROPIC_API_KEY", "")

	host := newConnectedFakeHost(t, FakeScenarioInteractive)
	done := startFakeTurn(context.Background(), host, "无环境也要跑通")
	nextFakeAction(t, host).Resolve(ActionDecision{Kind: DecisionAllowOnce})
	question := nextFakeAction(t, host)
	question.Resolve(ActionDecision{Kind: DecisionAnswer, Answers: [][]string{
		{question.Spec().Questions[0].Options[0].Label},
		{question.Spec().Questions[1].Options[0].Label},
	}})
	result := awaitFakeTurn(t, done)
	if result.err != nil {
		t.Fatalf("RunTurn() error = %v, want nil without any Claude environment", result.err)
	}
	if last := result.events[len(result.events)-1]; last.Kind != EventTurnCompleted {
		t.Fatalf("last event = %#v, want turn_completed", last)
	}
}

// TestFakeHostScriptCarriesNoRealData 钉住桩数据的安全边界：脚本是固定假值，
// 不得包含真实路径、token 或用户内容。
func TestFakeHostScriptCarriesNoRealData(t *testing.T) {
	host := newConnectedFakeHost(t, FakeScenarioInteractive)
	done := startFakeTurn(context.Background(), host, "检查桩数据")
	approval := nextFakeAction(t, host)
	spec := approval.Spec()
	approval.Resolve(ActionDecision{Kind: DecisionAllowOnce})
	question := nextFakeAction(t, host)
	question.Resolve(ActionDecision{Kind: DecisionAnswer, Answers: [][]string{
		{question.Spec().Questions[0].Options[0].Label},
		{question.Spec().Questions[1].Options[0].Label},
	}})
	result := awaitFakeTurn(t, done)

	collected := []string{spec.Preview}
	for _, event := range result.events {
		collected = append(collected, event.Text)
	}
	for _, text := range collected {
		lowered := strings.ToLower(text)
		for _, forbidden := range []string{"/users/", "/home/", "sk-ant", "bearer ", "token"} {
			if strings.Contains(lowered, forbidden) {
				t.Fatalf("fake script text %q contains %q, which must never appear in stub data", text, forbidden)
			}
		}
	}
	if !strings.Contains(spec.Preview, "fake") {
		t.Fatalf("preview = %q, want an obviously synthetic value", spec.Preview)
	}
}
