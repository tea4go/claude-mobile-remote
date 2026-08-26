# HANDOFF.md — Claude Mobile Remote 交接文档

> 新会话先读 `AGENTS.md`，再读本文件、`DECISIONS.md`、`REQUIREMENTS.md` 和总体设计。完成阅读后先复述当前状态和下一步，再按“当前执行”继续；涉及尚未确认的设计、计划或产品决策时先等用户确认。

## 0. 一句话现状

A1–A5.2 已完成并提交；A6 的代码、双机说明和单机自动门禁已完成，当前等待 Windows 机器 B 完成人工跨机器门禁。

## 当前执行（AI 自动维护）

- **子阶段**：A6 局域网跨机器桌面浏览器联调（实施）
- **当前任务**：准备执行 A6 Task 4。Windows 机器 B 计划从远端 Git 仓库获取工程，但修复 `npm ci` 的 lockfile 尚未提交/推送；当前等待用户明确授权一个 Task 1–3 中间提交与推送，或指定其他文件同步方式，然后再启动双机门禁。
- **范围**：修改 `cmd/claude-mobile-remote/config.go` 与测试、`README.md`、`apps/mobile/README.md`、`HANDOFF.md`；修复阻断机器 B `npm ci` 的 `apps/mobile/package-lock.json` 可选平台元数据，但不改变 `package.json` 或依赖版本；双机验收完成后再创建 `docs/testing/A6-result.md`。不修改协议、状态、配对负载结构、前端业务代码、SDK 或后期功能，不执行 Git 写操作。
- **通过条件**：默认 Pair Token 为 12 小时且可配置覆盖，默认 loopback 与 localhost Origin 不变；Go/前端自动门禁通过；机器 B 通过 A 的显式 LAN `wsUrl` 完成首轮、双向同步、FIFO、错误凭证和重连验证；结果不外推到手机/VPN/C2。
- **状态**：Task 1–3 已完成。默认 TTL 的测试先按预期因旧值 10 分钟失败，改为 12 小时后通过；`npm ci` 的既有 lockfile 空版本问题已在空目录复现，并通过补齐同一 `oxlint 1.79.0` 的 8 个可选平台元数据修复。Fresh 门禁：Go 8 包 race、vet、build 通过；`npm ci` 成功，Mobile Web 167/167、lint、build 通过；gofmt、lockfile、凭证字面量和 `git diff --check` 检查通过。尚未把这些自动结果推断为 A6 人工通过。由于 B 从远端 Git 获取工程，必须先让 B 取得本轮 lockfile 修复；AI 尚未获得本次 commit/push 授权，未执行任何 Git 写操作。执行技能要求的 worktree 同样因 Git 写操作授权约束未创建。

## 1. 当前目标

下一门禁为 A6 局域网跨机器桌面浏览器联调：机器 A 运行 Standalone CLI、Claude 和 Remote Server，Windows 机器 B 只运行 `apps/mobile` 并通过 A 的显式 LAN `wsUrl` 控制同一 Session。

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
- 开发按里程碑和直接依赖门禁推进，当前下一阶段为 A6 局域网跨机器桌面浏览器联调；
- AI 可主动使用只读 `git diff` 检查改动；Git 写操作与其他未明确允许的 Git 命令仍需用户授权（D23）。

## 3. 文档状态

| 文件 | 状态 | 作用 |
| --- | :---: | --- |
| `REQUIREMENTS.md` | ✅ 用户已确认 | 产品需求、阶段范围和验收标准 |
| `AGENTS.md` | ✅ 用户已确认 | 长期约束和协作规则 |
| `CLAUDE.md` | ✅ | Claude Code 阅读入口 |
| `DECISIONS.md` | ✅ D39–D43 已确认 | 新项目决策日志 |
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
| A6 实施计划 | ✅ 用户已确认 / 执行中 | `docs/plans/2026-08-26-A6-lan-cross-machine-browser.md` |
| 业务代码 | ✅ A5.2 及之前已提交（本地，未推送） | 薄 SDK Adapter + Session Core + Channel/Transport + 配对与测试客户端 + Mobile Web |

## 4. 下一步

1. 用户明确授权将 A6 Task 1–3 当前改动提交并推送到当前远端分支，或指定其他方式让机器 B 取得修复后的 `apps/mobile/package-lock.json`；
2. 机器 B 获取后按 A6 清单完成错误凭证、首轮、双向同步、多入口 FIFO 和刷新重连验证；
3. AI 根据用户反馈创建 `docs/testing/A6-result.md` 并更新本文件；
3. 额度恢复后重跑真实 Claude 门禁（`TestLiveReservedSessionIDPreTurn`），补验首轮完成、跨 Turn 上下文和 Resume 同 ID；
4. 按需补做 A5.2 重连/resume 人工步骤并补写 `docs/testing/A5-result.md`；
5. SDK `WithSessionID` 发布正式版本后，把本地 `replace` 换回正式版本并同步 `go.mod.base`。

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
- Android SDK / Android Studio 与 DevEco Studio 尚未安装，分别在进入 C1、D1 前准备；
- antd-mobile 5.42.3 的部分传递依赖与 React 19 peer 范围不一致；用户已评估为当前可忽略，A1 测试、构建和 lint 均通过。

## 7. 参考提醒

WhaleTerm 的旧远程控制方案、阶段和记忆不读取也不继承。当前只参考 SDK 已验证用法；进入 W1 后再按已确认的单 Session 边界检查 WhaleTerm 当前 Claude manager/client 接口。
