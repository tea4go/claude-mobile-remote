// Package claude 封装 claude-agent-sdk-go 的最小托管生命周期。
package claude

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	claudecode "github.com/tea4go/claude-agent-sdk-go"
)

type sdkClient interface {
	Connect(context.Context, ...claudecode.StreamMessage) error
	Query(context.Context, string) error
	ReceiveResponse(context.Context) claudecode.MessageIterator
	Interrupt(context.Context) error
	Disconnect() error
}

// DefaultActionTimeout 是单个 pending action 的等待上限（设计 §7）。它属于 PC
// 侧服务配置，不进入三端协议，超时后只拒绝本次工具，Turn 继续。
const DefaultActionTimeout = 10 * time.Minute

// permissionDestinationSession 是 SDK 权限建议的会话级作用域，B2 只允许把
// Claude 自己给出的建议限制到当前会话，不自行拼接权限规则。
const permissionDestinationSession = "session"

// Option 配置 Host。A2 只使用默认生产 Client；测试在包内注入 fake。
type Option func(*Host)

func withClient(client sdkClient) Option {
	return func(host *Host) { host.client = client }
}

// WithActionTimeout 覆盖单个 pending action 的等待上限。
func WithActionTimeout(timeout time.Duration) Option {
	return func(host *Host) {
		if timeout > 0 {
			host.actionTimeout = timeout
		}
	}
}

// WithToolPreviewMaxRunes 覆盖工具预览的截断上限。
func WithToolPreviewMaxRunes(maxRunes int) Option {
	return func(host *Host) {
		if maxRunes > 0 {
			host.previewMaxRunes = maxRunes
		}
	}
}

// WithCwd sets the working directory used when the SDK starts Claude Code.
// An empty value keeps the SDK default, which is the current process directory.
func WithCwd(cwd string) Option {
	return func(host *Host) { host.cwd = cwd }
}

// Host 持有一个长生命周期 SDK Client。RunTurn 必须由上层串行调用。
type Host struct {
	client       sdkClient
	sessionID    string
	sessionIDSet bool
	startedSent  bool
	// mapper 跨 Turn 保留 toolUseId → toolName，供 tool_result 的结果修正使用。
	mapper          *blockMapper
	previewMaxRunes int
	actionTimeout   time.Duration
	cwd             string

	// actions 是 pending action 的旁路上行通道（设计 §3.4）：权限回调跑在 SDK
	// 读循环上，不能复用 RunTurn 的 emit 路径，否则与上层的无缓冲事件通道互等。
	actions chan *ActionRequest
	// stopped 在 Close 时关闭，用来解开仍在等待决议的权限回调。
	stopped  chan struct{}
	stopOnce sync.Once

	// lifecycleMu 只保护连接状态：Interrupt 由上层另一个 goroutine 调用，
	// 与 Connect/Close 并发。
	lifecycleMu sync.Mutex
	connected   bool
	closed      bool

	// turnMu 保护当前 Turn 的取消信号，权限回调据此感知 Turn 取消。
	turnMu   sync.Mutex
	turnDone <-chan struct{}
}

// NewHost 创建使用 SDK 默认权限模式的 Host。工具请求统一进入权限回调，由上层
// 通过旁路通道给出决议（B2 §3.1）。
func NewHost(options ...Option) *Host {
	host := &Host{
		mapper:          newBlockMapper(),
		previewMaxRunes: DefaultToolPreviewMaxRunes,
		actionTimeout:   DefaultActionTimeout,
		actions:         make(chan *ActionRequest),
		stopped:         make(chan struct{}),
	}
	for _, option := range options {
		option(host)
	}
	if host.client == nil {
		host.client = claudecode.NewClient(host.sessionSDKOptions(sessionModeNew, "")...)
	}
	return host
}

// Actions 返回 pending action 的上行通道。它只能有一个消费者（Session
// Controller），Adapter 自身不做并发或权限判定。
func (h *Host) Actions() <-chan *ActionRequest { return h.actions }

// sessionMode 区分预分配 ID 的两种来源（A5.2 设计 §4）：新建会话把 Service
// 生成的 UUID 指定给 Claude，恢复会话沿用已有 ID。两种模式的 SDK 选项互斥。
type sessionMode int

const (
	sessionModeNew sessionMode = iota
	sessionModeResume
)

// sessionSDKOptions 按模式构造 SDK 选项：新建只映射 --session-id，恢复只
// 映射 --resume（设计 §5）。SDK 侧 Validate 会拒绝两者并用。
func (h *Host) sessionSDKOptions(mode sessionMode, sessionID string) []claudecode.Option {
	options := []claudecode.Option{claudecode.WithCanUseTool(h.canUseTool)}
	if h.cwd != "" {
		options = append(options, claudecode.WithCwd(h.cwd))
	}
	if sessionID == "" {
		return options
	}
	if mode == sessionModeResume {
		return append(options, claudecode.WithResume(sessionID))
	}
	return append(options, claudecode.WithSessionID(sessionID))
}

// NewHostWithReservedSessionID 创建以既有 UUID 预分配新 Claude session 的
// Host（A5.2）。ID 通过 SDK WithSessionID 映射到 Claude CLI --session-id；
// Host 从 Connect 前持有预期 ID，首个 init/Result 必须返回相同 ID。
func NewHostWithReservedSessionID(sessionID string, options ...Option) (*Host, error) {
	return newReservedHost(sessionModeNew, sessionID, options...)
}

// NewHostForResume 创建恢复已有会话的 Host（A5.2 设计 §5.2）。已有 ID 通过
// SDK WithResume 映射到 Claude CLI --resume，不传 --session-id；Claude 返回
// 其他 ID 时按 Session 级失败处理。
func NewHostForResume(sessionID string, options ...Option) (*Host, error) {
	return newReservedHost(sessionModeResume, sessionID, options...)
}

func newReservedHost(mode sessionMode, sessionID string, options ...Option) (*Host, error) {
	if !isValidUUID(sessionID) {
		return nil, fmt.Errorf("预分配 session ID 必须是合法 UUID: %q", sessionID)
	}
	host := NewHost(append([]Option{withReservedSessionID(mode, sessionID)}, options...)...)
	return host, nil
}

func withReservedSessionID(mode sessionMode, sessionID string) Option {
	return func(host *Host) {
		host.sessionID = sessionID
		host.sessionIDSet = true
		if host.client == nil {
			host.client = claudecode.NewClient(host.sessionSDKOptions(mode, sessionID)...)
		}
	}
}

// PrepareNewSession 为新会话生成 UUID v4 并预分配（A5.2 设计 §3.1）。
// 只能调用一次；Connect 前调用。
func (h *Host) PrepareNewSession() error {
	if h.sessionIDSet || h.isConnected() || h.isClosed() {
		return wrapError(ScopeSession, "预分配 session ID", errors.New("session ID 已确定或 Host 已启动"))
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return wrapError(ScopeSession, "生成 session UUID", err)
	}
	raw[6] = (raw[6] & 0x0f) | 0x40 // version 4
	raw[8] = (raw[8] & 0x3f) | 0x80 // variant 10
	sessionID := fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16])
	withReservedSessionID(sessionModeNew, sessionID)(h)
	h.client = claudecode.NewClient(h.sessionSDKOptions(sessionModeNew, sessionID)...)
	return nil
}

func isValidUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, char := range value {
		switch i {
		case 8, 13, 18, 23:
			if char != '-' {
				return false
			}
		default:
			isHex := (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')
			if !isHex {
				return false
			}
		}
	}
	return true
}

// Connect 启动并连接由 SDK 托管的 Claude CLI。
func (h *Host) Connect(ctx context.Context) error {
	if h.isConnected() || h.isClosed() {
		return wrapError(ScopeSession, "连接 Claude CLI", errors.New("Host 已连接或已关闭"))
	}
	if err := h.client.Connect(ctx); err != nil {
		return wrapError(ScopeSession, "连接 Claude CLI", err)
	}
	h.lifecycleMu.Lock()
	h.connected = true
	h.lifecycleMu.Unlock()
	return nil
}

func (h *Host) isConnected() bool {
	h.lifecycleMu.Lock()
	defer h.lifecycleMu.Unlock()
	return h.connected
}

func (h *Host) isClosed() bool {
	h.lifecycleMu.Lock()
	defer h.lifecycleMu.Unlock()
	return h.closed
}

// Interrupt 请求 Claude 中断当前 Turn。它由 Controller 在另一个 goroutine 调用，
// 因此绝不能在存在 pending action 时使用：控制响应只能由已被阻塞的同一读循环
// 投递，必然超时（设计 §3.3）。
func (h *Host) Interrupt(ctx context.Context) error {
	h.lifecycleMu.Lock()
	client, usable := h.client, h.connected && !h.closed
	h.lifecycleMu.Unlock()
	if !usable {
		return wrapError(ScopeSession, "中断 Claude Turn", errors.New("Host 未连接"))
	}
	if err := client.Interrupt(ctx); err != nil {
		return wrapError(ScopeTurn, "中断 Claude Turn", err)
	}
	return nil
}

// SessionID 返回 SDK 提供的真实 session ID；首轮 init/Result 前为空。
// A5.2 预分配后从 Connect 前即返回预留 ID。
func (h *Host) SessionID() string { return h.sessionID }

// SessionConfirmed 报告 Claude init/Result 是否已确认预留 ID（A5.2 §4）。
// 未预留 ID 的 Host 在首个非空 init/Result 接受后同样视为已确认。
func (h *Host) SessionConfirmed() bool { return h.startedSent }

// RunTurn 在当前 Client 上执行一个文本 Turn，并同步发出 A2 内部事件。
func (h *Host) RunTurn(ctx context.Context, prompt string, emit func(Event)) error {
	if !h.isConnected() || h.isClosed() {
		return wrapError(ScopeSession, "执行 Claude Turn", errors.New("Host 未连接"))
	}
	if strings.TrimSpace(prompt) == "" {
		return wrapError(ScopeTurn, "执行 Claude Turn", errors.New("prompt 不能为空"))
	}
	if emit == nil {
		emit = func(Event) {}
	}
	// 权限回调收到的 ctx 是传输层上下文，不随 Turn 取消结束，因此必须由 Host
	// 自己持有当前 Turn 的取消信号（设计 §3.1）。
	h.setTurnDone(ctx.Done())
	defer h.setTurnDone(nil)
	if err := h.client.Query(ctx, prompt); err != nil {
		return wrapError(ScopeSession, "发送 Claude Query", err)
	}

	iterator := h.client.ReceiveResponse(ctx)
	defer iterator.Close()
	emittedText := false
	for {
		message, err := iterator.Next(ctx)
		if err != nil {
			if errors.Is(err, claudecode.ErrNoMoreMessages) {
				return wrapError(ScopeSession, "接收 Claude 响应", errors.New("流结束但未收到 ResultMessage"))
			}
			return wrapError(ScopeSession, "接收 Claude 响应", err)
		}

		parsed := h.mapper.parse(message)
		if parsed.result != nil && parsed.sessionID == "" {
			return wrapError(ScopeSession, "完成 Claude Turn", errors.New("ResultMessage 未提供 session ID"))
		}
		if parsed.assistantErr != nil {
			// 带 Error 的 AssistantMessage 终止当前流（SDK 语义，与 WhaleTerm
			// 验证行为一致）：server/rate_limit 类瞬时错误只结束本 Turn，
			// 其余（认证、计费等）视为 Session 级不可恢复。
			scope := ScopeTurn
			if !isTransientAssistantError(parsed.assistantErr.errorType) {
				scope = ScopeSession
			}
			message := parsed.assistantErr.text
			if message == "" {
				message = "Claude API Error: " + parsed.assistantErr.errorType
			} else {
				message = fmt.Sprintf("Claude API Error (%s): %s", parsed.assistantErr.errorType, message)
			}
			sessionID := h.sessionID
			if sessionID == "" {
				sessionID = parsed.sessionID
			}
			emit(Event{Kind: EventTurnFailed, SessionID: sessionID, Text: message})
			return wrapError(scope, "Claude Turn 失败", errors.New(message))
		}
		if parsed.sessionID != "" {
			if err := h.acceptSessionID(parsed.sessionID, parsed.source, emit); err != nil {
				return err
			}
		}
		for _, item := range parsed.items {
			switch item.kind {
			case EventText:
				emittedText = true
				emit(Event{Kind: EventText, SessionID: h.sessionID, Text: item.text})
			case EventContentBlock:
				emit(Event{Kind: EventContentBlock, SessionID: h.sessionID, Block: item.block})
			case EventUnsupported:
				emit(Event{Kind: EventUnsupported, SessionID: h.sessionID, Code: item.code})
			}
		}
		if parsed.result == nil {
			continue
		}
		if h.sessionID == "" {
			return wrapError(ScopeSession, "完成 Claude Turn", errors.New("ResultMessage 未提供 session ID"))
		}
		if parsed.result.isError {
			details := make([]string, 0, 1+len(parsed.result.errors))
			if parsed.result.text != "" {
				details = append(details, parsed.result.text)
			}
			details = append(details, parsed.result.errors...)
			if len(details) == 0 {
				details = append(details, "Claude 返回未知错误")
			}
			message := strings.Join(details, "; ")
			emit(Event{Kind: EventTurnFailed, SessionID: h.sessionID, Text: message})
			return wrapError(ScopeTurn, "Claude Turn 失败", errors.New(message))
		}
		if !emittedText && parsed.result.text != "" {
			emit(Event{Kind: EventText, SessionID: h.sessionID, Text: parsed.result.text})
		}
		emit(Event{Kind: EventTurnCompleted, SessionID: h.sessionID})
		return nil
	}
}

// isTransientAssistantError 判断 AssistantMessage 错误类型是否只影响当前 Turn。
// server_error 与 rate_limit 通常可重试；认证、计费、无效请求等无法通过
// 重试恢复，保守视为 Session 级失败。类型集合来源：WhaleTerm 验证行为。
func isTransientAssistantError(errorType string) bool {
	return errorType == string(claudecode.AssistantMessageErrorServer) ||
		errorType == string(claudecode.AssistantMessageErrorRateLimit)
}

func (h *Host) acceptSessionID(value string, source SessionIDSource, emit func(Event)) error {
	if h.sessionID != "" && h.sessionID != value {
		return wrapError(ScopeSession, "校验 Claude session ID", fmt.Errorf("session ID 从 %q 变为 %q", h.sessionID, value))
	}
	if h.sessionID == "" {
		h.sessionID = value
	}
	if !h.startedSent {
		h.startedSent = true
		emit(Event{Kind: EventSessionStarted, SessionID: value, Source: source})
	}
	return nil
}

// Close 幂等关闭 SDK Client。失败不会导致重复 Disconnect。
func (h *Host) Close() error {
	h.lifecycleMu.Lock()
	if h.closed {
		h.lifecycleMu.Unlock()
		return nil
	}
	h.closed = true
	wasConnected := h.connected
	h.connected = false
	h.lifecycleMu.Unlock()

	// 先解开仍在等待决议的权限回调，再断开 Client：否则读循环无法退出。
	h.stopOnce.Do(func() { close(h.stopped) })
	if !wasConnected {
		return nil
	}
	if err := h.client.Disconnect(); err != nil {
		return fmt.Errorf("关闭 Claude CLI: %w", err)
	}
	return nil
}

func (h *Host) setTurnDone(done <-chan struct{}) {
	h.turnMu.Lock()
	h.turnDone = done
	h.turnMu.Unlock()
}

func (h *Host) currentTurnDone() <-chan struct{} {
	h.turnMu.Lock()
	defer h.turnMu.Unlock()
	return h.turnDone
}

// ActionKind 区分两类需要用户决议的交互（设计 §2.1）：工具授权与 Claude 提问。
type ActionKind string

const (
	ActionToolApproval    ActionKind = "tool_approval"
	ActionAskUserQuestion ActionKind = "ask_user_question"
)

// QuestionInputType 表示一条提问的作答方式：Claude 给出选项时为单/多选，未给出
// 选项时只能自由文本作答。
type QuestionInputType string

const (
	QuestionChoice QuestionInputType = "choice"
	QuestionText   QuestionInputType = "text"
)

// QuestionOption 是一条提问的一个可选项。Description 可能为空。
type QuestionOption struct {
	Label       string
	Description string
}

// Question 是一条待用户回答的提问。
type Question struct {
	Header      string
	Question    string
	InputType   QuestionInputType
	MultiSelect bool
	Options     []QuestionOption
}

// ActionSpec 是一个 pending action 的可展示内容。它是纯值类型（无锁无通道），
// 可安全复制；不含工具原始 input，预览已按白名单抽取并截断（设计 §6）。
type ActionSpec struct {
	Kind     ActionKind
	ToolName string
	Preview  string
	// AllowSessionAvailable 报告“本会话都允许”是否可用：只有 SDK 给出权限建议时
	// 才为真，Adapter 不自行拼接权限规则。
	AllowSessionAvailable bool
	Questions             []Question
}

// ActionDecisionKind 是上层对一个 pending action 的决议类型。
type ActionDecisionKind string

const (
	DecisionAllowOnce    ActionDecisionKind = "allow_once"
	DecisionAllowSession ActionDecisionKind = "allow_session"
	DecisionDeny         ActionDecisionKind = "deny"
	DecisionAnswer       ActionDecisionKind = "answer"
)

// ActionDecision 是一次决议。Answers 与 Spec.Questions 按下标一一对应；Interrupt
// 只在取消 Turn 时置真，用来解开被阻塞的读循环（设计 §3.3）。
type ActionDecision struct {
	Kind      ActionDecisionKind
	Answers   [][]string
	Reason    string
	Interrupt bool
}

// ActionRequest 是旁路通道上的一次 pending action。它持有 sync.Once 与通道，
// 只能按指针传递；决议通道容量为 1 且只投递一次（设计 §3.4）。
type ActionRequest struct {
	spec     ActionSpec
	decision chan ActionDecision
	once     sync.Once
}

// NewActionRequest 创建一个待决议请求。spec 在此处即深拷贝，回调侧之后不再持有
// 与消费者共享的切片。
func NewActionRequest(spec ActionSpec) *ActionRequest {
	return &ActionRequest{spec: cloneActionSpec(spec), decision: make(chan ActionDecision, 1)}
}

// Spec 返回展示内容的深拷贝：消费者的任何修改都不会影响回调侧。
func (r *ActionRequest) Spec() ActionSpec { return cloneActionSpec(r.spec) }

// Resolve 投递决议。首次调用返回 true；重复决议返回 false，由调用方按
// action_not_pending 处理。因通道容量为 1，本方法永不阻塞。
func (r *ActionRequest) Resolve(decision ActionDecision) bool {
	accepted := false
	r.once.Do(func() {
		r.decision <- decision
		accepted = true
	})
	return accepted
}

// Decision 返回决议通道，供等待中的权限回调选择。
func (r *ActionRequest) Decision() <-chan ActionDecision { return r.decision }

func cloneActionSpec(spec ActionSpec) ActionSpec {
	cloned := spec
	if len(spec.Questions) == 0 {
		cloned.Questions = nil
		return cloned
	}
	cloned.Questions = make([]Question, len(spec.Questions))
	for index, question := range spec.Questions {
		cloned.Questions[index] = question
		if len(question.Options) > 0 {
			cloned.Questions[index].Options = append([]QuestionOption(nil), question.Options...)
		}
	}
	return cloned
}

// canUseTool 是 SDK 权限回调（设计 §3.1）。它跑在 SDK 的单个读循环 goroutine 上，
// 任何无上界的等待都会挂死整个 Turn，因此上行与下行都带取消、停止与超时分支。
// 回调可能被并发调用：它只处理自己那一次请求，并发判定归 Controller。
func (h *Host) canUseTool(
	ctx context.Context,
	toolName string,
	input map[string]any,
	permissionContext claudecode.ToolPermissionContext,
) (claudecode.PermissionResult, error) {
	if strings.HasPrefix(toolName, mcpToolPrefix) {
		// MCP 工具由 PC 侧服务端配置背书，直接放行且不修改 input，也不产生
		// pending action。
		return allowResult(input, nil), nil
	}
	request := NewActionRequest(h.buildActionSpec(toolName, input, permissionContext))
	decision, resolved := h.waitForDecision(ctx, request)
	if !resolved {
		// 超时、Turn 取消与 Session 停止都只拒绝本次工具，不中断 Turn。
		return claudecode.NewPermissionResultDeny("操作未获批准：等待用户决议结束"), nil
	}
	return h.permissionResult(decision, input, permissionContext), nil
}

func (h *Host) buildActionSpec(
	toolName string,
	input map[string]any,
	permissionContext claudecode.ToolPermissionContext,
) ActionSpec {
	if toolName == askUserQuestionToolName {
		// 提问不是权限决定，会话级放行对它没有意义。
		return ActionSpec{
			Kind:      ActionAskUserQuestion,
			ToolName:  toolName,
			Questions: parseQuestions(input),
		}
	}
	preview, _ := ToolPreview(toolName, input, h.previewMaxRunes)
	return ActionSpec{
		Kind:     ActionToolApproval,
		ToolName: toolName,
		Preview:  preview,
		// 会话级放行只能复制 SDK 建议，没有建议时该决议不可用。
		AllowSessionAvailable: len(permissionContext.Suggestions) > 0,
	}
}

// waitForDecision 把请求交给上层并等待决议。两个方向都不得无限阻塞（设计 §3.4）。
func (h *Host) waitForDecision(ctx context.Context, request *ActionRequest) (ActionDecision, bool) {
	timer := time.NewTimer(h.actionTimeout)
	defer timer.Stop()
	turnDone := h.currentTurnDone()

	select {
	case h.actions <- request:
	case <-ctx.Done():
		return ActionDecision{}, false
	case <-turnDone:
		return ActionDecision{}, false
	case <-h.stopped:
		return ActionDecision{}, false
	case <-timer.C:
		return ActionDecision{}, false
	}

	select {
	case decision := <-request.Decision():
		return decision, true
	case <-ctx.Done():
		return ActionDecision{}, false
	case <-turnDone:
		return ActionDecision{}, false
	case <-h.stopped:
		return ActionDecision{}, false
	case <-timer.C:
		return ActionDecision{}, false
	}
}

func (h *Host) permissionResult(
	decision ActionDecision,
	input map[string]any,
	permissionContext claudecode.ToolPermissionContext,
) claudecode.PermissionResult {
	switch decision.Kind {
	case DecisionAllowOnce:
		return allowResult(input, nil)
	case DecisionAllowSession:
		scoped := sessionScopedSuggestions(permissionContext.Suggestions)
		if len(scoped) == 0 {
			return claudecode.NewPermissionResultDeny("无法按会话放行：Claude 未提供权限建议")
		}
		return allowResult(input, scoped)
	case DecisionAnswer:
		return allowResult(answeredInput(input, decision.Answers), nil)
	default:
		message := decision.Reason
		if message == "" {
			message = "用户拒绝了该操作"
		}
		deny := claudecode.NewPermissionResultDeny(message)
		deny.Interrupt = decision.Interrupt
		return deny
	}
}

// allowResult 构造放行结果。UpdatedInput 必须是一个非空 record，CLI schema 会
// 拒绝缺失该字段的响应。
func allowResult(input map[string]any, updates []claudecode.PermissionUpdate) claudecode.PermissionResultAllow {
	allow := claudecode.NewPermissionResultAllow()
	allow.UpdatedInput = copyInput(input)
	allow.UpdatedPermissions = updates
	return allow
}

func copyInput(input map[string]any) map[string]any {
	copied := make(map[string]any, len(input))
	for key, value := range input {
		copied[key] = value
	}
	return copied
}

// sessionScopedSuggestions 逐条复制 SDK 建议并把作用域指向当前会话。Adapter 不
// 生成任何自制规则，建议为空时调用方必须退化为拒绝。
func sessionScopedSuggestions(suggestions []claudecode.PermissionUpdate) []claudecode.PermissionUpdate {
	if len(suggestions) == 0 {
		return nil
	}
	scoped := make([]claudecode.PermissionUpdate, 0, len(suggestions))
	for _, suggestion := range suggestions {
		update := suggestion
		destination := permissionDestinationSession
		update.Destination = &destination
		scoped = append(scoped, update)
	}
	return scoped
}

// answeredInput 把协议里的答案数组压成 CLI 需要的扁平 map：键依次取 question、
// header、question_<n>，多选值用 ", " 拼成单个字符串（设计 §3.1）。
func answeredInput(input map[string]any, answers [][]string) map[string]any {
	updated := copyInput(input)
	questions := parseQuestions(input)
	flattened := make(map[string]any, len(questions))
	for index, question := range questions {
		if index >= len(answers) {
			break
		}
		flattened[answerKey(question, index)] = strings.Join(answers[index], ", ")
	}
	updated["answers"] = flattened
	return updated
}

func answerKey(question Question, index int) string {
	if question.Question != "" {
		return question.Question
	}
	if question.Header != "" {
		return question.Header
	}
	return fmt.Sprintf("question_%d", index+1)
}

// parseQuestions 读取 AskUserQuestion 的提问列表。任何一条提问缺少可展示文本时
// 整个列表作废：手机无法安全呈现它，Controller 会按 unsupported 处理。
func parseQuestions(input map[string]any) []Question {
	raw, ok := input["questions"].([]any)
	if !ok {
		return nil
	}
	questions := make([]Question, 0, len(raw))
	for _, entry := range raw {
		fields, ok := entry.(map[string]any)
		if !ok {
			return nil
		}
		question := Question{
			Header:      stringField(fields, "header"),
			Question:    stringField(fields, "question"),
			InputType:   QuestionText,
			MultiSelect: boolField(fields, "multiSelect"),
			Options:     parseQuestionOptions(fields["options"]),
		}
		if question.Header == "" && question.Question == "" {
			return nil
		}
		if len(question.Options) > 0 {
			question.InputType = QuestionChoice
		}
		questions = append(questions, question)
	}
	if len(questions) == 0 {
		return nil
	}
	return questions
}

func parseQuestionOptions(raw any) []QuestionOption {
	entries, ok := raw.([]any)
	if !ok {
		return nil
	}
	options := make([]QuestionOption, 0, len(entries))
	for _, entry := range entries {
		fields, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		label := stringField(fields, "label")
		if label == "" {
			continue
		}
		options = append(options, QuestionOption{Label: label, Description: stringField(fields, "description")})
	}
	if len(options) == 0 {
		return nil
	}
	return options
}

func stringField(fields map[string]any, key string) string {
	value, ok := fields[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func boolField(fields map[string]any, key string) bool {
	value, _ := fields[key].(bool)
	return value
}
