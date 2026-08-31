# B3 单 Session Remote Server 组件化设计

- 日期：2026-08-31
- 状态：按 P0 审查意见修订，待用户复核
- 前置：A1–A6、B1、B2、C1 已完成

## 1. 目标与边界

B3 将现有命令目录内的 Pairing、Session Channel 和 WebSocket Transport 组合抽取为根下 `remote/` 公共 Go 包。Standalone CLI 通过该包启动远程服务；后续 WhaleTerm 可在已有 Claude Session 上导入同一公共边界。

B3 不创建第二个 SDK Client，不接管 Claude 生命周期，不实现 WhaleTerm 本身的接入，不实现多 Session Registry、统一 daemon、Control API、持久 device grant、Journal 或 Relay。协议、状态模型、sessionId 隔离、审批/问答/取消命令语义保持 B2 不变。

## 2. 协议类型所有权

协议核心类型最终统一放在根下公共包 `protocol/`，而不是 `remote/`：

- `protocol` 不依赖本项目任何 `internal` 包、SDK 或 Transport 实现；它拥有 `Snapshot`、`Event`、`Input`、`ContentBlock`、`PendingAction`、`Question`、`QuestionAnswer`、`ActionDecision`、状态/事件/输入枚举、`ServiceConfig` 及协议所需的 JSON tag。
- `internal/session` 的现有同名类型改为 type alias/常量 alias 指向 `protocol`，保留现有包路径和绝大多数内部调用方的源码兼容性；不能继续在 alias 上定义的方法或实现细节改为 `protocol` 的无状态函数，或留在 `internal/session` 的辅助函数中。
- `internal/channel`、`internal/transport` 直接引用 `protocol` 类型；Transport 的 `SnapshotPayload`、`EventPayload` 和命令相关类型不再把 `internal/session` 作为协议类型来源。原有内部名称在必要处仅保留 alias，避免测试和 CLI 产生无意义改名。
- 预览截断的共享默认值与纯函数也下放到 `protocol`；`internal/claude` 可保留兼容 alias/wrapper，但不复制实现。

这样 JSON 形状、字段名、枚举值和三端语义只有一份 Go 来源；移动端仍按同一已确认 JSON 契约维护 TypeScript/ArkTS 实现，不新增另一套 Go DTO。`remote/` 可以安全依赖 `protocol`，同时依赖 `internal/channel` 和 `internal/transport`，而 `protocol` 不反向依赖 `remote`，不会产生 import cycle。

## 3. 公共 API

`remote/` 提供不依赖 SDK、且不要求调用方导入 `internal/` 的公共组件 API；协议数据类型统一引用 `protocol/`：

- `Controller`：宿主提供的单 Session 控制边界，包含快照、订阅、文本提交、审批/问答决议、当前 Turn 取消和排队请求取消；方法参数和返回值使用 `protocol` 纯数据类型。
- `Config`：当前 `SessionID`、`Controller`、监听/广播地址、Pair Token TTL、配对持久化文件和 Transport 安全配置。
- `Server`：`New(Config)`、`Start()`、`Pairings()`、`BeginShutdown()`、`Close(context.Context)`。
- `Pairing`：对外返回版本、WebSocket 地址、`sessionId`、Pair Token 和绝对过期时间。

`Server.Start()` 只启动远程服务、注册当前 Session Channel 并签发首个配对负载；不调用 Controller 的 Start/Stop。`Close()` 只关闭远程连接、撤销当前进程凭证并注销 Channel，不停止宿主 Session。凭证轮换仍通过 `Pairings()` 通知调用方。

公共边界不暴露 Registry、WebSocket connection、SDK Client 或内部实现类型。由于 `internal/session.Controller` 在迁移后使用的就是 `protocol` 类型，它可直接满足 `remote.Controller`，不需要逐字段 DTO 转换；WhaleTerm 后续实现同一公共契约即可，只需导入 `remote` 与 `protocol`，不需要依赖本仓库的 `internal` 包。

## 4. 组件与数据流

```text
宿主 Session / Controller
          │ public remote.Controller
          ▼
remote.Server ──> internal Channel ──> internal Transport
     │                  │                    │
     └── Pairing ────────┴── sessionId ──────┘
```

启动顺序由调用方负责：宿主先取得或确认真实 `sessionId`，再创建并启动 `remote.Server`。Server 启动后立即建立唯一 Session Channel 并返回配对负载，因此首个 Turn 前配对能力不回退。所有网络命令仍经 Channel 转到 Controller，Controller 继续负责 FIFO 和状态校验。

Standalone CLI 保留现有终端输入、Controller Start/Stop 和 EOF 结算逻辑，只把当前 `remoteRuntime` 的直接内部组合替换为公共 Server 与适配器。真实 Claude Host 的 Start/Stop 不由 Remote Server 触发。

## 5. 生命周期与错误处理

- `New` 拒绝空 Session ID、空 Controller、无效地址/TTL 或无效 Transport 配置。
- `Start` 是幂等边界之外的单次操作；启动 Transport、注册 Channel 或签发凭证任一步失败，都回滚已完成的注册/监听资源。
- 启动后 Pairing Manager 的过期轮换继续运行；轮换负载通过有界 `Pairings()` 通道发布，慢消费者只保留最新值。
- `BeginShutdown` 立即阻止新连接并使 Pairing Manager 停止接受新认证。
- `Close(ctx)` 等待远程连接关闭；超时返回 context 错误并强制收尾。关闭操作可重复调用。
- Remote Server 关闭不改变宿主 Session 状态；CLI 退出时仍由 CLI 显式停止 Controller。
- 所有 public-to-internal 转换都校验 `sessionId`，不允许错误 Session 穿透到 Channel 或 Controller。

## 6. 测试与验收

新增公共包测试和 CLI 回归测试，至少覆盖：

1. `protocol` 是所有共享协议核心 Go 类型的唯一来源；序列化 JSON 与 B2 fixture 逐项保持不变。
2. 外部测试包可同时导入 `remote`/`protocol`，实现并调用公共 Controller 契约，不需要导入 `internal/`。
3. Server 启动返回当前 Session 的首个 Pairing，并注册唯一 Channel。
4. 配对轮换可从 `Pairings()` 取得最新负载，旧凭证不产生新的认证权限。
5. WebSocket 鉴权、订阅、文本/审批/问答/取消命令仍通过公共 Controller，错误 Session 被拒绝。
6. Server 关闭会断开远程连接并注销 Channel，但宿主 Controller 不收到 Stop 调用。
7. Standalone CLI 的首轮前配对、双向文本、重连和现有 B2 行为回归通过。

验证命令：`gofmt -l .`、`GOFLAGS=-buildvcs=false go vet ./...`、`GOFLAGS=-buildvcs=false go test -count=1 ./...`；新增外部 API 编译测试必须在仓库外观测到“不依赖 internal”；若改动只涉及 Go，不新增前端门禁。
