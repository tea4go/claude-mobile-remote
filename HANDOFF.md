# HANDOFF.md — Claude Mobile Remote 交接文档

> 新会话先读 `AGENTS.md`，再读本文件、`DECISIONS.md`、`REQUIREMENTS.md` 和总体设计。完成阅读后先复述当前状态和下一步，确认后再修改代码。

## 0. 一句话现状

A1 已提交；A2 SDK 托管设计已写入并提交，等待用户书面复核。

## 当前执行（AI 自动维护）

- **子阶段**：A2 SDK 托管
- **当前任务**：固化已确认的 A2 SDK 最小托管设计
- **状态**：已完成（待用户书面复核）
- **范围**：新增 A2 分阶段设计文档并记录 D21，明确薄 SDK Adapter、真实 `sessionId`、多 Turn/单 Session、错误处理与测试门禁；不修改 Go 依赖或业务代码
- **通过条件**：设计覆盖架构、数据流、错误处理、依赖策略、单元测试与真实 Claude 双轮门禁；不提前实现 A3 Session Controller 或 A4 协议
- **完成结果**：新增 `docs/specs/2026-08-21-A2-sdk-hosting-design.md`，覆盖薄 Adapter、SDK 生命周期、sessionId 来源/一致性、多 Turn、错误 scope、权限边界、依赖策略、fake 测试和真实 Claude 双轮门禁；DECISIONS.md 新增 D21
- **最近验证**：本地维护版 SDK 为干净的 `v0.1.1`，Claude Code CLI 为 2.1.179；设计文档无 TBD/TODO、Markdown 围栏成对、无行尾空白，D21/HANDOFF/设计交叉引用一致
- **未验证项**：A2 实施计划和代码尚未开始；真实 Claude 调用将在 A2 实施门禁执行
- **下一步**：用户书面复核 A2 设计；确认后调用 writing-plans 编写实施计划

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
- AI 默认不执行 Git；用户明确要求时，AI 仅在该次授权范围内执行必要的 Git 操作（D20）。

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
| A2 分阶段设计 | ✅ 待书面复核 | `docs/specs/2026-08-21-A2-sdk-hosting-design.md` |
| 业务代码 | 🚧 A1 完成 | Go CLI + 移动 Web 骨架；A2 SDK 托管待计划 |

## 4. 下一步

1. 用户书面复核 A2 SDK 最小托管设计；
2. 设计复核通过后编写 A2 实施计划并提交用户确认；
3. A2 验证 SDK 最小托管与真实 `sessionId` 获取（含设计假设核对：init 事件 session_id、Turn/Session 语义）；
4. A2 门禁通过后，再按 A3 → A6 顺序逐阶段验证单会话闭环；
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
