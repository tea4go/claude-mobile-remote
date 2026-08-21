# A2 SDK 托管设计

- 日期：2026-08-21
- 子阶段：A2 SDK 托管（里程碑 A）
- 状态：已确认并完成实施
- 依据：`REQUIREMENTS.md` §9.1、总体设计 §2–§4、D2、D15、D18

## 1. 目标

A2 在独立项目中完成 `github.com/tea4go/claude-agent-sdk-go` 的最小托管验证：CLI Service 持有一个 SDK Client，顺序执行多个文本 Turn，输出 Claude 文本事件，捕获真实 `sessionId`，并在 `ResultMessage` 后继续使用同一个 Session。

A2 必须用真实 Claude Code CLI 完成双轮验证，确认以下设计假设：

1. `SystemMessage(subtype=init)` 在首轮最终结果前提供真实 `session_id`；
2. `ResultMessage.SessionID` 可作为校验和兜底来源；
3. 一个 Turn 的 `ResultMessage` 不会结束 SDK Client，第二个 Turn 可延续上下文；
4. `ReceiveResponse` 能同时传递消息与流错误，适合作为项目统一接收入口。

## 2. 范围边界

### 2.1 A2 实现

- `internal/claude` 薄 SDK Adapter；
- SDK Client 的连接、Query、消息接收和关闭；
- Assistant 文本提取；
- 真实 `sessionId` 捕获、来源标记和一致性校验；
- 单 Client 上的顺序多 Turn；
- 简单行式 CLI：输入一行执行一个 Turn，`:quit` 或 EOF 停止；
- fake 单元测试和真实 Claude 双轮门禁。

### 2.2 A2 不实现

- A3 的 Session/Turn 状态机、统一输入队列、snapshot 和并发控制；
- A4 的 Channel、WebSocket、鉴权、JSON envelope 与配对；
- 二维码、移动端数据流、审批 UI、取消命令和恢复协议；
- 多会话、持久化 transcript 或应用自造 session ID。

A2 CLI 自身按读取顺序同步调用 Adapter，因此不会产生并发 Query。Adapter 的 `RunTurn` 明确不支持并发调用；A3 再由 Session Controller 统一保证串行。

## 3. 方案选择

采用“薄 SDK Adapter”方案：CLI 依赖项目自有接口，不直接处理 SDK Message；Adapter 只转换 A2 所需事实，不提前定义 Session Controller 或网络协议。

未采用的方案：

- CLI 直接调用 SDK：文件少，但 SDK 类型和生命周期会进入 `main.go`，A3 需要重新拆分；
- A2 提前实现完整 Session Controller：会混入 A3 状态机与并发范围，扩大门禁失败面。

## 4. 组件与文件

```text
cmd/claude-mobile-remote/
├── main.go                 # flag、信号、退出码
└── run.go                  # 可测试的行式 CLI 编排

internal/claude/
├── host.go                 # Host 生命周期与 RunTurn
├── event.go                # A2 内部事件模型
├── messages.go             # SDK Message 转换
├── host_test.go            # fake Client/iterator 生命周期测试
├── messages_test.go        # 消息转换与 sessionId 测试
└── live_test.go            # build tag=integration 的真实双轮门禁
```

`main.go` 只负责进程入口；`run.go` 接收输入输出和 Host 接口，使 CLI 行为可用 fake 自动测试。SDK 具体类型只允许出现在 `internal/claude`。

## 5. Adapter 接口

核心对象为单 Client 的 `Host`：

```go
type Host struct { /* SDK client、sessionId、连接状态 */ }

func NewHost(options ...Option) *Host
func (h *Host) Connect(ctx context.Context) error
func (h *Host) RunTurn(ctx context.Context, prompt string, emit func(Event)) error
func (h *Host) SessionID() string
func (h *Host) Close() error
```

内部使用窄化接口包装 SDK Client，测试 fake 只需实现 `Connect`、`Query`、`ReceiveResponse` 与 `Disconnect`。生产工厂调用 `claudecode.NewClient`。

事件仅供 CLI 和后续 A3 适配，不是三端通信协议：

```go
type Event struct {
    Kind      EventKind
    SessionID string
    Text      string
    Source    SessionIDSource
}
```

A2 事件种类：

- `session_started`：首次捕获真实 `sessionId`；
- `text`：Assistant 文本块或必要的 Result 文本兜底；
- `turn_completed`：收到成功 `ResultMessage`；
- `turn_failed`：收到错误 Result；普通 Go 错误仍由 `RunTurn` 返回。

## 6. 生命周期与数据流

启动流程：

1. CLI 创建 Host；
2. Host 以 Service 生命周期 context 调用 SDK `Connect`；
3. CLI 循环读取非空行；
4. 每行调用一次 `RunTurn`；
5. `RunTurn` 调用 `Query`，随后创建 `ReceiveResponse` iterator；
6. 持续 `Next(ctx)`，直到成功或失败的 `ResultMessage`；
7. 返回 CLI 输入循环，Client 保持连接；
8. `:quit`、EOF、进程信号或启动失败后调用 `Close`。

每个 Turn 使用独立 context；Host 连接使用更长生命周期 context。Turn context 取消只终止当前等待，A2 不把它实现成产品取消命令。B2 才接入 `Interrupt` 与远程取消。

## 7. sessionId 与 Turn 语义

Host 不生成 session ID。捕获规则：

1. 首选 `SystemMessage` 的 `subtype == "init"` 且 `Data["session_id"]` 为非空字符串；
2. 若 init 未提供，则使用 `ResultMessage.SessionID` 兜底；
3. 已有 session ID 后，任何非空的新值必须与其一致，否则返回可诊断的一致性错误；
4. `session_started` 只发射一次，并用 `Source` 标记来自 `init` 或 `result`；
5. 真实门禁必须确认 Source 为 `init`，不能只依赖兜底路径通过。

收到成功 `ResultMessage` 只发射 `turn_completed` 并结束当前 `RunTurn`。Host 不关闭 Client、不清空 session ID，随后可以执行下一 Turn。

错误 Result 只使当前 Turn 失败，不自动把 Session 判定为终止；A3 根据 Adapter 错误类别维护 Session/Turn 状态。

## 8. 消息转换

- `AssistantMessage`：只处理顶层 Assistant 的 `TextBlock`；带 `Error` 的消息终止当前流并按 §10 分级；A2 不渲染 Thinking、Tool 或子代理消息；
- `SystemMessage(init)`：捕获 session ID，不输出完整 Data；
- `ResultMessage`：校验 session ID、结束 Turn；若该 Turn 尚未输出 Assistant 文本且 `Result` 非空，可作为文本兜底，避免重复输出；
- 其他消息：A2 忽略，但不得因此提前结束 iterator；
- iterator 在 Result 前返回 `ErrNoMoreMessages`：视为异常结束；
- iterator 的其他错误：原样包装为接收错误。

## 9. 权限与安全

- 使用 SDK 默认权限模式，不启用 `bypassPermissions`；
- 配置 `WithCanUseTool`，对进入回调的工具请求统一拒绝，理由为“A2 尚未实现审批”；
- Claude Code 自动允许而不进入回调的只读工具不作为 A2 权限控制承诺；真实门禁 prompt 明确要求不使用工具；
- 不打印用户 Claude 配置、环境变量、认证信息或完整 SDK 原始消息；
- `sessionId` 可显示用于验证，但不与未来 `pairToken` 混淆。

## 10. 错误处理

错误需要保留根因并增加阶段上下文。Adapter 使用可判定 scope 的错误包装：`turn` 表示当前 Turn 失败但 Client 仍可使用，`session` 表示 SDK 流状态已不可信，CLI 必须关闭 Host。A3 后续直接复用该分类驱动状态机。

| 场景 | A2 行为 |
| --- | --- |
| Claude CLI 不存在 | 返回 session scope 的“连接 Claude CLI”错误，CLI 非零退出 |
| SDK Connect 失败 | 返回 session scope 错误，不进入输入循环，调用安全关闭 |
| 空输入 | 忽略，不创建 Turn |
| Query 失败 | 返回 session scope 错误，CLI 停止；不自动重试 |
| Assistant 带错误 | 错误 assistant 终止当前流：发射 `turn_failed`；`server_error`/`rate_limit` 为 turn scope（可重试），认证/计费等为 session scope。错误信息含类型与文本 |
| Result `IsError` | 发射 `turn_failed` 并返回包含 SDK result/errors 的 turn scope 错误；CLI 报告后继续读取下一行 |
| Result 前流结束 | 返回 session scope 的“未收到 ResultMessage”错误 |
| session ID 缺失 | 当前 Turn 结束时返回 session scope 的验证错误 |
| session ID 改变 | 返回 session scope 错误并包含旧值/新值，但日志不包含任何凭证 |
| Disconnect 失败 | CLI 输出关闭错误；若已有主错误，不覆盖主错误 |

A2 不设计自动重连或 resume；这些能力在 A3/B4 结合状态机设计。

## 11. 依赖策略

- `go.mod.base` 增加 `github.com/tea4go/claude-agent-sdk-go v0.1.1`；
- 本地忽略的 `go.mod` 使用 `replace github.com/tea4go/claude-agent-sdk-go => ../claude-agent-sdk-go`，验证用户维护版源码；
- 本地 SDK 已确认位于 `/Users/zhangym/code/work/utils/claude-agent-sdk-go`，工作区干净且 HEAD/tag 为 `v0.1.1`；
- `go mod tidy` 生成的 `go.sum` 继续按 D18 忽略；
- 不复制 WhaleTerm 类型、状态机或 Remote Agent 代码，只参考 SDK 生命周期用法。

## 12. 测试与门禁

### 12.1 单元测试

使用 fake Client 与 fake iterator 覆盖：

- Connect/Close 正常与错误路径；
- Query 只按调用顺序执行；
- init 捕获 session ID；
- Result 兜底捕获；
- init/Result 或跨 Turn session ID 不一致；
- Assistant 文本与 Result 文本去重；
- 成功 Result 结束 Turn 但不关闭 Host；
- 错误 Result、iterator 错误和 Result 前 EOF；
- CLI 空行、`:quit`、EOF、两行顺序与错误退出。

### 12.2 真实 Claude 门禁

`live_test.go` 同时使用 `//go:build integration` 与显式环境变量保护，普通 `go test ./...` 不消耗 Claude 额度。门禁命令显式开启真实测试。

真实测试在同一个 Host 上执行两个 Turn：

1. 生成无敏感信息的随机 code word，要求 Claude 记住并原样回复，且明确禁止使用工具；
2. 要求只返回上一 Turn 的 code word；
3. 断言首轮在 Result 前收到来源为 init 的非空 session ID；
4. 断言两轮 Result 的 session ID 一致；
5. 断言第二轮文本包含 code word，证明上下文延续；
6. 第二轮完成后再关闭 Host。

门禁命令：

```bash
go test ./...
go test -race ./internal/claude
gofmt -l .
go vet ./...
go build ./...
CLAUDE_MOBILE_REMOTE_LIVE=1 go test -tags=integration -run TestLiveTwoTurnSession -v ./internal/claude
```

真实测试记录只保存命令、Claude Code/SDK 版本、session ID 是否非空且一致、Turn 数与通过/失败结论；不保存完整会话内容或认证信息。

## 13. 三端共用契约核对

A2 只新增 PC 内部 SDK Adapter，不修改总体设计 §8.1 的通信协议、Session/Turn 状态模型、配对负载或三端验收用例。因此本设计不会引入浏览器、Capacitor 或 HarmonyOS 特有依赖，也不需要新增协议决策。

A3 将把 Adapter 事件映射到正式 Session/Turn 状态；A4 定义网络消息时，必须再次执行三端共用契约核对并记录决策。

## 14. 验收结论

A2 只有在单元测试、构建检查和真实 Claude 双轮测试全部通过后才可标记完成。若 init 不提供 session ID、两轮 session ID 改变、Result 后无法继续 Query，视为设计假设失败，停止进入 A3 并更新总体设计与决策记录。
