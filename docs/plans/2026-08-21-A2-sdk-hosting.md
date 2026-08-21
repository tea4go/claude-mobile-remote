# A2 SDK 托管实施计划

> **执行要求：** 实施时必须使用 `superpowers:test-driven-development`，按本计划逐项执行并在检查点暂停。本计划实施期间未获 Git 授权；实施收口时用户新增 D23，允许 AI 主动运行只读 `git diff`，Git 写操作仍需另行明确授权。

**目标：** 在单个长生命周期 `claude-agent-sdk-go` Client 上顺序执行多个文本 Turn，输出 Claude 文本，捕获并校验真实 `sessionId`，并通过真实 Claude 双轮测试证明上下文延续。

**架构：** `internal/claude.Host` 是 SDK 的薄 Adapter，SDK 类型不越过该包；`cmd/claude-mobile-remote/run.go` 只依赖项目自有 `turnHost` 接口，按行同步调用 `RunTurn`。A2 只验证 SDK 生命周期，不实现 A3 状态机、输入队列或 A4 网络协议。

**技术栈：** Go 1.25.3、`github.com/tea4go/claude-agent-sdk-go v0.1.1`、标准库 `bufio/context/errors/fmt/io/os/signal/syscall/testing`。

---

## 文件地图

| 文件 | 职责 |
| --- | --- |
| `go.mod.base` | 提交态依赖基准，增加 SDK v0.1.1 |
| `go.mod` | 本地构建文件，增加 SDK 与指向兄弟目录的 `replace`；保持忽略 |
| `internal/claude/event.go` | A2 内部事件、session ID 来源、错误 scope |
| `internal/claude/messages.go` | SDK Message 到 A2 事实的纯转换 |
| `internal/claude/host.go` | Client 连接、单 Turn 接收、session ID 一致性、关闭 |
| `internal/claude/messages_test.go` | 消息转换纯单元测试 |
| `internal/claude/host_test.go` | fake Client/iterator 生命周期和错误测试 |
| `internal/claude/live_test.go` | build tag + 环境变量双保护的真实 Claude 双轮门禁 |
| `cmd/claude-mobile-remote/run.go` | 可测试的行式 CLI 编排和事件输出 |
| `cmd/claude-mobile-remote/run_test.go` | CLI fake 测试 |
| `cmd/claude-mobile-remote/main.go` | flags、信号 context、真实 Host 装配、退出码 |
| `HANDOFF.md` | 当前执行、验证、缺口和下一步 |
| `docs/testing/A2-result.md` | 仅在 A2 全部门禁通过后创建 |

## Task 1：建立 SDK 依赖基线

**Files:**
- Modify: `go.mod.base`
- Modify locally: `go.mod`
- Generate locally: `go.sum`（保持忽略，不提交）

- [x] **Step 1.1：更新当前执行记录**

在 `HANDOFF.md` 将当前任务改为“执行 A2 Task 1：SDK 依赖基线”，范围仅包含 `go.mod.base`、本地 `go.mod` 和依赖解析；通过条件为 `go list` 能解析 SDK v0.1.1 且 A1 测试仍通过。

- [x] **Step 1.2：更新提交态基准**

将 `go.mod.base` 改为：

```go
module github.com/tea4go/claude-mobile-remote

go 1.25.3

require github.com/tea4go/claude-agent-sdk-go v0.1.1
```

- [x] **Step 1.3：更新本地构建文件**

将本地忽略的 `go.mod` 改为：

```go
module github.com/tea4go/claude-mobile-remote

go 1.25.3

require github.com/tea4go/claude-agent-sdk-go v0.1.1

replace github.com/tea4go/claude-agent-sdk-go => ../claude-agent-sdk-go
```

此时不运行 `go mod tidy`：在 Task 3 出现首个 SDK import 前，`tidy` 会按 Go module 规则删除尚未使用的 requirement。Task 3 完成消息转换后再执行依赖整理。

- [x] **Step 1.4：验证依赖和 A1 回归**

Run: `go list -m github.com/tea4go/claude-agent-sdk-go`

Expected: `github.com/tea4go/claude-agent-sdk-go v0.1.1 => ../claude-agent-sdk-go`

Run: `go test ./...`

Expected: `internal/version` 通过，CLI 包可构建；此时尚无 `internal/claude`。

**检查点：** 更新 `HANDOFF.md` 记录依赖解析结果；本步骤执行时未运行 Git 命令。

## Task 2：定义 Adapter 事件与错误契约（TDD）

**Files:**
- Create: `internal/claude/event.go`
- Create: `internal/claude/event_test.go`

- [x] **Step 2.1：写错误 scope 的失败测试**

创建 `internal/claude/event_test.go`：

```go
package claude

import (
	"errors"
	"testing"
)

func TestScopeOf(t *testing.T) {
	root := errors.New("root")
	turnErr := wrapError(ScopeTurn, "run turn", root)
	if got := ScopeOf(turnErr); got != ScopeTurn {
		t.Fatalf("ScopeOf(turnErr) = %q, want %q", got, ScopeTurn)
	}
	if !errors.Is(turnErr, root) {
		t.Fatal("wrapped error must preserve its cause")
	}
	if got := ScopeOf(root); got != ScopeSession {
		t.Fatalf("unknown errors must default to session scope, got %q", got)
	}
}
```

- [x] **Step 2.2：确认测试失败**

Run: `go test ./internal/claude -run TestScopeOf -v`

Expected: FAIL，至少包含 `undefined: wrapError`。

- [x] **Step 2.3：实现完整事件与错误类型**

创建 `internal/claude/event.go`：

```go
package claude

import (
	"errors"
	"fmt"
)

type EventKind string

const (
	EventSessionStarted EventKind = "session_started"
	EventText           EventKind = "text"
	EventTurnCompleted  EventKind = "turn_completed"
	EventTurnFailed     EventKind = "turn_failed"
)

type SessionIDSource string

const (
	SessionIDFromInit   SessionIDSource = "init"
	SessionIDFromResult SessionIDSource = "result"
)

type Event struct {
	Kind      EventKind
	SessionID string
	Text      string
	Source    SessionIDSource
}

type ErrorScope string

const (
	ScopeTurn    ErrorScope = "turn"
	ScopeSession ErrorScope = "session"
)

type scopedError struct {
	scope ErrorScope
	op    string
	err   error
}

func (e *scopedError) Error() string { return fmt.Sprintf("%s: %v", e.op, e.err) }
func (e *scopedError) Unwrap() error { return e.err }

func wrapError(scope ErrorScope, op string, err error) error {
	if err == nil {
		return nil
	}
	return &scopedError{scope: scope, op: op, err: err}
}

func ScopeOf(err error) ErrorScope {
	var target *scopedError
	if errors.As(err, &target) {
		return target.scope
	}
	return ScopeSession
}
```

- [x] **Step 2.4：确认测试通过**

Run: `go test ./internal/claude -run TestScopeOf -v`

Expected: PASS。

## Task 3：实现消息转换与 Host 生命周期（TDD）

**Files:**
- Create: `internal/claude/messages.go`
- Create: `internal/claude/messages_test.go`
- Create: `internal/claude/host.go`
- Create: `internal/claude/host_test.go`

- [x] **Step 3.1：写消息转换失败测试**

创建 `internal/claude/messages_test.go`，用表驱动测试覆盖以下精确输入和断言：

```go
package claude

import (
	"reflect"
	"testing"

	claudecode "github.com/tea4go/claude-agent-sdk-go"
)

func stringPointer(value string) *string { return &value }

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
				&claudecode.ThinkingBlock{Thinking: "private"},
			}},
			want: parsedMessage{text: []string{"hello"}},
		},
		{
			name: "subagent assistant ignored",
			msg:  &claudecode.AssistantMessage{ParentToolUseID: stringPointer("tool-1"), Content: []claudecode.ContentBlock{&claudecode.TextBlock{Text: "hidden"}}},
			want: parsedMessage{},
		},
		{
			name: "result",
			msg:  &claudecode.ResultMessage{SessionID: "session-1", Result: stringPointer("fallback")},
			want: parsedMessage{sessionID: "session-1", source: SessionIDFromResult, result: &resultFact{text: "fallback"}},
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
```

- [x] **Step 3.2：确认转换测试失败**

Run: `go test ./internal/claude -run TestParseMessage -v`

Expected: FAIL，包含 `undefined: parsedMessage`。

- [x] **Step 3.3：实现纯消息转换**

创建 `internal/claude/messages.go`：

```go
package claude

import claudecode "github.com/tea4go/claude-agent-sdk-go"

type resultFact struct {
	isError bool
	errors  []string
	text    string
}

type parsedMessage struct {
	sessionID string
	source    SessionIDSource
	text      []string
	result    *resultFact
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
```

- [x] **Step 3.4：确认转换测试通过**

Run: `go test ./internal/claude -run TestParseMessage -v`

Expected: PASS。

- [x] **Step 3.5：写 Host 生命周期失败测试**

创建 `internal/claude/host_test.go`。测试文件定义 `fakeClient` 和 `fakeIterator`；`fakeClient` 精确实现：

```go
type fakeClient struct {
	connectErr    error
	queryErr      error
	disconnectErr error
	queries       []string
	iterators     []claudecode.MessageIterator
	connectCalls  int
	disconnects   int
}

func (f *fakeClient) Connect(context.Context, ...claudecode.StreamMessage) error {
	f.connectCalls++
	return f.connectErr
}
func (f *fakeClient) Query(_ context.Context, prompt string) error {
	f.queries = append(f.queries, prompt)
	return f.queryErr
}
func (f *fakeClient) ReceiveResponse(context.Context) claudecode.MessageIterator {
	iterator := f.iterators[0]
	f.iterators = f.iterators[1:]
	return iterator
}
func (f *fakeClient) Disconnect() error {
	f.disconnects++
	return f.disconnectErr
}

type iteratorStep struct {
	message claudecode.Message
	err     error
}

type fakeIterator struct {
	steps  []iteratorStep
	closed int
}

func (f *fakeIterator) Next(context.Context) (claudecode.Message, error) {
	step := f.steps[0]
	f.steps = f.steps[1:]
	return step.message, step.err
}
func (f *fakeIterator) Close() error { f.closed++; return nil }
```

在同一文件实现并断言这些用例：

1. `TestHostTwoTurnsKeepClientConnected`：两个 iterator 分别返回 init/text/result 与 text/result；两次 `RunTurn` 后 queries 顺序为 `first, second`，session ID 始终为 `session-1`，每个 iterator 关闭一次，`Disconnect` 只在显式 `Close` 后为 1。
2. `TestHostResultSessionFallback`：只有成功 Result，断言先发 `session_started(Source=result)`，再发 Result 文本兜底和 `turn_completed`。
3. `TestHostDoesNotDuplicateResultText`：Assistant 已发 `same`，Result 也为 `same`，断言仅一个 `text` 事件。
4. `TestHostRejectsChangedSessionID`：init 为 `session-1`、Result 为 `session-2`，断言 session scope 错误同时包含旧、新 ID。
5. `TestHostResultErrorIsTurnScoped`：错误 Result 返回 turn scope，发 `turn_failed`，Host 仍允许下一 Turn。
6. `TestHostEOFBeforeResultIsSessionScoped`：iterator 返回 `claudecode.ErrNoMoreMessages`，断言 session scope 且错误包含“未收到 ResultMessage”。
7. `TestHostIteratorErrorIsSessionScoped`：iterator 返回 sentinel，断言 `errors.Is` 保留 sentinel。
8. `TestHostConnectQueryAndCloseErrors`：分别断言 Connect/Query 是 session scope；Close 原样包装错误；重复 Close 不重复 Disconnect。
9. `TestHostRequiresConnectAndNonEmptyPrompt`：未连接调用、空 prompt 均返回错误且不调用 SDK Query。

- [x] **Step 3.6：确认 Host 测试失败**

Run: `go test ./internal/claude -run TestHost -v`

Expected: FAIL，包含 `undefined: Host` 或 `undefined: NewHost`。

- [x] **Step 3.7：实现 Host**

创建 `internal/claude/host.go`，实现以下公开面和行为：

```go
package claude

import (
	"context"
	"errors"
	"fmt"
	"strings"

	claudecode "github.com/tea4go/claude-agent-sdk-go"
)

type sdkClient interface {
	Connect(context.Context, ...claudecode.StreamMessage) error
	Query(context.Context, string) error
	ReceiveResponse(context.Context) claudecode.MessageIterator
	Disconnect() error
}

type Option func(*Host)

func withClient(client sdkClient) Option { return func(host *Host) { host.client = client } }

type Host struct {
	client     sdkClient
	connected  bool
	closed     bool
	sessionID  string
	startedSent bool
}

func NewHost(options ...Option) *Host {
	host := &Host{client: claudecode.NewClient(claudecode.WithCanUseTool(func(
		context.Context,
		string,
		map[string]any,
		claudecode.ToolPermissionContext,
	) (claudecode.PermissionResult, error) {
		return claudecode.NewPermissionResultDeny("A2 尚未实现审批"), nil
	}))}
	for _, option := range options {
		option(host)
	}
	return host
}

func (h *Host) Connect(ctx context.Context) error {
	if h.connected || h.closed {
		return wrapError(ScopeSession, "连接 Claude CLI", errors.New("Host 已连接或已关闭"))
	}
	if err := h.client.Connect(ctx); err != nil {
		return wrapError(ScopeSession, "连接 Claude CLI", err)
	}
	h.connected = true
	return nil
}

func (h *Host) SessionID() string { return h.sessionID }

func (h *Host) RunTurn(ctx context.Context, prompt string, emit func(Event)) error {
	if !h.connected || h.closed {
		return wrapError(ScopeSession, "执行 Claude Turn", errors.New("Host 未连接"))
	}
	if strings.TrimSpace(prompt) == "" {
		return wrapError(ScopeTurn, "执行 Claude Turn", errors.New("prompt 不能为空"))
	}
	if emit == nil {
		emit = func(Event) {}
	}
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

		parsed := parseMessage(message)
		if parsed.sessionID != "" {
			if err := h.acceptSessionID(parsed.sessionID, parsed.source, emit); err != nil {
				return err
			}
		}
		for _, text := range parsed.text {
			emittedText = true
			emit(Event{Kind: EventText, SessionID: h.sessionID, Text: text})
		}
		if parsed.result == nil {
			continue
		}
		if h.sessionID == "" {
			return wrapError(ScopeSession, "完成 Claude Turn", errors.New("ResultMessage 未提供 session ID"))
		}
		if parsed.result.isError {
			message := parsed.result.text
			if message == "" {
				message = strings.Join(parsed.result.errors, "; ")
			}
			if message == "" {
				message = "Claude 返回未知错误"
			}
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

func (h *Host) Close() error {
	if h.closed {
		return nil
	}
	h.closed = true
	if !h.connected {
		return nil
	}
	h.connected = false
	if err := h.client.Disconnect(); err != nil {
		return fmt.Errorf("关闭 Claude CLI: %w", err)
	}
	return nil
}
```

实现测试时若 `ThinkingBlock` 的必填字段与 SDK 当前类型不符，仅使用 `&claudecode.ThinkingBlock{}`；测试目标是确认非 TextBlock 被忽略，不依赖其内容。

- [x] **Step 3.8：运行并修正到 Host 全绿**

Run: `GOCACHE=/private/tmp/claude-mobile-remote-gocache go mod tidy`

Expected: SDK requirement 保留，成功生成本地忽略的 `go.sum`。

Run: `gofmt -w internal/claude && GOCACHE=/private/tmp/claude-mobile-remote-gocache go test ./internal/claude -v`

Expected: 全部事件、消息和 Host 测试 PASS；`go test -race ./internal/claude` 也 PASS。若 race 发现并发问题，不给 `RunTurn` 增加并发支持；只保证 permission callback 无共享可变状态，并在注释中明确 `RunTurn` 必须串行。

**检查点：** 更新 `HANDOFF.md` 记录 fake 门禁结果；不运行真实 Claude。

## Task 4：实现行式 CLI（TDD）

**Files:**
- Create: `cmd/claude-mobile-remote/run.go`
- Create: `cmd/claude-mobile-remote/run_test.go`
- Modify: `cmd/claude-mobile-remote/main.go`

- [x] **Step 4.1：写 CLI 失败测试**

在 `run_test.go` 定义 `fakeHost`，记录 `connectCalls`、`prompts`、`closeCalls`，并允许为每个 prompt 配置事件和错误。覆盖：

1. 输入 `"\nfirst\nsecond\n:quit\nignored\n"`：只运行 `first, second`，连接/关闭各一次，退出码 0。
2. init 事件输出 `session: session-1\n`；两个文本事件按顺序写 stdout，不向 stdout 输出原始 SDK 消息。
3. 第一 Turn 返回 `ScopeTurn` 错误：stderr 记录错误但继续第二 Turn。
4. Connect 或 `ScopeSession` 错误：退出码 1，不再读后续 Turn。
5. Scanner 读取错误：stderr 包含“读取输入”，退出码 1。
6. Close 失败：若之前成功则退出码变为 1；若已有主错误则保留主错误并追加关闭错误。
7. EOF 等同正常退出；空行不调用 Host。

测试调用签名固定为：

```go
exitCode := run(context.Background(), strings.NewReader(input), &stdout, &stderr, host)
```

- [x] **Step 4.2：确认 CLI 测试失败**

Run: `go test ./cmd/claude-mobile-remote -run TestRun -v`

Expected: FAIL，包含 `undefined: run`。

- [x] **Step 4.3：实现 run.go**

创建 `cmd/claude-mobile-remote/run.go`：

```go
package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	projectclaude "github.com/tea4go/claude-mobile-remote/internal/claude"
)

type turnHost interface {
	Connect(context.Context) error
	RunTurn(context.Context, string, func(projectclaude.Event)) error
	SessionID() string
	Close() error
}

func run(ctx context.Context, input io.Reader, output, errorOutput io.Writer, host turnHost) (exitCode int) {
	defer func() {
		if err := host.Close(); err != nil {
			fmt.Fprintf(errorOutput, "关闭错误: %v\n", err)
			if exitCode == 0 {
				exitCode = 1
			}
		}
	}()

	if err := host.Connect(ctx); err != nil {
		fmt.Fprintf(errorOutput, "启动错误: %v\n", err)
		return 1
	}

	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		prompt := strings.TrimSpace(scanner.Text())
		if prompt == "" {
			continue
		}
		if prompt == ":quit" {
			return 0
		}

		turnCtx, cancelTurn := context.WithCancel(ctx)
		err := host.RunTurn(turnCtx, prompt, func(event projectclaude.Event) {
			switch event.Kind {
			case projectclaude.EventSessionStarted:
				fmt.Fprintf(output, "session: %s\n", event.SessionID)
			case projectclaude.EventText:
				fmt.Fprint(output, event.Text)
			}
		})
		cancelTurn()
		if err == nil {
			fmt.Fprintln(output)
			continue
		}
		fmt.Fprintf(errorOutput, "Turn 错误: %v\n", err)
		if projectclaude.ScopeOf(err) == projectclaude.ScopeSession {
			return 1
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(errorOutput, "读取输入错误: %v\n", err)
		return 1
	}
	return 0
}
```

- [x] **Step 4.4：实现真实入口**

将 `main.go` 改为：

```go
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	projectclaude "github.com/tea4go/claude-mobile-remote/internal/claude"
	"github.com/tea4go/claude-mobile-remote/internal/version"
)

func main() {
	showVersion := flag.Bool("version", false, "打印版本并退出")
	flag.Parse()
	if *showVersion {
		fmt.Println(version.Version)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	exitCode := run(ctx, os.Stdin, os.Stdout, os.Stderr, projectclaude.NewHost())
	stop()
	os.Exit(exitCode)
}
```

- [x] **Step 4.5：运行 CLI 测试和非额度验证**

Run: `gofmt -w cmd/claude-mobile-remote && go test ./cmd/claude-mobile-remote -v`

Expected: 全部 PASS。

Run: `go run ./cmd/claude-mobile-remote -version`

Expected: `0.1.0-dev`，不连接 Claude。

Run: `printf ':quit\n' | go run ./cmd/claude-mobile-remote`

Expected: SDK 成功 Connect 后立即关闭，退出码 0；若本机 Claude CLI/认证不可用，将结果记录为 A2 环境阻塞而不是修改错误语义。

**检查点：** 更新 `HANDOFF.md`；确认 CLI 仍是同步单入口，不存在 goroutine 并发 Query。

## Task 5：真实 Claude 双轮门禁

**Files:**
- Create: `internal/claude/live_test.go`

- [x] **Step 5.1：创建双重保护的 integration test**

`live_test.go` 第一行必须是：

```go
//go:build integration
```

测试 `TestLiveTwoTurnSession` 必须：

1. 若 `CLAUDE_MOBILE_REMOTE_LIVE != "1"`，调用 `t.Skip`；
2. 用 `crypto/rand` 生成 8 bytes，再用 `hex.EncodeToString` 得到无敏感信息 code word；
3. `context.WithTimeout(context.Background(), 3*time.Minute)`；
4. 创建一个 `Host`，Connect 一次，defer Close；
5. 第一 prompt 明确“不使用任何工具，只回复并记住 code word”；
6. 收集事件，断言首个 `session_started` 的 Source 是 `SessionIDFromInit` 且发生在 `turn_completed` 前；
7. 第一 Turn 后保存非空 `SessionID()`；
8. 第二 prompt 要求“不使用工具，只回复上一轮 code word”；
9. 断言第二 Turn 后 `SessionID()` 未变化，且拼接的第二轮 `text` 包含 code word；
10. 日志只写 `session_id_non_empty=true`、`session_id_consistent=true`、`turns=2`，不写 session ID、prompt 或完整回复。

关键断言结构：

```go
if firstSource != SessionIDFromInit {
	t.Fatalf("session source = %q, want init", firstSource)
}
if firstSessionID == "" || host.SessionID() != firstSessionID {
	t.Fatal("session ID must be non-empty and stable across two turns")
}
if !strings.Contains(secondText.String(), codeWord) {
	t.Fatal("second turn did not preserve first-turn context")
}
t.Log("session_id_non_empty=true session_id_consistent=true turns=2")
```

- [x] **Step 5.2：验证普通测试不会调用 Claude**

Run: `go test ./...`

Expected: PASS；`live_test.go` 因 build tag 不参与编译。

Run: `go test -tags=integration -run TestLiveTwoTurnSession -v ./internal/claude`

Expected: SKIP，原因包含需要 `CLAUDE_MOBILE_REMOTE_LIVE=1`。

- [x] **Step 5.3：执行真实双轮门禁**

Run: `CLAUDE_MOBILE_REMOTE_LIVE=1 go test -tags=integration -run TestLiveTwoTurnSession -count=1 -v ./internal/claude`

Expected: PASS，日志仅显示非空/一致布尔值与 `turns=2`。

若以下任一项失败，立即停止，不进入 A3：init 未在 Result 前给出 session ID、两轮 session ID 不一致、第二 Turn 无法延续 code word、Result 后 Client 无法再次 Query。将实际事实写入 `HANDOFF.md`，并等待用户确认是否修改设计/决策。

## Task 6：A2 全部门禁与文档收尾

**Files:**
- Modify: `HANDOFF.md`
- Create after all gates pass: `docs/testing/A2-result.md`
- Do not modify: `DECISIONS.md`（除非真实门禁推翻已确认设计且用户确认新决策）

- [x] **Step 6.1：运行全部自动门禁**

依次运行，不能用前一次缓存结果代替：

```bash
go test ./...
go test -race ./internal/claude ./cmd/claude-mobile-remote
gofmt -l .
go vet ./...
go build ./...
CLAUDE_MOBILE_REMOTE_LIVE=1 go test -tags=integration -run TestLiveTwoTurnSession -count=1 -v ./internal/claude
```

Expected: 测试、race、vet、build、integration 均 PASS；`gofmt -l .` 无输出。

- [x] **Step 6.2：核对安全和阶段边界**

Run: `rg -n "bypassPermissions|pairToken|WebSocket|session_started|SessionIDFromInit" internal cmd`

Expected:

- 无 `bypassPermissions`、`pairToken`、WebSocket 实现；
- `session_started` 只属于 `internal/claude.Event`，未变成三端协议；
- 无日志打印完整 SDK 原始消息、认证信息或真实 session ID；
- CLI 输入同步执行，无 A3 状态机、队列、snapshot。

- [x] **Step 6.3：创建门禁记录**

仅在 Step 6.1 全部通过后先运行 `claude --version`，再创建 `docs/testing/A2-result.md`。版本行必须填写该命令的原样输出（不含配置路径），记录结构为：

```markdown
# A2 SDK 托管门禁记录

- 日期：2026-08-21
- 子阶段：A2 SDK 托管
- 结论：通过
- SDK：github.com/tea4go/claude-agent-sdk-go v0.1.1（本地 replace 验证）
- Claude Code：由紧邻执行的 `claude --version` 原样填写

## 自动验证

| 命令 | 结果 |
| --- | --- |
| `go test ./...` | PASS |
| `go test -race ./internal/claude ./cmd/claude-mobile-remote` | PASS |
| `gofmt -l .` | PASS（无输出） |
| `go vet ./...` | PASS |
| `go build ./...` | PASS |
| 真实 Claude 双轮 integration test | PASS |

## 设计假设核对

- init 在首轮 Result 前提供非空 session ID：通过
- Result session ID 与 init 一致：通过
- 同一 Client 顺序执行两个 Turn：通过
- 第二 Turn 延续第一 Turn 上下文：通过
- ReceiveResponse 统一传递消息与流错误：通过（fake 错误测试 + 真实流）

记录不包含真实 session ID、prompt、回复、认证信息或用户配置。
```

创建记录时必须把上面的说明行替换成实际版本文本，不得保留“由紧邻执行……”这句说明。

- [x] **Step 6.4：更新交接文档**

将 `HANDOFF.md` 更新为：A2 完成；列出实际通过的命令、未验证项（如有）、真实门禁结论；下一步是“起草 A3 Session Core 分阶段设计”，不得直接开始 A3 代码。

- [x] **Step 6.5：提供人工验收清单**

向用户提供以下简短清单：

1. 运行 CLI 后输入第一条文本，能看到 session 行和 Claude 文本；
2. 再输入依赖上一轮上下文的文本，Claude 能延续回答；
3. 输入 `:quit` 后正常退出；
4. 使用会触发工具的 prompt 时，A2 拒绝审批而不静默越权。

用户反馈通过或问题后，AI 负责将结果写回 `HANDOFF.md`；若属于 A2 门禁补充，同步更新 `docs/testing/A2-result.md`。

---

## 计划自检

- A2 设计 §1–§12 均映射到 Task 1–6；真实双轮、init 来源、Result 兜底、同 Session、多 Turn、权限拒绝和 ReceiveResponse 错误均有门禁。
- 未实现 A3 Session/Turn 状态机、串行队列、snapshot，也未实现 A4 Channel/协议/鉴权。
- A2 内部事件不属于总体设计 §8.1 三端共用契约，因此无需修改 `DECISIONS.md`。
- 实施任务不包含 Git 写操作；按后续确认的 D23，AI 可主动使用只读 `git diff` 做收口检查，其他 Git 操作仍需用户另行明确授权。

## 完成前评审修正

- [x] 强制每个 `ResultMessage` 自带非空且一致的 session ID，并补首轮/后续 Turn 空 ID 测试；
- [x] 提取并自动测试 A2 工具拒绝回调；
- [x] 让 CLI 空闲等待输入时可被 context 取消，并验证 Host 关闭；
- [x] 合并错误 Result 的 `Result` 与 `Errors` 诊断，失败 Turn 的部分文本后补换行；
- [x] 重新执行全部自动门禁与只读代码评审。
