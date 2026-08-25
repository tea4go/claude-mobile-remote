# HANDOFF.md — Claude Mobile Remote 交接文档

> 新会话先读 `AGENTS.md`，再读本文件、`DECISIONS.md`、`REQUIREMENTS.md` 和总体设计。完成阅读后先复述当前状态和下一步，再按“当前执行”继续；涉及尚未确认的设计、计划或产品决策时先等用户确认。

## 0. 一句话现状

A1–A5 已完成并提交（A5 提交 `74bda86`）；A5.1 终端输入实时同步设计已获用户确认，当前编写实施计划，尚未修改 A5.1 代码。

## 当前执行（AI 自动维护）

- **子阶段**：A5.1 终端输入实时同步
- **当前任务**：A5.1 已完成（含人工验收），按用户授权提交
- **状态**：A5.1 代码、测试与门禁记录已完成；人工浏览器验收通过（含首轮流式回复回归修复）；resume 启动即配对作为 A5.2 候选、新会话先配对（D28 延迟绑定）作为待决策项
- **范围**：复用 `turn_started.text` 传输规范化后的输入正文；不同手机统一为 `source=mobile` 并进入现有 FIFO，不增加设备身份或抢占；覆盖 A3 发布、A4 有界传输、A5.1 消息归并与三端契约；不实现历史、replay、snapshot 输入正文、取消或 Markdown
- **通过条件**：计划逐项覆盖 A3 事件发布、A4 协议与有界传输、多手机真实 WebSocket、A5.1 decoder/reducer/UI、自动门禁和人工验收，且每项按测试先行拆分并写明精确文件与命令
- **完成结果**：按 `docs/plans/2026-08-25-A5.1-terminal-input-sync.md` 7 个任务 TDD 完成：A3 `turn_started` 携带规范化正文（含首轮 init 补发与排队可见性测试）；A4 `validateEvent` kind 级语义（入站+出站）与派生预算 `outboundEventMaxBytes = MaxPromptBytes+64KiB`、outbox/early = 2×；多手机并发/全局顺序/最大正文/慢连接隔离 WebSocket 集成测试；前端 decoder `validateEventSemantics`、reducer `InputMessage` 归并（去重/冲突/重复 Turn 拒绝）、MessageFeed 电脑端/手机端展示与 App 闭环
- **最近验证**：2026-08-25 自动门禁全绿——Go：`go test -race -count=1` session/channel/transport 三包、全仓 `go test -count=1 ./...`（9/9 包）、`go vet`、`go build`；前端：`npm test` 162/162、`npm run lint`、`npm run build`（含 tsc）；`git diff --check` 干净
- **未验证项**：A5.1 双页面并发与慢连接人工验证（自动测试覆盖）；A5 正式门禁记录 `docs/testing/A5-result.md` 尚待补写
- **下一步**：提交 A5.1；决策 A5.2（resume 启动即配对）与新会话延迟绑定的取舍；完成后进入 A6 VPN 联调
- **复审记录（2026-08-24，独立 Claude 会话，已被最终审查修复替代）**：只读门禁均 PASS，并确认 A2 仅复用 WhaleTerm SDK 用法、A3 保持 Controller/唯一 Worker 分层；其记录的 Session failure 部分文本换行 Minor 已在 Task 7 最终审查修复中关闭。
- **最终审查记录（2026-08-24）**：首次整体审查发现 EOF 观察调度可能造成同一 Session failure 的 Turn+Session 双诊断；按 TDD 改为基于 Worker settlement/下一 `turn_started` 延迟 Turn 诊断，并补直接 Session failure 换行。原审查代理复核后确认 Critical/Important/Minor 均为零，Ready for user diff review / final Git authorization。
- **A4 验收前审查记录（2026-08-24，独立会话）**：对 A4 全部新代码（pairing/channel/transport/CLI/channel-test-client）做只读审查。A4-result.md 声称证据 fresh 复现全部通过：全仓 `-count=1`、五个 A4 包 race、gofmt/vet/build、`git diff --check`、client race `-count=100`、Transport 三类关闭 race `-count=20`、fuzz 2s。发现 0 个 Critical、0 个 Important；2 个 Minor 观察（① `channel-test-client` 解析配对使用 `time.Now` 而非可注入时钟，`ExpiresAt` 在过期瞬间启动可能误报“配对信息无效”，重跑即可，不影响 Service 端正确性；② `commandGate.Abort` 生产未使用，仅测试调用，属轻微 API 冗余）。分层核对：Pairing/Channel/Transport 职责边界干净，Transport 未触碰 SDK，Channel 强制 `InputMobile`，D25 顺序门（`Begin → Submit → Finish(result 先入队 → 按原 seq 释放 early 事件)）` 实现正确；D27/D28 与实现一致。已向用户澄清：A4 范围即服务端管道 + Go 测试客户端，移动 Web 连接属 A5，无需在 A4 写移动端测试代码。
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
| A4 分阶段设计 | ✅ 已确认 | `docs/specs/2026-08-24-A4-channel-transport-design.md` |
| A4 实施计划 | ✅ 已确认 | `docs/plans/2026-08-24-A4-channel-transport.md` |
| A4 门禁记录 | ✅ 自动门禁与人工验收通过 | `docs/testing/A4-result.md` |
| 业务代码 | ✅ A4 已完成并由用户提交（`669e23d`） | 薄 SDK Adapter + Session Core + Channel/Transport + 配对与测试客户端 |

## 4. 下一步

1. 只读探索 `apps/mobile/` 现有 React/Vite 骨架、依赖和测试配置；
2. 逐项确认 A5 Mobile Web 的交互、状态归并、凭证承载和本机验收边界；
3. 写入 A5 分阶段设计并交用户检查，获确认后编写实施计划；
4. A5 门禁通过后进入 A6 VPN 真机闭环。

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
