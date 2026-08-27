# B2 结构化会话控制实施计划

> **执行方式：** 按 Task 顺序逐项实施，每个 Task 内先写测试确认 RED 再实现 GREEN。步骤使用 `- [ ]` 复选框跟踪进度。Git 写操作需用户单独授权。

**Goal:** 在不改变单 Session Host 和 Session Controller 单写入者边界的前提下，实现 Thinking 与 Tool/MCP（含按普通工具展示的 `Task`）结构化展示、工具审批（白名单 `preview`）、AskUserQuestion 单选/多选、当前 Turn 与排队输入取消、Session 终态移动端呈现，并提供不依赖真实 Claude 额度的假 Host 桩模式。

**Architecture:** SDK Adapter 负责 Claude 消息与权限回调映射并抽取审批预览；Session Controller 是 action、FIFO、取消和状态的唯一写入者，pending action 经专用旁路通道上下行；Channel 扩展命令面并保持 `sessionId` 隔离；Transport 负责版本 2 严格校验与路由；Mobile Web 负责默认折叠的精简 UI。MCP `mcp__<server>__<tool>` 自动放行，不产生 pending action；`Task` 与其他内置工具同构。

**Tech Stack:** Go（`gofmt`、`go test`，命令统一带 `GOFLAGS=-buildvcs=false`）、`github.com/tea4go/claude-agent-sdk-go`（本地 replace）、gorilla/websocket；React 19、TypeScript、antd-mobile 5、Vite、Vitest、Testing Library。协议类型只在 `internal/transport/protocol.go` 与 `apps/mobile/src/protocol/types.ts` 两处定义，逐字段对齐。`ServiceConfig.ToolPreviewMaxRunes` 默认 200，`ServiceConfig.AskUserTextEnabled` 默认关闭。

---

## 文件与边界

- Modify: `internal/claude/messages.go`、`internal/claude/event.go` 及测试；Thinking/Tool/ToolResult 映射与未知类型降级。`messages.go` 中 `ParentToolUseID != nil` 的子代理过滤保持不动。
- Modify: `internal/claude/host.go` 及测试；用 Interaction Broker 替换现有 `denyToolUse` 硬编码拒绝（`host.go:34` 与 49/68/101 三处 `WithCanUseTool` 装配点）。
- Create: `internal/claude/preview.go`、`preview_test.go`；工具白名单预览抽取与按 rune 边界截断。
- Create: `internal/claude/fakehost.go`、`fakehost_test.go`；实现 `session.Host` 窄接口的脚本化假 Host。
- Modify: `internal/session/model.go`、`event.go`、`controller.go` 及测试；`PendingAction` 从空结构体扩展，新增结构化事件、`queuedRequests`、action 决议、取消与超时。
- Modify: `internal/session/worker.go`、`worker_test.go`；新增 action 旁路通道，保持无缓冲通道死锁不变量。
- Modify: `internal/channel/channel.go`、`registry.go` 及测试；`Controller` 接口从只有 `Submit` 扩展到 action 决议与两类取消，Channel 暴露对应方法。
- Modify: `internal/transport/protocol.go`、`connection.go`、`server.go` 及测试；`Version` 常量 1 → 2，新增结构化 payload、action/cancel 命令、排队列表与错误码。
- Modify: `cmd/claude-mobile-remote/config.go`、`main.go`、`run.go` 及测试；新增 `--fake-host` 场景开关，默认空值走真实 Host。
- Modify: `cmd/channel-test-client/main.go`、`main_test.go`；补审批决议与取消命令，便于联调。
- Modify: `apps/mobile/src/protocol/{types.ts,decode.ts,client.ts}`、`session/reducer.ts` 及测试；与 Go 协议逐字段同步。
- Modify: `apps/mobile/src/components/MessageFeed.tsx`、`SessionStatus.tsx` 及测试；Create: 结构化内容块、审批/问答卡片、排队列表组件及测试；只用现有 antd-mobile。
- Create: `docs/testing/B2-result.md`；记录自动门禁、自由文本 CLI 实证结论和人工浏览器验收，不记录真实 token、`sessionId`、会话内容、工具原始 `input` 或预览文本。
- Modify: `HANDOFF.md`；记录实际完成项、验证结论、未验证项和下一步。
- 不修改 `go.mod`/`go.sum`（如需依赖变更须先同步 `go.mod.base` 并经用户确认）；不修改 Capacitor 工程、扫码、安全存储、VPN、系统推送、HarmonyOS 工程或 PC TUI；不解析 JSONL 替代 SDK。

### Task 1: 固化 Go 侧协议与领域模型

**Files:**
- Modify: `internal/session/model.go`, `internal/session/model_test.go`
- Modify: `internal/session/event.go`
- Modify: `internal/transport/protocol.go`, `internal/transport/protocol_test.go`

- [x] **Step 1: 先写协议契约失败测试**：断言 `Version == 2`；版本不匹配返回 `unsupported_version` 且 payload 带 `expectedVersion`、不含任何 Session 数据；`content_block` 三类 `kind` 各自的字段白名单（多余字段拒绝）；`unsupported` 只允许固定安全 `code`，不作为第四种 `content_block.kind`；`pendingAction` 含 `actionId`/`requestId`/`kind`/`toolName`/`preview`/`allowSessionAvailable`/`questions` 且**不含**原始 `input`、`toolClass`、`toolUseId`。
- [x] **Step 2: 写命令与错误码失败测试**：`resolve_action`、`cancel_turn`、`cancel_queued` 的解码与校验；`answers` 为 `[{questionId, values}]`，单选恰好 1 个值、多选 ≥1 个值且都命中 `options[].label`、缺答拒绝；错误码常量 `invalid_action`、`action_not_pending`、`invalid_answer`、`turn_not_cancellable`、`request_not_queued`、`request_already_finished`、`action_timeout` 齐备。
- [x] **Step 3: 写领域模型失败测试**：`PendingAction` 扩展后字段完整，`questions[].inputType` 属于问题而非 action 顶层；`QueuedRequest` 与 `queuedRequests` 保证 `queueDepth == len(queuedRequests)`，`preview` 按固定上限截断；`session_failed`/`session_stopped`/`turn_completed`/`turn_failed` 终态语义不变。
- [x] **Step 4: 运行定向测试确认 RED**：执行 `GOFLAGS=-buildvcs=false go test ./internal/transport/ ./internal/session/ -run 'Protocol|PendingAction|Queued|Version'`，预期新字段与错误码尚不存在导致失败。
- [x] **Step 5: 实现协议与模型**：升级 `Version` 常量并让 `NewEnvelope` 统一使用；实现字段白名单校验与错误码；`unsupported` 使用独立事件类型且不携带原始数据；`inputType` 默认固定为 `choice`，只有 CLI 实证通过且 `AskUserTextEnabled=true` 时才放行 `text`。
- [x] **Step 6: 运行 GREEN 与格式化**：执行 `gofmt -l internal cmd` 应无输出，再执行 `GOFLAGS=-buildvcs=false go test ./internal/transport/ ./internal/session/`。

### Task 2: SDK Adapter 消息映射与审批预览

**Files:**
- Modify: `internal/claude/messages.go`, `internal/claude/messages_test.go`
- Modify: `internal/claude/event.go`, `internal/claude/event_test.go`
- Create: `internal/claude/preview.go`, `internal/claude/preview_test.go`

- [x] **Step 1: 做自由文本 CLI 实证**：用真实 CLI 触发一次 `AskUserQuestion`，在 `UpdatedInput["answers"]` 中回传一个**不属于任何 `options[].label`** 的字符串，观察 CLI 是否接受、是否报 schema 错误。结论（通过/不通过）写入 `docs/testing/B2-result.md`；不通过则本阶段全程只实现 `choice`，并保持 `AskUserTextEnabled=false`；通过也必须显式开启该配置，不能由实证自动改变默认行为。
- [x] **Step 2: 写内容块映射失败测试**：`ThinkingBlock` → `thinking`；`ToolUseBlock` → `tool_use`（含 `mcp__<server>__<tool>` 解析出 `toolClass=mcp` 与 `mcpServer`，`Task` 按普通内置工具输出，并携带可选白名单 `preview`）；`ToolResultBlock` → `tool_result` 只带 `toolUseId` 与 `isError`；维护 `toolUseID → toolName` 映射；未知 block 与未知 `SystemMessage` subtype 降级为 `unsupported` 且不透传原始 `Data`。断言 `ParentToolUseID != nil` 的子代理消息仍被丢弃。
- [x] **Step 3: 写预览抽取失败测试**：白名单每个工具各一例（`Bash`/`Read`/`Write`/`Edit`/`NotebookEdit`/`Glob`/`Grep`/`WebFetch`/`WebSearch`）；`Glob`/`Grep` 带 `path` 时拼为 `<pattern> @ <path>`；未命中白名单、字段缺失、字段非字符串三种情况都**不返回** `preview`；超长输入按 rune 边界截断并补 `…`，断言结果是合法 UTF-8。
- [x] **Step 4: 运行定向测试确认 RED**：执行 `GOFLAGS=-buildvcs=false go test ./internal/claude/ -run 'Block|Preview|Unsupported|Parent'`。
- [x] **Step 5: 实现映射与抽取**：实现结构化事件映射与 `preview.go`；截断上限统一来自 `ServiceConfig.ToolPreviewMaxRunes`（默认 200）而非散落常量；抽取只读 `input`，不修改也不缓存到日志。
- [x] **Step 6: 运行 GREEN**：执行 `gofmt -l internal` 与 `GOFLAGS=-buildvcs=false go test ./internal/claude/`。

### Task 3: 权限回调、Interaction Broker 与旁路通道

**Files:**
- Modify: `internal/claude/host.go`, `internal/claude/host_test.go`
- Modify: `internal/session/worker.go`, `internal/session/worker_test.go`
- Modify: `internal/session/controller.go`, `internal/session/controller_test.go`

- [x] **Step 1: 写权限回调失败测试**：MCP 前缀工具自动 Allow 且 `UpdatedInput` 非空、不产生 pending action；非 MCP 工具进入 Broker 等待决议；`allow_once` 不带 `UpdatedPermissions`；`allow_session` 只使用 SDK `Suggestions` 并把 `Destination` 设为 `session`，`Suggestions` 为空时该决议被拒绝；`deny` 带安全说明；`AskUserQuestion` 的 `tool_result.is_error=true` 被修正为 `false`；已有 pending action 时第二个并发回调直接 Deny 并发 `unsupported`，不覆盖当前 action。
- [x] **Step 2: 写旁路通道与死锁不变量失败测试**：pending action 阻塞期间分别执行「提交新输入」「`cancel_turn`」「`cancel_queued`」「关闭 Session」，断言均不死锁、不丢事件；断言决议下行通道只投递一次，重复决议得到 `action_not_pending`；测试必须带超时守卫，死锁表现为超时失败而非挂起。
- [x] **Step 3: 写取消顺序失败测试**：有 pending action 时先返回 `Deny{Interrupt:true}` 解阻塞再收尾且**不调用** SDK `Interrupt`（用假 Host/间谍断言调用次数为 0）；无 pending action 时才调用 `Interrupt`；`action_timeout` 使用 `Deny{Interrupt:false}` 且 Turn 继续；Session stop/fail、Turn fail/cancel 时未决 action 统一结束。
- [x] **Step 4: 运行定向测试确认 RED**：执行 `GOFLAGS=-buildvcs=false go test ./internal/claude/ ./internal/session/ -run 'Permission|Action|Cancel|Deadlock|Broker'`。
- [x] **Step 5: 实现 Broker 与旁路通道**：扩展 `Host` 接口暴露 action 上行与决议下行；action 状态与 `actionId` 生成只在 Controller 内完成，Turn 置 `waiting` 与 snapshot/事件原子更新；旁路通道发送侧同时 `select` 取消、停止与超时分支；同步更新所有 `Host` test double。
- [x] **Step 6: 实现排队取消与一致性**：按 `requestId` 从 FIFO 删除，维护 `queuedRequests` 与 `queueDepth` 一致并保留 `terminal`/`mobile` 来源与截断 `preview`。
- [x] **Step 7: 运行 GREEN 与全量 Go 回归**：执行 `gofmt -l internal cmd` 与 `GOFLAGS=-buildvcs=false go test ./...`，预期 A2/A3/A4 既有测试全部通过。

### Task 4: 假 Host 桩模式

**Files:**
- Create: `internal/claude/fakehost.go`, `internal/claude/fakehost_test.go`
- Modify: `cmd/claude-mobile-remote/config.go`, `cmd/claude-mobile-remote/config_test.go`
- Modify: `cmd/claude-mobile-remote/main.go`, `cmd/claude-mobile-remote/run_test.go`

- [x] **Step 1: 写桩行为失败测试**：假 Host 实现 `Connect`/`RunTurn`/`Close`，按脚本依次产出 Thinking、`tool_use`、工具审批 action（含白名单 `preview`）、`tool_result`、`AskUserQuestion`（多问题、单选 + 多选）、文本回复、`ResultMessage`，并可脚本化触发 Session 失败与停止；断言其不发起任何子进程、不读取用户 Claude 配置。
- [x] **Step 2: 写 CLI 装配失败测试**：`--fake-host <scenario>` 解析、未知场景名报错、默认空值仍返回真实 Host 配置；断言桩模式不改变监听、配对与 Origin 校验行为。
- [x] **Step 3: 运行定向测试确认 RED**：执行 `GOFLAGS=-buildvcs=false go test ./internal/claude/ ./cmd/claude-mobile-remote/ -run 'FakeHost|Config'`。
- [x] **Step 4: 实现桩与开关**：在 Host 构造点按 flag 选择实现；桩场景数据为固定假值（不含真实路径、token 或用户内容），Session ID 使用明显的测试用 UUID。
- [x] **Step 5: 运行 GREEN**：执行 `GOFLAGS=-buildvcs=false go build ./... && GOFLAGS=-buildvcs=false go test ./internal/claude/ ./cmd/...`。

### Task 5: Channel 命令面与 Transport v2

**Files:**
- Modify: `internal/channel/channel.go`, `internal/channel/registry.go`, `internal/channel/channel_test.go`
- Modify: `internal/transport/connection.go`, `internal/transport/server.go`, `internal/transport/server_test.go`
- Modify: `cmd/channel-test-client/main.go`, `cmd/channel-test-client/main_test.go`

- [x] **Step 1: 写 Channel 命令面失败测试**：`Controller` 接口在 `Submit` 之外新增 action 决议、`cancel_turn`、`cancel_queued`；Channel 暴露对应方法并按 `sessionId` 隔离，跨 Session 命令被拒绝；所有命令都经 Controller 单一入口，Channel 不直接接触 SDK。
- [x] **Step 2: 写 Transport 路由与顺序失败测试**：三个新命令的路由与 `command_result`；`command_result` 仍先于同 `requestId` 的 `turn_started`；结构化事件按 `seq` 单调递增；保持单 reader/单 writer、有界队列与订阅 snapshot 边界；旧版本客户端被拒绝且响应不含 Session 数据；未知字段、错误类型、超长 payload、错关联字段一律拒绝。
- [x] **Step 3: 运行定向测试确认 RED**：执行 `GOFLAGS=-buildvcs=false go test ./internal/channel/ ./internal/transport/ -run 'Command|Action|Cancel|Version|Order'`。
- [x] **Step 4: 实现命令面与路由**：扩展接口与连接处理；Transport 不持有 SDK callback、不保存历史、不实现 action 所有权。
- [x] **Step 5: 补测试客户端**：让 `cmd/channel-test-client` 能发送审批决议与两类取消，作为联调兜底手段。
- [x] **Step 6: 运行 GREEN 与 A4 回归**：执行 `gofmt -l internal cmd` 与 `GOFLAGS=-buildvcs=false go test ./...`，覆盖连接、鉴权、订阅、重连、队列溢出和多客户端竞态。

### Task 6: Mobile Web 协议与 reducer 升级

**Files:**
- Modify: `apps/mobile/src/protocol/types.ts`
- Modify: `apps/mobile/src/protocol/decode.ts`, `apps/mobile/src/protocol/decode.test.ts`
- Modify: `apps/mobile/src/protocol/client.ts`, `apps/mobile/src/protocol/client.test.ts`
- Modify: `apps/mobile/src/session/reducer.ts`, `apps/mobile/src/session/reducer.test.ts`

- [x] **Step 1: 写解码失败测试**：`PROTOCOL_VERSION` 为 2；版本不匹配走既有严格拒绝并能读出 `expectedVersion`；三类 `content_block` 与 `pendingAction`（含可缺省 `preview`、`questions[].inputType`）解码；`tool_use.preview` 可选且只作为字符串摘要；`queuedRequests` 解码；未知 `kind`/多余字段按既有策略拒绝。
- [x] **Step 2: 写 reducer 失败测试**：归并结构化事件顺序、`waiting` snapshot、pending action 出现与清除、排队列表、action 决议在途禁用、竞态拒绝（`action_not_pending` 显示「已由其他客户端处理」）、`session_failed`/`session_stopped` 终态禁用输入、重连从 snapshot 恢复 action 与队列且不伪造历史。
- [x] **Step 3: 运行定向测试确认 RED**：执行 `cd apps/mobile && npx vitest run src/protocol/decode.test.ts src/protocol/client.test.ts src/session/reducer.test.ts`。
- [x] **Step 4: 实现类型、解码与 reducer**：类型逐字段对齐 Go 协议，不复制出第二套定义；保持内存状态边界，不使用 `localStorage`/IndexedDB。
- [x] **Step 5: 运行 GREEN 与类型检查**：执行 `cd apps/mobile && npx vitest run src/protocol src/session && npm run build`。

### Task 7: Mobile Web 结构化 UI 与控制交互

**Files:**
- Create: `apps/mobile/src/components/StructuredBlock.tsx`, `StructuredBlock.test.tsx`
- Create: `apps/mobile/src/components/PendingActionCard.tsx`, `PendingActionCard.test.tsx`
- Create: `apps/mobile/src/components/QueuedRequestList.tsx`, `QueuedRequestList.test.tsx`
- Modify: `apps/mobile/src/components/MessageFeed.tsx`, `MessageFeed.test.tsx`
- Modify: `apps/mobile/src/components/SessionStatus.tsx`, `SessionStatus.test.tsx`
- Modify: `apps/mobile/src/App.tsx`, `App.test.tsx`

- [x] **Step 1: 写结构化内容块失败测试**：Thinking、Tool、MCP、`Task` 都是默认折叠的摘要行；断言文本与 block 按事件顺序渲染；工具调用与结果合并为一条，标题显示安全 `preview`（无 preview 时回退工具名）；连续工具归入一个默认折叠面板；断言**不渲染**原始输入参数、文件详情和完整结果；`Task` 不出现任务卡片、状态、耗时或聚合数量；工具结果只显示成功/失败，`AskUserQuestion` 的结果不显示为错误；替换 B1 的 `StructuredBlockPlaceholder`。
- [x] **Step 2: 写审批卡片失败测试**：底部卡片显示工具名；有 `preview` 时额外显示单行预览，无 `preview` 时只显示工具名且不自行拼接；`allowSessionAvailable` 为假时不渲染「本会话允许」；提交后按钮立即禁用直到 `command_result`。
- [x] **Step 3: 写问答卡片失败测试**：多问题、`header` 小标题 + `question` 正文两行、单选 Radio、多选 Checkbox、必答校验、非法选项拒绝；文本输入框只在 Task 2 实证通过时存在（未通过则断言不渲染输入框）。
- [x] **Step 4: 写取消与终态失败测试**：运行中 Turn 显示取消按钮；排队列表按 `queuedRequests` 渲染预览与逐项取消；`waiting` 期间禁用文本输入与新 Turn；`session_failed`/`session_stopped` 显示终态横幅并禁用输入且保留已收内容。
- [x] **Step 5: 运行定向测试确认 RED**：执行 `cd apps/mobile && npx vitest run src/components/StructuredBlock.test.tsx src/components/PendingActionCard.test.tsx src/components/QueuedRequestList.test.tsx src/components/MessageFeed.test.tsx src/components/SessionStatus.test.tsx src/App.test.tsx`。
- [x] **Step 6: 实现 UI**：按 `docs/specs/images/session_detail.png` 组织 Thinking 下的工具活动、按 `session_approve.png` 实现底部审批/问答面板；只使用现有 antd-mobile 组件（Card、Collapse、Radio、Checkbox、Button、Dialog、Popup、Toast），不引入新 UI 库；不展示 Pair Token 或真实敏感内容。
- [x] **Step 7: 运行 GREEN 与前端门禁**：执行 `cd apps/mobile && npm test && npm run lint && npm run build`。

### Task 8: 门禁、人工验收与交接

**Files:**
- Create: `docs/testing/B2-result.md`
- Modify: `HANDOFF.md`
- Modify: `docs/specs/2026-08-26-B2-interaction-control-design.md` only to close the free-text conditional clauses

- [x] **Step 1: 运行完整自动门禁**：执行 `gofmt -l internal cmd`（应无输出）、`GOFLAGS=-buildvcs=false go vet ./...`、`GOFLAGS=-buildvcs=false go test ./...`，以及 `cd apps/mobile && npm ci && npm test && npm run lint && npm run build`；记录测试数量与结果。
- [ ] **Step 2: 做人工浏览器验收**：用 `--fake-host` 场景走通默认折叠、MCP 无审批、`Task` 只作工具行、审批三决议与 `preview` 有/无两种形态、AskUserQuestion 多问题单选/多选、当前 Turn 与排队取消、pending action 期间取消不卡死、Session error/finish 终态、双客户端 action 竞态、重连 snapshot 恢复、B1 客户端被拒绝并提示升级。
- [ ] **Step 3: 复测真实 Claude**：在有额度时用真实 Host 复测一轮审批放行/拒绝后 Turn 继续，以及一次真实工具调用（项目此前从未跑通，`host.go` 一直硬编码拒绝）。
- [x] **Step 4: 关闭自由文本条件条款**：按 Task 2 Step 1 的实证结论，把设计中「实证通过后生效」的自由文本条款改为已启用或明确后置，并同步 `docs/testing/B2-result.md`。
- [x] **Step 5: 检查敏感信息与产物**：执行 `git diff --check`；确认无真实 Pair Token、`sessionId`、会话内容、工具原始 `input`、预览文本、命令、构建产物或本机路径；确认 `go.mod`/`go.sum` 未入库。
- [x] **Step 6: 更新记录与交接**：创建 `docs/testing/B2-result.md` 门禁记录，明确系统推送、原生能力、真机和完整历史未在 B2 验证；更新 `HANDOFF.md` 后交用户验收；不执行 Git add/commit/push，除非用户另行明确授权。

## B2 门禁结论

- 必须通过：`gofmt` 无差异、`go vet`、Go 全量测试（含旁路通道死锁不变量与取消顺序）、Mobile Web test/lint/build、协议 v2 严格校验、预览白名单抽取、action 竞态与重连 snapshot。
- 必须人工通过：参考图布局下的默认折叠、工具/MCP/`Task` 精简显示、审批三决议与 `preview` 两种形态、AskUserQuestion 多问题、取消（含 pending action 期间）、Session error/finish、排队列表、双客户端竞态。
- B2 不宣称：Task 进度聚合与任务卡片、工具输入完整下发、工具结果摘要、系统通知、后台推送、推送 Token、Capacitor、扫码、安全存储、VPN 真机、HarmonyOS 真机、完整历史和公网 Relay 已通过。
- 自由文本是否纳入 B2 由 Task 2 Step 1 的 CLI 实证结论决定，并在 `docs/testing/B2-result.md` 中明确记录。
