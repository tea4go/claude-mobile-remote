package claude

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// FakeScenario 是假 Host 的脚本名。桩模式让 B2 的交互门禁与真实 Claude 额度解耦：
// 同一套 Controller、Channel 和 Transport 代码路径照常运行，只有 Claude 侧被替换。
type FakeScenario string

const (
	// FakeScenarioInteractive 走完整交互脚本：思考、工具调用、工具审批、工具结果、
	// AskUserQuestion（单选 + 多选）、文本回复、Turn 完成。
	FakeScenarioInteractive FakeScenario = "interactive"
	// FakeScenarioText 只回一段文本，用来验证不含交互的基础链路。
	FakeScenarioText FakeScenario = "text"
	// FakeScenarioTurnFailure 触发 Turn 级失败：Session 存活，可继续下一轮。
	FakeScenarioTurnFailure FakeScenario = "turn-failure"
	// FakeScenarioSessionFailure 触发 Session 级失败：会话进入终态。
	FakeScenarioSessionFailure FakeScenario = "session-failure"
)

// FakeSessionID 是桩模式固定使用的 session ID。它刻意由 0 构成，任何验证记录、
// 日志或截图都能一眼看出不是真实会话。
const FakeSessionID = "00000000-0000-4000-8000-000000000000"

// 桩脚本的固定假数据。全部为合成值：不含真实路径、凭证或用户内容。
const (
	fakeToolName            = "Bash"
	fakeToolCommand         = "echo fake-preview"
	fakeToolUseID           = "fake-tool-use-1"
	fakeQuestionToolUseID   = "fake-tool-use-2"
	fakeThinkingText        = "先看清要改什么，再决定动手顺序。"
	fakeTurnFailureText     = "桩脚本触发的 Turn 级失败"
	fakeSessionFailureText  = "桩脚本触发的 Session 级失败"
	fakeTextScenarioReply   = "这是假 Host 的固定回复，不来自真实 Claude。"
	fakeToolApprovedPhrase  = "已获批准"
	fakeToolDeniedPhrase    = "被拒绝"
	fakeInterruptedTurnText = "桩 Turn 已被中断"
	fakeClosedHostText      = "桩 Host 已关闭"
)

// FakeScenarios 返回全部可用脚本名，供 CLI 校验和错误提示使用。
func FakeScenarios() []FakeScenario {
	return []FakeScenario{
		FakeScenarioInteractive,
		FakeScenarioText,
		FakeScenarioTurnFailure,
		FakeScenarioSessionFailure,
	}
}

// ParseFakeScenario 严格解析脚本名：不做 trim、不忽略大小写，避免把拼错的场景名
// 当成有效开关静默接受。
func ParseFakeScenario(value string) (FakeScenario, error) {
	for _, scenario := range FakeScenarios() {
		if value == string(scenario) {
			return scenario, nil
		}
	}
	names := make([]string, 0, len(FakeScenarios()))
	for _, scenario := range FakeScenarios() {
		names = append(names, string(scenario))
	}
	return "", fmt.Errorf("unknown fake host scenario %q: want one of %s", value, strings.Join(names, ", "))
}

// FakeHost 是脚本化的 Host 实现，满足 session.Host 接口。它不启动子进程、不读取
// 用户 Claude 配置，也不打开任何网络连接：全部输出来自本文件的固定假数据。
type FakeHost struct {
	scenario  FakeScenario
	sessionID string
	// actions 与真实 Host 一样是无缓冲旁路通道：脚本在这里阻塞等待决议，
	// 因此桩同样能暴露上层的死锁问题（设计 §3.4）。
	actions  chan *ActionRequest
	stopped  chan struct{}
	stopOnce sync.Once

	mu          sync.Mutex
	connected   bool
	closed      bool
	startedSent bool
	interrupts  int
	// turnDone 是当前 Turn 的中断信号，由 Interrupt 或 Close 关闭。
	turnDone chan struct{}
}

// NewFakeHost 按脚本名创建桩 Host。未知脚本名直接报错，不回退到任何默认脚本。
func NewFakeHost(scenario FakeScenario) (*FakeHost, error) {
	return NewFakeHostForSession(scenario, FakeSessionID)
}

// NewFakeHostForSession gives concurrent fake sessions distinct identities.
// The historical constructor keeps its fixed ID for existing fixtures.
func NewFakeHostForSession(scenario FakeScenario, sessionID string) (*FakeHost, error) {
	if _, err := ParseFakeScenario(string(scenario)); err != nil {
		return nil, err
	}
	if sessionID == "" {
		return nil, errors.New("fake session ID is required")
	}
	return &FakeHost{
		scenario:  scenario,
		sessionID: sessionID,
		actions:   make(chan *ActionRequest),
		stopped:   make(chan struct{}),
	}, nil
}

// SessionID 返回固定的桩 session ID。它在 Connect 前即可用，因此桩可以直接走
// A5.2 的预留 ID 装配路径。
func (h *FakeHost) SessionID() string { return h.sessionID }

// Actions 返回 pending action 的上行通道。
func (h *FakeHost) Actions() <-chan *ActionRequest { return h.actions }

// InterruptCount 报告 Interrupt 被调用的次数，供测试断言取消路径。
func (h *FakeHost) InterruptCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.interrupts
}

// Connect 只翻转本地状态：桩没有可连接的对端。
func (h *FakeHost) Connect(context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return wrapError(ScopeSession, "连接桩 Host", errors.New("Host 已关闭"))
	}
	h.connected = true
	return nil
}

// Interrupt 结束当前桩 Turn。与真实 Host 一样，它由上层的另一个 goroutine 调用。
func (h *FakeHost) Interrupt(context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.connected || h.closed {
		return wrapError(ScopeSession, "中断桩 Turn", errors.New("Host 未连接"))
	}
	h.interrupts++
	h.closeTurnLocked()
	return nil
}

// Close 停止桩并解开所有等待中的脚本。它是幂等的：上层的停止路径可能重复调用。
func (h *FakeHost) Close() error {
	h.stopOnce.Do(func() { close(h.stopped) })
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	h.connected = false
	h.closeTurnLocked()
	return nil
}

func (h *FakeHost) closeTurnLocked() {
	if h.turnDone != nil {
		close(h.turnDone)
		h.turnDone = nil
	}
}

// beginTurn 校验可用性并建立本 Turn 的中断信号。
func (h *FakeHost) beginTurn() (chan struct{}, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.connected || h.closed {
		return nil, wrapError(ScopeSession, "执行桩 Turn", errors.New("Host 未连接"))
	}
	h.closeTurnLocked()
	h.turnDone = make(chan struct{})
	return h.turnDone, nil
}

func (h *FakeHost) endTurn() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closeTurnLocked()
}

// markSessionStarted 报告本轮是否需要宣告 Session 已开始：只有首轮需要。
func (h *FakeHost) markSessionStarted() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.startedSent {
		return false
	}
	h.startedSent = true
	return true
}

// RunTurn 按脚本产出事件。它与真实 Host 共享同一组语义：Turn 完成不结束 Session，
// Turn 级失败允许下一轮继续，Session 级失败使会话进入终态。
func (h *FakeHost) RunTurn(ctx context.Context, prompt string, emit func(Event)) error {
	turnDone, err := h.beginTurn()
	if err != nil {
		return err
	}
	defer h.endTurn()
	if strings.TrimSpace(prompt) == "" {
		return wrapError(ScopeTurn, "执行桩 Turn", errors.New("prompt 不能为空"))
	}
	if emit == nil {
		emit = func(Event) {}
	}
	if h.markSessionStarted() {
		emit(Event{Kind: EventSessionStarted, SessionID: h.sessionID, Source: SessionIDFromInit})
	}

	switch h.scenario {
	case FakeScenarioText:
		emit(Event{Kind: EventText, SessionID: h.sessionID, Text: fakeTextScenarioReply})
		emit(Event{Kind: EventTurnCompleted, SessionID: h.sessionID})
		return nil
	case FakeScenarioTurnFailure:
		emit(Event{Kind: EventTurnFailed, SessionID: h.sessionID, Text: fakeTurnFailureText})
		return wrapError(ScopeTurn, "执行桩 Turn", errors.New(fakeTurnFailureText))
	case FakeScenarioSessionFailure:
		emit(Event{Kind: EventTurnFailed, SessionID: h.sessionID, Text: fakeSessionFailureText})
		return wrapError(ScopeSession, "执行桩 Turn", errors.New(fakeSessionFailureText))
	default:
		return h.runInteractive(ctx, turnDone, emit)
	}
}

// runInteractive 是完整交互脚本。两次 action 之间照常产出结构化块，使上层能验证
// 「等待决议」与「事件推进」在同一 Turn 内交替出现。
func (h *FakeHost) runInteractive(ctx context.Context, turnDone <-chan struct{}, emit func(Event)) error {
	emit(Event{Kind: EventContentBlock, SessionID: h.sessionID, Block: &ContentBlock{
		Kind: BlockThinking, Thinking: fakeThinkingText,
	}})
	emit(Event{Kind: EventContentBlock, SessionID: h.sessionID, Block: &ContentBlock{
		Kind: BlockToolUse, ToolUseID: fakeToolUseID, ToolClass: ToolBuiltin, ToolName: fakeToolName,
	}})

	// 预览走真实的白名单抽取，桩因此也覆盖「只下发白名单字段」这条约束。
	preview, _ := ToolPreview(fakeToolName, map[string]any{"command": fakeToolCommand}, DefaultToolPreviewMaxRunes)
	approval, err := h.requestAction(ctx, turnDone, ActionSpec{
		Kind:                  ActionToolApproval,
		ToolName:              fakeToolName,
		Preview:               preview,
		AllowSessionAvailable: true,
	})
	if err != nil {
		return err
	}
	denied := approval.Kind == DecisionDeny
	// tool_result 只带关联 ID 与成败状态（B2 §4.2）：工具名只在 tool_use 上出现，
	// 桩不能比真实 Adapter 多下发字段。
	emit(Event{Kind: EventContentBlock, SessionID: h.sessionID, Block: &ContentBlock{
		Kind: BlockToolResult, ToolUseID: fakeToolUseID, IsError: &denied,
	}})

	emit(Event{Kind: EventContentBlock, SessionID: h.sessionID, Block: &ContentBlock{
		Kind: BlockToolUse, ToolUseID: fakeQuestionToolUseID, ToolClass: ToolBuiltin, ToolName: askUserQuestionToolName,
	}})
	answered, err := h.requestAction(ctx, turnDone, ActionSpec{
		Kind:      ActionAskUserQuestion,
		ToolName:  askUserQuestionToolName,
		Questions: fakeQuestions(),
	})
	if err != nil {
		return err
	}
	// 提问的 tool_result 永远不是错误：结果修正已在映射层完成（设计 §3.1）。
	questionFailed := false
	emit(Event{Kind: EventContentBlock, SessionID: h.sessionID, Block: &ContentBlock{
		Kind: BlockToolResult, ToolUseID: fakeQuestionToolUseID, IsError: &questionFailed,
	}})

	emit(Event{Kind: EventText, SessionID: h.sessionID, Text: describeFakeOutcome(denied, answered.Answers)})
	emit(Event{Kind: EventTurnCompleted, SessionID: h.sessionID})
	return nil
}

// requestAction 把一个 pending action 交给上层并等待决议。上行与下行都带取消、
// 中断和停止分支：桩绝不能引入真实 Host 不会有的无上界等待。
func (h *FakeHost) requestAction(ctx context.Context, turnDone <-chan struct{}, spec ActionSpec) (ActionDecision, error) {
	request := NewActionRequest(spec)
	select {
	case h.actions <- request:
	case <-ctx.Done():
		return ActionDecision{}, wrapError(ScopeTurn, "投递桩 action", ctx.Err())
	case <-turnDone:
		return ActionDecision{}, wrapError(ScopeTurn, "投递桩 action", errors.New(fakeInterruptedTurnText))
	case <-h.stopped:
		return ActionDecision{}, wrapError(ScopeSession, "投递桩 action", errors.New(fakeClosedHostText))
	}
	select {
	case decision := <-request.Decision():
		if decision.Interrupt {
			// 取消 Turn 的决议只用来解开等待，脚本不再继续产出事件（设计 §3.3）。
			return decision, wrapError(ScopeTurn, "执行桩 Turn", context.Canceled)
		}
		return decision, nil
	case <-ctx.Done():
		return ActionDecision{}, wrapError(ScopeTurn, "等待桩 action 决议", ctx.Err())
	case <-turnDone:
		return ActionDecision{}, wrapError(ScopeTurn, "等待桩 action 决议", errors.New(fakeInterruptedTurnText))
	case <-h.stopped:
		return ActionDecision{}, wrapError(ScopeSession, "等待桩 action 决议", errors.New(fakeClosedHostText))
	}
}

// fakeQuestions 提供一组固定提问：一条单选、一条多选，覆盖移动端两种作答控件。
// 两条都带选项，因此桩不依赖自由文本开关。
func fakeQuestions() []Question {
	return []Question{
		{
			Header:    "实现方式",
			Question:  "这次改动用哪种实现方式？",
			InputType: QuestionChoice,
			Options: []QuestionOption{
				{Label: "方案 A", Description: "直接改现有函数"},
				{Label: "方案 B", Description: "抽出一个新函数"},
			},
		},
		{
			Header:      "验证范围",
			Question:    "需要覆盖哪些验证？",
			InputType:   QuestionChoice,
			MultiSelect: true,
			Options: []QuestionOption{
				{Label: "单元测试", Description: "跑定向用例"},
				{Label: "竞态检测", Description: "开启竞态检查复跑"},
				{Label: "手工验收", Description: "在手机上走一遍"},
			},
		},
	}
}

// describeFakeOutcome 回显本轮决议。回显是桩可用的前提：没有它就无法判断决议真的
// 抵达了脚本。
func describeFakeOutcome(denied bool, answers [][]string) string {
	outcome := fakeToolApprovedPhrase
	if denied {
		outcome = fakeToolDeniedPhrase
	}
	chosen := make([]string, 0, len(answers))
	for _, values := range answers {
		chosen = append(chosen, strings.Join(values, " / "))
	}
	if len(chosen) == 0 {
		return fmt.Sprintf("桩脚本已完成：工具%s，未收到任何选择。", outcome)
	}
	return fmt.Sprintf("桩脚本已完成：工具%s，选择了 %s。", outcome, strings.Join(chosen, "；"))
}
