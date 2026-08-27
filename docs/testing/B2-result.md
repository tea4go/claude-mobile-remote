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

## 3. B2 未在本阶段验证的项（明确不宣称）

- **人工浏览器验收（Step 2）待做**：新增的结构化内容块、审批卡片、问答表单、取消按钮、排队列表与终态横幅只经 jsdom 验证，尚未在真实浏览器里比对参考图布局与手势；
- **真实 Claude 端到端（Step 3）待做**：工具审批放行/拒绝后 Turn 继续、一次真实工具调用尚未用真实 CLI 端到端验证（`AskUserQuestion` 自由文本已单独实证，见 §1）；
- **系统推送、原生能力与真机**：后台推送、推送 Token、Capacitor、扫码、安全存储、VPN 真机、HarmonyOS 真机均不属 B2，未验证；
- **完整历史与公网 Relay**：手机不重建完整上下文，公网 Relay 未实现，不属 B2；
- **边界场景**：`action_timeout` 超时分支未在真实 10 分钟窗口下验证；多问题混合（部分 `choice` 部分 `text`）与自由文本长度上限未实证；双客户端真实竞态只有单元/集成测试覆盖，未做人工双端对打。
