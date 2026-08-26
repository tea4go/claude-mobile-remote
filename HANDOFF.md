# HANDOFF.md — Claude Mobile Remote 交接文档

> 新会话先读 `AGENTS.md`，再读本文件、`DECISIONS.md`、`REQUIREMENTS.md` 和总体设计。完成阅读后先复述当前状态和下一步，再按“当前执行”继续；涉及尚未确认的设计、计划或产品决策时先等用户确认。

## 0. 一句话现状

A1–A6 已提交；B1 已完成并通过门禁（自动化 14 files / 240 tests / lint 0 / build 通过 + 用户浏览器人工验收通过）；下一步进入 B2 设计，C1 待 JDK/Android SDK 安装后推进。

## 当前执行（AI 自动维护）

- **子阶段**：B2 结构化会话控制（设计）
- **当前任务**：按 SDK 源码与 WhaleTerm 实证核对结论修订 B2 设计草案（文件改名为 `docs/specs/2026-08-26-B2-interaction-control-design.md`）；暂不修改代码或实施计划。
- **范围**：设计真实 Thinking/Tool、MCP 工具自动放行与展示、Task 进度展示、工具权限审批、`AskUserQuestion` 单选/多选/自由文本、当前 Turn 取消、排队输入取消，以及 Session 终态的移动端呈现；统一设计 Go SDK Adapter、Session Controller、WebSocket 协议和 Mobile Web 的边界；不涉及 Capacitor、扫码、安全存储、VPN 真机或系统推送实现。
- **通过条件**：B2 设计明确 SDK 事件/回调映射、阻塞审批与取消的解阻塞顺序、action ID/request ID/seq 关联、审批/问答/取消的协议与状态语义、排队项可见性、超时/断线/重复操作/多客户端处理和 HarmonyOS NEXT 可实现性；用户确认设计后再写 `DECISIONS.md` 与实施计划。
- **已确认核对结论**（用户 2026-08-26 确认，待随设计定稿写入 `DECISIONS.md`）：
  1. pending action 期间不得直接调 SDK `Interrupt`（控制响应被阻塞的单读循环挡住、必然 5s 超时），取消统一先回 `PermissionResultDeny{Interrupt: true}` 解阻塞再收尾；
  2. `AskUserQuestion` 与工具权限共用 `CanUseTool` 链路，答案经 `UpdatedInput["answers"]` 回传（WhaleTerm 已实证）；协议保留数组 + `questionId`，由 Adapter 转成 CLI 期望的扁平 map；
  3. MCP 工具（`mcp__<server>__<tool>`）自动放行、不进审批，`pendingAction` 去掉 `toolClass`；
  4. Task 保留独立展示，支持 `kind=task`、`taskId`、`taskStatus`、`taskSummary` 和聚合数量；移动端默认折叠；
  5. 结构化事件 `kind` 收敛为 `thinking`/`tool_use`/`tool_result`（`mcp_tool` 合并进 `tool_use`）；
  6. Session 终态沿用 A3/A4 既有 `session_failed`/`session_stopped`，B2 只补移动端 UI；
  7. `AskUserQuestion` 回答的 `tool_result` 会带 `is_error: true`，必须修正为非错误；
  8. B2 同时支持单选、多选和自由文本回答；自由文本使用非空字符串回传。
- **状态**：B1 已完成；B2 设计已按上述结论修订为 `2026-08-26-B2-interaction-control-design.md`，待用户复核。C1 因 JDK/Android SDK 尚未安装，保持环境前置。

## 1. 当前目标

B2 先完成结构化 Thinking/Tool、审批、AskUserQuestion 与取消的设计确认；C1 等 JDK/Android SDK 就绪后再实施。

```text
机器 A：PC 输入 ──> Session Controller ──> Claude
                         └──LAN WebSocket──> 机器 B 浏览器
```

## 2. 已确认事项

- 项目仓库：`/Users/zhangym/code/work/utils/claude-mobile-remote`；
- 项目独立于 WhaleTerm，旧方案、阶段和记忆不继承；W1 只复用本项目新的单 Session Remote Server；
- 使用 `github.com/tea4go/claude-agent-sdk-go` 托管 Claude；
- 一个 Standalone CLI 进程只管理一个 Session；低频并行 Session 启动多个进程并使用不同端口；
- CLI 保留简单行输入输出作为 PC 输入和调试入口，不产品化 Claude Code 终端；
- WhaleTerm 后续只为其已持有的一个 Session 启动进程内 Remote Server，不创建第二个 SDK Client；
- 扫码绑定真实 `sessionId`；
- 手机认证后显式订阅对应 Session Channel，Channel 只负责消息路由；
- 移动端后续安全保存多个已扫描 Session，一次只连接当前进入的 Session；
- Claude 的完整上下文、会话状态和所有操作执行都留在 PC，手机只展示数据并回传操作；
- 手机不展示扫码前完整历史，只展示当前状态和后续消息；
- Claude 的一轮输出完成只结束 Turn，不结束 Session；Session 可继续接收下一条消息；
- 里程碑 A 的手机端只收发文本消息，审批、回答和取消在 B2 实现；
- Pair Token 默认有效期调整为 12 小时；原生 App 使用系统安全存储，浏览器不写 `localStorage`，过期或 Remote Server 重启后重新扫码；
- A6 改为 LAN 跨机器桌面浏览器门禁；手机仍默认通过 VPN 直连 PC，并在 C2 做真机验收；
- iOS / Android 使用 Capacitor，HarmonyOS NEXT 独立开发；
- C1 在 A6 后可与 B 并行；C2 依赖 C1/B2，W1 依赖 B3；B3 是单 Session Remote Server 组件化，不再建设 B4 持久恢复阶段；
- 后期公网访问使用可信的每 CLI 单出站隧道 Relay，不建设 PC 聚合 daemon、云端 Session 中心或永久 route；
- 心跳、事件 replay、系统推送、单进程多 Session 和云端多 PC 管理后置；
- A1 只创建 Go CLI、React/Vite 与基础工程配置；Capacitor 工程在 C1 创建，HarmonyOS 工程在 D1 创建；
- 开发按里程碑和直接依赖门禁推进，A6 已通过，B1 已提交实施，C1 可并行设计和实施；
- AI 可主动使用只读 `git diff` 检查改动；Git 写操作与其他未明确允许的 Git 命令仍需用户授权（D23）。

## 3. 文档状态

| 文件 | 状态 | 作用 |
| --- | :---: | --- |
| `REQUIREMENTS.md` | ✅ 用户已确认 | 产品需求、阶段范围和验收标准 |
| `AGENTS.md` | ✅ 用户已确认 | 长期约束和协作规则 |
| `CLAUDE.md` | ✅ | Claude Code 阅读入口 |
| `DECISIONS.md` | ✅ D39–D45 已确认 | 新项目决策日志 |
| 总体设计 | ✅ 用户已确认 | 架构、Channel、协议和测试边界 |
| A5.2 后简化路线设计 | ✅ 用户已确认 | `docs/specs/2026-08-25-simplified-roadmap-and-session-remote-design.md` |
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
| A5 分阶段设计 | ✅ 已确认 | `docs/specs/2026-08-24-A5-mobile-web-design.md` |
| A5 门禁记录 | ⚠️ 待补写 | `docs/testing/A5-result.md` 尚未创建 |
| A5.1 分阶段设计 | ✅ 已确认 | `docs/specs/2026-08-25-A5.1-terminal-input-sync-design.md` |
| A5.1 实施计划 | ✅ 已执行 | `docs/plans/2026-08-25-A5.1-terminal-input-sync.md` |
| A5.1 门禁记录 | ✅ 通过 | `docs/testing/A5.1-result.md` |
| A5.2 分阶段设计 | ✅ 已确认 | `docs/specs/2026-08-25-A5.2-pre-turn-pairing-design.md`（D38） |
| A5.2 实施计划 | ✅ 已执行 | `docs/plans/2026-08-25-A5.2-pre-turn-pairing.md` |
| A5.2 门禁记录 | ⚠️ 自动门禁与人工验收通过；真实 Claude 门禁仅部分通过（额度受限） | `docs/testing/A5.2-result.md` |
| A6 实施计划 | ✅ 已执行 / A6 门禁通过 | `docs/plans/2026-08-26-A6-lan-cross-machine-browser.md` |
| A6 门禁记录 | ✅ 自动门禁与用户双机人工验收通过 | `docs/testing/A6-result.md` |
| B1 分阶段设计 | ✅ 用户已确认 | `docs/specs/2026-08-26-B1-mobile-ui-design.md` |
| B1 实施计划 | ✅ Task 1–5 已执行 | `docs/plans/2026-08-26-B1-mobile-ui.md` |
| B1 门禁记录 | ✅ 自动门禁与用户人工验收通过 | `docs/testing/B1-result.md` |
| B2 分阶段设计 | 📝 修订稿，待用户复核 | `docs/specs/2026-08-26-B2-interaction-control-design.md` |
| C1 分阶段设计 | ✅ 用户已确认（已按 D44/D45 修订） | `docs/specs/2026-08-26-C1-capacitor-shell-design.md` |
| C1 实施计划 | ✅ 用户已确认（已按 D44/D45 修订为 6 个 Task） | `docs/plans/2026-08-26-C1-capacitor-shell.md` |
| 业务代码 | ✅ B1 及之前已提交（本地，未推送） | 薄 SDK Adapter + Session Core + Channel/Transport + 配对与测试客户端 + Mobile Web（B1 新增列表/详情/Markdown/占位） |

## 4. 下一步

1. 完成 B2 设计：结构化 Thinking/Tool 事件、审批、AskUserQuestion 与取消；用户确认后编写实施计划（需改协议，按硬约束 11 先核对三端可实现性并记入 `DECISIONS.md`）；
2. 安装 JDK 与 Android SDK（Apple Silicon 选 arm64-v8a 镜像），作为 C1 环境前置；
3. 按 `docs/plans/2026-08-26-C1-capacitor-shell.md` 执行 Task 1→6；
4. 额度恢复后重跑真实 Claude 门禁（`TestLiveReservedSessionIDPreTurn`），补验首轮完成、跨 Turn 上下文和 Resume 同 ID；
5. 按需补做 A5.2 重连/resume 人工步骤并补写 `docs/testing/A5-result.md`；
6. 行内代码当前为 `wordBreak: 'break-all'`，会在任意字符处切断标识符（见 B1-result §8.3），待确认是否改为 `break-word`；
7. SDK `WithSessionID` 发布正式版本后，把本地 `replace` 换回正式版本并同步 `go.mod.base`。

## 5. 首期关键验证

- SDK 能否在独立最小程序中稳定提供 session、流式消息、错误和取消事件；
- 真实 `sessionId` 何时可用，以及二维码生成时机；
- Turn 完成后 Session 能否继续接收终端或手机的下一条消息；
- 终端与手机输入如何在单一 Session Controller 中串行；
- 界面同步与断线恢复所需的 snapshot 最小字段；
- 显式 listen、advertise URL 和 Origin 能否支持机器 B 稳定访问机器 A，同时保持默认 loopback 安全边界；
- 手机 VPN、原生扫码、安全存储和前后台恢复在 C2 真机阶段验证。

## 6. 环境与已知事项

- 本机已具备 Go、Node.js 和 Xcode；
- Android SDK / Android Studio 尚未安装，按 D44 属于 C1 环境前置，需在 C1 开始前完成（Apple Silicon 选 arm64-v8a 系统镜像）；DevEco Studio 在 D1 前准备；
- 模拟器只用于界面适配与构建验证；其网络经 NAT（宿主机为 `10.0.2.2`），不能代替 C2 的真机与 VPN 验证；
- antd-mobile 5.42.3 的部分传递依赖与 React 19 peer 范围不一致；用户已评估为当前可忽略，A1 测试、构建和 lint 均通过。

## 7. 参考提醒

WhaleTerm 的旧远程控制方案、阶段和记忆不读取也不继承。当前只参考 SDK 已验证用法；进入 W1 后再按已确认的单 Session 边界检查 WhaleTerm 当前 Claude manager/client 接口。
