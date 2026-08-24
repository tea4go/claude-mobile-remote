# HANDOFF.md — Claude Mobile Remote 交接文档

> 新会话先读 `AGENTS.md`，再读本文件、`DECISIONS.md`、`REQUIREMENTS.md` 和总体设计。完成阅读后先复述当前状态和下一步，再按“当前执行”继续；涉及尚未确认的设计、计划或产品决策时先等用户确认。

## 0. 一句话现状

A1–A2 已完成并提交（A2 提交 `2ee56de`）且人工验收通过；A3 Session Core 设计与实施计划已确认，用户已选择 Subagent-Driven 执行方式，先提交设计包再开始实施。

## 当前执行（AI 自动维护）

- **子阶段**：A3 Session Core
- **当前任务**：起草 A3 Session Core 实施计划
- **状态**：已完成（用户已选择 Subagent-Driven，先提交设计包）
- **范围**：确认 A3 设计状态，新增 `docs/plans/2026-08-24-A3-session-core.md` 并更新本文件；不修改代码、依赖或配置
- **通过条件**：计划按 TDD 拆成可独立执行的小步骤，给出精确文件、接口、测试、命令和预期结果，覆盖 A3 设计全部范围，并通过占位符、类型一致性和文档差异检查
- **完成结果**：用户已确认 A3 书面设计，设计状态更新为“已确认，待实施”；新增 `docs/plans/2026-08-24-A3-session-core.md`，按 7 个 Task 拆分领域模型、Host Worker、Actor/FIFO、事件与错误、订阅与停止、CLI 迁移和全量门禁，每个 Task 均包含 RED/GREEN 命令及需重新授权的 Git 检查点；未修改代码、依赖或配置
- **最近验证**：实施计划已按设计逐项核对 Actor/Worker、32 容量、request ID、状态/snapshot、事件、错误 scope、停止、CLI、门禁和三端兼容；占位符与类型别名扫描无遗留；tracked `git diff --check` 无输出且退出 0，新增计划的 no-index check 无空白诊断；本任务未运行代码测试或构建
- **未验证项**：A3 代码尚未开始，计划中的 RED/GREEN 与门禁命令均待实施时执行
- **下一步**：提交 A3 设计、计划、D24 与本交接记录；随后读取 Subagent-Driven、TDD 和 worktree 技能，更新本文件当前执行并开始 Task 1

## 1. 当前目标

首期完成“简单 CLI + SDK 托管 Claude + 单 `sessionId` + VPN WebSocket + 移动浏览器”的最小双向通信闭环。

```text
终端 ──> CLI Service ──> claude-agent-sdk-go ──> Claude
                 └────VPN WebSocket──────> 手机浏览器
```

## 2. 已确认事项

- 项目仓库：`/Users/zhangym/code/work/utils/claude-mobile-remote`；
- 与 WhaleTerm 无关，旧方案、阶段和记忆不继承；
- 使用 `github.com/tea4go/claude-agent-sdk-go` 托管 Claude；
- CLI 使用简单行输入输出，不复刻 Claude Code 完整终端；
- 扫码绑定真实 `sessionId`；
- 手机认证后显式订阅对应 Session Channel，Channel 只负责消息路由；
- Claude 的完整上下文、会话状态和所有操作执行都留在 PC，手机只展示数据并回传操作；
- 手机不展示扫码前完整历史，只展示当前状态和后续消息；
- Claude 的一轮输出完成只结束 Turn，不结束 Session；Session 可继续接收下一条消息；
- 里程碑 A 的手机端只收发文本消息，审批、回答和取消在 B2 实现；
- 手机默认连接 VPN 并可访问 PC，首期浏览器调试优先；
- iOS / Android 使用 Capacitor，HarmonyOS NEXT 独立开发；
- 心跳、完整重连、推送、公网中转、多会话和多机器后置；
- A1 只创建 Go CLI、React/Vite 与基础工程配置；Capacitor 工程在 C1 创建，HarmonyOS 工程在 D1 创建；
- 开发按里程碑和子阶段门禁推进，当前首期为 A1–A6，A6 为 VPN 手机联调；
- AI 可主动使用只读 `git diff` 检查改动；Git 写操作与其他未明确允许的 Git 命令仍需用户授权（D23）。

## 3. 文档状态

| 文件 | 状态 | 作用 |
| --- | :---: | --- |
| `REQUIREMENTS.md` | ✅ | 产品需求、阶段范围和验收标准 |
| `AGENTS.md` | ✅ | 长期约束和协作规则 |
| `CLAUDE.md` | ✅ | Claude Code 阅读入口 |
| `DECISIONS.md` | ✅ | 新项目决策日志 |
| 总体设计 | ✅ 已确认 | 架构、Channel、协议和测试边界 |
| A1 实施计划 | ✅ 已执行 | `docs/plans/2026-08-21-A1-project-skeleton.md` |
| A1 门禁记录 | ✅ 通过 | `docs/testing/A1-result.md` |
| A2 分阶段设计 | ✅ 已确认 | `docs/specs/2026-08-21-A2-sdk-hosting-design.md` |
| A2 实施计划 | ✅ 已执行 | `docs/plans/2026-08-21-A2-sdk-hosting.md` |
| A2 门禁记录 | ✅ 通过 | `docs/testing/A2-result.md` |
| A3 分阶段设计 | ✅ 已确认 | `docs/specs/2026-08-24-A3-session-core-design.md` |
| A3 实施计划 | ✅ 待执行 | `docs/plans/2026-08-24-A3-session-core.md` |
| 业务代码 | ✅ A2 完成 | 薄 SDK Adapter + 同步行式 CLI；A3 Session Core 待实施 |

## 4. 下一步

1. 按用户已选择的 Subagent-Driven 方式执行 A3 实施计划；
2. 按 TDD 实现 Session/Turn 状态机、输入队列和 snapshot；
3. 按 A3 → A6 顺序逐阶段验证单会话闭环；
4. 每项开发任务由 AI 在开始和结束时更新本文件；普通任务不单独创建测试记录。

## 5. 首期关键验证

- SDK 能否在独立最小程序中稳定提供 session、流式消息、错误和取消事件；
- 真实 `sessionId` 何时可用，以及二维码生成时机；
- Turn 完成后 Session 能否继续接收终端或手机的下一条消息；
- 终端与手机输入如何在单一 Session Controller 中串行；
- 界面同步与断线恢复所需的 snapshot 最小字段；
- 二维码中的 PC Service 地址能否被 VPN 内手机稳定访问。

## 6. 已知但不阻塞 A1

- 本机已具备 Go、Node.js 和 Xcode；
- Android SDK / Android Studio 与 DevEco Studio 尚未安装，分别在进入 C1、D1 前准备；
- antd-mobile 5.42.3 的部分传递依赖与 React 19 peer 范围不一致；用户已评估为当前可忽略，A1 测试、构建和 lint 均通过。

## 7. 参考提醒

WhaleTerm 只用于查阅 SDK 已验证用法，不读取或同步其 `AGENTS.md`、`HANDOFF.md`、`DECISIONS.md` 和远程控制设计。
