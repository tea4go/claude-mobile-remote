# W1 WhaleTerm 进程内无缝远程托管设计

- 日期：2026-08-31
- 状态：多 Session 事件隔离修订草案，待用户检查
- 前置：B3 单 Session Remote Server 组件化
- 关联决策：D57、D58、D62、D63、D64

## 1. 目标与边界

W1 为 WhaleTerm 已经持有的一个 Claude Session 增加“远程管理”入口。“已持有”既包括已启动 SDK Client 的 Session，也包括 WhaleTerm 已预分配稳定 UUID、但尚未发送首条消息的本地 Claude 草稿。用户点击入口后，WhaleTerm 在同一进程内启动本项目的 `remote.Server`，复用已有或随后由统一队列创建的唯一 Claude Client、Session Controller 和输入队列，让手机可以观察和操作当前会话，也可以发起首个 Turn。

核心目标是：Claude 正在执行任务时也可以开启远程管理，不停止 Claude、不恢复 Session、不创建第二个 SDK Client。

W1 不实现以下能力：

- Standalone CLI 的生产级 PTY 或终端模拟器；
- 普通终端中已运行 Claude 进程的事后接管；
- 第二个 SDK Client、第二个 `--resume` Host 或 stdout/JSONL 监听接管；
- Remote Server 全局 daemon、云端 Relay、多设备授权或原生 App 新能力。

Standalone CLI 继续作为独立的单 Session Host；WhaleTerm 负责本地 Claude 草稿和已运行桌面 Session 的无缝远程管理。

## 2. 方案选择

### 2.1 推荐：WhaleTerm 依赖公共 `remote`/`protocol` 包

WhaleTerm 直接导入：

```go
import (
    "github.com/tea4go/claude-mobile-remote/protocol"
    "github.com/tea4go/claude-mobile-remote/remote"
)
```

WhaleTerm 在自身进程内实现一个薄的 `remote.Controller` 适配器，再把适配器交给 `remote.New`。Remote Server 继续负责 pairing、Channel、WebSocket transport 和协议校验；WhaleTerm 继续负责 SDK Client、Claude 生命周期、权限回调、事件来源和输入串行化。

该方案复用 B3 已完成的公共边界，协议核心类型只有 `protocol` 一份，WhaleTerm 不需要导入本项目的 `internal/` 包。

### 2.2 不采用：复制 Remote Server 到 WhaleTerm

复制会形成两套 pairing、transport、生命周期和协议实现，后续修复或协议变更容易分叉，违背 B3 的组件化目标。

### 2.3 不采用：WhaleTerm 启动独立 `claude-mobile-remote` 子进程

独立进程无法访问 WhaleTerm 已有 Client，只能创建或恢复另一个 Host，不能满足当前 Turn 无缝接力和单 Host/单 SDK Client 约束。

## 3. 组件边界

### 3.1 WhaleTerm 宿主

WhaleTerm 保持以下资源的唯一所有权：

- Claude SDK Client；
- 当前 Session ID 和 Session 状态；
- Claude Turn 生命周期；
- PC 与手机输入的统一队列；
- 权限审批、AskUserQuestion 和取消逻辑；
- Claude 事件和快照的内部来源。

WhaleTerm 只在用户请求远程管理时创建 Remote Server，不把 SDK Client 暴露给网络层。

首轮前开启时，WhaleTerm 必须把当前完整 Claude 配置作为宿主模板交给适配器，并预分配合法 UUID。该动作不创建 SDK Client、不启动 Claude 进程；PC 或手机首条输入到达统一 FIFO 后，Manager 才以该模板和正式 `WithSessionID` 创建唯一 Client。

### 3.2 WhaleTerm 适配器

适配器实现 B3 的公共控制契约：

```go
type Controller interface {
    Submit(context.Context, protocol.Input) (string, error)
    Subscribe(context.Context, int) (protocol.Snapshot, <-chan protocol.Event, error)
    Snapshot(context.Context) (protocol.Snapshot, error)
    ResolveAction(context.Context, string, protocol.ActionDecision, []protocol.QuestionAnswer) error
    CancelTurn(context.Context, string) error
    CancelQueued(context.Context, string) error
}
```

适配器职责仅限于：

- 将手机输入转交 WhaleTerm 现有输入队列；
- 将当前 WhaleTerm 状态转换为 `protocol.Snapshot`；
- 将 WhaleTerm 事件转换为 `protocol.Event`；
- 将审批、提问、取消操作转交既有控制逻辑；
- 校验请求和快照中的 Session ID；
- 不直接调用 SDK Client 的非既有入口，不绕过 WhaleTerm 队列。

适配器可以转换 WhaleTerm 的内部 UI/业务模型，但不得复制一套独立的网络协议类型。线上 JSON 仍只由 `protocol` 契约定义。

### 3.3 `remote.Server`

W1 使用 B3 已有生命周期：

```text
WhaleTerm Session
    │
    ▼
WhaleTerm Controller Adapter
    │
    ▼
remote.Server
    ├── Pairing Manager
    ├── Session Channel
    └── WebSocket Transport
```

`remote.Server.Start()` 只启动远程服务、注册当前 Session Channel 并返回配对负载，不调用宿主的 Start/Stop。`remote.Server.Close()` 只关闭远程连接、撤销凭证并注销 Channel，不停止 WhaleTerm 的 Claude Session。

### 3.4 WhaleTerm 内部模块边界

Remote Control 在 WhaleTerm 内作为独立功能组织：

- `backend/remote/` 持有 Controller Adapter、Session Host、事件/协议映射以及 Remote Server 生命周期；
- `backend/services/TClaudeRemoteService` 作为独立 Wails 服务暴露开启、关闭和查询接口，不挂在 `TClaudeCodeService` 上；
- `backend/chat/` 只提供与网络协议无关的窄宿主能力，包括统一输入队列、状态读取、事件订阅和权限/取消控制；
- `frontend/src/features/claudeRemote/` 持有配对、管理 UI、API 与事件同步；会话页面只挂载 feature 并提供必要的会话上下文。

该边界只调整 WhaleTerm 内部依赖方向，不改变 `remote.Controller`、wire protocol、配对负载或移动端状态模型。

### 3.5 按 Session 的事件与广播路径

每个 WhaleTerm Controller Adapter 必须绑定一个确定的 `sessionId`，并且只订阅该 Session Host 的事件源。正确路径为：

```text
Claude Session A Host ──> Adapter A.Subscribe ──> remote.Server A / Channel A
                                                   ├── Subscriber A1 独立 channel
                                                   └── Subscriber A2 独立 channel

Claude Session B Host ──> Adapter B.Subscribe ──> remote.Server B / Channel B
                                                   └── Subscriber B1 独立 channel
```

一条 Session A 事件只发布到 Channel A；Channel A 将其广播给 A 的每个订阅者，各订阅者拥有独立输出 channel。Session B 的适配器和订阅者不得接收该事件。`sessionId` 既是协议字段，也是适配器、Channel、Snapshot 和事件的强一致性校验，不是移动端收到跨 Session 消息后再自行丢弃的 UI 过滤条件。

WhaleTerm 不得在公共 Remote 组件之外再建立广播层。特别禁止：

- Manager 把所有 Session 的事件扇出给所有 Remote Host，再由 Host 按 `chatKey` 或 `sessionId` 过滤；
- 每个 Remote Host 自建本地 Hub，重复实现订阅、广播、慢消费者或关闭语义；
- Host 之间保存“上一个宿主”并转发不匹配事件；
- 让一个 Session 的配对客户端收到另一个 Session 的消息，即使移动端能够按 `sessionId` 忽略。

WhaleTerm 侧允许存在的唯一事件适配工作，是把所属 Session Host 的内部事件映射成 `protocol.Event`。订阅者 fan-out、连接输出和慢订阅者处理属于 `claude-mobile-remote` 的 Channel/Transport 实现。

## 4. 开启与关闭生命周期

### 4.1 开启远程管理

WhaleTerm 当前 Claude 会话页面增加“远程管理”入口：

```text
用户点击远程管理
    │
    ├── 复用现有 Session ID；若为新草稿则预分配稳定 UUID
    ├── 固化当前完整 Claude 宿主配置
    ├── 检查该 Session 尚未启动 Remote Server
    ├── 创建 WhaleTerm Controller Adapter
    ├── 创建 remote.Server
    ├── 调用 server.Start()
    ├── 展示配对负载或二维码
    └── 等待手机通过现有协议连接
```

如果当前草稿尚无 Session ID，WhaleTerm 预分配合法 UUID 并把它作为后续 Claude CLI 的真实 Session ID；这不是临时 ID。Session 已终止时拒绝开启；已有 Remote Server 时复用当前实例，不启动第二个 Server。

开启 Remote Server 不改变当前 Turn；正在运行、等待审批、等待提问或已有排队输入的 Session 都应保持原状态。

### 4.2 关闭远程管理

用户点击“关闭远程管理”、关闭对应 WhaleTerm 页面，或 WhaleTerm 主动释放远程服务时：

```text
关闭 Remote Server
    ├── 阻止新连接和新认证
    ├── 断开手机连接
    ├── 撤销当前 Pair Token
    ├── 注销 Session Channel
    └── 保留 Claude Client、Session、Turn 和队列
```

关闭远程管理不得取消当前 Turn、停止 Claude、删除 Session 或清理上下文。再次开启时创建新的 Remote Server 并生成新的配对负载。

WhaleTerm 退出时应先关闭 Remote Server，再按自身既有流程停止 Claude Session；Remote Server 本身不能反向触发宿主停止。

## 5. 当前 Turn 与并发行为

当前 Turn 正在执行时，手机连接到同一 Session 后：

- 首次订阅收到当前 Snapshot；
- 后续事件来自 WhaleTerm 原有事件流；
- 手机文本进入现有输入队列；
- 审批、问答和取消沿用 WhaleTerm 现有控制语义；
- PC 输入和手机输入继续由同一个队列串行化；
- 不允许 Remote Server 直接并发写 SDK Client。

首轮前连接时 Snapshot 为 `starting/idle`。手机首条输入与 PC 首条输入遵守同一 FIFO；获接受的第一条触发 Client 创建，之后的输入排队。Remote Host 使用开启远程管理时保存的完整 Claude 配置，只替换本轮消息、Turn ID 和规范 Session ID，禁止用仅含项目路径/模型的稀疏配置启动 Client。

WhaleTerm 可为多个由其持有的 Claude Session 分别开启 Remote Server。Remote 模块只维护 `sessionId → Remote 实例` 的宿主生命周期索引；每个实例仍是单 Session Server，拥有独立 Host、监听端口和配对信息，不提供跨 Session 协议路由。移动端在 App 前台生命周期内可同时连接这些独立实例，所有状态和操作继续按 `sessionId` 隔离。

多个实例并发运行时，WhaleTerm 的宿主事件源也必须按 Session 隔离。不能因为 `remote.Server` 已按 Session 建立，就允许其上游继续使用 Manager 级全局事件回调；否则每个 Adapter 都会成为其他 Session 事件的真实订阅者，隔离依赖事后过滤，既会产生错误日志，也会把生命周期、取消和快照更新错误地关联到其他 Host。

## 6. 跨仓库依赖

W1 开发期允许 WhaleTerm 使用本地 `replace` 指向本项目，便于同时修改和验证。公共 API 稳定后，WhaleTerm 改为依赖 `github.com/tea4go/claude-mobile-remote` 的正式版本。

WhaleTerm 只能依赖：

- `remote` 公共 Server API；
- `protocol` 公共协议类型。

WhaleTerm 不得依赖 `internal/session`、`internal/channel`、`internal/transport`、`internal/claude` 或本项目 SDK 私有实现。若 WhaleTerm 需要调用某个功能，应通过 `remote.Controller` 所需的最小公共契约表达。

## 7. 错误处理

- 当前草稿不存在 Session ID：预分配 UUID 并返回给前端；预分配或配置校验失败时才拒绝开启；
- 当前 Session 已启用 Remote Server：复用当前实例或提示已开启，不创建该 Session 的第二个实例；其他 Session 的实例不受影响；
- 首选监听端口已被其他 Remote 实例占用：在 Remote 模块内选择可用端口，并同步生成与实际监听端口一致的配对地址；
- `server.Start()` 任一步失败：WhaleTerm 清理适配器和 Server 资源，不影响 Claude Session；
- 配对负载轮换：沿用 `remote.Server.Pairings()`，由 WhaleTerm 更新二维码或当前展示内容；
- Remote Server 异常关闭：通知 WhaleTerm 远程管理已断开，但不把 Claude Session 标记为失败；
- WhaleTerm Session 终止：先停止接受新的远程命令，再关闭 Remote Server，向手机发送既有终态事件或由重连 Snapshot 恢复终态；
- Session ID 不匹配：在适配器和 Channel 两侧拒绝，不触碰 SDK Client。

## 8. 测试与验收

### 8.1 WhaleTerm 适配器单元测试

至少覆盖：

1. `Submit` 进入 WhaleTerm 现有输入队列；
2. `Snapshot` 和 `Subscribe` 返回当前 Session 的 `protocol` 数据；
3. 审批、问答、当前 Turn 取消和排队取消全部转发到既有控制逻辑；
4. 错误 Session ID 被拒绝且不会调用宿主 Client；
5. 不存在第二条绕过适配器直接写 SDK 的路径。

### 8.2 Remote Server 集成测试

至少覆盖：

1. 已运行的 WhaleTerm Session 可以启动 Server 并返回配对负载；
2. 尚未发送首条消息的本地 Claude 草稿可预分配真实 ID、启动 Server，并由手机首条消息创建唯一 Client；
3. Server 启动和关闭不调用宿主 Start/Stop；
3. 手机 WebSocket 连接能订阅当前 Snapshot 并接收后续事件；
4. 远程输入、审批、问答、取消进入同一个宿主队列；
5. 当前 Turn 执行中开启远程管理不会中断 Turn；
6. 关闭远程管理后宿主 Session 仍可在 WhaleTerm 继续使用；
7. 重新开启生成新的配对负载并可再次连接；
8. 同一 Session 不能重复启动两个 Remote Server；
9. 两个不同 Session 可同时运行独立 Remote Server，分别查询、关闭且不会串线；
10. WhaleTerm 退出时关闭全部 Remote Server，不停止顺序之外的 Claude Session。
11. 两个 Session 同时运行 Turn 时，A/B Adapter 各自只收到所属 Host 的事件，双方 `turn_started`、增量、结果、权限和取消状态均不交叉；
12. 同一 Session 的两个订阅者都收到同一事件，断开或阻塞其中一个不影响另一个；
13. 任意 Snapshot 或事件的 `sessionId` 与 Adapter 绑定值不一致时在服务端拒绝并报告错误，不向移动端发送，也不转发给其他 Host。

### 8.3 人工验收

1. 在 WhaleTerm 中新建本地 Claude 会话但不发送消息，开启远程管理并由手机发送首条消息；
2. 确认 WhaleTerm 使用配对时返回的同一 Session ID，且只有一个 SDK Client；
3. 再在一个已运行 Claude 对话中执行后续验收；
2. 让 Claude 执行一个需要持续一段时间的任务；
3. 任务执行期间点击“远程管理”，扫码连接手机；
4. 手机收到当前状态，并能发送下一条消息或处理审批；
5. 回到 WhaleTerm，确认原 Session 未重启、上下文未丢失；
6. 关闭远程管理，确认 Claude 继续运行；
7. 再次开启远程管理，使用新二维码重新连接；
8. 确认关闭/开启过程没有创建第二个 SDK Client。

门禁命令由 WhaleTerm 的测试环境决定；本项目侧至少重跑 `gofmt`、`go vet`、`go test`，并执行 Remote Server 公共 API 回归。W1 的跨仓库集成证据应记录在 WhaleTerm 或本项目的 `docs/testing/` 中，不能只以编译通过代替当前 Turn 人工验收。

## 9. 交付拆分

W1 实施计划分为以下小步：

1. 盘点 WhaleTerm 当前 Claude Manager、Session 状态、事件流、输入队列和生命周期入口；
2. 确认模块依赖方式，并建立 WhaleTerm 到 `remote`/`protocol` 的最小编译接入；
3. 实现 WhaleTerm Controller Adapter 和单元测试；
4. 在当前 Session 页面接入 Remote Server 开启/关闭生命周期；
5. 接入配对负载展示；
6. 完成 WebSocket、当前 Turn、关闭重开和单 Client 验收；
7. 更新 WhaleTerm 使用文档、本项目 W1 验证记录和交接文档。

## 10. 成功标准

W1 完成的唯一核心判断是：

> WhaleTerm 中首轮前或已经运行的 Claude Session，都可在不创建第二个 SDK Client 的情况下通过“远程管理”生成配对信息；PC 或手机可发起首个 Turn，运行中可无缝接力，关闭远程管理后 Session 仍由 WhaleTerm 正常持有。
