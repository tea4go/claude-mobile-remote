# B2 结构化会话控制 · 验证记录

- 关联设计：`docs/specs/2026-08-26-B2-interaction-control-design.md`
- 关联计划：`docs/plans/2026-08-26-B2-interaction-control.md`
- 关联决策：D46、D47

## 1. Task 2 Step 1 · `AskUserQuestion` 自由文本 CLI 实证

设计 §3.1「自由文本待实证」要求在实施前用真实 CLI 验证一次：`PermissionResultAllow.UpdatedInput["answers"]` 中回传一个**不属于任何 `options[].label`** 的字符串，CLI 是否接受、是否报 schema 错误。

- **日期**：2026-08-27
- **实证手段**：`internal/claude/live_test.go` 的 `TestLiveAskUserQuestionFreeText`（`//go:build integration` + `CLAUDE_MOBILE_REMOTE_LIVE=1` 双重门控，默认不参与常规 `go test ./...`）
- **执行命令**：`CLAUDE_MOBILE_REMOTE_LIVE=1 GOFLAGS=-buildvcs=false go test -tags integration ./internal/claude/ -run TestLiveAskUserQuestionFreeText -v -count=1 -timeout 8m`
- **实证流程**：真实 CLI 被提示调用一次 `AskUserQuestion`（三个选项 `Red`/`Green`/`Blue`）→ `CanUseTool` 回调按设计 §3.1 的形状回传「原始 `input` 加上 `answers` 键」，`answers` 的唯一值是一次性随机口令（刻意不取任何 `option.label`）→ 观察 `ToolResultBlock` 与后续助手文本。
- **观察结果**（两次独立执行完全一致）：

  | 观察项 | 结果 |
  | --- | --- |
  | `answers` map 的 key 来源 | `question`（设计 §3.1 规则的第一优先级命中） |
  | CLI 提供的 `option.label` | `Red` / `Green` / `Blue` |
  | `ToolResultBlock.is_error` | 字段缺省（既非 `true` 也非 `false`） |
  | 工具结果正文是否回显自由文本 | 是 |
  | 后续助手文本是否回显自由文本 | 是 |
  | `ResultMessage.is_error` | `false` |

- **结论：通过**。CLI 接受不属于 `options[].label` 的任意字符串作为 `AskUserQuestion` 的答案，未产生 schema 错误，且该字符串确实作为答案送达模型。
- **对实施的影响**：允许实现 `questions[].inputType = text`（协议与服务端校验）。按计划 Step 1 的明确要求，**实证通过不自动改变默认行为**：`ServiceConfig.AskUserTextEnabled` 保持默认 `false`，自由文本问题只有在显式开启该配置后才会生成与被接受。
- **前提与局限**：
  1. 本机 CLI 当次路由到的模型为网关模型（非 Anthropic 官方模型名）。`AskUserQuestion` 由 CLI 本地执行、`answers` 的形状校验发生在 CLI 侧，因此结论与具体模型无关；但「模型是否倾向于调用 `AskUserQuestion`」这一行为可能因模型而异，实证脚本用显式提示词规避了该不确定性；
  2. 只实证了**单问题、单个自由文本值**。多问题混合（部分 `choice` 部分 `text`）与自由文本的长度上限未实证，B2 按协议校验规则约束（每个自由文本答案恰好 1 个去空白后非空的值）；
  3. 实证输出刻意不记录问题原文、工具结果正文和一次性口令的具体值，只记录长度与是否回显。

## 2. Task 8 Step 1 · 完整自动门禁

- **日期**：2026-08-27
- **Go 侧**：`gofmt -l internal cmd` 无输出；`GOFLAGS=-buildvcs=false go vet ./...` 无输出；`GOFLAGS=-buildvcs=false go test -count=1 ./...` → 8 个包全部 ok（channel-test-client、claude-mobile-remote、channel、claude、pairing、session、transport、version）。
- **前端**（`apps/mobile`）：`npm test` → 16 files / 371 tests 全绿；`npm run lint`（oxlint）→ 0 warnings / 0 errors；`npm run build`（`tsc -b && vite build`）→ 通过（`dist/assets/index-*.js` 460.07 kB / gzip 145.19 kB）。
- **敏感信息与产物**（Step 5）：`git diff --check` 无空白错误；`go.mod`/`go.sum`/`apps/mobile/dist` 均未入库；对全部变更与新增文件扫描 `sk-ant`/`Bearer`/`eyJ`/私钥头/`password=`/`secret=` 无真实凭证命中（仅有描述扫描动作本身的文字与断言“不得出现这些字符串”的守卫测试）。

## 3. B2 验收阶段 · 服务重启后的配对恢复（D50 / D51）

- **日期**：2026-08-27
- **背景**：B2 人工验收发现已配对会话在 server 重启后无法直连，只能重新扫码，与「扫码一次后可继续使用」预期不符。修复分两批：先落地本地 pairing 文件持久化（D50），再按 AI 复审结论修正 5 项（含 D51 默认 TTL）。
- **自动门禁**：`gofmt -l internal cmd` 无输出；`GOFLAGS=-buildvcs=false go vet ./...` 无输出；`go test ./internal/pairing/... ./cmd/...` → pairing、claude-mobile-remote、channel-test-client 三个包 ok；`git diff --check` 无空白错误。
- **新增自动用例**：重启后恢复未过期凭证（含文件权限 `0600` 断言）、过期/跨 Session 不复用、损坏文件降级重发、持久化路径不可用时仍能签发并鉴权且不泄漏临时文件、恢复时使用当前 `--advertise-url`、`PruneExpired` 只删过期记录。
- **人工验证**：用户执行并反馈通过（本轮未逐条报告操作细节）；验证对象为真实 Claude + `--resume` 下重启 Remote Server 后仍能直连原 `sessionId`，不需重新配对。
- **本项的验收边界（不宣称超过的部分）**：
  - 服务端重启后恢复未过期 token✅；适用于浏览器保持打开或同标签页刷新（Fragment 仍在）+ server 重启的场景；
  - 客户端跨重启/跨标签页恢复配对记录⏳：`apps/mobile/src/session/store.ts` 仍为纯内存，新开标签页或 hash 被清后列表为空，归 C2 移动端缓存；
  - 不得概括为「扫码一次后可持续使用」整条链路已打通；
  - 无主动撤销入口：`Invalidate`/`Revoke` 只影响当前进程内存与已建立连接，彻底失效需用户删除 pairing 文件；
  - TTL 调大只影响新签发的凭证，已落盘的 12 小时记录仍按原 `expiresAt` 过期，首次重扫后才享受 30 天。

## 4. B2 未在本阶段验证的项（明确不宣称）

- **人工浏览器验收（Step 2）待做**：新增的结构化内容块、审批卡片、问答表单、取消按钮、排队列表与终态横幅只经 jsdom 验证，尚未在真实浏览器里比对参考图布局与手势；
- **真实 Claude 端到端（Step 3）待做**：工具审批放行/拒绝后 Turn 继续、一次真实工具调用尚未用真实 CLI 端到端验证（`AskUserQuestion` 自由文本已单独实证，见 §1）；
- **系统推送、原生能力与真机**：后台推送、推送 Token、Capacitor、扫码、安全存储、VPN 真机、HarmonyOS 真机均不属 B2，未验证；
- **完整历史与公网 Relay**：手机不重建完整上下文，公网 Relay 未实现，不属 B2；
- **边界场景**：`action_timeout` 超时分支未在真实 10 分钟窗口下验证；多问题混合（部分 `choice` 部分 `text`）与自由文本长度上限未实证；双客户端真实竞态只有单元/集成测试覆盖，未做人工双端对打。
