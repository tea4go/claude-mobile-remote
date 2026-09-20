# W1 Task 1 盘点：WhaleTerm 当前 Claude 宿主边界

- 日期：2026-09-09
- 关联：W1 实施计划 Task 1（`docs/plans/2026-08-31-W1-whaleterm-in-process-remote-plan.md`）、W1 设计 §3（`docs/specs/2026-08-31-W1-whaleterm-in-process-remote-design.md`）
- 结论：**通过**。不需要创建第二个 SDK Client，不需要通过 stdout、JSONL 或 PTY 获取事件；可以为当前 Session 找到稳定的宿主控制边界（`chat.TClaudeCodeCliManagerNew` 单例）。
- 信息来源：WhaleTerm `backend/chat/` 全量盘点（前端/后端两个子代理报告）+ 关键文件人工复核。

## 1. 唯一 Host 对象

- **Host = `chat.TClaudeCodeCliManagerNew` 单例**（`backend/chat/claude_code_cli_new.go`，`GetClaudeCodeCliManagerNew()` :255，`Start(ctx)` :287）。
- 该单例按 agentID 维护多 Session 状态：`CreateClaudeCodeChat(aiConfig, chatKey)` (:494) 创建会话、`currentClaudeSessionID` (runtime :187-194) 与 `currentClaudeTurnID` (runtime :911-919) 给出当前 Session/Turn 映射。
- Session ID 来源：SDK `--session-id` 选项（runtime :486-491）；`SystemMessage init` 时经 `captureSessionID`（stream :544-580，:449-459）捕获回写。
- W1 首期只取一个已激活 Session（单 Session 边界，设计 §5），多 Session 同时远程管理明确排除。

## 2. W1 适配器方法映射（对应 W1 设计 §3.2 六方法）

| `remote.Controller` | WhaleTerm 现有入口 | 位置 |
| --- | --- | --- |
| `Submit` | 进入现有 Claude Turn 输入队列 `enqueueClaudeTurnChecked` / `CreateClaudeCodeChat` | lifecycle :24 / cli_new :494 |
| `Subscribe` | **需新增**（无 Go 侧事件订阅通道，见缺口 1） | — |
| `Snapshot` | **需新增**（读私有 map + JSONL 最近消息，见缺口 2） | — |
| `ResolveAction` | `RespondPermission(agentID, *PermissionDecision)` / `makePermissionCallback` | runtime :145-157 / :736-890 |
| `CancelTurn` | `CancelChatStream` / `cancelChatStream` | lifecycle :523-525 / :528-591 |
| `CancelQueued` | **需新增**（按 TurnID 移除，见缺口 3） | — |

## 3. 状态 → `protocol` 映射

- **Input**：手机文本/问题回答；WhaleTerm 现有 `buildPermissionAllowInput`（runtime :1075-1089）已合并 AskUserQuestion `answers` 键，适配器可复用同一组装逻辑。
- **Event**：WhaleTerm 内部经 Wails `runtime.EventsEmit(ctx, "chat:"+event+"-"+chatKey, msg)`（gemini.go :630-648）单向推送，事件种类 onmessage/onthinking/onerror/oncomplete/onabort/onturnstart/onqueueempty/onpermission/onsteerstatus/ontokenusage/onautonomypause；`protocol.Event` 由适配器做种类转换。
- **Snapshot**：`SessionMetadata{SessionID, ProjectPath, CreatedAt, IsStarted}`（claude_code_cli.go :14-19）+ `LoadClaudeLocalSessionDetail`（claude_local_session.go :219，JSONL 最近消息）+ `AutonomySessionStatus`（cli_new :420-441，Turn/队列/暂停态）。
- **ActionDecision / QuestionAnswer**：`PermissionRequestData` / `PermissionDecision`（cli_new :91-104）与 protocol 对应结构转换。

## 4. 长生命周期持有者

- **推荐新建 `TAutonomyService` 风格的 `TRemoteService` 单例**（backend/services/remote_service.go），在 main.go `OnStartup` 启动、`OnBeforeClose` 先于 Claude 关闭。
- 不把 Server 建在 chat manager 内：避免 chat 包反向依赖本项目公共包，保持 chat 包只做宿主能力。
- 参照 `autonomy_service.go` 单例模式（backend/services/autonomy_service.go :17-34）。**注意**：现有 OnStartup 未调用 `autonomySvc.Start`（main.go :308-342 仅调 claudeCodeCliSvc.Start），此为本盘点发现的既有缺口，不复制该缺口；W1 的 RemoteService.Start 必须在 OnStartup 显式接入。

## 5. 不应触碰的旧远程控制代码

- `TAgentsService`（backend/services/agents_service.go）只是 prompt 存储，不是 Remote Agent 运行时，**不相关，勿混淆**。
- WhaleTerm 不得 import 本项目 `internal/session`、`internal/channel`、`internal/transport`、`internal/claude`（W1 设计 §6）。
- 不复制 `protocol` DTO；线上 JSON 只由 `protocol` 契约定义。
- 不旁路 SDK Client 读写（设计 §3.2），不创建第二个 `--resume` Host。

## 6. 缺口清单（Task 3 需要补齐的宿主能力）

1. **Go 侧事件订阅/序号/重放通道缺失**：WhaleTerm 事件只经 Wails 单向推送，无 Go 内可订阅带序号且支持重放的 `protocol.Event` 流。测试缝 `turnEventEmitter`（cli_new :50，lifecycle.go :330 读取，生产从未赋值）可用作最小侵入钩子。
2. **Snapshot 访问器缺失**：SessionID↔agentID 映射、Turn/队列状态、最近消息都在 chat 包私有 map 中，需在 chat 包内新增读取方法（in-package）供适配器调用。
3. **`CancelQueued` 按请求 ID 移除缺失**：现有排队移除逻辑只在 `SteerClaudeTurn`（lifecycle :190，队列项定位/移除 :288-312）内按 chatKey+TurnID 出现一次，需抽为独立入口。
4. **actionID 并发拒绝缺失**：审批决议无 actionID 到权限请求的绑定校验，需防止过期决议重复生效。
5. **手机来源标记缺失**：现有提交入口不区分 PC/手机来源，若要满足「同一串行队列」的观测与回显需要来源标记。
6. **无直接事件 channel**：`Subscribe` 需新建聚合器，把内部事件源转为 `protocol.Event` 并处理断线重放（seq）。

## 7. 通过条件复核

- 不需要第二个 Client：**满足**，宿主就是现有 `TClaudeCodeCliManagerNew` 单例，适配器只持有对它的引用；
- 不需要 stdout/JSONL/PTY 获取事件：**满足**，事件可从包内事件源（缺口 1 补齐）转换，不新增进程/PTY；
- 存在稳定的宿主控制边界：**满足**，上述六方法都能映射到既有入口或既有逻辑的最小扩展。
