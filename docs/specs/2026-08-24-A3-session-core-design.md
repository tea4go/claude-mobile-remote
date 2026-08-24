# A3 Session Core 设计

- 日期：2026-08-24
- 子阶段：A3 Session Core（里程碑 A）
- 状态：已确认，待实施
- 依据：`REQUIREMENTS.md` §5.1、§9.1，总体设计 §2–§4、§8.1，D15、D19、D21、D24

## 1. 目标

A3 在 A2 薄 SDK Adapter 之上建立单 Session 的业务核心：由一个 `SessionController` 统一接收终端和后续手机文本输入，分配 request ID，通过有界 FIFO 串行执行 Turn，维护 Session/Turn 状态与最小 snapshot，并按单调递增的 `seq` 发布领域事件。

A3 完成后，CLI 不再直接调用 `claude.Host.RunTurn`。A4 的 Session Channel 也必须复用同一个 Controller 输入和事件边界，任何入口都不能绕过 Controller 并发写 SDK Client。

## 2. 范围边界

### 2.1 A3 实现

- `internal/session` 的 Session/Turn 状态模型和纯数据 snapshot；
- 单 Actor 事件循环，独占 Session 状态、输入 FIFO、当前 Turn、request ID 和 `seq`；
- 容量为 32 的等待队列；
- `Submit` 的异步接受语义和服务端 request ID；
- 唯一 Host Worker，串行调用 A2 `Host.RunTurn`；
- Adapter 事件到 A3 领域事件、状态和错误 scope 的转换；
- 多个可信进程内订阅者的有序事件发布；
- Controller 停止、当前 Turn 取消和 Host 关闭；
- CLI 迁移到 Controller，并支持连续输入真实排队、EOF 排空与主动停止；
- fake Host 驱动的状态、并发、snapshot、CLI 和 race 自动化测试。

### 2.2 A3 不实现

- A4 的 Session Channel、WebSocket、JSON envelope、鉴权、二维码或配对 token；
- 网络慢客户端的发送缓冲、断线重连和订阅隔离；
- Tool 审批、待回答事项和独立 Turn 取消命令；
- SDK resume、自动重试、完整事件历史或 transcript 持久化；
- 多 Session、多进程恢复、多机器和任何移动端代码。

## 3. 方案选择

采用“单 Actor + 单 Host Worker”方案：

- Actor 是 Session 状态、FIFO、当前 Turn、request ID 分配和事件 `seq` 的唯一写入者；
- Worker 是 `claude.Host` 的唯一调用者，只负责 `Connect`、`RunTurn` 和 `Close`；
- `Submit`、`Snapshot`、订阅和停止请求都先进入 Actor；
- Worker 把 Adapter 事件和调用完成结果送回 Actor，不直接修改领域状态或发布领域事件。

未采用的方案：

- mutex + condition variable：可以串行执行，但状态、队列、snapshot、停止和事件排序分散在多个锁域，race 和死锁面更大；
- Controller 内同步调用 `RunTurn`：实现简单，但会阻塞 snapshot、停止、连续输入和事件订阅，无法形成 A4 所需的异步边界。

## 4. 组件与文件

```text
internal/session/
├── controller.go       # 对外 API、Actor 命令和生命周期
├── model.go            # Session/Turn 状态、snapshot 和错误数据
├── input.go            # 输入来源、request ID、校验与容量为 32 的 FIFO
├── event.go            # A3 领域事件与单调 seq
├── worker.go           # 唯一 Host Worker 与 Adapter 事件回传
└── *_test.go           # 状态、并发、错误、订阅和停止测试

cmd/claude-mobile-remote/
├── main.go             # 进程入口和信号保持不变
├── run.go              # 改为 Controller 的异步输入/事件编排
└── run_test.go         # EOF 排空、排队、停止和退出码
```

`internal/session` 只依赖项目自有的窄 Host 接口和 `internal/claude` 事件，不暴露 SDK 类型。A2 Adapter 继续负责 SDK 消息解析与错误 scope；A3 不重复解析 Claude 私有输出。

## 5. Controller 接口与所有权

接口以职责为准，实施时可在不改变语义的前提下调整 Go 命名：

```go
type Input struct {
    SessionID string
    Source    InputSource
    Text      string
}

type Controller struct { /* actor、worker、service context */ }

func NewController(host Host, options ...Option) *Controller
func (c *Controller) Start(ctx context.Context) error
func (c *Controller) Submit(ctx context.Context, input Input) (requestID string, err error)
func (c *Controller) Snapshot(ctx context.Context) (Snapshot, error)
func (c *Controller) Subscribe(ctx context.Context, capacity int) (Snapshot, <-chan Event, error)
func (c *Controller) Stop(ctx context.Context) error
```

`Start` 启动 Actor 和 Worker，并等待 `Host.Connect` 成功或失败。Connect 成功只表示 Claude CLI 已连接；在 Adapter 提供真实 `sessionId` 前，Session 仍为 `starting`。

传给 `Start` 的 service context 用于触发主动停止，但 Actor 使用独立的内部生命周期收敛状态和最终事件；每个 Turn 另有可单独取消的 context。service context 取消时，Controller 发起与显式 `Stop` 相同的流程，而不是直接杀死 Actor。

`Subscribe` 在 Actor 内原子完成“登记订阅者 + 读取 snapshot”，返回的 snapshot 记录基准 `seq`，随后该订阅只接收更大的 `seq`，避免 A4 在“先 snapshot、再订阅”之间丢失事件。独立 `Snapshot` 用于普通状态查询。

订阅者是 CLI、A4 Channel 等可信进程内消费者。Actor 对每个订阅者保持同序背压以避免静默丢事件；订阅容量必须大于 0，消费者必须持续读取或及时取消自己的订阅 context。A4 的网络连接不得直接订阅 Controller，而由 Channel 快速消费后自行处理慢客户端。Controller 不为不消费且不取消的错误内部订阅者提供可用性保证。

## 6. 输入与 request ID

### 6.1 接受规则

`Submit` 由 Actor 串行执行以下步骤：

1. Session 只能处于 `starting` 或 `ready`；`stopped`、`failed` 拒绝新输入；
2. 文本去除首尾空白后必须非空，并沿用 CLI 的 1 MiB 上限；
3. 在真实 session ID 已知时，非空目标 ID 必须严格相等；手机命令在 A4 必须始终带目标 ID；
4. 只有真实 session ID 尚未产生时，终端首条输入才允许目标 ID 为空；
5. 等待队列未满时，使用标准库 `crypto/rand` 生成 128-bit 小写十六进制 request ID；生成器通过测试选项注入；
6. Actor 先向 `Submit` 返回接受结果，再派发 `turn_started`，保证调用方可以先登记 request ID；
7. 当前没有运行中 Turn 时立即派发，否则进入 FIFO。

容量 32 只计算等待中的输入，不计算正在执行的 Turn，因此上限为“1 个运行中 + 32 个等待中”。FIFO 顺序以 Actor 实际收到 `Submit` 命令的顺序为准，不承诺不同 goroutine 的墙上时钟先后关系。

### 6.2 同步拒绝

入队前拒绝不生成 request ID，也不发布 Turn 事件：

| 错误 | 场景 |
| --- | --- |
| `ErrInvalidInput` | 空白文本、超过输入上限或非法来源 |
| `ErrQueueFull` | 已有 32 个等待项 |
| `ErrSessionMismatch` | 目标 session ID 与当前真实 ID 不同 |
| `ErrSessionUnavailable` | Session 已停止、失败或 Controller 未启动 |
| `ErrInternal` | request ID 生成等内部步骤失败 |

## 7. 状态模型与 snapshot

### 7.1 Session 与 Turn 状态

Session 沿用总体设计状态：

- `starting`：Host 已开始生命周期，但尚未取得真实 session ID；
- `ready`：已取得真实 session ID，可以接受和执行 Turn；Session 为 `ready` 时 Turn 可以同时为 `running`；
- `stopped`：Controller 被主动停止；
- `failed`：Host 流或 Session 一致性已不可信。

Turn 沿用总体设计状态：

- `idle`：尚无 Turn；
- `running`：队首输入正在执行；
- `completed`：最近一次 Turn 成功结束；
- `failed`：最近一次 Turn 因 Turn scope 错误结束，或运行中遇到 Session scope 错误；
- `cancelled`：A3 仅在整个 Controller 停止并取消当前 Turn 时产生；独立 Turn 取消仍留给 B2；
- `waiting`：A3 保留枚举但不产生，留给 B2 的审批或问答等待。

Session 与 Turn 的关键转换：

| 触发 | Session | Turn | 后续动作 |
| --- | --- | --- | --- |
| Connect 成功 | `starting` | `idle` | 等待首条输入 |
| 首条输入派发 | `starting` | `running` | Worker 执行 `RunTurn` |
| 捕获真实 session ID | `ready` | 保持当前值 | 发布 `session_started` |
| Turn 成功 | `ready` | `completed` | 有队列则开始下一项 |
| Turn scope 错误 | `ready` | `failed` | 有队列则开始下一项 |
| Session scope 错误 | `failed` | 运行中则 `failed` | 清空队列，拒绝新输入 |
| Stop 且有运行中 Turn | `stopped` | `cancelled` | 取消 Turn、清空队列、关闭 Host |
| Stop 且无运行中 Turn | `stopped` | 保留最近终态 | 清空队列、关闭 Host |

若 Session 已经 `failed`，后续 `Stop` 只完成资源关闭，不把状态覆盖为 `stopped`，以保留根因。

### 7.2 消息保留

`running` 时累积当前 Turn 的 `text_delta`。Turn 结束后只保留最近一次 Turn 的结果；下一次 Turn 开始时清空旧文本、错误和 request ID 并建立新的当前 Turn。A3 不读取、保存或下发完整历史。

### 7.3 Snapshot

Snapshot 是可直接映射为 JSON 的纯数据：

```text
Snapshot
├── sessionId          真实 ID；init 前为空
├── sessionStatus      starting / ready / stopped / failed
├── sessionError       null 或 { code, message }
├── seq                最近发布的领域事件序号
├── turn
│   ├── status         idle / running / waiting / completed / cancelled / failed
│   ├── requestId      当前或最近 Turn 的 request ID
│   ├── source         terminal / mobile
│   ├── message        当前累积或最近一次输出
│   ├── error          null 或 { code, message }
│   └── pendingAction  A3 固定为 null
└── queueDepth         等待队列数量，不含运行中 Turn
```

总体设计中的连接状态和 `clients` 由 A4 Channel/Transport 持有；A4 返回订阅 snapshot 时组合连接信息，不让 Session Core 依赖网络对象。

## 8. 事件模型与顺序

A3 领域事件不是 A4 JSON envelope，但字段必须保持纯数据并可无损映射：

```text
Event
├── seq
├── kind
├── sessionId
├── requestId          Session 事件可为空
├── source             Turn 事件携带
├── text               仅 text_delta 使用
├── status             必要的 Session/Turn 终态
└── error               null 或 { code, message }
```

事件种类：

- `session_started`；
- `turn_started`；
- `text_delta`；
- `turn_completed`；
- `turn_failed`；
- `session_failed`；
- `session_stopped`。

Actor 是唯一 `seq` 分配者。初值为 0，每发布一个领域事件递增 1；读取 snapshot 不递增。所有订阅者获得相同事件内容和顺序。

首个 Turn 可能在真实 ID 产生前已进入内部 `running`。正常路径中，Actor 暂存预初始化文本等 Adapter 事件；取得 ID 后按 `session_started → turn_started → text_delta` 顺序发布，保证未来可映射到协议的事件都有有效 session ID。若 Session 在取得 ID 前失败或被停止，则向本地进程内订阅者发布 session ID 为空的 `session_failed` 或 `session_stopped` 以结束 CLI 生命周期；A4 此时尚不能建立 Channel，不得把该内部事件序列化为协议消息。

Adapter 的 `turn_completed` / `turn_failed` 是 Turn 终态的首要事实。Worker 完成通知负责释放执行槽；若调用异常返回但此前没有终态事件，Actor 按 `claude.ScopeOf(err)` 补齐 Turn 或 Session 失败，且不得重复发布同一 Turn 的终态。

## 9. Worker、错误与停止

Worker 一次只接收一个任务，并为每个任务使用可独立取消的 Turn context。它按以下顺序持有 Host：

1. 启动时调用一次 `Connect`；
2. 每次只调用一个 `RunTurn`；
3. Adapter 回调只写入 Actor 事件通道；
4. Turn 返回后报告完成结果，再等待下一项；
5. 收到 Actor 的关闭命令后，在当前调用退出后调用一次幂等 `Close`。

Turn scope 错误将当前 Turn 置为 `failed`，Session 保持 `ready` 并继续 FIFO。Session scope 或未知错误将 Session 置为 `failed`、清空等待队列并发布一次 `session_failed`；已接受但尚未执行的请求随 Session 终态统一失效，不伪造 `turn_started` 或 `turn_failed`。

`Stop` 先通过 Actor 设置内部 stopping 标记并拒绝新输入，再取消当前 Turn context；stopping 不是新的对外 Session 状态。不能先取消整个 Actor 生命周期，否则会丢失最终 `session_stopped` 事件。Worker 从 `RunTurn` 返回并独占完成 `Host.Close` 后，Actor 再收敛最终状态：

- 立即拒绝后续 `Submit`；
- 当前 Turn 如存在则记为 `cancelled`；
- 清空等待队列；
- Worker 独占调用 `Host.Close`；
- 主动停止发布一次 `session_stopped`；
- 最终事件发布后关闭订阅通道并结束 Actor 生命周期；
- 多次 `Stop` 幂等，Close 失败由 `Stop` 返回，但主动停止状态仍为 `stopped`。

若 context 取消来自主动停止，不把 Adapter 返回的取消错误误报为 `session_failed`。

## 10. CLI 迁移

CLI 使用一个 select 循环同时处理扫描结果、Controller 事件和进程 context：

- 普通非空行立即 `Submit`，不等待当前 Turn 完成；
- `Submit` 成功后按 request ID 记录尚未终止的已接受请求；
- `text_delta` 保持流式输出，Turn 终态负责换行和错误输出；
- 入队拒绝打印清晰错误，但除 Session 不可用外不主动结束 Service；
- EOF 表示不再接受终端输入，CLI 继续消费事件，直到已接受请求全部结束后正常停止；
- `:quit` 或进程 context 取消立即调用 `Stop`，不等待队列排空；
- Session scope 失败输出错误并以退出码 1 结束；正常 EOF、`:quit` 和信号停止沿用成功退出语义；
- CLI 不直接调用 Host，也不自行维护 Session/Turn 真相。

Scanner 的 1 MiB 限制保留，同时 Controller 再做同一上限校验，避免 A4 输入绕过边界。

## 11. 安全与数据边界

- request ID 使用加密随机值，但不是凭证；不得与未来 pair token 复用；
- snapshot 和事件只保留当前或最近 Turn 的界面数据，不落盘 transcript；
- 不记录 prompt、完整 Claude 原始消息、用户配置或认证信息；
- 错误对领域层转换为稳定 code 和可诊断 message，不暴露 Go error 类型；
- 预初始化事件缓冲只存在于当前进程内，在 Turn/Session 终止或 Stop 后释放；
- A3 不生成二维码，也不接触任何配对凭证。

## 12. 测试与门禁

### 12.1 Controller 与模型测试

使用可控 fake Host 覆盖：

- Start/Connect 成功和失败，真实 session ID 仍由 Adapter 事件产生；
- 全部合法状态转换和非法终态后的 Submit 拒绝；
- 首 Turn 的预初始化事件按 `session_started → turn_started → text_delta` 发布；
- init 前 Session 失败可以终止本地订阅者，但不能形成无 session ID 的网络契约；
- 多 goroutine、terminal/mobile 来源并发 Submit 后，执行顺序与 Actor 接收顺序一致且 Host 最大并发数为 1；
- 1 个运行中和 32 个等待中可接受，第 33 个等待项返回 `ErrQueueFull`；
- request ID 唯一，拒绝请求不分配 ID，接受回复先于对应 `turn_started`；
- snapshot 在 `starting`、`running`、`completed`、Turn 失败、Connect/Session 失败和 `stopped` 下正确，Session 失败根因保留在顶层 `sessionError`；
- 文本累积、最近 Turn 保留和下一 Turn 清空；
- Turn scope 错误后继续下一项；Session scope 错误清空队列并拒绝新输入；
- Stop 取消当前 Turn、清空队列、关闭 Host 一次，重复 Stop 幂等；
- 多订阅者收到相同 `seq` 和顺序；原子 Subscribe 的 snapshot `seq` 与后续事件无缺口。

### 12.2 CLI 测试

- 连续多行在首 Turn 未结束时仍可成功入队；
- 流式文本、每 Turn 换行和错误输出；
- Queue full 等同步拒绝可诊断；
- EOF 等待已接受任务排空；
- `:quit` 与 context 取消立即停止；
- Turn scope 失败后继续，Session scope 失败以退出码 1 结束；
- 启动和关闭错误不丢失主错误。

### 12.3 门禁命令

```bash
go test -race ./internal/session ./cmd/...
go test -race ./...
gofmt -l .
GOFLAGS=-buildvcs=false go vet ./...
GOFLAGS=-buildvcs=false go build ./...
```

A3 门禁是自动化测试，不要求新增真实 Claude 消费测试；A2 已验证的 SDK 正常多 Turn 假设继续作为前置。若 Controller 需要改变 A2 Adapter 事件语义，必须先补 Adapter 回归测试。

## 13. 三端共用契约核对

A3 正式细化总体设计中的状态模型与 snapshot，因此属于 D19 要求核对的共用契约变更，结论记录为 D24：

- `sessionId`、request ID、状态、来源、`seq` 和错误 code 使用字符串；
- `queueDepth` 使用非负整数；message 使用字符串；error 与 `pendingAction` 使用可空纯数据对象；
- 所有字段均可用 JSON 表达，不依赖 Go channel/context/error，也不依赖 DOM、`localStorage` 或 Capacitor 插件；
- React/TypeScript、Capacitor iOS/Android 与 HarmonyOS ArkTS/ArkUI 均能实现相同枚举、snapshot 和事件归并；
- Actor、Worker 和订阅背压属于 PC 内部实现，不进入三端协议；
- A4 负责把领域事件映射到带 `version`、`type`、`sessionId`、request ID 和 `seq` 的 JSON envelope，并组合连接状态；
- init 前 session ID 为空的内部失败事件只供本地 CLI，不进入任何移动端协议。

因此 A3 不引入端特有依赖，也不改变配对负载。A4 固定 JSON 字段时仍需再次核对通信协议并更新决策。

## 14. 验收结论

A3 只有在 §12 自动化门禁全部通过，并在 `docs/testing/A3-result.md` 记录状态、队列、snapshot、停止和 race 结论后才可完成。门禁失败时不得进入 A4；若发现 snapshot 无法支持无历史恢复，先回到设计与 D24 核对，不得让移动端自行保存完整 Claude 上下文补洞。
