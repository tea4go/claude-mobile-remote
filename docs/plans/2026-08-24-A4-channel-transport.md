# A4 Channel Communication Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现短期 Pair Token、版本化 JSON 协议、按 `sessionId` 的 Channel、隔离慢客户端的 WebSocket Transport，以及可执行的 Go CLI 协议测试客户端，完成 A4 文本命令通信闭环。

**Architecture:** 每个已订阅 WebSocket 连接各自通过 Channel 持有一个 A3 Controller 原子订阅，由连接级事件泵立即消费领域事件并写入有界 outbox。连接在调用 `Controller.Submit` 期间使用有界顺序门控，确保 `command_result` 先于同一 request ID 的 `turn_started`；Pairing Manager 独立管理短期 token、自然轮换和主动撤销。

**Tech Stack:** Go 1.25.3、标准库 `context/crypto/rand/crypto/subtle/encoding/base64/encoding/json/net/http/sync/time`、`github.com/gorilla/websocket` v1.5.3、现有 `internal/session`、Go race detector 与 fuzz testing。

---

## 执行约束

- 开始实施前读取 `AGENTS.md`、`HANDOFF.md`、`DECISIONS.md`、`REQUIREMENTS.md`、总体设计、A4 设计和本计划，并先更新 `HANDOFF.md` 的“当前执行”。
- 用户已要求使用当前 `main` 工作区；不得创建 worktree 或分支。
- 每个行为使用 `superpowers:test-driven-development`：先写失败测试并确认 RED，再写最小实现并确认 GREEN。
- A4 不修改 A2 SDK Adapter 或 A3 Controller 公共契约；只依赖现有 `Submit` 与原子 `Subscribe`。
- 允许主动运行 `git diff`；不得自行执行 `git add`、`git commit` 或其他 Git 写操作。每个 Task 只形成可检查点，全部 Task 完成后交用户统一检查。
- 正式依赖只增加 `github.com/gorilla/websocket v1.5.3`；同步更新 `go.mod.base`。本地 `go.mod`/`go.sum` 用于构建但不纳入提交。
- `go vet`、`go build` 必须带 `GOFLAGS=-buildvcs=false`；不运行真实 Claude integration，A4 使用 controlled Controller/Host。
- 不输出或记录真实 token、用户 prompt、Claude 配置或会话内容；测试只使用确定性的虚构凭证。

## 2026-09-24 生命周期回归修复

本节处理现有 A4 Transport 的两个独立回归，不改变网络协议、配对语义或客户端状态模型。

1. 调整慢客户端隔离用例：每次发送大事件后先确认快连接已收到，再继续发送；慢连接保持不读取，最后确认其订阅独立退出，并用后续小事件验证快连接仍可用。避免测试自身的突发发送同时压满两条连接的有界队列。
2. 修正订阅源正常关闭时的事件泵：源关闭错误可能先于已缓冲事件到达，Transport 应先排空事件，再按服务停机状态处理连接，确保最终事件进入 outbox。
3. 分别验证服务停机和订阅源自行关闭时，最终事件先于连接关闭到达客户端；重跑定向测试、Transport race、全量 Go race 和构建。记录实际结果与未验证项。

## 文件地图

| 文件 | 操作 | 单一职责 |
| --- | --- | --- |
| `internal/pairing/payload.go` | Create | 三端共用配对负载及安全 JSON 编码 |
| `internal/pairing/manager.go` | Create | token 签发、校验、自然轮换、撤销和连接 lease |
| `internal/pairing/manager_test.go` | Create | 随机、过期、轮换、撤销、并发和泄密测试 |
| `internal/transport/protocol.go` | Create | envelope、payload、稳定错误码和严格 JSON 解码 |
| `internal/transport/protocol_test.go` | Create | 协议字段、类型、大小和 fuzz 测试 |
| `internal/channel/channel.go` | Create | Controller 窄接口、原子订阅和手机文本提交 |
| `internal/channel/registry.go` | Create | `sessionId → Channel` 注册与隔离 |
| `internal/channel/channel_test.go` | Create | 原子边界、来源强制、session 隔离与释放测试 |
| `internal/transport/outbox.go` | Create | 连接级数量/字节有界队列和提前事件顺序门控 |
| `internal/transport/outbox_test.go` | Create | result/event 顺序与两层溢出测试 |
| `internal/transport/server.go` | Create | HTTP/WebSocket Server、Origin、监听与停机入口 |
| `internal/transport/connection.go` | Create | 单 reader/writer、认证、订阅、命令和连接状态机 |
| `internal/transport/server_test.go` | Create | 真实 WebSocket 协议、安全、多客户端和停机测试 |
| `cmd/claude-mobile-remote/config.go` | Create | CLI 网络配置和重复 `--allowed-origin` 解析 |
| `cmd/claude-mobile-remote/remote.go` | Create | Pairing、Registry、Server 与 Controller 的生命周期组合 |
| `cmd/claude-mobile-remote/run.go` | Modify | 在现有 CLI 事件循环接入 remote runtime 和配对输出 |
| `cmd/claude-mobile-remote/main.go` | Modify | 解析配置并装配 A4 Service |
| `cmd/claude-mobile-remote/run_test.go` | Modify | A4 启动、激活、轮换和停机顺序测试 |
| `cmd/claude-mobile-remote/config_test.go` | Create | 默认监听与显式 VPN/Origin 参数测试 |
| `cmd/channel-test-client/main.go` | Create | 读取配对负载并执行人工协议闭环 |
| `cmd/channel-test-client/main_test.go` | Create | 使用内存 WebSocket Server 验证客户端流程 |
| `go.mod.base` | Modify | 增加 Gorilla WebSocket 正式依赖 |
| `docs/testing/A4-result.md` | Create at gate | A4 门禁与三端契约验证证据 |
| `HANDOFF.md` | Modify at events | 任务、验证、人工检查点和下一步 |

## Task 1：实现 Pairing Payload 与短期凭证生命周期（TDD）

**Files:**
- Create: `internal/pairing/payload.go`
- Create: `internal/pairing/manager.go`
- Create: `internal/pairing/manager_test.go`

- [ ] **Step 1.1：写签发、认证和 session 绑定失败测试**

测试使用固定 32-byte reader 和注入时钟，不读取真实随机数：

```go
func TestManagerIssueAndAuthenticate(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	m, err := NewManager("ws://100.64.0.10:8765/ws", 10*time.Minute,
		WithClock(func() time.Time { return now }),
		WithRandom(bytes.NewReader(bytes.Repeat([]byte{0x2a}, 32))),
	)
	if err != nil { t.Fatal(err) }

	payload, err := m.Issue("session-1")
	if err != nil { t.Fatal(err) }
	if payload.Version != 1 || payload.WSURL != "ws://100.64.0.10:8765/ws" || payload.SessionID != "session-1" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	if payload.PairToken == "" || !payload.ExpiresAt.Equal(now.Add(10*time.Minute)) {
		t.Fatalf("invalid credential metadata: %#v", payload)
	}

	lease, err := m.Authenticate("session-1", payload.PairToken)
	if err != nil { t.Fatalf("Authenticate() error = %v", err) }
	defer lease.Close()
	if _, err := m.Authenticate("session-2", payload.PairToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("cross-session error = %v, want ErrUnauthorized", err)
	}
	if _, err := m.Authenticate("session-1", "invalid"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("invalid token error = %v, want ErrUnauthorized", err)
	}
}
```

- [ ] **Step 1.2：运行测试确认 RED**

Run: `go test ./internal/pairing -run TestManagerIssueAndAuthenticate -v`

Expected: FAIL，包含 `directory not found` 或 `undefined: NewManager`。

- [ ] **Step 1.3：实现精确公共类型和最小签发/认证**

`payload.go` 的公共契约固定为：

```go
type Payload struct {
	Version   int       `json:"version"`
	WSURL     string    `json:"wsUrl"`
	SessionID string    `json:"sessionId"`
	PairToken string    `json:"pairToken"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func (p Payload) Marshal() ([]byte, error) {
	return json.Marshal(p)
}
```

`manager.go` 必须提供以下接口；内部只保存 token 的 SHA-256 digest，不保存可被日志格式化的明文副本，`Issue` 仅通过返回值交付明文：

```go
var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrNoCredential = errors.New("no active credential")
)

type Clock func() time.Time
type Option func(*Manager)

func WithClock(clock Clock) Option
func WithRandom(reader io.Reader) Option

func NewManager(wsURL string, ttl time.Duration, options ...Option) (*Manager, error)
func (m *Manager) Issue(sessionID string) (Payload, error)
func (m *Manager) Authenticate(sessionID, token string) (*Lease, error)

type leaseState struct {
	done chan struct{}
	once sync.Once
}

type Lease struct {
	manager *Manager
	id      uint64
	state   *leaseState
	once    sync.Once
}
func (l *Lease) Done() <-chan struct{}
func (l *Lease) Close()
```

`Manager` 的精确状态所有权为：

```go
type credential struct {
	sessionID string
	digest    [sha256.Size]byte
	expiresAt time.Time
}

type Manager struct {
	mu          sync.Mutex
	wsURL       string
	ttl         time.Duration
	clock       Clock
	random      io.Reader
	current     *credential
	invalidated bool
	leases      map[uint64]*leaseState
	nextLeaseID uint64
}
```

生成逻辑必须等价于：

```go
var raw [32]byte
if _, err := io.ReadFull(m.random, raw[:]); err != nil {
	return Payload{}, fmt.Errorf("generate pair credential: %w", err)
}
token := base64.RawURLEncoding.EncodeToString(raw[:])
digest := sha256.Sum256([]byte(token))
expiresAt := m.clock().Add(m.ttl)
```

认证对传入 token 求 SHA-256 后使用 `subtle.ConstantTimeCompare`，无活动凭证、过期、session 不符和 digest 不符都只返回 `ErrUnauthorized`。

- [ ] **Step 1.4：运行 focused 测试确认 GREEN**

Run: `gofmt -w internal/pairing && go test ./internal/pairing -run TestManagerIssueAndAuthenticate -v`

Expected: PASS。

- [ ] **Step 1.5：写自然过期、轮换和撤销 lease 失败测试**

```go
func TestManagerExpiryRotationAndRevocation(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	random := append(bytes.Repeat([]byte{0x11}, 32), bytes.Repeat([]byte{0x22}, 32)...)
	m, _ := NewManager("ws://pc/ws", 10*time.Minute,
		WithClock(func() time.Time { return now }), WithRandom(bytes.NewReader(random)))
	first, _ := m.Issue("session-1")
	lease, _ := m.Authenticate("session-1", first.PairToken)

	now = now.Add(11 * time.Minute)
	if _, err := m.Authenticate("session-1", first.PairToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expired Authenticate() error = %v", err)
	}
	select {
	case <-lease.Done(): t.Fatal("natural expiry closed established lease")
	default:
	}
	second, rotated, err := m.RotateIfExpired()
	if err != nil || !rotated || second.PairToken == first.PairToken {
		t.Fatalf("RotateIfExpired() = %#v, %v, %v", second, rotated, err)
	}
	m.Revoke()
	select {
	case <-lease.Done():
	case <-time.After(time.Second): t.Fatal("revocation did not close lease")
	}
	if _, err := m.Authenticate("session-1", second.PairToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("revoked Authenticate() error = %v", err)
	}
}
```

- [ ] **Step 1.6：运行新增测试确认 RED**

Run: `go test ./internal/pairing -run TestManagerExpiryRotationAndRevocation -v`

Expected: FAIL，包含 `m.RotateIfExpired undefined` 或 `m.Revoke undefined`。

- [ ] **Step 1.7：实现轮换、失效和撤销**

增加并实现：

```go
func (m *Manager) RotateIfExpired() (Payload, bool, error)
func (m *Manager) Invalidate()             // 禁止新认证，但暂不关闭 lease，供有界停机
func (m *Manager) Revoke()                 // Invalidate + 关闭全部 lease
```

`RotateIfExpired` 只在当前凭证存在且 `!clock().Before(expiresAt)` 时签发同一 session 的新 token；已有 lease 继续存活。`Issue`、`Authenticate`、`RotateIfExpired`、`Invalidate`、`Revoke` 和 `Lease.Close` 全部由同一 mutex 保护；关闭 lease channel 必须使用 `sync.Once`。

- [ ] **Step 1.8：运行 Pairing 全包 race 测试**

补 `TestManagerConcurrentAuthenticateAndRevoke`，用 32 个 goroutine 同时认证/关闭 lease/撤销，随后运行：

Run: `gofmt -w internal/pairing && go test -race ./internal/pairing`

Expected: PASS，且 race detector 无报告。

- [ ] **Step 1.9：形成检查点**

Run: `git diff --check && git diff -- internal/pairing HANDOFF.md`

Expected: 无 whitespace error；不执行暂存或提交。

## Task 2：固定版本 1 JSON 协议（TDD）

**Files:**
- Create: `internal/transport/protocol.go`
- Create: `internal/transport/protocol_test.go`

- [ ] **Step 2.1：写 envelope round-trip 和严格字段失败测试**

```go
func TestDecodeEnvelope(t *testing.T) {
	input := []byte(`{"version":1,"type":"command","sessionId":"session-1","commandId":"command-1","requestId":"","seq":0,"payload":{"kind":"send_text","text":"hello"}}`)
	envelope, err := DecodeEnvelope(input)
	if err != nil { t.Fatal(err) }
	if envelope.Version != Version || envelope.Type != TypeCommand || envelope.SessionID != "session-1" || envelope.CommandID != "command-1" {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
	var command CommandPayload
	if err := DecodePayload(envelope, &command); err != nil { t.Fatal(err) }
	if command.Kind != CommandSendText || command.Text != "hello" { t.Fatalf("unexpected command: %#v", command) }
}

func TestDecodeEnvelopeRejectsUnknownAndTrailingData(t *testing.T) {
	for _, input := range [][]byte{
		[]byte(`{"version":1,"type":"hello","sessionId":"s","commandId":"","requestId":"","seq":0,"payload":{},"extra":true}`),
		[]byte(`{"version":1,"type":"hello","sessionId":"s","commandId":"","requestId":"","seq":0,"payload":{}} {}`),
	} {
		if _, err := DecodeEnvelope(input); !errors.Is(err, ErrInvalidMessage) {
			t.Fatalf("DecodeEnvelope(%q) error = %v", input, err)
		}
	}
}
```

- [ ] **Step 2.2：运行测试确认 RED**

Run: `go test ./internal/transport -run 'TestDecodeEnvelope' -v`

Expected: FAIL，包含 `undefined: DecodeEnvelope`。

- [ ] **Step 2.3：实现协议公共类型、稳定 code 和严格 decoder**

```go
const Version = 1

type MessageType string
const (
	TypeHello MessageType = "hello"
	TypeHelloAck MessageType = "hello_ack"
	TypeSubscribe MessageType = "subscribe"
	TypeSubscribed MessageType = "subscribed"
	TypeUnsubscribe MessageType = "unsubscribe"
	TypeUnsubscribed MessageType = "unsubscribed"
	TypeSnapshot MessageType = "snapshot"
	TypeEvent MessageType = "event"
	TypeCommand MessageType = "command"
	TypeCommandResult MessageType = "command_result"
	TypeError MessageType = "error"
)

type Envelope struct {
	Version   int             `json:"version"`
	Type      MessageType     `json:"type"`
	SessionID string          `json:"sessionId"`
	CommandID string          `json:"commandId"`
	RequestID string          `json:"requestId"`
	Seq       uint64          `json:"seq"`
	Payload   json.RawMessage `json:"payload"`
}

type HelloPayload struct { PairToken string `json:"pairToken"` }
type CommandKind string
const CommandSendText CommandKind = "send_text"
type CommandPayload struct { Kind CommandKind `json:"kind"`; Text string `json:"text"` }
type CommandResultPayload struct { Accepted bool `json:"accepted"`; Code string `json:"code"`; Message string `json:"message"` }
type ErrorPayload struct { Code string `json:"code"`; Message string `json:"message"` }
type SnapshotPayload struct { ConnectionStatus string `json:"connectionStatus"`; Snapshot session.Snapshot `json:"snapshot"` }
type EventPayload struct { Event session.Event `json:"event"` }

var ErrInvalidMessage = errors.New("invalid protocol message")

func NewEnvelope(messageType MessageType, sessionID, commandID, requestID string, seq uint64, payload any) (Envelope, error)
func EncodeEnvelope(envelope Envelope) ([]byte, error)
func DecodeEnvelope(data []byte) (Envelope, error)
func DecodePayload(envelope Envelope, destination any) error
```

`DecodeEnvelope` 和 `DecodePayload` 都使用 `json.Decoder.DisallowUnknownFields()`，并再次 Decode 确认 EOF。`NewEnvelope` 必须始终生成 object payload；无字段消息使用 `struct{}{}`，不能使用 `null`。

- [ ] **Step 2.4：运行 focused 测试确认 GREEN**

Run: `gofmt -w internal/transport && go test ./internal/transport -run 'TestDecodeEnvelope' -v`

Expected: PASS。

- [ ] **Step 2.5：写消息语义表测试**

表驱动用例必须断言：所有消息 `version == 1`、`type/sessionId` 非空；hello 只允许 ID 为空和 seq 0；command 必须有 command ID、request ID 为空、seq 0；command result 必须有 command ID，成功时 request ID 非空、失败时为空；snapshot/event 的 seq 与 payload 一致；其他消息 ID 为空且 seq 0；未知类型和版本返回稳定错误。

```go
func TestValidateEnvelope(t *testing.T) {
	tests := []struct { name string; envelope Envelope; wantCode string }{
		{name: "unsupported version", envelope: Envelope{Version: 2, Type: TypeHello, SessionID: "s", Payload: json.RawMessage(`{}`)}, wantCode: CodeUnsupportedVersion},
		{name: "missing session", envelope: Envelope{Version: 1, Type: TypeHello, Payload: json.RawMessage(`{}`)}, wantCode: CodeInvalidMessage},
		{name: "command missing id", envelope: Envelope{Version: 1, Type: TypeCommand, SessionID: "s", Payload: json.RawMessage(`{"kind":"send_text","text":"x"}`)}, wantCode: CodeInvalidMessage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEnvelope(tt.envelope)
			var protocolErr *ProtocolError
			if !errors.As(err, &protocolErr) || protocolErr.Code != tt.wantCode { t.Fatalf("error = %v", err) }
		})
	}
}
```

- [ ] **Step 2.6：实现 `ProtocolError`、code 常量和消息语义校验**

稳定 code 固定为：

```go
const (
	CodeInvalidJSON = "invalid_json"
	CodeUnsupportedVersion = "unsupported_version"
	CodeInvalidMessage = "invalid_message"
	CodeInvalidState = "invalid_state"
	CodeUnauthorized = "unauthorized"
	CodeMessageTooLarge = "message_too_large"
	CodeInvalidCommand = "invalid_command"
	CodeDuplicateCommandID = "duplicate_command_id"
	CodeSessionUnavailable = "session_unavailable"
	CodeQueueFull = "queue_full"
	CodeInternal = "internal"
)

type ProtocolError struct { Code string; Message string }
func (e *ProtocolError) Error() string { return e.Code + ": " + e.Message }
func ValidateEnvelope(envelope Envelope) error
```

message 不得包含 token、原始 payload、Go error 类型或 session 存在性细节。

- [ ] **Step 2.7：增加 decoder fuzz 并运行协议全包**

```go
func FuzzDecodeEnvelope(f *testing.F) {
	f.Add([]byte(`{"version":1,"type":"hello","sessionId":"s","commandId":"","requestId":"","seq":0,"payload":{"pairToken":"fixture"}}`))
	f.Fuzz(func(t *testing.T, data []byte) { _, _ = DecodeEnvelope(data) })
}
```

Run: `gofmt -w internal/transport && go test ./internal/transport && go test ./internal/transport -run=^$ -fuzz=FuzzDecodeEnvelope -fuzztime=3s`

Expected: 普通测试 PASS；fuzz 3 秒内无 panic 或崩溃输入。

## Task 3：实现 Session Channel 与 Registry（TDD）

**Files:**
- Create: `internal/channel/channel.go`
- Create: `internal/channel/registry.go`
- Create: `internal/channel/channel_test.go`

- [ ] **Step 3.1：写手机来源强制与 session 隔离失败测试**

```go
func TestChannelSubmitTextForcesMobileSource(t *testing.T) {
	controller := &fakeController{submitID: "request-1"}
	channel := New("session-1", controller)
	requestID, err := channel.SubmitText(context.Background(), " hello ")
	if err != nil || requestID != "request-1" { t.Fatalf("SubmitText() = %q, %v", requestID, err) }
	want := session.Input{SessionID: "session-1", Source: session.InputMobile, Text: " hello "}
	if controller.lastInput != want { t.Fatalf("input = %#v, want %#v", controller.lastInput, want) }
}

func TestRegistryDoesNotCrossSessions(t *testing.T) {
	registry := NewRegistry()
	first := New("session-1", &fakeController{})
	if err := registry.Register(first); err != nil { t.Fatal(err) }
	if got, ok := registry.Get("session-1"); !ok || got != first { t.Fatalf("Get(session-1) = %v, %v", got, ok) }
	if _, ok := registry.Get("session-2"); ok { t.Fatal("unexpected cross-session channel") }
}
```

同一测试文件定义完整 fake，后续订阅测试继续复用：

```go
type fakeController struct {
	mu        sync.Mutex
	lastInput session.Input
	submitID  string
	submitErr error
	snapshot  session.Snapshot
	events    chan session.Event
}

func (f *fakeController) Submit(_ context.Context, input session.Input) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastInput = input
	return f.submitID, f.submitErr
}

func (f *fakeController) Subscribe(ctx context.Context, capacity int) (session.Snapshot, <-chan session.Event, error) {
	if capacity <= 0 { return session.Snapshot{}, nil, session.ErrInvalidInput }
	return f.snapshot, f.events, nil
}
```

- [ ] **Step 3.2：运行测试确认 RED**

Run: `go test ./internal/channel -run 'TestChannelSubmitTextForcesMobileSource|TestRegistryDoesNotCrossSessions' -v`

Expected: FAIL，包含 `undefined: New` 或 `undefined: NewRegistry`。

- [ ] **Step 3.3：实现窄 Controller、Channel 和 Registry API**

```go
type Controller interface {
	Submit(context.Context, session.Input) (string, error)
	Subscribe(context.Context, int) (session.Snapshot, <-chan session.Event, error)
}

type Channel struct { sessionID string; controller Controller }
func New(sessionID string, controller Controller) *Channel
func (c *Channel) SessionID() string
func (c *Channel) SubmitText(ctx context.Context, text string) (string, error) {
	return c.controller.Submit(ctx, session.Input{SessionID: c.sessionID, Source: session.InputMobile, Text: text})
}

var ErrAlreadyRegistered = errors.New("session channel already registered")
var ErrInvalidSequence = errors.New("invalid event sequence")
type Registry struct { mu sync.RWMutex; channels map[string]*Channel }
func NewRegistry() *Registry
func (r *Registry) Register(channel *Channel) error
func (r *Registry) Get(sessionID string) (*Channel, bool)
func (r *Registry) Unregister(sessionID string)
```

`Register` 拒绝空 ID、nil Channel 和重复 ID。下一步的 `OpenSubscription` 校验返回 snapshot 的非空 `SessionID` 必须等于 Channel ID；其后发现空或不匹配 event 属于内部隔离失败，不能转发到 Transport。

- [ ] **Step 3.4：写原子 snapshot/event 和取消释放失败测试**

fake Controller 的 `Subscribe` 返回 `Snapshot{SessionID:"session-1", Seq:7}` 和事件 channel；测试断言首个转发事件为 seq 8、取消调用方 context 后输出关闭；返回 session-2 snapshot 或 event 时返回/发布 `ErrSessionMismatch`，且错误事件不进入移动流。

- [ ] **Step 3.5：实现可取消 `Subscription` 包装**

使用以下返回类型，把数据流和内部隔离错误分开：

```go
type Subscription struct {
	Snapshot session.Snapshot
	Events   <-chan session.Event
	Errors   <-chan error
	Cancel   context.CancelFunc
}

func (c *Channel) OpenSubscription(ctx context.Context, capacity int) (*Subscription, error)
```

`OpenSubscription` 建立 child context 调用 Controller 原子 `Subscribe`，以 `lastSeq := snapshot.Seq` 开始持续读取源事件：只转发 session ID 匹配且 `event.Seq > lastSeq` 的事件，并在每次转发后更新 `lastSeq`；seq 不递增或 session 不匹配时向容量 1 的 Errors 写入 `session.ErrSessionMismatch`/`ErrInvalidSequence`，取消 child context 并关闭输出。Transport 必须持续读取 Events 和 Errors。

- [ ] **Step 3.6：运行 Channel race 测试**

Run: `gofmt -w internal/channel && go test -race ./internal/channel`

Expected: PASS，取消后 goroutine 和 Controller subscription 均收敛。

## Task 4：实现有界 Outbox 与 D25 顺序门控（TDD）

**Files:**
- Create: `internal/transport/outbox.go`
- Create: `internal/transport/outbox_test.go`

- [ ] **Step 4.1：写 result 必须先于提前 event 的失败测试**

```go
func TestCommandGateEnqueuesResultBeforeEarlyEvents(t *testing.T) {
	outbox := newOutbox(64, 1024*1024)
	gate := newCommandGate(outbox, 32, 256*1024)
	if err := gate.Begin("command-1"); err != nil { t.Fatal(err) }
	if err := gate.EnqueueEvent([]byte("event-1")); err != nil { t.Fatal(err) }
	if err := gate.EnqueueEvent([]byte("event-2")); err != nil { t.Fatal(err) }
	if err := gate.Finish([]byte("result")); err != nil { t.Fatal(err) }
	for _, want := range []string{"result", "event-1", "event-2"} {
		got, ok := outbox.Next()
		if !ok || string(got) != want { t.Fatalf("Next() = %q, %v, want %q", got, ok, want) }
	}
}
```

- [ ] **Step 4.2：运行测试确认 RED**

Run: `go test ./internal/transport -run TestCommandGateEnqueuesResultBeforeEarlyEvents -v`

Expected: FAIL，包含 `undefined: newOutbox`。

- [ ] **Step 4.3：实现 outbox 数量/字节记账和 gate**

```go
var (
	errOutboxOverflow = errors.New("outbox overflow")
	errEarlyBufferOverflow = errors.New("early event buffer overflow")
	errCommandInFlight = errors.New("command already in flight")
)

type outbox struct {
	mu       sync.Mutex
	cond     *sync.Cond
	items    [][]byte
	bytes    int
	maxItems int
	maxBytes int
	closed   bool
}
func newOutbox(maxItems, maxBytes int) *outbox
func (q *outbox) Enqueue(data []byte) error
func (q *outbox) Next() ([]byte, bool)
func (q *outbox) Close()

type commandGate struct {
	mu            sync.Mutex
	outbox        *outbox
	active        bool
	commandID     string
	early         [][]byte
	earlyBytes    int
	maxEarlyItems int
	maxEarlyBytes int
}
func newCommandGate(outbox *outbox, maxEarlyItems, maxEarlyBytes int) *commandGate
func (g *commandGate) Begin(commandID string) error
func (g *commandGate) EnqueueEvent(data []byte) error
func (g *commandGate) Finish(result []byte) error
func (g *commandGate) Abort(result []byte) error
```

所有入队复制 byte slice，避免调用方复用内存。`Finish`/`Abort` 在同一 gate 临界区先入队 result，再按到达顺序入队全部提前事件，最后清空 active/buffer；任一步溢出返回错误，由 connection 关闭自身，不能静默丢弃。

- [ ] **Step 4.4：写两层限制和关闭唤醒测试**

表驱动覆盖：第 65 个普通 item；总字节超过 1 MiB；第 33 个提前 event；提前总字节超过 256 KiB；`Close` 后 `Next` 返回 false；阻塞中的 `Next` 被 Close 唤醒；两个并发 `Begin` 返回 `errCommandInFlight`。

- [ ] **Step 4.5：运行 outbox race 测试**

Run: `gofmt -w internal/transport && go test -race ./internal/transport -run 'TestCommandGate|TestOutbox'`

Expected: PASS，race detector 无报告。

## Task 5：实现 WebSocket Server、鉴权和通信闭环（TDD）

**Files:**
- Modify: `go.mod.base`
- Create: `internal/transport/server.go`
- Create: `internal/transport/connection.go`
- Create: `internal/transport/server_test.go`

- [ ] **Step 5.1：登记 Gorilla WebSocket 依赖并恢复本地 module**

`go.mod.base` 修改为：

```go
module github.com/tea4go/claude-mobile-remote

go 1.25.3

require (
	github.com/gorilla/websocket v1.5.3
	github.com/tea4go/claude-agent-sdk-go v0.1.1
)
```

Run: `cp go.mod.base go.mod`

Run: `go mod tidy`

Expected: 本地解析成功；`go.mod`/`go.sum` 保持 ignored，不加入计划提交范围。

- [ ] **Step 5.2：写认证前无数据与正常订阅失败测试**

使用 `httptest.NewServer(server.Handler())` 和 `websocket.DefaultDialer.Dial`。测试辅助函数必须构造完整 envelope，并设置 read deadline；用受控 Pairing Manager、Registry 和 fake Controller：

```go
type serverFixture struct {
	testingServer *httptest.Server
	server        *Server
	token         string
	controller    *transportFakeController
}

type wsTestClient struct { connection *websocket.Conn; t *testing.T }
func newServerFixture(t *testing.T) *serverFixture
func (f *serverFixture) dial(t *testing.T, origin string) *wsTestClient
func (c *wsTestClient) write(t *testing.T, envelope Envelope)
func (c *wsTestClient) read(t *testing.T) Envelope
func mustEnvelope(messageType MessageType, sessionID, commandID, requestID string, seq uint64, payload any) Envelope
```

`newServerFixture` 必须用固定时钟/随机 reader 创建真实 Pairing Manager 并签发 session-1 token，Registry 注册 `channel.New("session-1", controller)`，再以 `httptest.NewServer(server.Handler())` 启动；cleanup 顺序为 client → testingServer → Server.Close。`wsTestClient.read` 每次设置 1 秒 deadline 并调用 `DecodeEnvelope`，不能无界阻塞。

```go
func TestWebSocketAuthenticateSubscribeSnapshot(t *testing.T) {
	fixture := newServerFixture(t)
	client := fixture.dial(t, "")
	defer client.Close()

	client.write(t, mustEnvelope(TypeHello, "session-1", "", "", 0, HelloPayload{PairToken: fixture.token}))
	if got := client.read(t); got.Type != TypeHelloAck { t.Fatalf("first response = %s", got.Type) }
	client.write(t, mustEnvelope(TypeSubscribe, "session-1", "", "", 0, struct{}{}))
	if got := client.read(t); got.Type != TypeSubscribed { t.Fatalf("second response = %s", got.Type) }
	snapshot := client.read(t)
	if snapshot.Type != TypeSnapshot || snapshot.Seq != 7 { t.Fatalf("snapshot = %#v", snapshot) }
}
```

另一个用例连接后不发送 hello，确保在测试注入的 20ms auth timeout 到期前读不到 session 数据，随后连接关闭。

- [ ] **Step 5.3：运行测试确认 RED**

Run: `go test ./internal/transport -run 'TestWebSocketAuthenticateSubscribeSnapshot|TestWebSocketRequiresHello' -v`

Expected: FAIL，包含 `undefined: NewServer` 或 `server.Handler undefined`。

- [ ] **Step 5.4：实现 Server 配置、Origin 和连接所有权**

```go
type Authenticator interface {
	Authenticate(sessionID, token string) (*pairing.Lease, error)
}

type Config struct {
	ListenAddress   string
	AllowedOrigins []string
	AuthTimeout     time.Duration
	CloseTimeout    time.Duration
	ReadLimit       int64
}

func DefaultConfig() Config {
	return Config{
		ListenAddress: "127.0.0.1:8765",
		AllowedOrigins: []string{"http://127.0.0.1:5173", "http://localhost:5173"},
		AuthTimeout: 5 * time.Second,
		CloseTimeout: time.Second,
		ReadLimit: int64(session.MaxPromptBytes + 64*1024),
	}
}

type Server struct {
	mu            sync.Mutex
	config        Config
	authenticator Authenticator
	registry      *channel.Registry
	upgrader      websocket.Upgrader
	httpServer    *http.Server
	listener      net.Listener
	connections   map[*connection]struct{}
	shuttingDown  bool
}
func NewServer(config Config, auth Authenticator, registry *channel.Registry) (*Server, error)
func (s *Server) Handler() http.Handler
func (s *Server) Start() error
func (s *Server) BeginShutdown()
func (s *Server) Close(ctx context.Context) error
```

`CheckOrigin` 规则：请求无 Origin 时允许；有 Origin 时必须与 allowlist 完整字符串相等。`Handler` 只暴露 `/ws`。每次 upgrade 后注册 connection，连接退出必须从 Server map 删除并 `Lease.Close`。

- [ ] **Step 5.5：实现连接状态机和串行 reader/writer**

```go
type connectionState uint8
const (
	stateConnected connectionState = iota
	stateAuthenticated
	stateSubscribed
	stateClosing
)

const (
	outboxMaxItems = 64
	outboxMaxBytes = 1024 * 1024
	earlyMaxItems = 32
	earlyMaxBytes = 256 * 1024
)

type connection struct {
	server       *Server
	socket       *websocket.Conn
	context      context.Context
	state        connectionState
	sessionID    string
	lease        *pairing.Lease
	subscriptionChannel *channel.Channel
	subscription *channel.Subscription
	outbox       *outbox
	gate         *commandGate
	commandIDs   map[string]struct{}
	cancel       context.CancelFunc
	done         chan struct{}
}
```

reader 只做：读取完整 text message → 大小/JSON/语义校验 → 按状态分派 hello/subscribe/unsubscribe/command。writer 是唯一调用 `WriteMessage` 的 goroutine，从 outbox 串行取数据。协议级错误尽可能先入队稳定 `error` 后关闭；鉴权错误统一 `unauthorized`，不区分 token/session 原因。

subscribe 的精确顺序：Registry.Get → Channel.OpenSubscription → 入队 `subscribed` → 入队 `snapshot` → 启动 event pump。unsubscribe 先 Cancel 并等待 pump 收敛，再入队 `unsubscribed`，状态回到 authenticated。

- [ ] **Step 5.6：写命令 result/event 顺序闭环失败测试**

fake Controller 的 `Submit` 在返回 request-1 前同步触发 `turn_started(request-1)` 到订阅 channel。客户端发送 command-1 后必须依次读到：

```go
result := client.read(t)
if result.Type != TypeCommandResult || result.CommandID != "command-1" || result.RequestID != "request-1" {
	t.Fatalf("first command response = %#v", result)
}
event := client.read(t)
if event.Type != TypeEvent || event.RequestID != "request-1" || event.Seq != 8 {
	t.Fatalf("second command response = %#v", event)
}
```

随后再次发送 command-1，断言失败 `command_result` code 为 `duplicate_command_id`，fake Controller Submit 仍只调用一次。

- [ ] **Step 5.7：实现命令映射和 D25 gate**

command handler 必须用连接生命周期内的 `commandIDs` 集合拒绝所有重复 ID，并按以下顺序执行：

```go
if _, duplicate := c.commandIDs[envelope.CommandID]; duplicate {
	if err := c.enqueueCommandFailure(envelope, CodeDuplicateCommandID, "duplicate command ID"); err != nil {
		c.closeForOverflow()
	}
	return
}
c.commandIDs[envelope.CommandID] = struct{}{}
if err := c.gate.Begin(envelope.CommandID); err != nil {
	c.closeForOverflow()
	return
}
requestID, submitErr := c.subscriptionChannel.SubmitText(c.context, payload.Text)
result := commandResultEnvelope(envelope.SessionID, envelope.CommandID, requestID, submitErr)
encoded, encodeErr := EncodeEnvelope(result)
if encodeErr != nil {
	c.fail(CodeInternal)
	return
}
if err := c.gate.Finish(encoded); err != nil {
	c.closeForOverflow()
	return
}
```

`commandResultEnvelope` 成功时设置 `Accepted:true` 和非空 request ID，失败时设置 `Accepted:false`、空 request ID 及下一段定义的稳定 code。`enqueueCommandFailure` 生成失败 result 后直接入普通 outbox，因为它没有对应 Controller 事件。

event pump 使用 `gate.EnqueueEvent(encodedEvent)`，不得绕过 gate 直接写 outbox。将 A3 错误映射为稳定业务 code：`ErrInvalidInput → invalid_command`、`ErrQueueFull → queue_full`、`ErrSessionMismatch/ErrSessionUnavailable → session_unavailable`，其他错误为 `internal`。

- [ ] **Step 5.8：补安全、多客户端、溢出和停机矩阵**

`server_test.go` 必须逐项覆盖并使用独立子测试：

- invalid/expired/cross-session token 都只收到 `unauthorized`，payload/message 不含输入 token；
- hello 前 subscribe/command、认证后再次 hello、订阅后切 session 为 `invalid_state` 并关闭；
- 浏览器 allowlist origin 成功，未知 origin upgrade 失败，无 Origin 成功；
- 两个连接同 token 同时订阅并收到同一事件；
- 一个连接填满 64/1 MiB outbox 后仅自身关闭，第二个仍收到下一事件；
- 断开重连得到 fake Controller 提供的更新 snapshot，不 replay 旧事件；
- Pairing `Revoke` 触发 lease.Done，关闭相关连接；
- `BeginShutdown` 拒绝新连接和新 command，但保留现有 writer；`Close` 在 deadline 内取消订阅并关闭 socket；
- session stopped/failed event 在 Close 前进入 outbox。

- [ ] **Step 5.9：运行 Transport 全包 race 测试**

Run: `gofmt -w internal/transport && go test -race ./internal/transport`

Expected: PASS；测试无真实 5 秒等待，连接/订阅 goroutine 无泄漏或 race。

- [ ] **Step 5.10：形成检查点**

Run: `git diff --check && git diff --stat`

Expected: tracked 差异无 whitespace error；`go.mod.base` 与已有文档改动可见，本地 `go.mod`/`go.sum` 不可见。对每个尚未 tracked 的 A4 Go 文件再运行 `git diff --no-index --check /dev/null <file>`，退出码 1 表示存在新增差异，输出必须为空才表示格式检查通过。

## Task 6：接入 CLI Service 生命周期与配置（TDD）

**Files:**
- Create: `cmd/claude-mobile-remote/config.go`
- Create: `cmd/claude-mobile-remote/config_test.go`
- Create: `cmd/claude-mobile-remote/remote.go`
- Modify: `cmd/claude-mobile-remote/run.go`
- Modify: `cmd/claude-mobile-remote/run_test.go`
- Modify: `cmd/claude-mobile-remote/main.go`

- [ ] **Step 6.1：写默认配置和 VPN 参数失败测试**

```go
func TestParseConfig(t *testing.T) {
	got, err := parseConfig(nil)
	if err != nil { t.Fatal(err) }
	if got.listen != "127.0.0.1:8765" || got.advertiseURL != "ws://127.0.0.1:8765/ws" || got.pairTokenTTL != 10*time.Minute {
		t.Fatalf("defaults = %#v", got)
	}

	got, err = parseConfig([]string{"--listen", "0.0.0.0:8765", "--advertise-url", "ws://100.64.0.10:8765/ws", "--allowed-origin", "capacitor://localhost", "--allowed-origin", "http://dev.local"})
	if err != nil { t.Fatal(err) }
	if !reflect.DeepEqual(got.allowedOrigins, []string{"http://127.0.0.1:5173", "http://localhost:5173", "capacitor://localhost", "http://dev.local"}) {
		t.Fatalf("allowed origins = %#v", got.allowedOrigins)
	}
}
```

- [ ] **Step 6.2：运行配置测试确认 RED**

Run: `go test ./cmd/claude-mobile-remote -run TestParseConfig -v`

Expected: FAIL，包含 `undefined: parseConfig`。

- [ ] **Step 6.3：实现独立 FlagSet 与配置校验**

```go
type cliConfig struct {
	showVersion bool
	listen string
	advertiseURL string
	pairTokenTTL time.Duration
	allowedOrigins []string
}

type stringListFlag []string
func (f *stringListFlag) String() string { return strings.Join(*f, ",") }
func (f *stringListFlag) Set(value string) error { *f = append(*f, value); return nil }
func parseConfig(args []string) (cliConfig, error)
```

使用 `flag.NewFlagSet("claude-mobile-remote", flag.ContinueOnError)`。校验 listen 可由 `net.SplitHostPort` 解析、advertise URL scheme 只能是 `ws/wss` 且 host/path 非空、TTL 大于 0、Origin 为带 scheme/host 且无 userinfo 的绝对 URL。

- [ ] **Step 6.4：写 remote 激活、轮换和停机顺序失败测试**

定义 fake remote，运行 `runWithRemote`；发送 `session_started` 后断言 `Activate("session-1")` 恰好一次且终端输出一条 `pairing:` JSON。触发退出后 calls 必须满足：

```go
wantOrder := []string{"remote_start", "remote_activate", "remote_begin_shutdown", "controller_stop", "remote_close"}
if !reflect.DeepEqual(calls, wantOrder) { t.Fatalf("calls = %v, want %v", calls, wantOrder) }
```

轮换 channel 推送第二个虚构 Payload 时，输出第二条 pairing line；不得通过 errorOutput 或日志输出 token。

- [ ] **Step 6.5：实现 remoteService 边界和组合器**

`run.go` 增加而不破坏原 A3 测试入口：

```go
type remoteService interface {
	Start() error
	Activate(sessionID string) (pairing.Payload, error)
	Pairings() <-chan pairing.Payload
	BeginShutdown()
	Close(context.Context) error
}

func run(ctx context.Context, input io.Reader, output, errorOutput io.Writer, controller sessionController) int {
	return runWithRemote(ctx, input, output, errorOutput, controller, nil)
}

func runWithRemote(ctx context.Context, input io.Reader, output, errorOutput io.Writer, controller sessionController, remote remoteService) (exitCode int)
```

`runWithRemote` 沿用现有顺序先 `controller.Start`、再建立 CLI Controller 订阅；随后调用 `remote.Start` 使 listener 在真实 session ID 出现前就绪。任一步失败都进入统一 defer：remote 尚未成功启动时其 BeginShutdown/Close 必须幂等。启动完成后才开始 scanner 和事件 select。

`remote.go` 的 concrete runtime 组合 `pairing.Manager`、`channel.Registry`、`transport.Server` 和 Controller：Activate 注册 `channel.New(sessionID, controller)` 后调用 `Manager.Issue`；背景 ticker 在凭证 expiresAt 到期时调用 `RotateIfExpired` 并把新 Payload 发送到容量 1 的 `Pairings`，不得阻塞 CLI；BeginShutdown 调用 `Manager.Invalidate` 和 `Server.BeginShutdown`；Close 在 Controller 已发布终态后先调用 `Server.Close` 等待事件泵和 outbox 在 context deadline 内收敛，再调用 `Manager.Revoke` 关闭剩余 lease，最后停止轮换 goroutine。

具体类型和构造函数固定为：

```go
type remoteRuntime struct {
	manager    *pairing.Manager
	registry   *channel.Registry
	server     *transport.Server
	controller channel.Controller
	pairings   chan pairing.Payload
	stop       chan struct{}
	done       chan struct{}
	stopOnce   sync.Once
}

func newRemoteRuntime(manager *pairing.Manager, registry *channel.Registry, server *transport.Server, controller channel.Controller) *remoteRuntime
func (r *remoteRuntime) Start() error
func (r *remoteRuntime) Activate(sessionID string) (pairing.Payload, error)
func (r *remoteRuntime) Pairings() <-chan pairing.Payload
func (r *remoteRuntime) BeginShutdown()
func (r *remoteRuntime) Close(ctx context.Context) error
```

轮换发布不能丢掉最新凭证：若容量 1 的 `pairings` 已有旧 Payload，先非阻塞取出旧值，再写入新值。`Invalidate` 后 ticker 不再轮换，Close 必须关闭 `done`，但不关闭 `pairings` 造成 CLI select 读取零值循环。

- [ ] **Step 6.6：重构 defer，落实最终事件停机顺序**

`runWithRemote` 的唯一收尾函数必须是：

```go
defer func() {
	if remote != nil { remote.BeginShutdown() }
	if err := controller.Stop(context.Background()); err != nil {
		fmt.Fprintf(errorOutput, "关闭错误: %v\n", err)
		if exitCode == 0 { exitCode = 1 }
	}
	if remote != nil {
		closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := remote.Close(closeCtx); err != nil {
			fmt.Fprintf(errorOutput, "远程服务关闭错误: %v\n", err)
			if exitCode == 0 { exitCode = 1 }
		}
	}
}()
```

事件 select 新增 `remote.Pairings()`；`EventSessionStarted` 调用 Activate 并用 `json.Marshal(payload)` 写 `pairing: <json>\n`。只有交互 output 接收配对负载，errorOutput 的错误必须只包含安全错误摘要。

- [ ] **Step 6.7：装配 main 且保持 `-version` 零副作用**

`main.go` 先 `parseConfig(os.Args[1:])`；`--version` 立即输出并返回，不创建 Host、listener 或 token。正常路径创建 Controller、Manager、Registry、Server、remote runtime，再调用 `runWithRemote`。配置错误打印到 stderr 并退出 2，启动/运行错误沿用退出 1。

- [ ] **Step 6.8：运行 CLI focused 和全包 race 测试**

Run: `gofmt -w cmd/claude-mobile-remote && go test -race ./cmd/claude-mobile-remote -run 'TestParseConfig|TestRunWithRemote|TestVersion'`

Expected: PASS；原 A3 `run_test.go` 全部继续通过。

Run: `go test -race ./cmd/claude-mobile-remote`

Expected: PASS。

## Task 7：实现最小 Go CLI 协议测试客户端（TDD）

**Files:**
- Create: `cmd/channel-test-client/main.go`
- Create: `cmd/channel-test-client/main_test.go`

- [ ] **Step 7.1：写 pairing payload 解析和闭环输出失败测试**

测试启动 `httptest` WebSocket fixture，提供虚构 Payload JSON，并向客户端输入一行 `hello`。断言观察到的消息类型依次为 hello、subscribe、command；服务端回写 ack/subscribed/snapshot/result/event，客户端输出包含状态与两个 ID，但不回显 pair token。

测试 fixture 使用 `httptest.NewServer` 加 Gorilla upgrader，handler 串行读取三条 envelope 并把类型写入 `received chan MessageType`，然后按固定顺序写出 hello_ack、subscribed、seq 7 snapshot、request-1 command_result 和 seq 8 turn_started event；fixture 暴露 `wsURL string` 和幂等 `Close()`，所有读写设置 1 秒 deadline。

```go
func TestRunClientClosedLoop(t *testing.T) {
	payload := pairing.Payload{Version: 1, WSURL: fixture.wsURL, SessionID: "session-1", PairToken: "fixture-credential", ExpiresAt: time.Now().Add(time.Hour)}
	raw, _ := json.Marshal(payload)
	var output bytes.Buffer
	exitCode := runClient(context.Background(), []string{"--pair", string(raw)}, strings.NewReader("hello\n"), &output, io.Discard)
	if exitCode != 0 { t.Fatalf("exit code = %d", exitCode) }
	if !strings.Contains(output.String(), "authenticated") || !strings.Contains(output.String(), "request-1") { t.Fatalf("output = %q", output.String()) }
	if strings.Contains(output.String(), payload.PairToken) { t.Fatal("client echoed pair token") }
}
```

- [ ] **Step 7.2：运行测试确认 RED**

Run: `go test ./cmd/channel-test-client -run TestRunClientClosedLoop -v`

Expected: FAIL，包含 `directory not found` 或 `undefined: runClient`。

- [ ] **Step 7.3：实现测试客户端状态机**

```go
func runClient(ctx context.Context, args []string, input io.Reader, output, errorOutput io.Writer) int
```

精确行为：

1. `--pair` 必须是符合 `pairing.Payload` 的 JSON，version 1、ws/wss URL、非空 session/token、未过期；
2. 使用 Gorilla Dialer 连接，不设置 Origin；
3. 发送 hello，必须收到 hello_ack；发送 subscribe，必须收到 subscribed 后 snapshot；
4. reader goroutine 持续打印 snapshot/event/result 的非敏感摘要；
5. scanner 每个非空行生成 128-bit 小写 hex command ID 并发送 send_text；`:quit` 正常关闭；
6. 不自动重发断线前命令，不输出 payload 或 token；
7. 协议错误和网络断开输出稳定诊断并返回 1。

所有网络写入必须由一个 writer goroutine 完成，scanner 通过 channel 发送 command envelope，避免后续 reader 控制消息造成并发写。

- [ ] **Step 7.4：补重连最新 snapshot 的人工入口测试**

测试连续运行两次 `runClient`，fixture 第二次返回更高 snapshot seq；断言第二次只显示新 snapshot，服务端没有收到客户端自动重发第一轮 command。

- [ ] **Step 7.5：运行客户端 race 测试与 build**

Run: `gofmt -w cmd/channel-test-client && go test -race ./cmd/channel-test-client`

Expected: PASS。

Run: `GOFLAGS=-buildvcs=false go build -o /private/tmp/claude-mobile-remote-channel-test-client ./cmd/channel-test-client`

Expected: PASS，构建产物只写入 `/private/tmp`，仓库内不生成二进制文件。

## Task 8：A4 全量门禁、记录与用户验收检查点

**Files:**
- Create: `docs/testing/A4-result.md`
- Modify: `HANDOFF.md`
- Review: `DECISIONS.md`
- Review: `docs/specs/2026-08-24-A4-channel-transport-design.md`

- [ ] **Step 8.1：执行 focused 安全与并发门禁**

Run: `go test -race ./internal/pairing ./internal/channel ./internal/transport`

Expected: PASS，无 race、无真实 sleep 超时依赖。

Run: `go test -race ./cmd/claude-mobile-remote ./cmd/channel-test-client`

Expected: PASS，A3 CLI 回归继续通过。

- [ ] **Step 8.2：执行全仓质量门禁**

Run: `gofmt -w internal/pairing internal/channel internal/transport cmd/claude-mobile-remote cmd/channel-test-client`

Run: `go test -race ./...`

Run: `GOFLAGS=-buildvcs=false go vet ./...`

Run: `GOFLAGS=-buildvcs=false go build ./...`

Expected: 全部 PASS；build 不向仓库写二进制。

- [ ] **Step 8.3：验证版本路径与凭证边界**

Run: `go run ./cmd/claude-mobile-remote -version`

Expected: 精确输出 `0.1.0-dev`，不启动 listener、不连接 Claude、不生成 Pair Token。

Run: `rg -n 'pairToken|PairToken|fixture-credential' --glob '!docs/specs/2026-08-24-A4-channel-transport-design.md' --glob '!docs/plans/2026-08-24-A4-channel-transport.md' .`

Expected: 只出现字段名、安全实现和虚构测试 fixture；不得出现高熵真实 token、用户会话或测试 snapshot 凭证。

- [ ] **Step 8.4：复核三端共用契约**

逐项检查并记录：

- envelope 字段与 D27 完全一致，所有 payload 只含 JSON 基础类型；
- snapshot/event 直接映射 A3 纯数据，无 Go/Web/Capacitor 专属字段；
- Pairing payload 与 D28 一致，URL Fragment 只属于 A5 Web 承载建议；
- Go 测试客户端固化认证、订阅、snapshot、result-before-event、重连最新 snapshot 五项行为；
- 无客户端乱序缓存、localStorage、DOM、浏览器线程或平台回调依赖。

Expected: React/TypeScript、Capacitor iOS/Android、HarmonyOS ArkTS/ArkUI 可实现性核对通过；若字段或语义偏离，先更新设计并等待用户确认，不直接改契约。

- [ ] **Step 8.5：写 A4 门禁记录**

`docs/testing/A4-result.md` 必须记录：执行日期、Task 1–7 实际完成范围、每条门禁命令和真实结果、协议闭环摘要、token 泄密扫描结果、三端核对结论、未运行的真实 Claude/移动 Web/VPN 项，以及已知非阻塞限制。不得粘贴配对负载、token 或真实 prompt。

- [ ] **Step 8.6：更新 HANDOFF 并执行最终 diff 检查**

HANDOFF 写明完成项、实际验证、未验证项、A5 前置条件和“等待用户统一检查；未授权 Git 写操作”。

Run: `git diff --check`

Run: `git diff --stat`

Expected: 无 whitespace error；tracked 差异范围与文件地图一致，`go.mod`、`go.sum`、凭证和构建产物不在差异中。对文件地图中的全部新增文件逐个执行 `git diff --no-index --check /dev/null <file>`，每条命令允许因“文件有新增差异”返回 1，但输出必须为空。

- [ ] **Step 8.7：提供人工验收清单并暂停**

用户只需验证：

1. Service 在产生真实 session ID 后输出一次配对负载；
2. `channel-test-client --pair '<payload>'` 显示 authenticated、subscribed 和 snapshot；
3. 输入一行文本后先显示 command accepted，再显示相同 request ID 的 Turn event；
4. 关闭并重连后得到最新 snapshot；
5. 未授权 token 无法获取 session 数据。

在用户反馈前不开始 A5，不执行 `git add`、`git commit` 或其他 Git 写操作。
