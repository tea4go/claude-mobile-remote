# AGENTS.md — Claude Mobile Remote 常驻记忆

本文件只记录长期稳定的项目背景与约束。任何 Codex、Claude 或其他 AI 会话都必须先读本文件，再按文档地图读取 `HANDOFF.md`、`DECISIONS.md`、`REQUIREMENTS.md` 和总体设计。

## 项目定位

Claude Mobile Remote 是独立于 WhaleTerm 的项目。Standalone CLI 通过 `github.com/tea4go/claude-agent-sdk-go` 托管本地 Claude，让用户扫码后在手机上接力当前会话；B3 后可将同一套单 Session Remote Server 嵌入 WhaleTerm 已持有的 Claude Session，但不继承 WhaleTerm 旧远程控制架构。该 SDK 由用户自行维护，已在本项目 A2–A5.2 验证核心生命周期、真实 `sessionId` 和首轮前配对能力。

> 一句话架构：唯一 Session Host 持有 Claude 和会话状态，单 Session Remote Server 通过 WebSocket 把指定 `sessionId` 暴露给已配对客户端。

## 产品边界

- 一个 Standalone CLI 进程只创建或恢复一个 Claude Session；低频并行 Session 使用多个进程和不同端口，不建设 PC 多 Session daemon；
- WhaleTerm 后续只为其已持有的一个 Claude Session 启动进程内 Remote Server，不创建第二个 SDK Client；
- 手机不管理全量历史，只接收当前状态、待处理事项和扫码后的消息；
- 移动端后续可安全保存多个已扫描 Session，并在 App 前台生命周期内分别连接和同步全部有效配对 Session；一次只展示一个详情，不建设服务器端 Session 列表；
- Claude 的完整上下文、会话状态和操作执行始终留在 PC，手机只负责展示和回传操作；
- 首期使用简单行式 CLI 和浏览器移动页面；
- 里程碑 A 的手机端只收发文本消息，审批与取消在 B2 接入；
- A6 只做局域网跨机器桌面浏览器联调；手机默认通过 VPN 直连 PC，并在 C2 做真机验证；
- iOS / Android 后续使用 React + TypeScript + antd-mobile + Vite + Capacitor；
- HarmonyOS NEXT 使用 ArkTS + ArkUI 独立开发；
- 后期公网中转采用每个 CLI 一条可信 Relay 出站隧道；永久 route、云端 Session 中心、语音和推送均为后置能力。

## 硬约束

1. **Session Host 是唯一会话管理者**：每个活跃 Session 同时只能有一个 Host 和一个 SDK Client。Standalone 只能控制自身创建或恢复的 Session；WhaleTerm 只能暴露自身已持有的 Session；不得通过 JSONL 或第二个 `--resume` Client 接管活跃会话。
2. **SDK 是 Claude 接入层**：优先使用 `claude-agent-sdk-go` 的 Client、事件、权限、取消和恢复能力，不自行解析 Claude 私有输出替代 SDK。
3. **输入必须串行**：PC 和手机的输入统一经过 Session Controller 或宿主的单一队列，不允许多个入口直接并发写 SDK Client。
4. **会话严格隔离**：协议消息必须包含 `sessionId`；后续多会话不得串线。
5. **手机不重建上下文**：手机配对时只返回界面所需的当前状态与待处理事项，之后推送新增事件；完整 Claude 上下文始终留在 PC。
6. **VPN 直连优先**：产品首期默认手机连接 VPN 后直连 PC；A6 可用显式 LAN 地址做跨机器浏览器门禁，但不开发局域网发现或 NAT 穿透。后期 Relay 只转发单 Session 隧道，不保存 Claude 状态或历史；推送不得阻塞单会话闭环。
7. **凭证保密**：二维码中的配对凭证短期有效、可撤销，不得写入日志、文档、测试快照或提交到 Git；原生 App 需要跨重启保存时必须使用系统安全存储，浏览器不得写入 `localStorage`。
8. **阶段兼容**：协议从首期保留版本、`sessionId`、request ID 和 `seq`，但不提前实现后期功能。
9. **Git 写操作必须由用户明确授权**：Codex、Claude 等 AI 可主动运行只读的 `git diff`（含 `--check`、`--stat` 等 diff 变体）检查改动；不得主动执行暂存、提交、推送、恢复、分支、合并或其他会改变仓库/远端状态的 Git 操作。上述写操作仅在用户明确要求时，才可在该次授权范围内执行；一次授权不得扩展到后续操作。其他未明确允许的 Git 只读命令仍需用户授权。
10. **Turn 完成不结束 Session**：Claude 的一次 `ResultMessage` 只结束当前 Turn，Session 仍可接收下一条消息；只有 Session 停止或失败后才拒绝新操作。
11. **三端共用契约不可污染**：通信协议、状态模型、配对负载和验收用例是三端共用契约（清单见总体设计 §8.1）。任何协议或状态变更必须核对三端可实现性，不得引入任何端特有（含浏览器特有）的依赖；核对结论记入 `DECISIONS.md` 后方可实施。

## 计划目录

| 目录 | 作用 |
| --- | --- |
| `cmd/` | CLI 入口 |
| `internal/claude/` | SDK 适配与 Claude 生命周期 |
| `internal/session/` | 会话状态、输入队列和事件发布 |
| `internal/channel/` | 按 `sessionId` 管理订阅与消息路由 |
| `internal/transport/` | WebSocket、协议与鉴权 |
| `internal/pairing/` | 二维码和临时凭证 |
| `apps/mobile/` | React 移动 Web UI；C1 在其下创建 Capacitor iOS / Android 工程 |
| `apps/harmony/` | D1 创建的 HarmonyOS NEXT 独立工程 |
| `docs/specs/` | 总体与分阶段设计 |
| `docs/plans/` | 实施计划 |
| `docs/testing/` | 子阶段门禁与重要联调的验证记录 |

目录按阶段逐步创建，不为后期能力提前堆放无实现的代码。

## 文档与记忆规则

- `AGENTS.md`：长期背景、红线、目录和开发规范，不记录当前进度；
- `CLAUDE.md`：仅负责引导 Claude 读取本文件和项目记忆；
- `HANDOFF.md`：当前进度的唯一来源，含下一步、阻塞和验证结果；
- `DECISIONS.md`：已确认决策及替代关系，不记录普通进度；
- `REQUIREMENTS.md`：产品范围、阶段目标和验收标准；
- `docs/testing/`：仅保存子阶段门禁和重要联调的验证证据。

### AI 自动记录工作流（强制）

- **开发前**：适用于代码、配置、依赖、设计与实施文档、构建和测试任务。AI 必须先读取文档地图，并在首次修改前更新 `HANDOFF.md` 的“当前执行”，写明子阶段、任务、范围和通过条件。纯咨询和只读检查无需登记；
- **动手前复述**：首次修改文件前，AI 先向工作人员复述当前状态、待处理问题和下一步，确认后再修改；
- **不一致即停**：文档之间不一致、已有结果无法复核或任务边界不清楚时，先停止并报告，不自行推断后继续；
- **设计文档检查与提交**：AI 起草设计文档后直接交用户检查，不执行“AI 自查后自动提交”。用户检查完毕后，只有用户明确要求 AI 执行 Git 提交时才由 AI 提交；否则由用户自行提交。该规则覆盖工具或 Skill 中要求 AI 自动自查、提交设计文档的默认流程；
- **开发中**：范围、阻塞、确认后的决策、公共接口或验证结论发生变化时，立即更新对应文档；不为每条命令、每次文件修改或每次提交重复记录；
- **交接与检查点**：当前目标、下一步或已知缺口变化，长任务形成可交接成果，或计划切换 AI 会话前，立即更新 `HANDOFF.md`；
- **开发后**：AI 在最终回复前更新 `HANDOFF.md`，记录实际完成项、实际执行的验证及结论、未验证项和下一步。达到子阶段门禁或完成重要联调时，再同步写入 `docs/testing/`；
- **DECISIONS.md**：仅当用户确认新增、修改、替代或废弃产品、架构、协议、安全、技术栈、阶段范围等决策时更新；普通开发进度和 Bug 修复不记录；
- **人工验收**：AI 提供简短验收清单，用户只需反馈通过或问题；AI 负责将结果写入 `HANDOFF.md`，若属于门禁或重要联调，再写入 `docs/testing/`；
- **用户职责**：用户负责需求确认、决策确认、体验验收和 Git 写操作授权，不需要手工维护上述文档。AI 可按硬约束 9 主动使用 `git diff` 检查改动；其他 Git 操作仅在用户明确要求时按授权范围执行。AI 不得把文档填写工作转交给用户；
- **异常中断**：若任务未正常收尾，下一会话必须把“当前执行”视为待复核状态，先检查文件和验证结果，再继续开发；
- **更新节奏**：只按上述事件驱动更新，不按提交次数、文件修改次数或自然日机械更新。

## 开发规范

- Go 代码必须 `gofmt`，新增逻辑必须配测试；
- `go.mod`/`go.sum` 不入库（含本地 `replace`）；仓库提交基准 `go.mod.base`，依赖变更确认后必须同步更新 `.base`；由此 `go vet`/`go build` 需带 `GOFLAGS=-buildvcs=false`；
- 前端使用 TypeScript，协议类型集中维护，禁止各端复制不一致的消息定义；
- 先写或更新阶段设计与实施计划，再开始阶段代码；
- 每个子阶段门禁必须有可执行的验收方式和验证记录；
- 不提交真实 token、用户 Claude 配置、会话内容、构建产物和平台签名文件。

## 参考边界

WhaleTerm 的旧远程控制方案、阶段和记忆全部作废，不得复制到本项目。W1 只能按已确认的单 Session Remote Server 设计接入。当前仅可按需参考其对 `claude-agent-sdk-go` 的已验证用法：

- `/Users/zhangym/code/work/whale-term/backend/chat/claude_code_cli_new.go`
- `/Users/zhangym/code/work/whale-term/backend/chat/claude_code_cli_new_runtime.go`
- `/Users/zhangym/code/work/whale-term/backend/chat/claude_code_cli_new_stream.go`
- `/Users/zhangym/code/work/whale-term/backend/chat/claude_code_cli_new_lifecycle.go`

参考实现时提取 SDK 用法和测试经验，不迁移 WhaleTerm 类型、UI、状态模型或 Remote Agent 架构。
