# A3 Session Core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 A2 SDK Adapter 之上实现单 Actor 的 `SessionController`，统一串行处理终端与手机文本输入，提供有界 FIFO、纯数据 snapshot、有序事件和可收敛停止，并把现有 CLI 迁移到异步 Controller。

**Architecture:** `internal/session` 的 Actor 是 Session/Turn 状态、等待队列、request ID 和事件 `seq` 的唯一写入者；唯一 Host Worker 独占调用 `claude.Host.Connect/RunTurn/Close`。CLI 与后续 A4 Channel 只调用 Controller，使用原子“订阅 + snapshot”消费领域事件，不直接接触 SDK。

**Tech Stack:** Go 1.25.3、标准库 `context/crypto/rand/encoding/hex/errors/fmt/io/strings/sync/testing/time`、现有 `internal/claude` Adapter、Go race detector。

---

## 执行约束

- 实施开始前必须读取 `AGENTS.md`、`HANDOFF.md`、`DECISIONS.md`、`REQUIREMENTS.md`、总体设计和 A3 设计，并先更新 `HANDOFF.md` 的“当前执行”。
- 实施必须使用 `superpowers:test-driven-development`；每个行为先运行失败测试，再写最小实现，再运行通过测试。
- writing-plans 建议在独立 worktree 执行，但创建 worktree 属于 Git 写操作。只有用户单独明确授权后才能创建；未获授权时不得自行执行 `git worktree`、分支或其他 Git 写操作。
- 每个 Task 末尾都列出 Git 检查点。`git diff` 可主动运行；`git add`、`git commit` 只在用户对该检查点重新明确授权后运行，一次授权不延伸到后续检查点。
- 不修改 `go.mod.base`、本地 `go.mod` 或依赖；A3 只使用标准库和现有 Adapter。
- 计划中的 `go vet`、`go build` 必须带 `GOFLAGS=-buildvcs=false`。

## 文件地图

| 文件 | 操作 | 职责 |
| --- | --- | --- |
| `internal/session/model.go` | Create | Session/Turn 状态、纯数据 snapshot、稳定错误数据 |
| `internal/session/input.go` | Create | 输入来源、1 MiB 校验、request ID 生成、容量常量 |
| `internal/session/event.go` | Create | A3 领域事件和事件种类 |
| `internal/session/worker.go` | Create | 唯一 Host Worker 与 Adapter 事件回传 |
| `internal/session/controller.go` | Create | Actor 命令、状态所有权、FIFO、订阅和停止 |
| `internal/session/model_test.go` | Create | 枚举、输入校验、request ID 与 snapshot 复制测试 |
| `internal/session/worker_test.go` | Create | Worker Connect/RunTurn/Close 独占测试 |
| `internal/session/controller_test.go` | Create | 状态、FIFO、事件、错误、订阅和停止测试 |
| `cmd/claude-mobile-remote/run.go` | Modify | 异步扫描、Submit、事件输出、EOF 排空和停止 |
| `cmd/claude-mobile-remote/run_test.go` | Modify | 用 fake Controller 替换 fake Host，覆盖 CLI 异步行为 |
| `cmd/claude-mobile-remote/main.go` | Modify | 装配 `session.NewController(claude.NewHost())` |
| `docs/testing/A3-result.md` | Create at gate | A3 自动化门禁证据，不保存会话内容 |
| `HANDOFF.md` | Modify at events | 实施开始、范围变化、门禁与下一步 |

## Task 1：建立领域模型与输入边界（TDD）

**Files:**
- Create: `internal/session/model.go`
- Create: `internal/session/input.go`
- Create: `internal/session/event.go`
- Create: `internal/session/model_test.go`

- [ ] **Step 1.1：写输入校验与 request ID 失败测试**

创建 `internal/session/model_test.go`，测试表和关键断言固定为：

```go
package session

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeInput(t *testing.T) {
	tests := []struct {
		name      string
		currentID string
		input     Input
		wantText  string
		wantErr   error
	}{
		{name: "first terminal input", input: Input{Source: InputTerminal, Text: "  hello  "}, wantText: "hello"},
		{name: "terminal after init may omit target", currentID: "session-1", input: Input{Source: InputTerminal, Text: "next"}, wantText: "next"},
		{name: "mobile exact target", currentID: "session-1", input: Input{SessionID: "session-1", Source: InputMobile, Text: "remote"}, wantText: "remote"},
		{name: "blank", input: Input{Source: InputTerminal, Text: "  "}, wantErr: ErrInvalidInput},
		{name: "too large", input: Input{Source: InputTerminal, Text: strings.Repeat("x", MaxPromptBytes+1)}, wantErr: ErrInvalidInput},
		{name: "unknown source", input: Input{Source: "other", Text: "x"}, wantErr: ErrInvalidInput},
		{name: "mobile before init", input: Input{Source: InputMobile, Text: "x"}, wantErr: ErrSessionUnavailable},
		{name: "mobile missing target", currentID: "session-1", input: Input{Source: InputMobile, Text: "x"}, wantErr: ErrSessionMismatch},
		{name: "wrong target", currentID: "session-1", input: Input{SessionID: "session-2", Source: InputMobile, Text: "x"}, wantErr: ErrSessionMismatch},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeInput(tt.currentID, tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("normalizeInput() error = %v, want %v", err, tt.wantErr)
			}
			if err == nil && got.Text != tt.wantText {
				t.Fatalf("normalized text = %q, want %q", got.Text, tt.wantText)
			}
		})
	}
}

func TestRandomRequestID(t *testing.T) {
	id, err := randomRequestID()
	if err != nil {
		t.Fatalf("randomRequestID() error = %v", err)
	}
	if len(id) != 32 {
		t.Fatalf("request ID length = %d, want 32", len(id))
	}
	for _, r := range id {
		if !strings.ContainsRune("0123456789abcdef", r) {
			t.Fatalf("request ID is not lowercase hex: %q", id)
		}
	}
}
```

- [ ] **Step 1.2：运行测试确认 RED**

Run: `go test ./internal/session -run 'TestNormalizeInput|TestRandomRequestID' -v`

Expected: FAIL，包含 `package .../internal/session is not in std` 或 `undefined: Input`。

- [ ] **Step 1.3：实现状态、snapshot、输入和事件类型**

`model.go` 必须定义这些精确公开类型和值：

```go
package session

type SessionStatus string

const (
	SessionStarting SessionStatus = "starting"
	SessionReady    SessionStatus = "ready"
	SessionStopped  SessionStatus = "stopped"
	SessionFailed   SessionStatus = "failed"
)

type TurnStatus string

const (
	TurnIdle      TurnStatus = "idle"
	TurnRunning   TurnStatus = "running"
	TurnWaiting   TurnStatus = "waiting"
	TurnCompleted TurnStatus = "completed"
	TurnCancelled TurnStatus = "cancelled"
	TurnFailed    TurnStatus = "failed"
)

type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type PendingAction struct{}

type TurnSnapshot struct {
	Status        TurnStatus
	RequestID     string
	Source        InputSource
	Message       string
	Error         *ErrorInfo
	PendingAction *PendingAction
}

type Snapshot struct {
	SessionID     string
	SessionStatus SessionStatus
	SessionError  *ErrorInfo
	Seq           uint64
	Turn          TurnSnapshot
	QueueDepth    int
}
```

`input.go` 必须实现统一校验和无依赖随机 ID：

```go
package session

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

const (
	QueueCapacity = 32
	MaxPromptBytes = 1024 * 1024
)

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrQueueFull          = errors.New("input queue full")
	ErrSessionMismatch    = errors.New("session ID mismatch")
	ErrSessionUnavailable = errors.New("session unavailable")
	ErrInternal           = errors.New("internal error")
)

type InputSource string

const (
	InputTerminal InputSource = "terminal"
	InputMobile   InputSource = "mobile"
)

type Input struct {
	SessionID string
	Source    InputSource
	Text      string
}

type requestIDGenerator func() (string, error)

func randomRequestID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("%w: generate request ID: %v", ErrInternal, err)
	}
	return hex.EncodeToString(raw), nil
}

func normalizeInput(currentID string, input Input) (Input, error) {
	if input.Source != InputTerminal && input.Source != InputMobile {
		return Input{}, fmt.Errorf("%w: unsupported source %q", ErrInvalidInput, input.Source)
	}
	input.Text = strings.TrimSpace(input.Text)
	if input.Text == "" || len(input.Text) > MaxPromptBytes {
		return Input{}, fmt.Errorf("%w: prompt must contain 1..%d bytes", ErrInvalidInput, MaxPromptBytes)
	}
	if input.Source == InputMobile && currentID == "" {
		return Input{}, ErrSessionUnavailable
	}
	if currentID == "" && input.SessionID != "" {
		return Input{}, ErrSessionMismatch
	}
	if currentID != "" && input.SessionID != "" && input.SessionID != currentID {
		return Input{}, ErrSessionMismatch
	}
	if input.Source == InputMobile && input.SessionID == "" {
		return Input{}, ErrSessionMismatch
	}
	return input, nil
}
```

`event.go` 必须定义：

```go
package session

type EventKind string

const (
	EventSessionStarted EventKind = "session_started"
	EventTurnStarted    EventKind = "turn_started"
	EventTextDelta      EventKind = "text_delta"
	EventTurnCompleted  EventKind = "turn_completed"
	EventTurnFailed     EventKind = "turn_failed"
	EventSessionFailed  EventKind = "session_failed"
	EventSessionStopped EventKind = "session_stopped"
)

type Event struct {
	Seq           uint64
	Kind          EventKind
	SessionID     string
	RequestID     string
	Source        InputSource
	Text          string
	SessionStatus SessionStatus
	TurnStatus    TurnStatus
	Error         *ErrorInfo
}
```

- [ ] **Step 1.4：格式化并确认 GREEN**

Run: `gofmt -w internal/session/model.go internal/session/input.go internal/session/event.go internal/session/model_test.go`

Run: `go test ./internal/session -run 'TestNormalizeInput|TestRandomRequestID' -v`

Expected: 两个测试 PASS。

- [ ] **Step 1.5：检查差异并停在 Git 授权点**

Run: `git diff --check`

Expected: 无输出，退出 0。

只有用户明确授权本检查点后才运行：

```bash
git add internal/session/model.go internal/session/input.go internal/session/event.go internal/session/model_test.go
git commit -m "feat: define A3 session domain model"
```

## Task 2：建立唯一 Host Worker（TDD）

**Files:**
- Create: `internal/session/worker.go`
- Create: `internal/session/worker_test.go`

- [ ] **Step 2.1：写 Worker 独占生命周期失败测试**

测试文件定义 `workerFakeHost`，其 `RunTurn` 用 channel 暂停并记录最大并发数。关键测试结构：

```go
func TestWorkerOwnsHostLifecycleAndSerializesTurns(t *testing.T) {
	host := newWorkerFakeHost()
	worker := newHostWorker(host)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	worker.start(ctx)
	if err := <-worker.connected; err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	first := workerRequest{RequestID: "request-1", Prompt: "first", Context: context.Background()}
	second := workerRequest{RequestID: "request-2", Prompt: "second", Context: context.Background()}
	worker.requests <- first
	host.waitStarted(t, "first")
	select {
	case worker.requests <- second:
		t.Fatal("worker accepted a second request while RunTurn was active")
	default:
	}
	host.finish(nil)
	if got := <-worker.results; got.RequestID != "request-1" || got.Err != nil {
		t.Fatalf("first result = %#v", got)
	}
	worker.requests <- second
	host.waitStarted(t, "second")
	host.finish(nil)
	<-worker.results
	close(worker.stop)
	if err := <-worker.closed; err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if host.connectCalls != 1 || host.closeCalls != 1 || host.maxActive != 1 {
		t.Fatalf("connect/close/maxActive = %d/%d/%d", host.connectCalls, host.closeCalls, host.maxActive)
	}
}
```

同文件增加 `TestWorkerForwardsAdapterEventsWithRequestID`，让 fake Host 发出 `projectclaude.Event{Kind: projectclaude.EventText, Text: "chunk"}`，断言 `worker.events` 同时保留 `RequestID == "request-1"` 和原 Adapter 事件。

- [ ] **Step 2.2：运行测试确认 RED**

Run: `go test ./internal/session -run '^TestWorker' -v`

Expected: FAIL，包含 `undefined: newHostWorker`。

- [ ] **Step 2.3：实现 Worker 的窄 Host 边界**

`worker.go` 使用这些精确类型和循环：

```go
package session

import (
	"context"

	projectclaude "github.com/tea4go/claude-mobile-remote/internal/claude"
)

type Host interface {
	Connect(context.Context) error
	RunTurn(context.Context, string, func(projectclaude.Event)) error
	Close() error
}

type workerRequest struct {
	RequestID string
	Prompt    string
	Context   context.Context
}

type workerEvent struct {
	RequestID string
	Adapter   projectclaude.Event
}

type workerResult struct {
	RequestID string
	Err       error
}

type hostWorker struct {
	host      Host
	requests  chan workerRequest
	events    chan workerEvent
	results   chan workerResult
	connected chan error
	stop      chan struct{}
	closed    chan error
}

func newHostWorker(host Host) *hostWorker {
	return &hostWorker{
		host: host, requests: make(chan workerRequest), events: make(chan workerEvent),
		results: make(chan workerResult), connected: make(chan error, 1),
		stop: make(chan struct{}), closed: make(chan error, 1),
	}
}

func (w *hostWorker) start(ctx context.Context) { go w.run(ctx) }

func (w *hostWorker) run(ctx context.Context) {
	if err := w.host.Connect(ctx); err != nil {
		w.connected <- err
		w.closed <- w.host.Close()
		return
	}
	w.connected <- nil
	for {
		select {
		case request := <-w.requests:
			err := w.host.RunTurn(request.Context, request.Prompt, func(event projectclaude.Event) {
				w.events <- workerEvent{RequestID: request.RequestID, Adapter: event}
			})
			w.results <- workerResult{RequestID: request.RequestID, Err: err}
		case <-w.stop:
			w.closed <- w.host.Close()
			return
		}
	}
}
```

Worker 的 event/result channel 不丢数据；停止时由 Controller 先取消当前 Turn，使阻塞的 `RunTurn` 返回，然后 Worker 才能读取 `stop`。

- [ ] **Step 2.4：格式化并确认 GREEN**

Run: `gofmt -w internal/session/worker.go internal/session/worker_test.go`

Run: `go test -race ./internal/session -run '^TestWorker' -v`

Expected: Worker 测试 PASS，race detector 无报告。

- [ ] **Step 2.5：检查差异并停在 Git 授权点**

Run: `git diff --check`

只有用户明确授权本检查点后才运行：

```bash
git add internal/session/worker.go internal/session/worker_test.go
git commit -m "feat: add serialized Claude host worker"
```

## Task 3：实现 Actor 启动、Submit 和有界 FIFO（TDD）

**Files:**
- Create: `internal/session/controller.go`
- Create: `internal/session/controller_test.go`

- [ ] **Step 3.1：建立可控 fake Host 测试夹具**

`controller_test.go` 的 fake Host 必须让测试逐个控制 Adapter event 与 Turn 返回：

```go
type controlledTurn struct {
	Prompt string
	Events chan projectclaude.Event
	Result chan error
}

type controlledHost struct {
	connectErr error
	closeErr   error
	started    chan *controlledTurn
	closed     chan struct{}
	mu         sync.Mutex
	active     int
	maxActive  int
	closeCalls int
}

func newControlledHost() *controlledHost {
	return &controlledHost{started: make(chan *controlledTurn, QueueCapacity+1), closed: make(chan struct{})}
}

func (h *controlledHost) Connect(context.Context) error { return h.connectErr }

func (h *controlledHost) RunTurn(ctx context.Context, prompt string, emit func(projectclaude.Event)) error {
	h.mu.Lock()
	h.active++
	if h.active > h.maxActive { h.maxActive = h.active }
	h.mu.Unlock()
	defer func() { h.mu.Lock(); h.active--; h.mu.Unlock() }()
	turn := &controlledTurn{Prompt: prompt, Events: make(chan projectclaude.Event), Result: make(chan error, 1)}
	h.started <- turn
	for {
		select {
		case event := <-turn.Events:
			emit(event)
		case err := <-turn.Result:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (h *controlledHost) Close() error {
	h.mu.Lock()
	h.closeCalls++
	h.mu.Unlock()
	select { case <-h.closed: default: close(h.closed) }
	return h.closeErr
}
```

- [ ] **Step 3.2：写启动、FIFO 和容量失败测试**

实现以下三个精确测试：

1. `TestControllerStartAndInitialSnapshot`：`Start` 后 snapshot 为 `SessionStarting`、Turn 为 `TurnIdle`、`Seq == 0`、`QueueDepth == 0`；Connect error 时 `Start` 返回原错误且最终 snapshot 为 `SessionFailed`，`SessionError.Code == "session_failed"`。
2. `TestControllerSubmitSerialFIFO`：注入依次返回 `request-01/request-02/request-03` 的 ID generator；提交 `first/second/third`，断言 fake Host 只先启动 `first`，依次结束后才启动 `second/third`，`maxActive == 1`。
3. `TestControllerQueueCapacity`：保持首 Turn 运行，成功提交 32 个等待项，snapshot `QueueDepth == 32`；下一次 Submit 返回 `ErrQueueFull` 且空 request ID。
4. `TestControllerRequestIDFailureDoesNotEnqueue`：注入返回 `fmt.Errorf("%w: entropy", ErrInternal)` 的 generator，断言 Submit 返回空 ID、`errors.Is(err, ErrInternal)`、QueueDepth 仍为 0、Host 未启动 Turn。

初始和所有 A3 snapshot 都必须断言 `Turn.PendingAction == nil`；A3 不创建 `waiting` 状态或非空 pending action。

顺序测试的 ID 注入固定为：

```go
next := 0
controller := NewController(host, withRequestIDGenerator(func() (string, error) {
	next++
	return fmt.Sprintf("request-%02d", next), nil
}))
```

- [ ] **Step 3.3：运行测试确认 RED**

Run: `go test ./internal/session -run 'TestController(Start|Submit|Queue)' -v`

Expected: FAIL，包含 `undefined: NewController`。

- [ ] **Step 3.4：定义 Controller API、命令和 Actor state**

`controller.go` 的公开面必须与设计一致：

```go
type Option func(*Controller)

func withRequestIDGenerator(generator requestIDGenerator) Option {
	return func(controller *Controller) { controller.generateRequestID = generator }
}

func NewController(host Host, options ...Option) *Controller
func (c *Controller) Start(ctx context.Context) error
func (c *Controller) Submit(ctx context.Context, input Input) (string, error)
func (c *Controller) Snapshot(ctx context.Context) (Snapshot, error)
func (c *Controller) WaitIdle(ctx context.Context) (Snapshot, error)
func (c *Controller) Subscribe(ctx context.Context, capacity int) (Snapshot, <-chan Event, error)
func (c *Controller) Stop(ctx context.Context) error
```

Actor state 必须集中为一个仅由 `run` goroutine 访问的结构：

```go
type actorState struct {
	snapshot             Snapshot
	queue                []queuedInput
	current              *queuedInput
	currentCancel         context.CancelFunc
	currentTerminal      bool
	turnStartedPublished bool
	preSessionEvents     []workerEvent
	subscribers          map[uint64]subscriber
	nextSubscriberID     uint64
	stopping             bool
	workerClosed         bool
	workerCloseErr       error
}

type queuedInput struct {
	RequestID string
	Input     Input
}
```

Actor command channel使用 `submitCommand`、`snapshotCommand`、`waitIdleCommand`、`subscribeCommand`、`unsubscribeCommand` 和 `stopCommand`。每个公开方法只负责把命令送入 Actor 并等待对应 response；不得读取或写入 `actorState`。

`NewController` 初始化 Session=`starting`、Turn=`idle` 和全部内部 channel；`Start` 只允许成功调用一次，启动 Actor 与 Worker并等待 Connect 结果。Connect 失败时设置 Session=`failed` 和顶层 `SessionError`，缓存失败 snapshot，Worker 完成 Close；Start 使用的 service context 后续取消时必须向 Actor 发起与显式 Stop 相同的命令，不能直接结束 Actor goroutine。

Task 3 同时实现基础 `Subscribe`：容量必须大于 0，Actor 在一个 command 内登记 buffered channel 并返回同一时点 snapshot，使 Task 4 可以观察事件。订阅 context 取消、最终 channel 关闭和并发 Stop 的完整收敛在 Task 5 加测试并固化。

`submitCommand.response` 使用无缓冲 channel；`Submit` 一旦成功把 command 交给 Actor，就等待该 response 而不再因调用方 context 提前放弃。这样 `command.response <- ...` 与 `Submit` goroutine 接收形成内部握手，Actor 随后才调用 `startNext`。该机制只保证接受结果已交给 `Submit` goroutine，不承诺 Go 方法已返回或外部调用方已完成登记；A3 不增加可执行回调或二阶段确认。

- [ ] **Step 3.5：实现 Submit 校验、接受握手与 startNext**

`handleSubmit` 的顺序必须固定为：

```go
func (c *Controller) handleSubmit(state *actorState, command submitCommand) {
	if state.stopping || state.snapshot.SessionStatus == SessionStopped || state.snapshot.SessionStatus == SessionFailed {
		command.response <- submitResponse{Err: ErrSessionUnavailable}
		return
	}
	input, err := normalizeInput(state.snapshot.SessionID, command.input)
	if err != nil {
		command.response <- submitResponse{Err: err}
		return
	}
	if state.current != nil && len(state.queue) >= QueueCapacity {
		command.response <- submitResponse{Err: ErrQueueFull}
		return
	}
	requestID, err := c.generateRequestID()
	if err != nil {
		command.response <- submitResponse{Err: err}
		return
	}
	state.queue = append(state.queue, queuedInput{RequestID: requestID, Input: input})
	state.snapshot.QueueDepth = len(state.queue)
	command.response <- submitResponse{RequestID: requestID}
	c.startNext(state)
}
```

A4 Channel 必须单独保证网络可观察顺序：移动命令的 `command_result` 进入该连接的有序输出队列后，才放行同一 request ID 的 `turn_started`；若 Controller 事件提前到达，则在 PC Service 内短暂缓冲，不要求 Web、Capacitor 或 HarmonyOS 客户端处理乱序。该要求不改变 Task 3 的 Controller API，并在 D25 记录三端可实现性。

`startNext` 只能在 `current == nil`、队列非空且 Session 非终态时执行：取队首、更新 `QueueDepth`，把 Turn 设为 `running`，清空上一 Turn 的 message/error/pendingAction，创建独立 Turn context；真实 session ID 已存在时先发布 `turn_started`，随后把 `workerRequest` 发送给 Worker。首 Turn尚无 ID 时只更新内部状态，事件在 Task 4 延后发布。

- [ ] **Step 3.6：确认 FIFO 与容量 GREEN**

Run: `gofmt -w internal/session/controller.go internal/session/controller_test.go`

Run: `go test -race ./internal/session -run 'TestController(Start|Submit|Queue)' -v`

Expected: 三组测试 PASS；容量精确为 1 个运行中加 32 个等待中；race detector 无报告。

- [ ] **Step 3.7：增加并发 Submit 验证**

增加 `TestControllerConcurrentSubmitIsSerialized`：首 Turn保持运行，再从 16 个 goroutine 提交唯一 prompt；按注入生成器产生的 request ID 数字排序期望 prompt，并逐 Turn释放 fake Host，断言执行顺序与 request ID 分配顺序一致、ID 无重复、`maxActive == 1`。

Run: `go test -race ./internal/session -run TestControllerConcurrentSubmitIsSerialized -count=20 -v`

Expected: 20 次全部 PASS，无 race。

- [ ] **Step 3.8：检查差异并停在 Git 授权点**

Run: `git diff --check`

只有用户明确授权本检查点后才运行：

```bash
git add internal/session/controller.go internal/session/controller_test.go
git commit -m "feat: add bounded session input actor"
```

## Task 4：映射 Adapter 事件、状态与错误 scope（TDD）

**Files:**
- Modify: `internal/session/controller.go`
- Modify: `internal/session/controller_test.go`

- [ ] **Step 4.1：写事件顺序和 snapshot 失败测试**

增加 `TestControllerFirstTurnEventOrderAndSnapshot`：

1. 原子订阅 Controller；提交 `first`；
2. fake Host 先发 `projectclaude.Event{Kind: projectclaude.EventText, Text: "pre"}`，此时订阅 channel 不得收到领域事件；
3. 再发 `projectclaude.Event{Kind: projectclaude.EventSessionStarted, SessionID: "session-1"}`、`projectclaude.Event{Kind: projectclaude.EventText, Text: "post"}`、`projectclaude.Event{Kind: projectclaude.EventTurnCompleted}` 并返回 nil；
4. 断言领域事件 kind 精确为 `session_started, turn_started, text_delta, text_delta, turn_completed`，`seq` 为 `1..5`，所有事件 session ID 均为 `session-1`；
5. 最终 snapshot 为 Session `ready`、Turn `completed`、message `prepost`、queueDepth 0、seq 5；
6. 提交第二 Turn 后 snapshot 立即为 `running` 且 message/error 已清空。

- [ ] **Step 4.2：写 Turn/Session 错误失败测试**

增加：

- `TestControllerTurnFailureContinuesQueue`：首 Turn 发 `turn_failed` 并返回可判定为 `projectclaude.ScopeTurn` 的错误，断言 Turn failed 事件只发布一次，Session 保持 ready，第二队列项随后启动；
- `TestControllerSessionFailureClearsQueue`：当前 Turn 返回 `projectclaude.ScopeSession` 错误，断言 Session failed、顶层 `SessionError.Code == "session_failed"`、QueueDepth 0、后续 Submit 为 `ErrSessionUnavailable`；
- `TestControllerSynthesizesMissingTerminal`：Host nil 返回但未发 completed/failed，保守进入 Session failed；返回 Turn scope 且无 Adapter 终态时补一个 `turn_failed`；
- `TestControllerPreInitFailureIsLocalOnly`：init 前 Turn scope 失败可以发布 session ID 为空的内部 `turn_failed` 并继续下一项；测试不得建立任何网络 envelope。

测试用 scope error：

```go
type scopedTestError struct { scope projectclaude.ErrorScope; err error }
func (e scopedTestError) Error() string { return e.err.Error() }
func (e scopedTestError) Unwrap() error { return e.err }
func (e scopedTestError) Scope() projectclaude.ErrorScope { return e.scope }
```

- [ ] **Step 4.3：运行测试确认 RED**

Run: `go test ./internal/session -run 'TestController(FirstTurn|TurnFailure|SessionFailure|Synthesizes|PreInit)' -v`

Expected: FAIL，首个失败应表现为未收到事件、状态未转换或 `seq` 不匹配。

- [ ] **Step 4.4：实现唯一 seq 分配和 Adapter 事件映射**

实现 `publish`，并保证只有 Actor 调用：

```go
func (c *Controller) publish(state *actorState, event Event) {
	state.snapshot.Seq++
	event.Seq = state.snapshot.Seq
	for id, subscriber := range state.subscribers {
		select {
		case subscriber.events <- event:
		case <-subscriber.ctx.Done():
			close(subscriber.events)
			delete(state.subscribers, id)
		}
	}
}
```

Adapter 映射规则固定为：

| Adapter 事实 | Actor 更新 | 领域事件 |
| --- | --- | --- |
| `EventSessionStarted` | 设置真实 ID，Session=`ready` | `session_started`；若当前 Turn 尚未发布 start，紧接 `turn_started`；再重放预 init 文本 |
| `EventText` | 追加 `Turn.Message` | `text_delta` |
| `EventTurnCompleted` | Turn=`completed`，记录 terminal | `turn_completed` |
| `EventTurnFailed` | Turn=`failed`，Error code=`turn_failed`，记录 terminal | `turn_failed` |

- [ ] **Step 4.5：实现 Worker result 的终态去重和错误 scope**

Worker result 处理固定为：

```go
switch {
case result.Err == nil && !state.currentTerminal:
	c.failSession(state, errors.New("Host returned without a terminal event"))
case result.Err != nil && projectclaude.ScopeOf(result.Err) == projectclaude.ScopeSession:
	c.failSession(state, result.Err)
case result.Err != nil && !state.currentTerminal:
	c.failTurn(state, result.Err)
}
```

随后释放当前执行槽；Session 仍为 `starting/ready` 时调用 `startNext`。`failSession` 必须设置顶层 `SessionError`、把运行中 Turn 置 failed、清空队列并取消当前 Turn；已经发布 `turn_failed` 后只再发布一个 `session_failed`，不得重复 Turn 终态。

所有返回 snapshot 的路径必须复制 `SessionError`、`Turn.Error` 指针，防止调用方修改 Actor 内部对象。Error code 固定为 `turn_failed`、`session_failed`、`session_stopped`，message 使用包装后的可诊断错误文本。

- [ ] **Step 4.6：确认状态与错误 GREEN**

Run: `gofmt -w internal/session/controller.go internal/session/controller_test.go`

Run: `go test -race ./internal/session -run 'TestController(FirstTurn|TurnFailure|SessionFailure|Synthesizes|PreInit)' -v`

Expected: 全部 PASS，终态事件不重复，Session scope 后不启动下一项。

- [ ] **Step 4.7：检查差异并停在 Git 授权点**

Run: `git diff --check`

只有用户明确授权本检查点后才运行：

```bash
git add internal/session/controller.go internal/session/controller_test.go
git commit -m "feat: map Claude events into session state"
```

## Task 5：固化原子订阅与可收敛停止（TDD）

**Files:**
- Modify: `internal/session/controller.go`
- Modify: `internal/session/controller_test.go`

- [ ] **Step 5.1：写订阅一致性失败测试**

增加 `TestControllerSubscribeSnapshotHasNoGap`：订阅 A 取得 seq 0，首 Turn产生 session_started；此时订阅 B，断言 B 的 snapshot seq 等于 A 已见最大 seq，之后 A/B 接收完全相同的更大 seq；取消 B context 后其 channel 关闭，A 继续接收。

增加 `TestControllerRejectsZeroSubscriptionCapacity`：`Subscribe(ctx, 0)` 返回 `ErrInvalidInput`，不登记订阅者。

- [ ] **Step 5.2：写 Stop 失败测试**

增加：

- `TestControllerStopCancelsRunningTurn`：当前 Turn运行时调用 Stop，fake Host 观察 context cancellation，snapshot 最终 Session stopped、Turn cancelled、QueueDepth 0，`session_stopped` 是最后事件，Host Close 一次；
- `TestControllerStopIsIdempotent`：顺序和并发调用 Stop 均返回同一 Close 结果，Host CloseCalls 始终为 1；
- `TestControllerStopAfterFailurePreservesFailed`：Session 已 failed 后 Stop 只做资源收敛，snapshot 仍 failed，且不发布 session_stopped；
- `TestControllerServiceContextCancellationUsesStopPath`：取消 Start 使用的 service context，与显式 Stop 得到相同 stopped/cancelled 结果。

增加结算边界测试：

- `TestControllerWaitIdleWaitsForWorkerResult`：Adapter 已发布 Turn 终态但 fake Host 尚未返回 Worker result 时，`WaitIdle` 不得提前返回；释放 result 后返回无当前 Turn、无队列的 Actor 一致 snapshot；
- `TestControllerWaitIdleReturnsFailedAfterSessionResult`：Turn 终态后返回 Session scope 错误，`WaitIdle` 返回保留根因的最终 failed snapshot。

- [ ] **Step 5.3：运行测试确认 RED**

Run: `go test ./internal/session -run 'TestController(Subscribe|RejectsZero|Stop|ServiceContext)' -v`

Expected: FAIL，表现为订阅缺口、Stop 未取消 Turn、重复 Close 或状态错误。

- [ ] **Step 5.4：实现原子订阅、取消订阅和 snapshot 基线**

`Subscribe` 必须把“登记订阅者”和“生成 snapshot”放在同一个 Actor command 中；返回的 channel 容量使用调用方参数。启动一个只等待订阅 context 的轻量 goroutine，取消时发送 `unsubscribeCommand`；Actor 关闭并删除该 channel。

- [ ] **Step 5.5：实现 Stop 状态收敛和幂等 Close**

停止顺序必须严格实现：

```text
set internal stopping flag
→ reject new Submit
→ clear waiting queue
→ mark running Turn cancelled
→ cancel current Turn context
→ wait worker result
→ close worker.stop exactly once
→ wait worker.closed
→ preserve failed OR set stopped
→ publish session_stopped only for active stop
→ close subscriber channels
→ answer every Stop waiter with the same Close error
→ close Controller done channel
```

使用 `sync.Once` 保护 Worker stop channel 和 Controller done channel；不要新增公开 `stopping` Session 枚举。`Stop` 在 command 已成功送入 Actor 后使用容量 1 的 response channel，因此调用方 context 超时不会阻塞 Actor 回写最终结果。Controller 已结束时，`Stop` 直接返回缓存的 Close 错误；`Snapshot` 返回缓存的最终深复制 snapshot。

`WaitIdle` 使用 Actor command 登记容量 1 的 waiter。只有 `current == nil && len(queue) == 0` 或 Session 已终止时才以深复制 snapshot 回应；Adapter 的 Turn 终态事件本身不得唤醒 waiter，必须由 `handleWorkerResult` 完成状态结算后统一唤醒。Controller 已结束时直接返回缓存的最终 snapshot。该方法不发布事件、不修改 `seq`，也不进入任何网络接口。

- [ ] **Step 5.6：确认订阅、停止和全包 race GREEN**

Run: `gofmt -w internal/session/controller.go internal/session/controller_test.go`

Run: `go test -race ./internal/session -count=10`

Expected: 10 次全包 PASS，无 goroutine 泄漏超时、send on closed channel 或 race 报告。

- [ ] **Step 5.7：检查差异并停在 Git 授权点**

Run: `git diff --check`

只有用户明确授权本检查点后才运行：

```bash
git add internal/session/controller.go internal/session/controller_test.go
git commit -m "feat: add session subscriptions and shutdown"
```

## Task 6：把 CLI 迁移到异步 Controller（TDD）

**Files:**
- Modify: `cmd/claude-mobile-remote/run.go`
- Modify: `cmd/claude-mobile-remote/run_test.go`
- Modify: `cmd/claude-mobile-remote/main.go`

- [ ] **Step 6.1：用 fake Controller 替换 CLI fake Host**

测试接口必须与生产调用面一致：

```go
type sessionController interface {
	Start(context.Context) error
	Submit(context.Context, projectsession.Input) (string, error)
	WaitIdle(context.Context) (projectsession.Snapshot, error)
	Subscribe(context.Context, int) (projectsession.Snapshot, <-chan projectsession.Event, error)
	Stop(context.Context) error
}
```

`fakeController` 记录 inputs，按顺序返回 request ID，并暴露容量足够的 `events chan projectsession.Event`。`Stop` 只关闭一次并记录 stopCalls；Start/Subscribe/Stop 可分别配置错误。

- [ ] **Step 6.2：写异步输入、EOF 和停止失败测试**

将旧同步 Host 测试替换为以下精确用例：

1. `TestRunSubmitsLinesWithoutWaitingForTurn`：输入 `first\nsecond\n`，在发送任何 completed event 前观察到两个 Submit；随后发送 session_started、两个 Turn 的 text/completed，断言输出和 pending 收敛；
2. `TestRunEOFWaitsForAcceptedRequests`：EOF 后 50ms 内 run 不得返回；发送对应 terminal event 后退出码 0；
3. `TestRunQuitStopsImmediately`：`:quit` 后不等待 queued terminal event，StopCalls 1；
4. `TestRunReportsSubmitRejectionAndContinues`：第一次 Submit 返回 `ErrQueueFull`，第二次成功，stderr 含“输入拒绝”，最终退出码 0；
5. `TestRunTurnFailureContinues`：turn_failed 输出错误并继续等待第二请求；
6. `TestRunSessionFailureExitsOne`：session_failed 后退出码 1，stderr 含 message；
7. `TestRunCancellationClosesBlockingReader`：沿用 `blockingReadCloser`，context cancel 后退出码 0、StopCalls 1；
8. `TestRunReportsStartSubscribeScannerAndStopErrors`：分别断言启动、订阅、扫描和关闭错误的退出码及主错误保留；
9. `TestRunAcceptsPromptLargerThanScannerDefault`：70 KiB prompt 仍成功 Submit；1 MiB 加 1 byte 的单行输入由 scanner 报“读取输入错误”并以退出码 1 结束。

质量回归补充：

10. `TestRunEOFWaitsForControllerSettlement`：使用真实 Controller 与可控 Host，让 Turn 终态事件先于 Session scope Worker result 到达；断言 EOF 不提前成功退出，最终只按 failed snapshot 报 Session 错误并返回 1；Turn scope result 则正常返回 0；
11. `TestRunCancelsSubscriptionBeforeStop`：fake Controller 的 Stop 等待 Subscribe context 取消，`:quit`、scanner error 和 EOF 返回路径均不得死锁；
12. `TestRunContextCancellationWinsErrorRaces`：context 与 Start、Subscribe、scanner、事件 channel 关闭及 `session_failed` 竞态时均返回 0 且不输出伪失败。

- [ ] **Step 6.3：运行 CLI 测试确认 RED**

Run: `go test ./cmd/claude-mobile-remote -run TestRun -v`

Expected: FAIL，因为现有 `run` 仍要求同步 `turnHost`。

- [ ] **Step 6.4：实现 CLI 的异步 select、pending 与 EOF 排空**

`run.go` 保留 `scanLines`，但 `run` 改为：

```go
func run(ctx context.Context, input io.Reader, output, errorOutput io.Writer, controller sessionController) (exitCode int)
```

执行顺序固定为：

1. defer 使用 `context.Background()` 调用幂等 `Stop`，Close 错误在原成功时把退出码改为 1，在已有主错误时追加输出；
2. `Start(ctx)`；
3. 创建独立 subscription context 并调用 `Subscribe(subscriptionCtx, 64)`；退出事件循环后先取消该订阅，再调用 `Stop`；
4. 启动 scanner channel；
5. 用一个 select 同时处理 `ctx.Done()`、扫描行和领域事件；
6. Submit 成功后把 request ID 放入 `map[string]bool`；
7. `text_delta` 直接输出并标记该 request 已输出文本；`turn_completed/turn_failed` 删除 pending，完成时换行，失败时输出 Error message；
8. EOF 只把扫描 channel 设为 nil；pending 清空后调用 `WaitIdle(ctx)`，依据 Actor 已结算的最终 snapshot 决定成功或 Session failure，不能只凭 Turn 终态成功退出；Turn failure 后 pending 已空时同样调用 `WaitIdle` 区分可恢复 Turn 与 Session failure，仍有 pending 时暂存 Turn 诊断，直到下一条 `turn_started` 证明上一 Turn 可恢复，若先收到 `session_failed` 则只输出 Session 根因；`:quit` 立即返回并由 defer 先取消订阅、再 Stop；
9. `session_failed` 返回 1；`session_stopped` 正常返回；事件 channel 意外关闭且仍有 pending 时返回 1。

所有错误出口在诊断前先检查 `ctx.Err()`；若进程 context 已取消，Start、Subscribe、scanner、事件关闭、WaitIdle 或失败事件都按用户主动停止处理，不输出错误且返回 0。

- [ ] **Step 6.5：切换 main.go 的真实 Controller 装配**

`main.go` 的真实装配改为：

```go
controller := projectsession.NewController(projectclaude.NewHost())
exitCode := run(ctx, os.Stdin, os.Stdout, os.Stderr, controller)
```

CLI 不保留 `turnHost`，不直接调用 `RunTurn`，也不从 `Host.SessionID()` 读取状态。

- [ ] **Step 6.6：格式化并确认 CLI GREEN**

Run: `gofmt -w cmd/claude-mobile-remote/run.go cmd/claude-mobile-remote/run_test.go cmd/claude-mobile-remote/main.go`

Run: `go test -race ./cmd/claude-mobile-remote -run TestRun -v`

Expected: 全部 CLI 测试 PASS，无 race。

Run: `go run ./cmd/claude-mobile-remote -version`

Expected: `0.1.0-dev`，不连接 Claude。

- [ ] **Step 6.7：检查差异并停在 Git 授权点**

Run: `git diff --check`

只有用户明确授权本检查点后才运行：

```bash
git add cmd/claude-mobile-remote/run.go cmd/claude-mobile-remote/run_test.go cmd/claude-mobile-remote/main.go
git commit -m "feat: route CLI through session controller"
```

## Task 7：A3 全量门禁、记录与交接

**Files:**
- Create: `docs/testing/A3-result.md`
- Modify: `HANDOFF.md`

- [ ] **Step 7.1：运行 focused race 门禁**

Run: `go test -race ./internal/session ./cmd/claude-mobile-remote`

Expected: 两个包全部 PASS，无 race。

- [ ] **Step 7.2：运行全量自动化门禁**

Run: `go test -race ./...`

Expected: 所有非 integration 测试 PASS；真实 Claude 测试因 build tag 不参与。

Run: `gofmt -l .`

Expected: 无输出。

Run: `GOFLAGS=-buildvcs=false go vet ./...`

Expected: 无输出，退出 0。

Run: `GOFLAGS=-buildvcs=false go build ./...`

Expected: 无输出，退出 0。

- [ ] **Step 7.3：运行重复并发稳定性门禁**

Run: `go test -race ./internal/session -run 'TestController(ConcurrentSubmit|QueueCapacity|Stop|Subscribe)' -count=50`

Expected: 50 次全部 PASS，无超时、panic 或 race。

- [ ] **Step 7.4：创建 A3 门禁记录**

若 §7.1–§7.3 全部通过，创建以下确定内容的 `docs/testing/A3-result.md`；执行日期与实际耗时可以追加，但不得改写命令和结论：

```markdown
# A3 Session Core 门禁记录

- 日期：2026-08-24
- 子阶段：A3 Session Core
- 结论：通过

## 自动验证

| 命令 | 结果 |
| --- | --- |
| `go test -race ./internal/session ./cmd/claude-mobile-remote` | PASS |
| `go test -race ./...` | PASS |
| `gofmt -l .` | PASS（无输出） |
| `GOFLAGS=-buildvcs=false go vet ./...` | PASS（无输出） |
| `GOFLAGS=-buildvcs=false go build ./...` | PASS（无输出） |
| Controller 重点并发用例 `-count=50` | PASS |

## 设计核对

- Host 最大并发调用数为 1；
- 等待容量为 32，运行中 Turn 不计入；
- Turn scope 失败后继续，Session scope 失败后清队列；
- snapshot、原子订阅和 seq 无缺口；
- Stop 取消当前 Turn，Host 只关闭一次；
- CLI EOF 排空，主动停止立即收敛；
- 未实现 A4 协议、鉴权、配对或 B2 操作。
```

任何命令失败时不得创建上述“通过”版本；改为创建结论“未通过”的记录，只列已执行命令及其真实 PASS/FAIL、首个失败摘要和复现命令，同时更新 `HANDOFF.md` 的阻塞与下一步，不得把 A3 标为完成。

同时逐字段复核 D24：`sessionId`、request ID、状态、来源、`seq`、`queueDepth`、message、可空 error 和 `pendingAction` 仍是 JSON 可表达的纯数据，React/TypeScript、Capacitor iOS/Android 与 HarmonyOS ArkTS/ArkUI 无需平台特有语义；复核 D25：A3 未引入回调协议，A4 的服务端有序输出要求未被错误下放给客户端；复核 D26：`WaitIdle` 只存在于 PC 进程内 Controller/CLI 调用面，没有新增协议消息、状态或客户端依赖。若实现改变任一共用字段或顺序语义，停止门禁，先更新设计并取得用户确认，再更新 `DECISIONS.md` 后继续。

- [ ] **Step 7.5：更新 HANDOFF 并执行只读差异检查**

若门禁通过，`HANDOFF.md` 记录实际完成文件、每条命令和结果、未运行真实 Claude integration 的原因、A3 无人工体验门禁、下一步为 A4 设计；若门禁失败，记录具体失败测试和复现命令。

Run: `git diff --check`

Expected: 无输出，退出 0。

Run: `git diff --stat`

Expected: 只包含 A3 Session Core、CLI、A3 文档和交接范围，不包含依赖、移动端、A4 或凭证文件。

- [ ] **Step 7.6：请求完成前代码审查**

使用 `superpowers:requesting-code-review` 检查设计覆盖、并发安全、事件终态去重、停止收敛和 CLI 兼容性。评审发现 Important/Critical 问题时，返回对应 Task 按 systematic-debugging + TDD 修复，并 fresh 重跑 §7.1–§7.3 全部门禁。

- [ ] **Step 7.7：停在最终 Git 授权点**

只有用户审阅 diff 并明确授权 A3 最终提交后，才运行与用户指定范围一致的 `git add` / `git commit`。推荐提交消息：

```bash
git add internal/session cmd/claude-mobile-remote docs/testing/A3-result.md HANDOFF.md docs/specs/2026-08-24-A3-session-core-design.md docs/plans/2026-08-24-A3-session-core.md DECISIONS.md
git commit -m "feat: add A3 session core"
```

若前面检查点已获授权并分别提交，最终提交只包含尚未提交的门禁记录和 HANDOFF；不得重复暂存或改写既有提交。

## 完成定义

A3 只有同时满足以下条件才完成：

- `internal/session` 的状态、FIFO、request ID、事件、snapshot、订阅和停止行为都有自动化测试；
- CLI 的所有输入均通过 Controller，EOF、`:quit`、context、错误和关闭语义通过；
- focused/full race、gofmt、vet、build 和重复并发门禁 fresh 通过；
- `docs/testing/A3-result.md` 与 `HANDOFF.md` 记录实际证据；
- 三端共用字段仍符合 D24，未提前引入 A4/B2；
- 用户完成所需检查并对任何 Git 写操作逐次明确授权。
