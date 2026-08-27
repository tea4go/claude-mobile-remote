# HANDOFF.md — Claude Mobile Remote 交接文档

> 新会话先读 `AGENTS.md`，再读本文件、`DECISIONS.md`、`REQUIREMENTS.md` 和总体设计。完成阅读后先复述当前状态和下一步，再按“当前执行”继续；涉及尚未确认的设计、计划或产品决策时先等用户确认。

## 0. 一句话现状

A1–A6 已提交；B1 已完成并通过门禁；B2 Task 1–Task 7 已实施（剩余人工浏览器验收与真实 Claude 端到端验证后置，不阻塞）；C1 已完成并通过门禁（Capacitor iOS/Android 工程 + Web 接入 + 局域网明文放行 + Android 模拟器壳内连通验证通过）；下一步进入 C2 设计（真机/VPN/扫码/安全存储），或回补 B2 人工验收。

## 当前执行（AI 自动维护）

- **子阶段**：C1 已完成，待定下一步（C2 设计 或 B2 人工验收回补）
- **C1 完成项（2026-08-27）**：
  - 环境前置：JDK 21（用户自行升级）+ Android SDK（platform-tools 37.0.1 / platforms 36 / build-tools 36.0.0 / emulator 37.1.11 / arm64-v8a 镜像）+ AVD pixel7；
  - Task 1–6 全部执行：Capacitor 8.5.0 同主版本安装、`cap init` + `cap add ios/android`、`sync:native` 脚本、iOS/Android 平台工程创建、`Info.plist` ATS 局域网放行、Android `network_security_config.xml`、`server.url` 默认关闭的 live reload、`./gradlew assembleDebug` 构建门禁、Android 模拟器壳内连通验证；
  - 壳内连通实测：页面正常渲染 → 配对成功 → 发送消息 → 收到 fake-host 结构化回显（思考/工具/权限卡片），非空布局，远超门禁最低要求；
  - 验证记录已写 `docs/testing/C1-result.md`。
- **C1 两个超出 D44 的实测发现**：
  1. **Capacitor 8 Android 默认 `https` scheme 拦截 `ws://`**（混合内容策略），已按用户确认方案 A 在 `capacitor.config.ts` 设 `server.androidScheme: 'http'` 解决，需补记 DECISIONS；
  2. **原生壳 Origin 为 `http://localhost`（无端口）**，不在默认白名单；本轮用 `--allowed-origin http://localhost` 显式 flag 联调，是否加入默认值用户决定暂不决策、留待 C2。
- **范围**：C1 只做原生壳 + Web 接入 + 局域网明文，不涉及扫码、安全存储、NativeBridge、生命周期、VPN 真机（均属 C2）；未修改协议/状态/配对负载。
- **本次会话（2026-08-27，B2 人工测试发现的三个 UI 缺陷，已修复并通过用户复测）**：
  1. Claude 文本在 reducer 中按 `requestId` 聚合为单条消息，而结构化 block 追加到消息尾部；现已让文本片段与 block 共用有序消息流。
  2. `tool_use` 和 `tool_result` 分开展示；现已按 `toolUseId` 合并为一条工具项，并在结果到达后显示最终状态。
  3. 工具标题只有 `工具：Bash`；现已通过安全的服务端 preview 展示命令/路径摘要，并将连续工具项归入默认折叠面板。
- **当前任务**：B2 当前修复已由用户测试通过，暂不继续扩展 B2 UI；保留后置人工浏览器验收和真实 Claude 端到端验证。下一阶段优先执行 C1，将现有 React/Vite Mobile Web 封装进 Capacitor，并在已安装的 Android AVD/模拟器内启动、调试和验证；不提前引入 C2 真机、推送或 HarmonyOS。
- **Task 5 状态更正（本次会话开工前复核）**：上一次会话在写入 HANDOFF 前中断，导致「Task 5 待开始」记录失真。实际 Task 5 代码已完整落地：`internal/channel/channel.go` 命令面、`internal/transport/connection.go` 命令路由与 `resolveAction` 前置校验、`cmd/channel-test-client/command.go` 联调命令；并修复了 unsupported 通知的下行路由缺陷（`connection.go` 新增 `outboundEvent`，把 `EventUnsupported` 转为独立的 `TypeUnsupported` envelope，避开 `validateEvent` 失败后 `shutdown()` 强杀订阅连接）。已按 AGENTS.md「异常中断」条款复核并重跑门禁确认通过（本轮又一次全量重跑：`go vet ./...` 无输出、`go test ./internal/... ./cmd/...` 8 个包 ok）。
- **范围**：设计真实 Thinking/Tool、MCP 工具自动放行与展示、工具权限审批（白名单 `preview`）、`AskUserQuestion` 单选/多选（自由文本 CLI 实证已通过，默认仍关闭）、当前 Turn 取消、排队输入取消、假 Host 桩模式，以及 Session 终态的移动端呈现；统一设计 Go SDK Adapter、Session Controller/worker、Channel、WebSocket 协议和 Mobile Web 的边界；Task 按普通工具展示，不做任务聚合；不涉及 Capacitor、扫码、安全存储、VPN 真机或系统推送实现。
- **通过条件**：按 Task 顶部的 `**Files:**` 边界实施；`gofmt -l internal cmd` 无输出、`GOFLAGS=-buildvcs=false go vet ./...` 与 `go test ./...` 全绿（含旁路通道死锁不变量与取消顺序）；`cd apps/mobile && npm test && npm run lint && npm run build` 全绿；协议类型只在 `internal/transport/protocol.go` 与 `apps/mobile/src/protocol/types.ts` 两处定义；不修改 `go.mod`/`go.sum`；不执行 Git 写操作。
- **已确认核对结论**（用户 2026-08-26 / 08-27 确认，已写入 D46）：
  1. pending action 期间不得直接调 SDK `Interrupt`（控制响应被阻塞的单读循环挡住、必然 5s 超时），取消统一先回 `PermissionResultDeny{Interrupt: true}` 解阻塞再收尾；
  2. `AskUserQuestion` 与工具权限共用 `CanUseTool` 链路，答案经 `UpdatedInput["answers"]` 回传（WhaleTerm 已实证）；协议保留数组 + `questionId`，由 Adapter 转成 CLI 期望的扁平 map；
  3. MCP 工具（`mcp__<server>__<tool>`）自动放行、不进审批，`pendingAction` 去掉 `toolClass`；
  4. **Task 降级为普通工具展示**（用户 08-27 确认，取代早前的独立 Task 方案）：砍掉 `kind=task` 与 `taskId`/`taskStatus`/`taskSummary`/`taskItems`；SDK 无任何 Task 类 SystemMessage subtype（仅 `SubtypeInitialize`），且 `messages.go:39-41` 的 `ParentToolUseID` 过滤保持不动；
  5. 结构化事件 `kind` 收敛为 `thinking`/`tool_use`/`tool_result`（`mcp_tool` 合并进 `tool_use`）；
  6. Session 终态沿用 A3/A4 既有 `session_failed`/`session_stopped`，B2 只补移动端 UI；
  7. `AskUserQuestion` 回答的 `tool_result` 会带 `is_error: true`，必须修正为非错误；
  8. **自由文本先做 CLI 实证**（用户 08-27）：WhaleTerm 的 `buildAskUserAnswerInput` 只产出 `options[].label`，无自由文本分支；实证不通过则回退为只支持单选/多选；
  9. **`tool_approval` 按工具白名单抽取字符串 `preview`**（用户 08-27）：原始 `input` 对象不下发；
  10. **删除 Task 耗时与 `tool_result` 摘要要求**（用户 08-27）；
  11. **新增假 Host 桩模式**（用户 08-27）：把 B2 门禁与真实 Claude 额度解耦；
  12. **`action_required`/`action_resolved` 事件对与 `snapshot.queuedRequests`**（用户 08-27 确认，已写入 D47）：事件对提供审批发生/清除的时刻，snapshot 提供重连后的当前状态；`queueDepth` 与 `queuedRequests` 必须同时写入。
- **实际完成项**（2026-08-27）：
  - `docs/specs/2026-08-26-B2-interaction-control-design.md` 356 → 393 行：文件头状态改「已确认」并关联 D46；§3.1 新增工具白名单预览抽取表与 4 条规则、新增「自由文本待实证」段、回调运行约束补第 5 条；新增 §3.4「旁路通道与死锁不变量」（5 条）；§4.2 收敛为 3 类 `kind` 并删除全部 `task*` 字段、补充独立 `unsupported` 事件、`tool_result` 改为只下发关联 ID 与成败状态；§4.3 `input` 改 `preview` 并补 `inputType` 层级说明；§4.5 补三类 `values` 长度规则；补充 `ToolPreviewMaxRunes` 与 `AskUserTextEnabled` 配置约定；§6/§7/§8/§9/§10 同步；
  - `DECISIONS.md` D46：4 条决策扩展为 6 条（新增 `preview` 白名单与假 Host 桩），日期由 08-27 改为 08-26（08-27 为用户复核收敛范围）；
  - `REQUIREMENTS.md:175`：B2 描述补「结构化 Thinking/Tool 展示」；
  - `docs/plans/2026-08-26-B2-interaction-control.md` 92 → 152 行：按 B1 格式重写为 8 个 Task，补齐 `internal/session/worker.go`、`internal/channel/`、`cmd/claude-mobile-remote/config.go`、`cmd/channel-test-client/` 与新建 `preview.go`/`fakehost.go`；自由文本 CLI 实证前置为 Task 2 Step 1；固定 `unsupported` 事件语义、自由文本开关和 preview 配置来源；死锁不变量测试要求带超时守卫；每个 Step 带可复制命令且 Go 命令统一 `GOFLAGS=-buildvcs=false`。
- **实际执行的验证**：跨文档一致性核对（3 类 `kind`、9 个白名单工具、`@ <path>` 拼接、200 字符 rune 截断、自由文本条件生效、假 Host 桩在设计/D46/计划三处表述一致）；残留 `task*` 引用逐条确认均为「明确不做」的说明性文字；`git diff --check` 无空白错误；敏感词扫描（pairToken/Bearer/secret/password/sk-ant/eyJ）命中项全部为 DECISIONS 既有协议字段名描述，无真实凭证；文档字符逐一复核无形近字残留。
- **本次修复实际验证**：前端 `npm test` 通过（16 files / 376 tests）、`npm run lint` 通过、`npm run build` 通过；Go `internal/claude` 与 `internal/session` 测试通过；全量 `go test ./...` 的 `cmd/channel-test-client` 与 `internal/transport` 集成测试因当前沙箱禁止 `httptest` 绑定 `[::1]:0` 失败，未发现代码级测试断言失败；`git diff --check` 通过。
- **Task 1 实际完成项**（2026-08-27，Go 侧协议与领域模型）：
  - `internal/claude/preview.go`（新建 32 行）：`DefaultToolPreviewMaxRunes = 200` 与按 rune 截断的 `TruncatePreview`；配 `preview_test.go`（58 行）覆盖边界、非法上限兜底与 UTF-8 合法性；
  - `internal/session/model.go` 47 → 176 行：新增 `ServiceConfig`/`DefaultServiceConfig`、`AskUserQuestionToolName`、`BoolPointer`、`ContentBlock` 与 `BlockKind`/`ToolClass`、`PendingAction` 与 `ActionKind`/`Question`/`QuestionOption`/`QuestionInputType`、`QueuedRequest`/`newQueuedRequest`、`Snapshot.QueuedRequests` 与唯一写入口 `setQueuedRequests`；
  - `internal/session/event.go` 26 → 67 行：新增 `content_block`/`action_required`/`action_resolved`/`unsupported` 四个 kind、`UnsupportedCode` 四个固定 code 与 `IsKnown()`，`Event` 增补 `Block`/`PendingAction`/`Code`；
  - `internal/session/controller.go`：新增 `config ServiceConfig` 字段、`WithServiceConfig` Option 与 `syncQueue`；`handleSubmit`/`startNext`/`failSession`/`beginStop` 四处直写 `QueueDepth` 统一改走 `syncQueue`；`cloneSnapshot` 与 `publish` 深拷贝 `QueuedRequests`、`PendingAction.Questions[].Options` 与 `Block`；
  - `internal/transport/protocol.go` 249 → 604 行：`Version` 1 → 2；新增 7 个 §4.5 错误码、3 个命令 kind、3 个 `ActionDecision`、`QuestionAnswer`、`TypeUnsupported` 与 `UnsupportedPayload`；`CommandPayload` 扩展为按 kind 互斥的字段子集；`ErrorPayload` 增补 `ExpectedVersion` 与唯一构造入口 `newErrorPayload`；新增 `validateCommand`/`validateContentBlock`/`validatePendingAction`/`validateQuestions`/`ValidateActionResolution`；`validateEvent` 前置结构化字段归属校验，并显式拒绝 `unsupported` 作为普通事件；
  - `internal/transport/connection.go`：`fail` 改用 `newErrorPayload`，使 `unsupported_version` 自动携带 `expectedVersion`（全项目唯一 `ErrorPayload{}` 构造点）；
  - 测试：`internal/session/model_test.go` 143 → 348 行、`internal/transport/protocol_test.go` 247 → 735 行，共 18 个新增/改写测试函数、约 70 个子用例；连带修正 `controller_test.go`（`Snapshot` 含切片后改用 `reflect.DeepEqual` 比较零值）与 `server_test.go`（首轮前 snapshot 字段白名单补 `queuedRequests`）。
- **Task 1 相对计划的两处偏离（已核实必要）**：
  1. `TruncatePreview` 与截断上限默认值提前落在 `internal/claude/preview.go`（计划把该文件列在 Task 2）。原因是 `internal/session` 已 import `internal/claude`，claude 不能反向 import session，而截断需同时服务工具审批预览（Task 2）与排队项预览（Task 1）；放在 claude 侧可避免两份重复实现。Task 2 再往同一文件追加白名单抽取；
  2. `cmd/channel-test-client/main_test.go` 顺带执行 `gofmt`（本机 go1.25.3 判其既有对齐不合规，该文件本轮未被功能性修改），以满足 Step 6 的 `gofmt -l internal cmd` 无输出门禁。
- **Task 1 实际执行的验证**：Step 4 RED 已确认（两包 build failed，报 `undefined: EventContentBlock` / `BlockThinking` / `ToolBuiltin` / `ActionToolApproval` / `newErrorPayload` / `CodeInvalidAction` 等新符号）；Step 6 GREEN：`gofmt -l internal cmd` 无输出、`GOFLAGS=-buildvcs=false go vet ./...` 无输出、`GOFLAGS=-buildvcs=false go test ./...` 全部 8 个包 ok。前端与人工验收本 Task 不涉及。
- **Task 2 实际完成项**（2026-08-27，SDK Adapter 消息映射与审批预览）：
  - **Step 1 自由文本 CLI 实证已做且通过**：`internal/claude/live_test.go` 新增 `TestLiveAskUserQuestionFreeText`（+217 行，`//go:build integration` + `CLAUDE_MOBILE_REMOTE_LIVE=1` 双重门控）。两次独立执行结论一致：CLI 接受不属于任何 `options[].label` 的字符串作为答案，无 schema 错误，且该字符串确实送达模型（工具结果与后续助手文本均回显）。证据写入新建 `docs/testing/B2-result.md`（32 行）。按 Step 1 明确要求，**实证通过不自动改变默认行为**：`ServiceConfig.AskUserTextEnabled` 仍为 `false`；
  - `internal/claude/event.go` 67 → 123 行：新增 `EventContentBlock`/`EventUnsupported` 两个 kind、`BlockKind`+3 常量、`ToolClass`+2 常量、`UnsupportedCode`+`unknown_block`/`unknown_subtype`、`ContentBlock`（不带 `ID`，由 Controller 分配）、未导出 `askUserQuestionToolName`；`Event` 增补 `Block`/`Code`；
  - `internal/claude/messages.go` 80 → 211 行：`parsedMessage.text []string` 换为保序的 `items []parsedItem`（`kind`/`text`/`block`/`code`）；纯函数 `parseMessage` 改造为有状态的 `blockMapper.parse`（持有 `toolUseId → toolName`）；新增 `UserMessage` 分支只取 `ToolResultBlock`（忽略回显的 `TextBlock` 与字符串 `Content`）；`splitToolName` 用 `strings.SplitN(name, "__", 3)` 解析 MCP 名；`IsError` 缺省补 `false`、`AskUserQuestion` 的 `true` 修正为 `false`；未知 block、空 `thinking`、缺 `ToolUseID`/`Name`、MCP 名残缺、孤立 `tool_result` 全部降级为 `unknown_block` 且不携带原始字段；未知 `SystemMessage` subtype 降级为 `unknown_subtype` 且不透传 `Data`；
  - `internal/claude/preview.go` 32 → 90 行：追加 `ToolPreview(toolName, input, maxRunes) (string, bool)` 与 9 工具白名单（`Bash`←`command`；`Read`/`Write`/`Edit`/`NotebookEdit`←`file_path`；`Glob`/`Grep`←`pattern` 且 `path` 为非空字符串时拼 `<pattern> @ <path>`；`WebFetch`←`url`；`WebSearch`←`query`）；只读 `input`，未命中/缺字段/非字符串/去空白后为空均不返回 preview；
  - 测试：`messages_test.go` 87 → 301 行（改写 `TestParseMessage`，新增 `TestBlockMapperMapsToolBlocks` 9 子用例、`TestBlockMapperCorrectsAskUserQuestionResult`、`TestBlockMapperDegradesIncompleteBlocks` 5 子用例 + 孤立结果）；`preview_test.go` 59 → 211 行（`TestToolPreview` 13 正例、`TestToolPreviewWithoutPreview` 9 反例、`TestToolPreviewTruncatesByRune`）；`event_test.go` 21 → 42 行（`TestStructuredEventValues` 钉住新枚举字符串取值）。
- **Task 2 相对计划的三处偏离（已核实必要）**：
  1. 实证测试写在 `internal/claude/live_test.go`（不在 Task 2 的 `**Files:**` 边界内）。原因是真实 CLI 实证必须走既有的 integration 双门控机制，新建文件会重复一套门控；该文件默认不参与常规 `go test ./...`，不影响门禁；
  2. `is_error` 结果修正提前在 Task 2 的 mapper 内实现（计划把该断言列在 Task 3 Step 1）。原因是修正依赖 `toolUseId → toolName` 映射，而该映射本身是 Task 2 Step 2 明确要求的产物，两者无法拆开；
  3. `internal/claude/host.go` 做了**最小编译适配**（不在 Task 2 边界内，属 Task 3）：`parseMessage` 变为 `blockMapper` 方法导致 `RunTurn` 无法编译。适配限于新增 `Host.mapper` 字段与改用 `h.mapper.parse`，并在 `items` 循环里只发 `EventText`（`content_block`/`unsupported` 的上行接线明确留给 Task 3），保持 A2 既有行为不变。
- **Task 2 实际执行的验证**：Step 4 RED 已确认（`internal/claude` build failed，报 `unknown field items` / `undefined: parsedItem` / `EventUnsupported` / `UnsupportedUnknownSubtype` / `EventContentBlock` / `ContentBlock` / `BlockThinking` / `newBlockMapper`）；Step 6 GREEN：`gofmt -l internal cmd` 无输出、`GOFLAGS=-buildvcs=false go test ./internal/claude/` ok。额外跑了全量回归确认签名变更无连带破坏：`go build ./...` 与 `go vet ./...` 无输出、`go test ./...` 全部 8 个包 ok。实证命令：`CLAUDE_MOBILE_REMOTE_LIVE=1 GOFLAGS=-buildvcs=false go test -tags integration ./internal/claude/ -run TestLiveAskUserQuestionFreeText -v -count=1 -timeout 8m`。
- **Task 3 实际完成项**（2026-08-27，权限回调、Interaction Broker 与旁路通道）：
  - `internal/claude/host.go`（现 741 行）：新增 `ActionKind`/`QuestionInputType`/`QuestionOption`/`Question`/`ActionSpec`/`ActionDecisionKind`（4 个取值）/`ActionDecision`/`ActionRequest` 与 `NewActionRequest`、`Spec()`、`Resolve()`、`Decision()`、`cloneActionSpec`；`ActionSpec` 与 `ActionRequest` 拆开（前者纯值可复制，后者含 `sync.Once` + chan 只能按指针传，避开 `go vet` copylocks）；`Host` 新增 `actions` 旁路通道、`stopped`/`stopOnce`、`actionTimeout`（`DefaultActionTimeout = 10m`）、`previewMaxRunes` 与 `WithActionTimeout`/`WithToolPreviewMaxRunes`；删除 A2 时代硬编码的 `denyToolUse`，`sessionSDKOptions` 改为 `*Host` 方法并注入真实 `canUseTool`；新增 `canUseTool`/`buildActionSpec`/`waitForDecision`/`permissionResult`/`sessionScopedSuggestions`/`answeredInput`/`parseQuestions` 等；MCP 工具（`mcp__` 前缀）直接 Allow 不进审批；`allow_session` 只使用 SDK `Suggestions` 并将 `Destination` 置 `session`；`Interrupt` 与 `Close` 按“先关 `stopped` 再 `Disconnect`”重写；
  - `internal/session/worker.go`（现 97 行）：`Host` 接口扩展 `Actions() <-chan *projectclaude.ActionRequest` 与 `Interrupt(context.Context) error`；`hostWorker` 新增 `actions()`/`interrupt()` 访问器（注释注明为何不能走 `requests`/`events`/`results`）；
  - `internal/session/controller.go`（Task 1 后 1072 行 → 现 1346 行）：新增 5 个哨兵错误（`ErrActionNotPending`/`ErrInvalidAction`/`ErrInvalidAnswer`/`ErrTurnNotCancellable`/`ErrRequestNotQueued`）、`ActionDecision`+3 常量、`QuestionAnswer`、7 个 deny reason 常量；`actorState` 新增 `pendingAction`/`pendingActionID`/`nextActionSeq`/`currentCancelled`；新增 3 个命令类型与 `ResolveAction`/`CancelTurn`/`CancelQueued` 三个公开方法（共享 `sendControl`）；`runActor` 新增 `case request := <-c.worker.actions()` 分支（与 Turn 事件同一 select）；新增 `handleActionRequest`/`denyAndNotify`/`newPendingAction`/`hasFreeTextQuestion`/`handleResolveAction`/`buildActionDecision`/`orderAnswers`/`handleCancelTurn`/`handleCancelQueued`/`clearPendingAction`；`actionId` = `<requestId>-action-<n>`、`questionId` = `<actionId>-q<n>`，两者均不消耗 `requestIDGenerator`；
  - **5 处 Turn/Session 终止路径统一改走 `clearPendingAction`**（`handleWorkerEvent` 的 `EventTurnCompleted`、`handleWorkerResult`、`recordTurnFailure`、`beginStop`、`failSession`），保证任何终止都不会遗弃被阻塞的回调；`beginStop` 使用 `Interrupt: true`，其余用 `Interrupt: false`；
  - `internal/session/controller_test.go`（现 2672 行）：新增 15 个测试与 `activeSession`/`approvalSpec`/`questionSpec`/`choiceQuestion`/`awaitDecision`/`assertNoDecision`/`awaitActionRequired` 等辅助设施；`controlledHost` 补 `Actions()`/`Interrupt()`/`interruptCount()`/`requestAction()`/`awaitInterrupt()`；`internal/claude/host_test.go`（现 1076 行）新增 15 个 Broker 测试与 `connectHost`/`brokerHost`/`callTool`/`resolveNextAction`/`awaitSpec`/`requireAllow`/`requireDeny`。
- **Task 3 已定型的三个实现决定**（需在 Task 5/6 沿用）：
  1. **`cancel_turn` 用 `turn_completed` + `turnStatus=cancelled` 表达**：session 侧无 `turn_cancelled` 事件 kind，新增会越出 Task 3 的 `**Files:**` 边界且与 A3/A4 已有语义重叠。Task 6 移动端必须按 `turnStatus` 而非事件 kind 渲染取消结果；
  2. **session 侧自备 `ActionDecision`/`QuestionAnswer`**：包依赖方向为 `transport → session → claude` 单向，session 不能反向 import transport，因此两侧存在同名类型，Task 5 在 transport 处做一次显式转换（不是协议类型重复定义，后者仍只在 `protocol.go` 与 `types.ts` 两处）；
  3. **校验职责划分**：Controller 只做原子竞态判定（action 是否仍 pending）、形状互斥（approval 不接受 answers、question 不接受 decision）与覆盖性（每题恰答一次）；选项归属与单/多选元数等深度校验归 transport 的 `ValidateActionResolution`。`cancel_queued` 不发领域事件（无对应 kind），只改 snapshot，Task 5 由命令 ack 承载。
- **Task 3 相对计划的两处偏离（已核实必要）**：
  1. `cmd/claude-mobile-remote/run_test.go` 的 `runControlledHost` 补了 `Actions()`/`Interrupt()`（不在 Task 3 边界内）。原因是 `Host` 接口扩容后不补则整仓不编译，Step 5 也明文要求“同步更新所有 `Host` test double”；改动限于机械适配，不涉及行为；
  2. 删除了 A2 时代的 `TestDenyToolUse`。该测试钉的是“所有工具硬编码拒绝”，而用真实 Broker 取代它正是 B2 的目的；其意图已被新增的 15 个 Broker 测试覆盖。
- **Task 3 实际执行的验证**：Step 4 RED 已确认（两包 build failed，报 `undefined: ActionSpec` / `NewActionRequest` / `ErrActionNotPending` / `host.Actions undefined` 等）。Step 7 GREEN：`gofmt -l internal cmd` 无输出、`GOFLAGS=-buildvcs=false go vet ./...` 无输出、`GOFLAGS=-buildvcs=false go test ./...` 全部 8 个包 ok（A2/A3/A4/A5.2 回归无破坏）。额外跑了并发安全验证：`GOFLAGS=-buildvcs=false go test -race -count=2 ./internal/session/ ./internal/claude/` 两包均 ok，无 data race。旁路通道 15 个新测试已用 `-v` 逐个确认实跑 PASS（非 Skip），含“pending action 期间提交新输入不死锁”“有 pending action 时 `interruptCount()==0`”“无 pending action 时才调 `Interrupt`”三条关键不变式。前端与人工验收本 Task 不涉及。
- **Task 4 实际完成项**（2026-08-27，假 Host 桩模式）：
  - `internal/claude/fakehost.go`（新建 342 行）：`FakeScenario` + 4 个脚本（`interactive`/`text`/`turn-failure`/`session-failure`）、`FakeScenarios()`、严格解析的 `ParseFakeScenario`（不 trim、不忽略大小写）、固定 `FakeSessionID = 00000000-0000-4000-8000-000000000000`，以及满足 `session.Host` 接口的 `FakeHost`（`Connect`/`RunTurn`/`Actions`/`Interrupt`/`Close` + 测试用 `SessionID()`/`InterruptCount()`）。桩与真实 Host 共用同一套语义：`actions` 是无缓冲旁路通道（因此同样能暴露上层死锁）、`requestAction` 的上行与下行都带 ctx / 中断 / 停止三个分支、`Close` 幂等且解开等待中的脚本、`Interrupt` 关闭当前 Turn 信号、Turn 完成不结束 Session（第二轮不再发 `session_started`）；
  - `interactive` 脚本顺序：`session_started`（仅首轮）→ `thinking` → `tool_use:Bash` → **工具审批 action**（预览走真实 `ToolPreview` 白名单抽取，`AllowSessionAvailable: true`）→ `tool_result`（`isError` 随 deny 决议翻转）→ `tool_use:AskUserQuestion` → **`AskUserQuestion` action**（2 题：单选 + 多选，各带 2–3 个选项）→ `tool_result:isError=false`（结果修正）→ `text`（回显本轮决议）→ `turn_completed`；
  - `cmd/claude-mobile-remote/config.go`（119 → 157 行）：`cliConfig` 新增 `fakeHostScenario`、新增 `--fake-host` flag 与 `isFlagSet`/`fakeHostScenarioList` 两个辅助函数；
  - `cmd/claude-mobile-remote/main.go`（80 → 92 行）：Host 构造点抽出 `buildHost(config) (projectsession.Host, string, error)`，桩模式在此分流并复用 A5.2 的预留 ID 装配路径，`buildRuntime` 其余部分（配对、Registry、Server、Origin 白名单）逐字不变；
  - 测试：`internal/claude/fakehost_test.go`（新建 397 行，13 个测试）、`cmd/claude-mobile-remote/config_test.go`（74 → 130 行，2 个测试）。
- **Task 4 已定型的两个实现决定**：
  1. **`--fake-host` 与 `--resume` 互斥**（计划未要求，主动加）：桩没有可恢复的历史，静默忽略 `--resume` 会让用户误以为接上了旧上下文；
  2. **`--fake-host ""` 报错而非回退**：用 `flags.Visit` 区分「显式给了空值」与「完全没给」，拼错的开关不得静默退回真实 Host。
- **Task 4 实际执行的验证**：Step 3 RED 已确认（两包 build failed，报 `undefined: FakeHost` / `FakeScenario` / `NewFakeHost` / `FakeScenarios` / `ParseFakeScenario` / `cliConfig has no field fakeHostScenario` / `projectclaude.FakeSessionID`）。Step 5 GREEN：`GOFLAGS=-buildvcs=false go build ./...` 与 `go test ./internal/claude/ ./cmd/...` 全绿。额外跑了全量与并发回归：`gofmt -l internal cmd` 无输出、`go vet ./...` 无输出、`go test ./...` 全部 8 个包 ok、`go test -race -count=2 ./internal/claude/ ./cmd/claude-mobile-remote/` 两包 ok 无 data race；15 个新测试已用 `-v` 逐个确认实跑 PASS。**「不依赖真实 Claude」是可执行断言而非声明**：单测清空 `PATH` 并把 `HOME` 指向不存在目录后脚本仍完整跑通；另外真跑了一次 CLI（`PATH=/usr/bin:/bin:$GOROOT/bin`，其中无 `claude` 二进制）—— `--fake-host text` 下配对负载正常生成、会话正常启动、桩文本正常回显、进程正常退出。
- **Task 6 实际完成项**（2026-08-27，Mobile Web 协议与 reducer 升级）：
  - `apps/mobile/src/protocol/types.ts`（现 275 行，整体重写）：`PROTOCOL_VERSION` 1 → 2；新增 `ASK_USER_QUESTION_TOOL_NAME`、`ContentBlock`（按 kind 不相交的三个分支）、`PendingAction`（审批/问答两个分支）、`Question`/`QuestionOption`、`QueuedRequest`、`UnsupportedCode` 四码与 `UnsupportedPayload`、`CommandKind`/`ActionDecision`/`QuestionAnswer` 与按 kind 互斥的 `CommandPayload`；`MessageType` 与 `SERVER_TO_CLIENT_TYPES` 加入 `unsupported`；`ErrorPayload` 补 `expectedVersion`；`TurnSnapshot` 补 `pendingAction`、`SessionSnapshot` 补 `queuedRequests`；
  - `apps/mobile/src/protocol/decode.ts`（295 → 约 600 行）：新增 `decodeContentBlock`/`decodeQuestionOption`/`decodeQuestions`/`decodePendingAction`/`decodeQueuedRequests`/`decodeUnsupportedPayload` 与 4 个通用 helper（`requireNonEmptyString`/`requireBoolean`/`optionalString`/`rejectUnknownKeys`）；`decodeTurnSnapshot` 加「`pendingAction` 非空 ⇒ status 为 waiting 且 requestId 一致」不变量；`decodeSessionSnapshot` 加「`queueDepth === queuedRequests.length`」恒等（空队列在线上为 `null`，视为空数组）；`decodeEventPayload` 按 kind 校验 `block`/`pendingAction` 归属并拒绝事件携带 `code`；
  - `apps/mobile/src/protocol/client.ts`（+48 −5）：新增 `resolveAction`/`cancelTurn`/`cancelQueued` 三个命令方法与私有 `sendCommand`（与 `sendText` 共用单在途门控）、`ActionResolution` 两种形态、`ClientEvent` 新增 `unsupported`；`unsupported` 按通知处理（上报后继续使用当前连接，不断连），其他 session 的 unsupported 仍按协议错误关闭；
  - `apps/mobile/src/protocol/pairing.ts`：拆出 `PAIRING_PAYLOAD_VERSION = 1` 并用它校验配对负载，不再引用 `PROTOCOL_VERSION`（修复下方 §6 记录的待修缺陷）；连带修正 `apps/mobile/src/App.tsx` 用 `PROTOCOL_VERSION` 构造配对记录 `version` 的同类误用；
  - `apps/mobile/src/session/reducer.ts`（387 → 约 590 行）：新增 `BlockMessage`（入 `Message` 联合）、`Notice`；`ClaudeMessage.status` 新增 `cancelled`；`PendingCommand` 扩为 `{commandId, kind, text, targetId}`；`MobileState` 新增 `pendingAction`/`queuedRequests`/`lastNotice`/`canResolveAction`；新增 4 个 action 构造器（`actionResolutionSent`/`turnCancelSent`/`queuedCancelSent`/`noticeReceived`）；`canSend` 补「`pendingAction === null`」；`applyEvent` 新增 `content_block`/`action_required`/`action_resolved` 分支，`turn_completed` 按 `turnStatus === 'cancelled'` 渲染为已取消，`turn_started` 把对应项移出队列，Session 终态清空待处理事项与队列；`commandAccepted`/`commandRejected` 按 pending kind 与错误码分支（`cancel_queued` accepted 后本地移除、`invalid_answer` 保留事项供重填、`action_not_pending`/`action_timeout` 清除事项、`request_not_queued` 清除陈旧队列项）；
  - `apps/mobile/src/components/MessageFeed.tsx`（+12）：新增 `block` 分支复用 `StructuredBlockPlaceholder` 按块类别占位（明细渲染归 Task 7）；
  - 测试：`decode.test.ts`（现 1415 行 / 103 用例）、`client.test.ts`（529 行 / 28）、`pairing.test.ts`（187 行 / 21）、`reducer.test.ts`（947 行 / 64）。
- **Task 6 的三处修正与偏离（均已核实必要）**：
  1. 修正了自己在 Step 2 写错的两条 reducer 断言（`reducer.test.ts` 的「决议被接受」与「cancel_turn 在途」用例）：原写 `filter(kind === 'input')` 长度为 0，但夹具里的 `turn_started(source=terminal)` 按 A5.1 既有契约必然创建一条终端输入，两者直接矛盾。用例的真实意图是「决议/取消命令不产生手机输入正文」，已改为限定 `source === 'mobile'`；
  2. `apps/mobile/src/App.test.tsx` 的 wire 夹具仍为协议 v1（服务端消息 `version: 1`、snapshot 缺 `queuedRequests`），被 v2 解码器拒绝导致 16 个用例红。已改为引用 `PROTOCOL_VERSION` 并补 `queuedRequests: []`；配对负载夹具的 `version: 1` 保持不动（两个独立契约）；
  3. `MessageFeed.tsx` 不在 Task 6 的 `**Files:**` 边界内，但 `Message` 联合加入 `BlockMessage` 后 `tsc -b` 必然失败，故做了最小占位适配；丰富渲染仍留给 Task 7。
- **Task 6 实际执行的验证**：Step 3 RED 已确认（`Test Files 3 failed (3)`、`Tests 70 failed | 125 passed (195)`，典型报错 `TypeError: noticeReceived is not a function`）。Step 5 GREEN：`npx vitest run src/protocol src/session` → 5 files / 233 tests 全绿；`npm run build`（`tsc -b && vite build`）通过。额外跑了全量回归：`npm test` → 14 files / 324 tests 全绿、`npm run lint` → 0 warning 0 error、Go 侧 `go vet ./...` 无输出与 `go test ./internal/... ./cmd/...` 8 个包 ok（确认本轮改动未注入 Go 侧回归）。写入中文注释时又出现 3 处形近字污染（「伪造」「拒绍」「逐项」被写成形近字），已逐一 grep 定位并修正，并对 `src/` 做了繁体字扫描确认无残留。
- **Task 7 实际完成项**（2026-08-27，Mobile Web 结构化 UI 与控制交互）：
  - 新建 `apps/mobile/src/components/StructuredBlock.tsx`（62 行）：Thinking 用 `Collapse.Panel` 默认折叠（`destroyOnClose` 写在 **Panel** 上，实现读的是 `panel.props`）且正文按纯文本渲染；tool_use 只出摘要行（`工具：<name>` 或 `MCP：<server> / <name>`）；tool_result 只出成/败 Tag；协议不下发工具 `input`，因此工具行无可展开明细、无任何交互控件；
  - 新建 `PendingActionCard.tsx`（188 行）：审批与问答共用底部卡片；审批按 `allowSessionAvailable` 控制「本会话允许」，`preview` 为空时只显工具名且不自行拼接；问答按 `inputType`/`multiSelect` 分派 Radio/Checkbox/TextArea，`valuesOf` 丢弃不属于 `options` 的选项、自由文本去空白后为空则不可提交；提交后本地立即禁用，`enabled` 落假再恢复即解除（依靠 React「prop 变化时在渲染期调整 state」模式，不用 effect）；不展示 `actionId` 与权限规则原文；
  - 新建 `QueuedRequestList.tsx`（47 行）：空队列不渲染；按 `queuedRequests` 顺序展示来源 Tag 与服务端截断后的 preview，逐项取消只回传对应 request ID，不展示 request ID 本身；标题用「等待处理的输入」而非「排队中的输入」，避开与测试夹具 preview 文本撞名；
  - `MessageFeed.tsx`（+24 −26）：`block` 分支换成 `StructuredBlock`，新增纯函数 `resolveToolName(messages, index, toolUseId)` 向前关联工具名（孤立结果返回空串，不编造）；claude 分支补 `status === 'cancelled'` 的「已取消」Tag；
  - `SessionStatus.tsx`（+24）：新增 `session-terminal-banner`（stopped →「会话已结束，不再接收新输入」/ failed →「会话已失败，不再接收新输入」），非终态不渲染，不推测原因；
  - `SessionDetail.tsx`（+55 −7）：删除两个 B1 只读占位；接入 `PendingActionCard`（固定在输入区上方，`enabled={state.canResolveAction}`）、`QueuedRequestList`、「取消当前 Turn」按钮、`session-notice` 与「已请求取消」提示；取消类命令的可用性用独立的 `commandReady`（已连接且无在途命令）而不复用 `canSend`，因为等待待处理事项时自由文本被禁用但仍应可取消；
  - `App.tsx`（+58 −2）：`onEvent` 新增 `case 'unsupported' → noticeReceived`；新增 `handleResolveAction`/`handleCancelTurn`/`handleCancelQueued` 三个命令回调（各自生成 commandId 后 dispatch 对应构造器）；`turnStatus` 在有待处理事项时优先为 `waiting`；新增 `cancelTarget` 推导（待处理事项的 requestId 优先，否则取最后一个 `streaming` 的 claude 消息）；
  - 删除 `StructuredBlockPlaceholder.tsx` 与 `StructuredBlockPlaceholder.test.tsx`（计划 Step 1 明确要求替换）；
  - 测试：新增 `StructuredBlock.test.tsx`（123 行 / 11）、`PendingActionCard.test.tsx`（267 行 / 18）、`QueuedRequestList.test.tsx`（53 行 / 5）；`MessageFeed.test.tsx` +75（5 例）、`SessionStatus.test.tsx` +56（4 例）、`App.test.tsx` +276（9 例集成，含审批决议完整链路、竞态拒绝、重连 snapshot 恢复、两类取消、终态与 unsupported 通知）。
- **Task 7 相对计划的两处偏离（已核实必要）**：
  1. 额外改了 `SessionDetail.tsx` 与 `SessionDetail.test.tsx`（不在计划 `**Files:**` 内）。原因是 B1 重构后详情页已不在 `App.tsx` 里渲染，卡片、队列、取消按钮和通知只能接在 `SessionDetail`；该组件新增 4 个必填 prop 后，其测试的 9 处 render 必须同步补齐（不补则 `tsc -b` 不过）；
  2. 删除了 `StructuredBlockPlaceholder` 及其测试文件（计划只写「替换」，未列入 Files 的 Delete 项）。
- **Task 7 实际执行的验证**：Step 5 RED 已确认（`Test Files 6 failed (6)`、`Tests 16 failed | 39 passed (55)`，失败原因全为组件模块不存在或目标文案/元素缺失，无一是夹具或既有契约问题）。Step 7 GREEN 与前端门禁：`npm test` → **16 files / 371 tests 全绿**、`npm run lint` → **0 warning 0 error**、`npm run build`（`tsc -b && vite build`）通过；额外跑了 Go 侧回归 `GOFLAGS=-buildvcs=false go test ./...` → 8 个包 ok（本轮未动 Go 代码）。
- **Task 7 本轮修正的四个自身错误**：
  1. 写测试时把「拒绝」误写成形近字（`App.test.tsx`），另在注释里又出现一次同类污染，已全部修正；本轮最后对 `apps/mobile/src` 做了繁体字与已知形近字扫描，确认无残留；
  2. `PendingActionCard.test.tsx` 的「三个按钮分别回传决议」用例原本在同一渲染里连续点三个按钮，与设计 §6「提交后立即禁用」直接矛盾（第二、三次点击必然无效）。已改为每个决议各自独立渲染一次；
  3. antd-mobile 把 `data-testid` 挂在包装 `div` 上而不是真实 `textarea`，导致两处断言（`composer-input` 的 `toBeDisabled`、自由文本的 `fireEvent.change`）失效，已改用 `getByRole('textbox')`；
  4. `PendingActionCard` 初版用两个 `useEffect` 做重置，触发 oxlint 的 `react(set-state-in-effect)` 两条警告（计划要求 lint 全绿），已改为渲染期比较上一次 prop 的官方模式。
- **Task 8 已完成的 AI 侧子项**（2026-08-27）：
  - **Step 1 完整自动门禁全绿**：`gofmt -l internal cmd` 无输出、`go vet ./...` 无输出、`go test -count=1 ./...` 8 个包 ok；前端 `npm test` 16 files / 371 tests、`npm run lint` 0/0、`npm run build` 通过；
  - **Step 4 关闭自由文本条件条款**：设计 §标题/§1目标/§3.1/§4.2 的“实证通过后生效”措辞已改为“实证已通过、`inputType: text` 已启用、作为可选开关默认关闭”；
  - **Step 5 敏感信息与产物**：`git diff --check` 无空白错误；`go.mod`/`go.sum`/`dist` 未入库；全量变更与新增文件扫描无真实凭证（仅命中描述扫描本身的文字与守卫测试）；
  - **Step 6 记录**：`docs/testing/B2-result.md` 已追加§2（完整自动门禁）与§3（B2 未验证项明确不宣称）；计划 Step 1/4/5/6 已勾选。
- **Task 8 待用户参与的两项**：**Step 2 人工浏览器验收**（`--fake-host` 场景，需人眼看布局与手势）与 **Step 3 真实 Claude 端到端**（工具审批放行/拒绝后 Turn 继续、一次真实工具调用）。
- **未验证项**：**自 B1 以来一直未做人工浏览器验收**（审批卡片、问答表单、取消按钮、队列列表与终态横幅只经 jsdom 验证）；**真实 Claude 下的工具审批与取消尚未端到端验证**（`AskUserQuestion` 自由文本已单独实证）；参考图 `session_detail.png` / `session_approve.png` 布局契合度未经人眼比对；`action_timeout` 超时分支未在真实 10 分钟窗口下验证；多问题混合与自由文本长度上限未实证；双客户端真实竞态只有单元/集成测试覆盖，未做人工双端对打。
- **实施进展**（按 Task 更新）：**Task 1–Task 7 已完成**；Task 8 的 AI 侧子项（Step 1/4/5/6）已完成，仅剩 Step 2（人工浏览器）与 Step 3（真实 Claude）待用户参与。
- **已消除的拍板项**：`action_required` / `action_resolved` 两个事件 kind 与 `Event.PendingAction` 字段经用户 2026-08-27 确认保留，三端可实现性核对结论已随 `snapshot.queuedRequests` 一并写入 `DECISIONS.md` D47，Task 3 / Task 6 不再被阻塞。自由文本 CLI 实证已完成，不再阻塞 Task 7 Step 3。
- **下一步**：Task 8 仅剩两个需用户参与的 Step。**Step 2 人工浏览器验收**：启动 `--fake-host` 后在手机/浏览器走通默认折叠、MCP 无审批、`Task` 只作工具行、审批三决议与 `preview` 有/无两种形态、问答多问题单选/多选、当前 Turn 与排队取消、pending action 期间取消不卡死、终态横幅、双客户端竞态、重连 snapshot 恢复、B1 客户端被拒；**Step 3 真实 Claude 复测**：有额度时用真实 Host 复测一轮审批放行/拒绝后 Turn 继续与一次真实工具调用。两项完成后 B2 即可整体交接。
- **与文档不一致的一个实证观察**：设计 §3.1 与 WhaleTerm 参考实现均称 `AskUserQuestion` 的 `tool_result` 会带 `is_error: true`，但本次实证观察到该字段**缺省**。修正逻辑仍保留（对缺省情形无副作用），差异已记入 `docs/testing/B2-result.md`。
- **状态**：B1 已完成；B2 四份文档已经用户复核确认，Task 1–Task 7 代码已实施并通过 Go 侧与前端全量自动化验证；Task 8 的自动门禁、自由文本条款收敛、敏感信息检查与 B2-result.md 记录已完成，仅剩人工浏览器验收（Step 2）与真实 Claude 复测（Step 3）待用户反馈。本轮无任何 Git 写操作。C1 因 JDK/Android SDK 尚未安装，保持环境前置。

## 1. 当前目标

B2 设计与实施计划已确认，Task 1–Task 7 已实施（Go 侧协议 v2 与领域模型、SDK Adapter 结构化映射与工具预览、权限回调与 Interaction Broker/取消旁路通道、假 Host 桩模式、Channel 命令面与 Transport v2 路由、Mobile Web 协议与 reducer 升级、Mobile Web 结构化 UI 与控制交互），下一步从 Task 8 跑完整门禁、做人工浏览器验收并写入验证记录；C1 等 JDK/Android SDK 就绪后再实施。

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
| B2 分阶段设计 | ✅ 用户已确认并提交 | `docs/specs/2026-08-26-B2-interaction-control-design.md` |
| B2 实施计划 | ✅ Task 1–6 已执行 | `docs/plans/2026-08-26-B2-interaction-control.md` |
| C1 分阶段设计 | ✅ 用户已确认（已按 D44/D45 修订） | `docs/specs/2026-08-26-C1-capacitor-shell-design.md` |
| C1 实施计划 | ✅ 用户已确认（已按 D44/D45 修订为 6 个 Task） | `docs/plans/2026-08-26-C1-capacitor-shell.md` |
| C1 门禁记录 | ✅ 自动门禁与 Android 模拟器壳内连通通过 | `docs/testing/C1-result.md` |
| 业务代码 | ✅ B1 及之前已提交（本地，未推送） | 薄 SDK Adapter + Session Core + Channel/Transport + 配对与测试客户端 + Mobile Web（B1 新增列表/详情/Markdown/占位） |

## 4. 下一步

1. C1 已完成并通过门禁；下一步进入 C2 设计（扫码、安全存储、多配对列表、两端真机、VPN 与生命周期）或回补 B2 人工浏览器验收，待用户定序；
2. B2 暂停项保留：人工浏览器验收、真实 Claude 端到端验证、UI 细节调整，不阻塞 C1 后续；
3. C1 遗留一个待办：发现 2（Origin 默认值是否加 `http://localhost`）用户决定留待 C2；发现 1（`androidScheme: 'http'`）已补记 D49；
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
- Android SDK 已就绪（C1 已装）：platform-tools 37.0.1 / platforms 36 / build-tools 36.0.0 / emulator 37.1.11 / system-images android-36 arm64-v8a，AVD `pixel7`；`ANDROID_HOME=~/Library/Android/sdk` 已写入 `~/.zshrc`；JDK 21 由用户 JVMS 管理；
- 模拟器只用于界面适配与构建验证；其网络经 NAT（宿主机为 `10.0.2.2`），不能代替 C2 的真机与 VPN 验证；
- antd-mobile 5.42.3 的部分传递依赖与 React 19 peer 范围不一致；用户已评估为当前可忽略，A1 测试、构建和 lint 均通过；
- **C1 两个平台实测结论**（详见 `docs/testing/C1-result.md` §7）：① Android 壳需 `server.androidScheme: 'http'` 才能连 `ws://` 明文（默认 https 会混合内容拦截）；② 原生壳 Origin 为 `http://localhost`（无端口），联调需 `--allowed-origin http://localhost`；
- **B2 中间态已解除：移动端配对已随 Task 6 恢复（待浏览器验证）**。Task 1 把 `transport.Version` 升到 2 后，移动端 `PROTOCOL_VERSION` 一度停留在 1，手机 `hello` 被服务端按 `unsupported_version` 拒绝；Task 6 已把移动端升到 2 并完成新事件/字段的解码，理论上配对已可用，但**尚未在真实浏览器里验证**（人工验收建议在 Task 7 UI 就绪后一并做）。二维码里的 `"version":1` 属独立常量 `pairing.PayloadVersion`，本来就无需改动；
- **已修的版本混淆缺陷**（Task 6 已处理）：`apps/mobile/src/protocol/pairing.ts` 原本用 `PROTOCOL_VERSION` 校验**配对负载**的 `version`，把传输协议版本与配对负载版本混为一谈（两者同为 1 时巧合通过）。现已拆出 `PAIRING_PAYLOAD_VERSION = 1`，并同步修正了 `App.tsx` 里同类的误用；`pairing.test.ts` 新增一组用例钉住「两个版本契约相互独立」；
- Task 6 已把新事件与新字段全部接入移动端解码：`content_block`/`action_required`/`action_resolved` 已在事件白名单内，`unsupported` 走独立 envelope，`pendingAction` 按真实字段严格校验（不再要求空对象）。

## 7. 参考提醒

WhaleTerm 的旧远程控制方案、阶段和记忆不读取也不继承。当前只参考 SDK 已验证用法；进入 W1 后再按已确认的单 Session 边界检查 WhaleTerm 当前 Claude manager/client 接口。
