# AGENTS.md — Claude Mobile Remote 常驻记忆

本文件只记录长期稳定的项目背景与约束。任何 Codex、Claude 或其他 AI 会话都必须先读本文件，再按文档地图读取 `HANDOFF.md`、`DECISIONS.md`、`REQUIREMENTS.md` 和总体设计。

## 项目定位

Claude Mobile Remote 是独立于 WhaleTerm 的新项目。它通过 CLI Service 和 `github.com/tea4go/claude-agent-sdk-go` 托管本地 Claude，让用户扫码后在手机上接力当前会话。

> 一句话架构：CLI Service 持有 Claude 和会话状态，手机通过局域网 WebSocket 订阅指定 `sessionId` 并发送操作。

## 产品边界

- 首期只支持一个由 Service 启动和托管的 Claude 会话；
- 手机不管理全量历史，只接收当前状态、待处理事项和扫码后的消息；
- 首期使用简单行式 CLI 和浏览器移动页面；
- iOS / Android 后续使用 React + TypeScript + antd-mobile + Vite + Capacitor；
- HarmonyOS NEXT 使用 ArkTS + ArkUI 独立开发；
- 多会话、多机器、公网中转、语音和推送均为后续能力。

## 硬约束

1. **Service 是唯一会话管理者**：只能远程控制由本 Service 启动的 Claude，不承诺接管任意外部 Claude 进程。
2. **SDK 是 Claude 接入层**：优先使用 `claude-agent-sdk-go` 的 Client、事件、权限、取消和恢复能力，不自行解析 Claude 私有输出替代 SDK。
3. **输入必须串行**：终端和手机的输入统一经过 Session Controller，不允许两个入口直接并发写 SDK Client。
4. **会话严格隔离**：协议消息必须包含 `sessionId`；后续多会话不得串线。
5. **不返回完整历史**：手机配对时只返回当前状态与待处理事项，之后推送新增事件。
6. **本地优先**：首期局域网直连；公网中转和推送不得阻塞单会话闭环。
7. **凭证保密**：二维码中的配对凭证短期有效、可撤销，不得写入日志、文档、测试快照或提交到 Git。
8. **阶段兼容**：协议从首期保留版本、`sessionId`、request ID 和 `seq`，但不提前实现后期功能。

## 计划目录

| 目录 | 作用 |
| --- | --- |
| `cmd/` | CLI 入口 |
| `internal/claude/` | SDK 适配与 Claude 生命周期 |
| `internal/session/` | 会话状态、输入队列和事件发布 |
| `internal/transport/` | WebSocket、协议与鉴权 |
| `internal/pairing/` | 二维码和临时凭证 |
| `apps/mobile-web/` | React 移动 Web UI |
| `apps/ios/`、`apps/android/` | Capacitor 原生工程 |
| `apps/harmony/` | HarmonyOS NEXT 独立工程 |
| `docs/specs/` | 总体与分阶段设计 |
| `docs/plans/` | 实施计划 |
| `docs/testing/` | 测试方案与验证记录 |

目录按阶段逐步创建，不为后期能力提前堆放无实现的代码。

## 文档与记忆规则

- `AGENTS.md`：长期背景、红线、目录和开发规范，不记录当前进度；
- `CLAUDE.md`：仅负责引导 Claude 读取本文件和项目记忆；
- `HANDOFF.md`：当前进度、下一步、阻塞和验证结果；
- `DECISIONS.md`：已确认决策及替代关系，不记录普通进度；
- `REQUIREMENTS.md`：产品范围、阶段目标和验收标准；
- 每个阶段或重要检查点结束后更新 `HANDOFF.md`；仅在决策发生变化时更新 `DECISIONS.md`。

## 开发规范

- Go 代码必须 `gofmt`，新增逻辑必须配测试；
- 前端使用 TypeScript，协议类型集中维护，禁止各端复制不一致的消息定义；
- 先写或更新阶段设计与实施计划，再开始阶段代码；
- 每个阶段必须有可执行的验收用例和测试记录；
- 不提交真实 token、用户 Claude 配置、会话内容、构建产物和平台签名文件。

## 参考边界

WhaleTerm 的旧远程控制方案、阶段和记忆全部作废，不得复制到本项目。仅可按需参考其对 `claude-agent-sdk-go` 的已验证用法：

- `/Users/zhangym/code/work/whale-term/backend/chat/claude_code_cli_new.go`
- `/Users/zhangym/code/work/whale-term/backend/chat/claude_code_cli_new_runtime.go`
- `/Users/zhangym/code/work/whale-term/backend/chat/claude_code_cli_new_stream.go`
- `/Users/zhangym/code/work/whale-term/backend/chat/claude_code_cli_new_lifecycle.go`

参考实现时提取 SDK 用法和测试经验，不迁移 WhaleTerm 类型、UI、状态模型或 Remote Agent 架构。
