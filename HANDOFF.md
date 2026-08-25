# HANDOFF.md — Claude Mobile Remote 交接文档

> 新会话先读 `AGENTS.md`，再读本文件、`DECISIONS.md`、`REQUIREMENTS.md` 和总体设计。完成阅读后先复述当前状态和下一步，再按“当前执行”继续；涉及尚未确认的设计、计划或产品决策时先等用户确认。

## 0. 一句话现状

A1–A5.1 已完成、验证并提交；A5.2 首轮前配对已实施，首轮前扫码连接缺陷已修复并经用户实机验收通过，自动门禁全绿且已提交；但真实 Claude 门禁仍因账号额度只部分通过，尚不满足进入 A6 的门禁条件。

## 当前执行（AI 自动维护）

- **子阶段**：A5.2 首轮前配对（代码完成；人工验收通过；已按用户授权提交；真实 Claude 门禁未完全通过）
- **当前任务**：`docs/plans/2026-08-25-A5.2-pre-turn-pairing.md` 6 个任务已实施完毕，并修复了用户实机验收报告的首轮前配对阻塞缺陷；用户确认验证通过并授权提交，已完成本地提交（未推送）；剩余欠口为真实 Claude 额度相关门禁
- **状态**：SDK 侧走本地 `replace` 验证（`go.mod` 不入库、`go.mod.base` 未变），SDK 变更（`WithSessionID`）由用户随后自行发布新版本；项目侧改动已按用户授权提交到本地分支（未推送）
- **范围**：SDK 增加 `WithSessionID`；新会话预分配 UUID、Resume 预置已有 ID；Controller 区分 ID 已预留与 Claude init 已确认；Remote 在首轮前激活；前端解码器接受首轮前的空 `turn.source`；保持现有协议结构、真实 `sessionId`、Controller FIFO、多手机平等发送和 `session_started → turn_started` 顺序
- **A5.2 实际完成项**：① SDK `Options.SessionID` + `WithSessionID` + `--session-id` 映射 + UUID 校验 + 与 `Resume` 互斥；② Host `PrepareNewSession` 生成 UUID v4，新建走 `NewHostWithReservedSessionID`→`WithSessionID`、恢复走 `NewHostForResume`→`WithResume`（由 `sessionSDKOptions` 集中保证互斥），init ID 不一致按 Session 级失败；③ Controller `WithReservedSessionID` + `sessionConfirmed`，未确认即可 Submit，`turn_started` 暂缓至 init 确认后补发；④ CLI Connect 成功后即以预留 ID `Activate` 并输出配对，`session_started` 不重复激活，激活失败逆序清理退出 1，新增 `--resume <uuid>`；⑤ 首轮前 snapshot 与手机首条消息的 WebSocket 回归；⑥ 门禁记录 `docs/testing/A5.2-result.md`
- **实施与验收中修复的缺陷**：① resume 路径原误用 `--session-id`（违背设计 §4/§5.2 与 D38），已拆分 `NewHostForResume` 走 `WithResume`；② `TestRunWithRemoteActivatesBeforeFirstInput` 共享 `bytes.Buffer` 导致 `-race` DATA RACE，已改为互斥 `syncBuffer` + `waitForOutput`；③【用户人工验收发现，阻断 A5.2 主目标】首轮前扫码必连接失败、只有 PC 先完成一轮会话后重连才成功——首轮前 wire 上 `turn.source` 为空串（Go 零值），而 `decode.ts` 的 `decodeTurnSnapshot` 用 `requireNonEmptyEnum` 校验 `source`，解码抛错后 `client.ts` 以 `protocolError` 终止连接；已改用 `requireEnum` + 语义校验（`requestId` 非空时 `source` 不得为空），`types.ts` 将 `TurnSnapshot.source` 放宽为 `InputSource | ''`，`reducer.ts` 的 `applySnapshot` 增加空 `source` 守卫
- **缺陷③的门禁盲区与补测**：原仅断言 snapshot 键集、未断言 `turn.source` 的值，且前端 fixture 用了不真实组合 `{requestId:'', source:'mobile'}`。已补三层测试：`TestWebSocketPreTurnSnapshotTurnWireShape`（固化首轮前 Turn 的精确 wire JSON，属三端共用契约）、`decode.test.ts` 两个用例、`client.test.ts` 的「首轮前 snapshot 仍能进入已连接」
- **前端结论**：`canSendNow` 已允许 `sessionStatus === 'starting'`，首轮前配对无需新增前端状态、页面或命令类型；前端仅因缺陷③修改了 `decode.ts`/`types.ts`/`reducer.ts`
- **最近验证（2026-08-25，修复后全量复跑）**：Go `go test -race -count=1 ./...` 8/8、`go vet`、`go build`、`git diff --check` 干净；SDK 仓库 `go test ./...` 全绿；前端 `npm test` 167/167、`npm run lint` 0/0、`npm run build` 成功。真实 Service 端到端复验：启动后不在 PC 侧做任何会话，用真实前端 `ProtocolClient` + `decodeServerMessage` 直连 pairing，`hello_ack → subscribed → snapshot` 全部成功（`sessionStatus=starting`、`turn.source=''`，无 `protocolError`）；验证脚本为临时件，已删除不入库，配对凭证未写入任何文件。真实 Claude 门禁部分通过：`Connect(--session-id UUID)` 成功、Connect 后无业务 transcript、首个 init 返回预留 UUID 均已验证
- **人工验收（2026-08-25，用户实机）**：通过。启动 Service 后立即把 pairing 复制进 Web 即连接成功（无需先在 PC 会话），且首条输入来自手机时会话正常建立并回显；设计 §12.3 的重连、resume 等剩余步骤用户未逐条报告
- **未验证项**：① 真实 Claude 首轮完成/Result 同 ID/跨 Turn 上下文/Resume 同 ID（设计 §12.2 第 3–6 项）被账号额度阻断（`rate_limit 429` 后重跑 `context deadline exceeded`），`TestLiveReservedSessionIDPreTurn` 当前以 `t.Skip` 结束且不计入门禁；② 设计 §12.3 的重连与 resume 等剩余人工步骤未逐条确认；③ 计划 Task 5 Step 4 resume 人工验收未执行；④ `docs/testing/A5-result.md` 仍未补写（A5 遗留）；⑤ 既有 `cmd/channel-test-client/main_test.go` 存在与 A5.2 无关的 `gofmt` 对齐漂移，本次未改动以免污染 diff
- **下一步**：额度恢复后重跑 `CLAUDE_MOBILE_REMOTE_LIVE=1 go test -tags integration ./internal/claude -run TestLiveReservedSessionIDPreTurn` 并把结果补入 `docs/testing/A5.2-result.md`；补写 `docs/testing/A5-result.md`；真实 Claude 门禁补齐后才进入 A6 VPN 联调；SDK `WithSessionID` 由用户发布新版本后，将本地 `replace` 换回正式版本并同步 `go.mod.base`
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
| A5 分阶段设计 | ✅ 已确认 | `docs/specs/2026-08-24-A5-mobile-web-design.md` |
| A5 门禁记录 | ⚠️ 待补写 | `docs/testing/A5-result.md` 尚未创建 |
| A5.1 分阶段设计 | ✅ 已确认 | `docs/specs/2026-08-25-A5.1-terminal-input-sync-design.md` |
| A5.1 实施计划 | ✅ 已执行 | `docs/plans/2026-08-25-A5.1-terminal-input-sync.md` |
| A5.1 门禁记录 | ✅ 通过 | `docs/testing/A5.1-result.md` |
| A5.2 分阶段设计 | ✅ 已确认 | `docs/specs/2026-08-25-A5.2-pre-turn-pairing-design.md`（D38） |
| A5.2 实施计划 | ✅ 已执行 | `docs/plans/2026-08-25-A5.2-pre-turn-pairing.md` |
| A5.2 门禁记录 | ⚠️ 自动门禁与人工验收通过；真实 Claude 门禁仅部分通过（额度受限） | `docs/testing/A5.2-result.md` |
| 业务代码 | ✅ A5.2 及之前已提交（本地，未推送） | 薄 SDK Adapter + Session Core + Channel/Transport + 配对与测试客户端 + Mobile Web |

## 4. 下一步

1. 额度恢复后重跑真实 Claude 门禁（`TestLiveReservedSessionIDPreTurn`），补验首轮完成、跨 Turn 上下文和 Resume 同 ID；
2. 按需补做 A5.2 设计 §12.3 剩余人工步骤（重连、resume 验收），结果写回 `docs/testing/A5.2-result.md`；
3. 补写 A5 正式门禁记录 `docs/testing/A5-result.md`；
4. SDK `WithSessionID` 发布新版本后，把本地 `replace` 换回正式版本并同步 `go.mod.base`；
5. A5.2 门禁全部通过后进入 A6 VPN 真机闭环。

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
