# W1 WhaleTerm 进程内无缝远程托管实施计划

- 日期：2026-08-31
- 状态：设计已确认，计划待执行
- 关联设计：`docs/specs/2026-08-31-W1-whaleterm-in-process-remote-design.md`
- 关联决策：D57、D58
- 前置：B3 `remote`/`protocol` 公共包

## 1. 实施目标

在 WhaleTerm 已有 Claude Session 上增加远程管理入口。用户点击入口后，WhaleTerm 在自身进程内创建并启动 `remote.Server`，使用当前 Session ID 和一个 WhaleTerm Controller Adapter，把手机连接到 WhaleTerm 已有的 Claude Client、Session 状态和输入队列。

完成后必须满足：

- 当前 Turn 执行中可以开启远程管理；
- 首轮前可以预分配真实 Session ID 并开启远程管理，PC 或手机均可发起首个 Turn；
- 不停止、不重启 Claude Session；
- 不创建第二个 SDK Client；
- PC 输入和手机输入仍经过同一队列；
- 关闭远程管理只关闭 Remote Server，Claude Session 继续运行；
- CLI 不新增生产级 PTY；
- 普通终端 Claude 不增加事后接管能力。

## 2. 实施范围与仓库边界

本轮涉及两个本地仓库：

- 本项目 `/Users/zhangym/code/work/utils/claude-mobile-remote`：维护 `remote`/`protocol` 公共 API、W1 文档和必要的公共 API 回归；原则上不再改变 wire contract；
- WhaleTerm `/Users/zhangym/code/work/whale-term`：实现 Controller Adapter、入口 UI、远程 Server 生命周期和 WhaleTerm 侧测试。

除非编译或集成验证发现公共 API 无法表达 WhaleTerm 当前能力，否则不修改本项目协议类型、WebSocket 消息形状或 `remote.Server` 的 Session 语义。若确需公共 API 变化，必须先更新设计、核对三端契约并记录新的决策。

## 3. 阶段拆分

### Task 1：盘点 WhaleTerm 当前 Claude 宿主边界

**目标**：确认已有 Claude Client、Session ID、快照、事件、输入队列、审批/问答/取消和生命周期入口的真实代码位置与调用关系。

**检查范围**：

- `backend/chat/claude_code_cli_new.go`；
- `backend/chat/claude_code_cli_new_runtime.go`；
- `backend/chat/claude_code_cli_new_stream.go`；
- `backend/chat/claude_code_cli_new_lifecycle.go`；
- WhaleTerm 当前会话详情页和“远程管理”入口所在 UI/API；
- WhaleTerm 的 Go module、依赖管理和测试启动方式。

**产出**：

- 明确一个 Session 的唯一 Host 对象；
- 明确 W1 适配器应调用的现有方法；
- 明确哪些 WhaleTerm 状态可以直接映射到 `protocol`，哪些需要适配器内转换；
- 明确远程 Server 应由哪个长生命周期对象持有；
- 记录不应触碰的旧远程控制代码。

**通过条件**：不需要创建第二个 Client，不需要通过 stdout、JSONL 或 PTY 获取事件；可以为当前 Session 找到稳定的宿主控制边界。

### Task 2：建立跨仓库公共依赖

**目标**：让 WhaleTerm 能导入本项目公共包，同时不导入任何 `internal/` 包。

**实施内容**：

1. 在 WhaleTerm 开发环境增加本项目模块依赖；
2. 本地联调阶段使用精确的 `replace` 指向本项目目录；
3. 验证 WhaleTerm 可以导入：

   ```go
   github.com/tea4go/claude-mobile-remote/remote
   github.com/tea4go/claude-mobile-remote/protocol
   ```

4. 编写最小外部编译测试，确认 WhaleTerm 代码不需要导入本项目 `internal/`；
5. 记录正式版本依赖的切换条件，稳定后再替换本地 `replace`。

**通过条件**：WhaleTerm 能编译调用 `remote.New`/`Server.Start`/`Server.Close`，协议类型来自 `protocol` 单一来源，没有复制 DTO。

**注意**：不得把 `go.mod`、`go.sum` 或本地 `replace` 写回本项目提交基准；本项目的模块依赖规则仍按 `AGENTS.md` 执行。

### Task 3：实现 WhaleTerm Controller Adapter

**目标**：将 WhaleTerm 已有 Session Host 适配为 `remote.Controller`。

**建议位置**：WhaleTerm `backend/remote/` 下新增 W1 适配文件；具体文件名以 Task 1 盘点结果为准。不得复用或扩展 WhaleTerm 已废弃的旧 Remote Agent 架构。

**方法映射**：

| `remote.Controller` | WhaleTerm 侧行为 |
| --- | --- |
| `Submit` | 进入现有 Claude Turn 输入队列 |
| `Subscribe` | 订阅现有 Session 事件并返回 `protocol.Snapshot` |
| `Snapshot` | 读取当前 Session 状态 |
| `ResolveAction` | 调用现有审批/问答决议逻辑 |
| `CancelTurn` | 调用现有当前 Turn 取消逻辑 |
| `CancelQueued` | 调用现有排队请求取消逻辑 |

**实现约束**：

- 适配器只持有对 WhaleTerm 现有宿主/Controller 的引用；
- 不在适配器内创建 SDK Client；
- 不从 SDK Client 旁路读取或写入命令；
- 不创建第二份网络协议类型；
- 所有输出都使用 `protocol` 类型；
- 所有请求和快照都校验当前 Session ID；
- WhaleTerm 现有 PC 输入和手机输入必须进入同一个串行队列。

**测试先行**：先为适配器写失败测试，再实现方法映射。至少覆盖输入转发、快照、事件、审批、问答、当前 Turn 取消、排队取消和错误 Session 拒绝。

### Task 4：接入 WhaleTerm 远程管理生命周期

**目标**：在 WhaleTerm 当前 Session 页面增加远程管理入口，并让一个长生命周期对象持有 Remote Server。

**开启流程**：

1. 用户点击“远程管理”；
2. 复用当前真实 Session ID；若为首轮前草稿则预分配稳定 UUID，并固化当前完整 Claude 配置；
3. 检查当前 Session 没有已运行的 Remote Server；
4. 创建 Controller Adapter；
5. 创建 `remote.Config`，填充 Session ID、Controller、监听地址、可达地址、Origin 和配对持久化策略；
6. 调用 `remote.New` 和 `Server.Start`；
7. 将返回的 Pairing 交给 WhaleTerm 的二维码/配对展示层；
8. 监听 `Server.Pairings()` 的轮换通知，更新当前配对展示；
9. 更新 WhaleTerm UI 状态为“远程管理已开启”。

**关闭流程**：

1. 阻止新的远程管理操作；
2. 调用 `Server.Close(ctx)`；
3. 清理 WhaleTerm 持有的 Server、Adapter 和配对展示状态；
4. 保留 Claude Client、Session、当前 Turn 和输入队列；
5. 更新 UI 状态为“远程管理已关闭”。

**边界**：

- `Server.Close` 不得触发 WhaleTerm Claude 的 Stop；
- WhaleTerm 退出时先关闭 Remote Server，再按既有流程关闭 Claude；
- 同一 Session 的重复点击应复用当前 Server 或给出“已开启”状态，不创建第二个 Server；
- 首期不支持同一 WhaleTerm 进程中多个 Session 同时开启远程管理；
- Server 启动失败必须清理已创建资源且不影响 Claude Session。

### Task 5：接入配对展示与开发配置

**目标**：让 WhaleTerm 能展示 W1 返回的配对信息，并适配当前开发/调试网络环境。

**实施内容**：

- 复用本项目现有 Pairing Fragment 和二维码内容，不重新定义协议；
- 优先复用 WhaleTerm 现有二维码展示能力；若没有，则先显示可复制的 Pairing Fragment，再决定是否加入二维码组件；
- 开发期支持本机联调地址；
- 不在 WhaleTerm 日常 UI 暴露 `--listen`、`--advertise-url` 等 CLI 参数；
- 网络地址、Origin 和端口由 WhaleTerm 的开发配置或应用配置提供；
- 配对凭证不写入日志、快照、测试 fixture 或 Git。

**通过条件**：手机可以使用现有 Mobile Web 扫描或粘贴配对负载连接 WhaleTerm；配对展示不会泄露真实凭证到日志。

### Task 6：完成自动化测试

**本项目测试**：

- `remote` 公共 API 编译测试保持通过；
- `remote.Server` 启动、关闭、重复启动和宿主不被停止的测试保持通过；
- 如公共 API 因 WhaleTerm 接入需要变化，补充对应的协议兼容测试。

**WhaleTerm 测试**：

1. Adapter 的六个公共方法映射测试；
2. 当前 Turn 执行期间开启 Remote Server 不触发 Stop/Resume；
3. 远程 Submit 与 PC 输入共享同一队列；
4. 审批、AskUserQuestion、当前 Turn 取消和排队取消通过现有逻辑；
5. 重复开启不创建第二个 Server 或 SDK Client；
6. 关闭 Remote Server 不停止 Claude；
7. WhaleTerm 退出顺序为 Remote Server → Claude Session；
8. 启动失败回滚不污染宿主状态；
9. 当前 Session 之外的 Session ID 被拒绝；
10. 配对轮换能更新展示且旧凭证不再认证。
11. 首轮前开启返回稳定 Session ID，远程首条消息使用完整宿主配置并只创建一个 SDK Client。
12. 首轮 SDK options 使用正式 `WithSessionID`，不再把 `session-id` 放入 `WithExtraArgs`。

### Task 7：跨仓库人工验收

使用假 Host 或可控 Claude 任务完成以下流程：

1. 在 WhaleTerm 中新建本地 Claude 会话但不发送消息，开启远程管理并从手机发送首条消息；
2. 核对配对 ID、WhaleTerm 当前 ID 和 Claude init/result ID 一致，且只有一个 SDK Client；
3. 再创建或恢复一个已运行 Session；
2. 让 Claude 执行一个持续时间较长的任务，或进入审批/提问等待状态；
3. 任务未结束时点击“远程管理”；
4. 扫码或粘贴 Pairing Fragment 连接 Mobile Web；
5. 确认手机收到当前 Snapshot 和后续事件；
6. 从手机发送消息、处理审批或回答问题；
7. 确认 WhaleTerm 中的同一 Session 继续运行，没有重启或丢失上下文；
8. 关闭远程管理；
9. 确认 Claude 仍在 WhaleTerm 中可继续交互；
10. 再次开启远程管理，确认新配对负载可以重新连接；
11. 检查进程和日志，确认全流程只有一个 SDK Client。

人工验收证据应记录实际运行命令、网络地址类型、Session ID 脱敏值、通过项和未验证项，不记录 Pair Token 或会话正文。

## 4. 推荐实施顺序

```text
Task 1 盘点 WhaleTerm 宿主边界
    │
    ▼
Task 2 公共依赖接入
    │
    ▼
Task 3 Controller Adapter + 单元测试
    │
    ▼
Task 4 Server 开启/关闭生命周期
    │
    ├──> Task 5 配对展示/配置
    │
    ▼
Task 6 自动化测试
    │
    ▼
Task 7 跨仓库人工验收
```

Task 5 可以与 Task 4 并行实现，但必须先确定 WhaleTerm 的生命周期持有对象。Task 6 中的 Adapter 测试应在 Task 3 完成后持续运行，不推迟到最后集中补写。

## 5. 失败与回滚策略

- 公共依赖无法在 WhaleTerm 当前 Go module 中稳定接入：暂停实现，先处理模块版本/发布方式，不复制代码；
- WhaleTerm 当前控制边界无法满足 `remote.Controller`：只扩展最小公共宿主接口，不暴露 SDK Client；
- Server 启动失败：释放 Server、Adapter 和监听资源，保留 Claude Session；
- 手机断线：只影响远程连接，WhaleTerm Claude Session 继续运行；
- Remote Server 关闭超时：按 `remote.Server.Close(ctx)` 语义强制收尾并报告远程服务关闭异常，不把宿主 Session 误标记为失败；
- 发现需要第二个 SDK Client、PTY、JSONL 监听或旧 Remote Agent 才能实现：停止该路径并回到 W1 设计评审，不扩大范围。

## 6. 交付与文档

完成后更新：

- WhaleTerm 的开发/使用文档：说明“远程管理”作用于当前已有 Session；
- 本项目 `docs/testing/`：记录 W1 自动化和人工验收结果；
- 本项目 `HANDOFF.md`：记录跨仓库实际改动、验证命令、人工验收和遗留项；
- 必要时更新 `DECISIONS.md`：仅记录新增公共 API、依赖发布方式或范围变化等确认决策。

不更新为“支持普通终端 Claude 事后接管”，也不把 PTY 实现列为 W1 未完成项；它属于明确排除范围。

## 7. 完成门禁

W1 只有在以下条件全部满足时才算完成：

1. WhaleTerm 已运行 Session 可在当前 Turn 中启动远程管理；
2. 手机能完成配对、订阅、消息发送和至少一种交互控制；
3. 关闭远程管理不影响 WhaleTerm Session；
4. 自动化测试覆盖 Adapter、生命周期、单 Client 和 Session 隔离；
5. 本项目 Go 格式化、vet、测试通过；
6. WhaleTerm 对应测试、构建和 lint 通过；
7. 人工验收确认没有重启 Claude、没有第二个 SDK Client、没有 PTY 依赖；
8. 文档明确 Standalone CLI 与 WhaleTerm 两种托管模式的边界；
9. 凭证、会话正文和构建产物没有进入提交。
