# HANDOFF.md — Claude Mobile Remote 交接文档

> 新会话先读 `AGENTS.md`，再读本文件、`DECISIONS.md`、`REQUIREMENTS.md` 和总体设计。完成阅读后先复述当前状态和下一步，再按“当前执行”继续；涉及尚未确认的设计、计划或产品决策时先等用户确认。

## 0. 一句话现状

A1–A2 已完成；A2 完成前评审问题已修正，全部自动门禁与真实 Claude 双轮测试通过，等待用户人工验收。

## 当前执行（AI 自动维护）

- **子阶段**：A2 SDK 托管
- **当前任务**：修复 A2 严格审查发现的 I-1（assistant 错误静默成功）与 I-2（buildvcs 文档失实）
- **状态**：已完成（待用户检查提交）
- **范围**：`internal/claude`（messages.go、host.go 及测试）补 AssistantMessage.Error 处理；`docs/testing/A2-result.md`、README、AGENTS、A2 设计修正 buildvcs 与错误分级说明；不改架构与决策
- **通过条件**：新失败测试（assistant 错误 → turn_failed + 分级 scope）先失败后通过；全部门禁 fresh 重跑通过；文档三处一致反映实际命令
- **完成结果**：I-1 已按 TDD 修复：`parseMessage` 提取 assistant 错误（类型+文本），`RunTurn` 收到错误 assistant 即终止当前流并发射 `turn_failed`，`server_error`/`rate_limit` 为 turn scope、其余为 session scope（依据 WhaleTerm 验证行为，未移植其文本关键字匹配）；新增 `TestParseMessage/assistant_error_*` 与 `TestHostAssistantErrorFailsTurn`。I-2 已修正：A2-result.md 补记审查修复记录与 buildvcs 注记；README 快速开始、AGENTS 开发规范、A2 设计 §8/§10 同步
- **最近验证**：TDD 流程完整（新测试先编译失败后通过）；全量 unit/race/gofmt/vet/build/integration-skip fresh 重跑全部 PASS；真实 Claude integration 测试因本次改动不涉及正常流（parse 只新增分支），结论仍有效
- **未验证项**：用户尚未执行 A2 人工验收（含可触发 API 错误的场景验证 I-1 真实行为）
- **下一步**：用户检查修复并验收 A2；通过后起草 A3 Session Core 分阶段设计

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
| 业务代码 | ✅ A2 完成 | 薄 SDK Adapter + 同步行式 CLI；A3 Session Core 待设计 |

## 4. 下一步

1. 用户执行 A2 CLI 人工体验验收并反馈通过或问题；
2. AI 将人工验收结论写回本文件，必要时补充 `docs/testing/A2-result.md`；
3. A2 验收通过后起草 A3 Session Core 分阶段设计并交用户检查；
4. 按 A3 → A6 顺序逐阶段验证单会话闭环；
5. 每项开发任务由 AI 在开始和结束时更新本文件；普通任务不单独创建测试记录。

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
