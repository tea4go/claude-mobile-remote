# HANDOFF.md — Claude Mobile Remote 交接文档

> 新会话先读 `AGENTS.md`，再读本文件、`DECISIONS.md`、`REQUIREMENTS.md` 和总体设计。完成阅读后先复述当前状态和下一步，确认后再修改代码。

## 0. 一句话现状

需求与总体设计已经用户确认；业务代码尚未开始，下一步编写 A1 项目骨架实施计划。

## 当前执行（AI 自动维护）

- **子阶段**：规划准备
- **当前任务**：补充 README 接手与开始指引
- **状态**：已完成
- **范围**：更新 `README.md` 和 `HANDOFF.md`，不修改业务代码
- **通过条件**：工作人员与 AI 均能从 README 找到接手流程；当前进度只以 `HANDOFF.md` 为准；README 不复制动态进度或详细协作规则
- **完成结果**：README 已增加统一接手流程，删除容易过期的动态“当前状态”，并明确文档读取、状态复述、计划门禁、异常停止和人员/AI 职责
- **最近验证**：上手流程关键内容、文档引用、README/AGENTS/HANDOFF 规则一致性、Markdown 围栏及行尾空白检查通过
- **未验证项**：无业务运行验证，本次不属于子阶段门禁或重要联调
- **下一步**：编写 A1 项目骨架实施计划并提交用户确认

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
- AI 不执行任何 Git 命令，文件检查与提交由用户完成。

## 3. 文档状态

| 文件 | 状态 | 作用 |
| --- | :---: | --- |
| `REQUIREMENTS.md` | ✅ | 产品需求、阶段范围和验收标准 |
| `AGENTS.md` | ✅ | 长期约束和协作规则 |
| `CLAUDE.md` | ✅ | Claude Code 阅读入口 |
| `DECISIONS.md` | ✅ | 新项目决策日志 |
| 总体设计 | ✅ 已确认 | 架构、Channel、协议和测试边界 |
| A1 实施计划 | ⬜ | 下一步编写并提交用户确认 |
| 业务代码 | ⬜ | 实施计划确认后开始 |

## 4. 下一步

1. 只编写 A1 项目骨架的实施计划与门禁；
2. 用户确认 A1 计划后再创建 Go CLI 与 React/Vite 骨架；
3. A1 门禁通过后更新 `docs/testing/A1-result.md` 和本文件，再编写 A2 SDK 托管计划；
4. 按 A1 → A6 顺序逐阶段验证单会话闭环；
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
- 当前 `.gitignore` 主要覆盖 Go，A1 计划需要补充 React/Vite 及后续原生工程的忽略规则。

## 7. 参考提醒

WhaleTerm 只用于查阅 SDK 已验证用法，不读取或同步其 `AGENTS.md`、`HANDOFF.md`、`DECISIONS.md` 和远程控制设计。
