# AGENTS.md — Claude Mobile Remote 常驻记忆

本文件只记录长期稳定的项目背景与约束。任何 Codex、Claude 或其他 AI 会话都必须先读本文件，再按文档地图读取 `HANDOFF.md`、`DECISIONS.md`、`REQUIREMENTS.md` 和总体设计。

## 项目定位

Claude Mobile Remote 是独立于 WhaleTerm 的新项目。它通过 CLI Service 和 `github.com/tea4go/claude-agent-sdk-go` 托管本地 Claude，让用户扫码后在手机上接力当前会话。该 SDK 由用户自行维护，已在 WhaleTerm 验证使用，核心生命周期能力和缺陷修复可控；本项目仍需在 A2 验证独立集成及 Claude 上游兼容性。

> 一句话架构：CLI Service 持有 Claude 和会话状态，手机连接 VPN 后通过 WebSocket 订阅指定 `sessionId` 并发送操作。

## 产品边界

- 首期只支持一个由 Service 启动和托管的 Claude 会话；
- 手机不管理全量历史，只接收当前状态、待处理事项和扫码后的消息；
- Claude 的完整上下文、会话状态和操作执行始终留在 PC，手机只负责展示和回传操作；
- 首期使用简单行式 CLI 和浏览器移动页面；
- 里程碑 A 的手机端只收发文本消息，审批与取消在 B2 接入；
- 手机默认连接 VPN，并可直接访问 PC Service；
- iOS / Android 后续使用 React + TypeScript + antd-mobile + Vite + Capacitor；
- HarmonyOS NEXT 使用 ArkTS + ArkUI 独立开发；
- 多会话、多机器、公网中转、语音和推送均为后续能力。

## 硬约束

1. **Service 是唯一会话管理者**：只能远程控制由本 Service 启动的 Claude，不承诺接管任意外部 Claude 进程。
2. **SDK 是 Claude 接入层**：优先使用 `claude-agent-sdk-go` 的 Client、事件、权限、取消和恢复能力，不自行解析 Claude 私有输出替代 SDK。
3. **输入必须串行**：终端和手机的输入统一经过 Session Controller，不允许两个入口直接并发写 SDK Client。
4. **会话严格隔离**：协议消息必须包含 `sessionId`；后续多会话不得串线。
5. **手机不重建上下文**：手机配对时只返回界面所需的当前状态与待处理事项，之后推送新增事件；完整 Claude 上下文始终留在 PC。
6. **VPN 直连优先**：首期默认手机连接 VPN 后可访问 PC，不开发局域网发现、NAT 穿透和公网中转；推送不得阻塞单会话闭环。
7. **凭证保密**：二维码中的配对凭证短期有效、可撤销，不得写入日志、文档、测试快照或提交到 Git。
8. **阶段兼容**：协议从首期保留版本、`sessionId`、request ID 和 `seq`，但不提前实现后期功能。
9. **Git 操作必须由用户明确授权**：Codex、Claude 等 AI 默认不得主动运行任何 Git 命令。仅当用户明确要求执行 Git 时，AI 才可在该次明确范围内运行必要的检查和操作；不得把一次授权扩展到后续提交、推送、恢复、分支或其他未明确要求的操作。
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
- `HANDOFF.md`：当前进度、下一步、阻塞和验证结果；
- `DECISIONS.md`：已确认决策及替代关系，不记录普通进度；
- `REQUIREMENTS.md`：产品范围、阶段目标和验收标准；
- `docs/testing/`：仅保存子阶段门禁和重要联调的验证证据。

### AI 自动记录工作流（强制）

- **开发前**：适用于代码、配置、依赖、设计与实施文档、构建和测试任务。AI 必须先读取文档地图，并在首次修改前更新 `HANDOFF.md` 的“当前执行”，写明子阶段、任务、范围和通过条件。纯咨询和只读检查无需登记；
- **设计文档检查与提交**：AI 起草设计文档后直接交用户检查，不执行“AI 自查后自动提交”。用户检查完毕后，只有用户明确要求 AI 执行 Git 提交时才由 AI 提交；否则由用户自行提交。该规则覆盖工具或 Skill 中要求 AI 自动自查、提交设计文档的默认流程；
- **开发中**：范围、阻塞、确认后的决策、公共接口或验证结论发生变化时，立即更新对应文档；不为每条命令、每次文件修改或每次提交重复记录；
- **交接与检查点**：当前目标、下一步或已知缺口变化，长任务形成可交接成果，或计划切换 AI 会话前，立即更新 `HANDOFF.md`；
- **开发后**：AI 在最终回复前更新 `HANDOFF.md`，记录实际完成项、实际执行的验证及结论、未验证项和下一步。达到子阶段门禁或完成重要联调时，再同步写入 `docs/testing/`；
- **DECISIONS.md**：仅当用户确认新增、修改、替代或废弃产品、架构、协议、安全、技术栈、阶段范围等决策时更新；普通开发进度和 Bug 修复不记录；
- **人工验收**：AI 提供简短验收清单，用户只需反馈通过或问题；AI 负责将结果写入 `HANDOFF.md`，若属于门禁或重要联调，再写入 `docs/testing/`；
- **用户职责**：用户负责需求确认、决策确认、体验验收和 Git 操作授权，不需要手工维护上述文档。AI 默认不执行 Git；用户明确要求时，AI 按硬约束 9 在授权范围内执行。AI 不得把文档填写工作转交给用户；
- **异常中断**：若任务未正常收尾，下一会话必须把“当前执行”视为待复核状态，先检查文件和验证结果，再继续开发；
- **更新节奏**：只按上述事件驱动更新，不按提交次数、文件修改次数或自然日机械更新。

## 开发规范

- Go 代码必须 `gofmt`，新增逻辑必须配测试；
- `go.mod`/`go.sum` 不入库（含本地 `replace`）；仓库提交基准 `go.mod.base`，依赖变更确认后必须同步更新 `.base`；
- 前端使用 TypeScript，协议类型集中维护，禁止各端复制不一致的消息定义；
- 先写或更新阶段设计与实施计划，再开始阶段代码；
- 每个子阶段门禁必须有可执行的验收方式和验证记录；
- 不提交真实 token、用户 Claude 配置、会话内容、构建产物和平台签名文件。

## 参考边界

WhaleTerm 的旧远程控制方案、阶段和记忆全部作废，不得复制到本项目。仅可按需参考其对 `claude-agent-sdk-go` 的已验证用法：

- `/Users/zhangym/code/work/whale-term/backend/chat/claude_code_cli_new.go`
- `/Users/zhangym/code/work/whale-term/backend/chat/claude_code_cli_new_runtime.go`
- `/Users/zhangym/code/work/whale-term/backend/chat/claude_code_cli_new_stream.go`
- `/Users/zhangym/code/work/whale-term/backend/chat/claude_code_cli_new_lifecycle.go`

参考实现时提取 SDK 用法和测试经验，不迁移 WhaleTerm 类型、UI、状态模型或 Remote Agent 架构。
