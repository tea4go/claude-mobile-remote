# HANDOFF.md — Claude Mobile Remote 交接文档

> 新会话先读 `AGENTS.md`，再读本文件、`DECISIONS.md`、`REQUIREMENTS.md` 和总体设计。完成阅读后先复述当前状态和下一步，再按“当前执行”继续；涉及尚未确认的设计、计划或产品决策时先等用户确认。

## 0. 一句话现状

A1–A2 已完成并提交（A2 提交 `2ee56de`）且人工验收通过；A3 Session Core 实现、自动门禁与最终代码审查已通过，正在当前 `main` 工作区等待用户统一 diff 检查和 Git 授权。

## 当前执行（AI 自动维护）

- **子阶段**：A3 Session Core
- **当前任务**：Task 7——A3 全量门禁、记录与交接
- **状态**：已完成；自动门禁与最终整体审查通过，停在用户统一检查和最终 Git 授权点
- **范围**：补两请求 EOF 下首 Turn 导致 Session scope failure 的真实 Controller 回归，确保只报告一次 Session 根因并正确结束已输出文本行；补齐 1 MiB LF/CRLF/无换行/+1 测试矩阵；随后执行 Task 7 focused/full race、gofmt、vet、build、重复并发、三端契约复核、门禁记录和最终整体审查，不改变 A4 协议、三端字段、状态枚举或依赖
- **通过条件**：新增边界严格经历 RED/GREEN；Task 7 §7.1–§7.3 所有命令 fresh PASS；D24/D25/D26 复核通过；完成前审查无未关闭 Critical/Important；`docs/testing/A3-result.md` 与本文件记录真实证据
- **完成结果**：A3 Session Core、唯一 Host Worker、Actor/FIFO、领域事件/snapshot、原子订阅/Stop 与异步 CLI 均已实现。Task 7 按 TDD 补齐 EOF/非 EOF 终态竞态下 Session failure 的单根因诊断、部分文本换行和 1 MiB LF/CRLF/无换行/+1 边界；新增 `docs/testing/A3-result.md` 记录门禁与 D24/D25/D26 复核结论；最终审查无 Critical、Important 或 Minor
- **最近验证**：最终审查指出 EOF 观察调度仍可导致双错误；新增无 sleep、Reader 保持打开的回归后旧实现稳定 RED，改为按 Worker settlement/下一 Turn started 延迟诊断后相关 focused race `-count=20` PASS。取消测试同步修正后 race `-count=100` PASS；随后 fresh focused 两包 race、全仓四包 race、gofmt、vet、build 全 PASS，Controller 重点并发 race `-count=50` PASS（24.356s）。原审查代理复核确认 Critical/Important/Minor 均为零；tracked 与全部新增 A3 文件的 `git diff --check`/no-index check 无输出，差异范围无依赖、移动端、A4 或凭证文件，`-version` 精确输出 `0.1.0-dev`
- **未验证项**：用户统一 diff 检查与 A3 最终 Git 写操作授权；未重复运行真实 Claude integration（A2 已完成真实双轮门禁，A3 按计划使用 controlled Host）
- **下一步**：向用户提供 A3 变更摘要和检查清单；用户确认后，仅在该次明确授权范围内执行最终 `git add` / `git commit`，随后开始 A4 设计
- **复审记录（2026-08-24，独立 Claude 会话，已被最终审查修复替代）**：只读门禁均 PASS，并确认 A2 仅复用 WhaleTerm SDK 用法、A3 保持 Controller/唯一 Worker 分层；其记录的 Session failure 部分文本换行 Minor 已在 Task 7 最终审查修复中关闭。
- **最终审查记录（2026-08-24）**：首次整体审查发现 EOF 观察调度可能造成同一 Session failure 的 Turn+Session 双诊断；按 TDD 改为基于 Worker settlement/下一 `turn_started` 延迟 Turn 诊断，并补直接 Session failure 换行。原审查代理复核后确认 Critical/Important/Minor 均为零，Ready for user diff review / final Git authorization。
- **独立走查记录（2026-08-24，第二个独立会话）**：对 `docs/testing/A3-result.md` 及最终工作区做只读走查。全部声称证据 fresh 复现通过：focused 两包 race、全仓 race `-count=1`、gofmt/vet/build、Controller 重点并发 `-count=50`（23.9s）、CLI EOF/Session failure focused `-count=20`、取消用例 `-count=100`、`-version` 输出 `0.1.0-dev` 且不连接 Claude、`git diff --check` 干净、差异范围仅含 A3 相关文件。逐行核对 `run.go` 延迟 Turn 诊断（`deferredTurnFailures` 暂存→`turn_started`/`WaitIdle`/Session failure 三处收敛）、`scannerDone` EOF 提升竞态修复、`readStarted` 取消测试同步修正及 1 MiB LF/CRLF/无换行/+1 测试矩阵，均与 A3-result.md「Task 7 前回归收口」「最终审查修复记录」逐条对应；D24/D25/D26 复核结论与实现一致。未发现新问题；已知可接受行为：`session_stopped`/scanner error 急停路径会静默丢弃未 flush 的延迟 Turn 诊断（急停语义，符合规格）。

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
| A3 实施计划 | ✅ 已执行 | `docs/plans/2026-08-24-A3-session-core.md` |
| A3 门禁记录 | ✅ 通过 | `docs/testing/A3-result.md` |
| 业务代码 | ✅ A3 完成待提交 | 薄 SDK Adapter + Session Core + 异步行式 CLI |

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
