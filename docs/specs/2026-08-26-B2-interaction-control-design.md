# B2 结构化会话控制设计

- 日期：2026-08-26
- 状态：已确认（用户 2026-08-27 复核并确认 Task 降级、自由文本实证、`preview` 白名单、删除耗时/摘要、假 Host 桩模式）
- 前置阶段：B1 Mobile Web 已完成并通过门禁；A3 Session Core、A4 Channel/Transport 已通过
- 关联决策：D7、D13、D15、D16、D19、D25、D27、D40、D41、D42、D46
- 实证来源：`claude-agent-sdk-go` 源码（本地 replace）与 WhaleTerm 已验证用法（AGENTS.md 参考边界内，只提取 SDK 用法，不迁移其架构）
- 配置约定：`ServiceConfig.ToolPreviewMaxRunes` 默认 `200`；`ServiceConfig.AskUserTextEnabled` 默认 `false`，自由文本 CLI 实证已通过（见 §3.1 与 `docs/testing/B2-result.md`），作为可选开关默认仍关闭。

## 1. 目标

B2 在 B1 文本和 UI 基础上补齐一次 Claude Turn 内的结构化内容和任务控制：

1. 展示 Claude SDK 提供的 Thinking、工具调用和工具结果；MCP 工具与 `Task` 工具都按普通工具呈现；
2. 将工具权限请求转成手机可处理的审批卡片；
3. 支持 Claude `AskUserQuestion` 的单选与多选回答，含一次返回多个问题；自由文本回答的 CLI 实证已通过（见 §3.1），作为可选开关默认关闭；
4. 支持取消当前 Turn，以及取消尚未开始的排队输入；
5. 在移动端呈现 Session 级错误与结束终态（事件沿用 A3/A4，B2 只补 UI）；
6. 保持 PC 为唯一 Session Host，所有操作仍经过 Session Controller 的单一队列。

B2 不做：PC 端 TUI、多 Session daemon、Task 进度聚合与子任务日志、工具输入完整下发、原生扫码/安全存储/VPN/生命周期、系统通知与后台推送、完整历史同步。

## 2. 核心原则

- SDK 是唯一 Claude 事件和权限来源；项目只做适配、状态归并和传输，不解析 JSONL 替代 SDK；
- 一个 Session 同时最多运行一个 Turn；一个运行中的 Turn 同时最多存在一个待处理交互 action，第二个并发权限回调直接拒绝（见 §3.1）；
- 所有已认证客户端默认拥有同等操作能力。多个手机可以发送消息或处理 action，但同一 action 只接受第一个有效决议；不增加设备所有权和抢占协议；
- `command_result.accepted` 只表示命令已被 Controller 接受，不表示 Turn 已完成；Turn 结果和 Session 终态均由事件表达；
- pending action 不写磁盘、不进入日志、不进入完整历史；重连只从 snapshot 恢复当前待处理 action；
- 下发给客户端的字段按需最小化：移动端不展示的内容不进协议（见 §4.2）；
- 不支持旧客户端与 B2 新服务的混合协议连接，使用新的协议版本明确拒绝不兼容客户端，避免为兼容层增加复杂状态。

## 3. 数据流和边界

```text
Claude SDK
   │ AssistantMessage: text / thinking / tool_use / tool_result
   │ CanUseTool(toolName, input, permCtx.Suggestions)
   ▼
Claude Adapter ──> Session Controller / Interaction Broker
                              │
                  snapshot + structured events
                              ▼
                    Channel / WebSocket
                              ▲
                 action / cancel commands
                              │
                         Mobile Web
```

### 3.1 SDK Adapter

**内容块映射**：`AssistantMessage.Content` 中的 `TextBlock` 延续现有文本事件；`ThinkingBlock`、`ToolUseBlock`、`ToolResultBlock` 映射为 §4.2 的结构化内容事件。Adapter 维护 `toolUseId → toolName` 映射，供工具结果关联与 `is_error` 修正使用。

**Task 工具按普通工具处理**：SDK 未提供任何 Task 类 `SystemMessage` subtype（全包仅 `SubtypeInitialize`），Claude Code 的 Task 本质是一个工具：它以 `ToolUseBlock{Name:"Task"}` 出现，子代理消息通过 `ParentToolUseID` 关联。B2 不做任务聚合，也不汇总状态、耗时或子任务数量；`Task` 与其他内置工具走同一条 `tool_use` / `tool_result` 路径。A2 在 `internal/claude/messages.go` 中丢弃 `ParentToolUseID != nil` 子代理消息的过滤**保持不动**，因此子代理内部过程不会下发到客户端。

**权限链路**：SDK 的 `CanUseTool` 回调是唯一权限入口。回调不能直接操作 WebSocket，而是把工具名、审批预览和 `Suggestions` 是否非空交给 Interaction Broker，并阻塞等待决议；决议结果必须先回到 Controller，再由 Adapter 返回 `PermissionResult`。

**回调的既有运行约束**（源自 SDK 实现，设计必须适配而非假设）：

- 回调在 SDK 的单个 stdout 读循环内**同步**调用。回调阻塞期间，该 Session 的全部 SDK 消息读取暂停，PC 本地同样收不到新消息；这是可接受代价，但超时上限必须有界（§7）；
- **pending action 期间不得调用 SDK `Interrupt`**：`Interrupt` 是控制请求且等待控制响应，而该响应只能由已被阻塞的同一读循环投递，必然超时失败。取消统一走 §3.3 的两步顺序；
- 回调收到的 `ctx` 是传输层上下文，不是 Turn 上下文。Turn 级取消必须由 Adapter 自己在等待决议时 `select`，不得依赖 `ctx.Done()` 传导；
- SDK 文档声明回调可能被并发调用。若在已有 pending action 期间进入第二个回调，直接返回 Deny 并发出一条 `unsupported` 安全事件，不排队、不覆盖当前 action；不使用单槽覆盖式的等待表；
- **回调不在 `hostWorker` goroutine 上**：现有 `Host.RunTurn(ctx, prompt, emit)` 的 `emit` 只在 worker goroutine 内被调用，而权限回调运行在 SDK 读循环 goroutine。因此 pending action 的上行与决议的下行必须走**不经过被阻塞读循环的旁路通道**（§3.4）。

**MCP 工具自动放行**：工具名以 `mcp__` 前缀标识 MCP 外部工具，命名格式为 `mcp__<server>__<tool>`（CLI 内部生成的标准格式）。按照 WhaleTerm 已验证的权限策略，MCP 工具由用户主动配置，Adapter 直接放行并不生成 pending action；前缀匹配失败时按普通工具走审批链路。该策略属于已确认的宿主权限策略，B2 不在移动端重复增加 MCP 审批。

**AskUserQuestion**：与工具权限共用同一条 `CanUseTool` 链路，Adapter 按工具名 `AskUserQuestion` 识别，把 `input.questions` 规范化为 §4.3 的问题列表。回答通过 `PermissionResultAllow.UpdatedInput` 回传，形状为「原始 `input` 加上 `answers` 键」。Adapter 负责协议数组到 CLI 扁平 map 的转换，规则固定：

- map 的 key 依次取 `question`、`header`、`question_<序号>`（序号从 1 开始）；
- 单选的值为所选 `option.label`；
- 多选的值为所选各 `option.label` 按 `", "` 拼接成的**单个字符串**，不是数组。

**自由文本实证已通过（Task 2 Step 1）**：单选与多选已由 WhaleTerm 实证，但其 `buildAskUserAnswerInput` 的值**全部取自 `options[].label`，没有任何自由文本分支**，也不存在 `inputType` 概念。实施阶段已用真实 CLI 实证一次：`answers` 回传一个**不属于任何 `options[].label`** 的字符串被 CLI 接受、无 schema 错误、且确实送达模型（证据见 `docs/testing/B2-result.md`）。因此本设计中所有自由文本条款（`inputType: text`、`options` 可缺、文本类答案回传）在协议与服务端校验中**已启用**；但作为可选开关，服务端只有在 `ServiceConfig.AskUserTextEnabled=true` 时才生成和接受 `inputType: text`，**默认仍关闭**（B2 默认只启用单选/多选）。

- 已实证结论：自由文本的值为用户输入的单个去空白后非空字符串，自由文本问题不要求存在 `options`；
- 默认关闭时：`inputType` 固定为 `choice`，服务端对文本类问题降级为 `unsupported` 安全事件，`questions[].inputType` 保留在协议中作为已验证的可选扩展位。

实证结果已写入 `docs/testing/B2-result.md`。

**审批预览（工具白名单抽取）**：服务端**不下发工具原始 `input` 对象**，而是按工具名从 `input` 中抽取单个字符串 `preview`（§4.3）。映射表固定：

| 工具名 | `preview` 来源 |
| --- | --- |
| `Bash` | `input.command` |
| `Read` / `Write` / `Edit` / `NotebookEdit` | `input.file_path` |
| `Glob` / `Grep` | `input.pattern`；`input.path` 非空时拼为 `<pattern> @ <path>` |
| `WebFetch` | `input.url` |
| `WebSearch` | `input.query` |

规则：

- 工具名未命中白名单、字段缺失或不是字符串时，**不下发 `preview`**，移动端只展示工具名；不做通用序列化兼容；
- `preview` 统一按 PC Service 配置的上限截断（默认 200 字符），超长时末尾补 `…`；截断按 rune 边界，不得产生无效 UTF-8；
- 抽取只在服务端进行，抽取后的字符串不回写 SDK；回答 SDK 时仍按下方「权限结果构造」使用未修改的原始 `input`；
- MCP 工具自动放行、不产生 pending action，因此不适用白名单。

**权限结果构造**：

- `UpdatedInput` 必须是非空 record（CLI 的 schema 要求，不能省略）。手机不编辑工具输入，因此 `tool_approval` 一律回传服务端保留的原始 `input`；
- `allow_once`：Allow 且不带 `UpdatedPermissions`；
- `allow_session`：Allow 且把 `permCtx.Suggestions` 每条的 `Destination` 设为 `session` 后作为 `UpdatedPermissions` 回传。只使用 SDK 给出的 Suggestions，禁止自行拼接权限规则；`Suggestions` 为空时该选项不可用；
- `deny`：Deny 并带安全的拒绝说明。

**结果修正**：`AskUserQuestion` 的用户回答会以 `is_error: true` 的 `ToolResultBlock` 返回，这并非真实错误。Adapter 按 `toolUseId → toolName` 判定为交互类工具后，把 `isError` 修正为 `false`，避免手机把正常问答显示成失败。

**降级**：SDK 不提供或尚未验证的结构化消息类型（含未知 `SystemMessage` subtype）只作为 `unsupported` 的安全事件记录，不把原始私有对象序列化到客户端。

### 3.2 Session Controller

- Controller 继续是 Session 状态、Turn 状态和输入 FIFO 的唯一写入者；
- action 由 Controller 生成稳定的 `actionId`，与当前 `requestId` 绑定；不使用 SDK 内部 permission request ID，也不下发 `toolUseId`（权限回调不提供该字段）；
- action 出现时 Turn 进入 `waiting`，snapshot 和事件同时反映该 action；
- 处理 action 的结果必须回到 Controller，再由 Host/Adapter 完成 SDK 回调，禁止 Transport 直接唤醒 SDK；
- 排队输入取消只从 FIFO 删除目标 request，不影响当前 Turn 和其他排队项；
- action 超时、Session 停止、Turn 失败或取消时，所有未决 action 统一结束，后续决议返回稳定的 `action_not_pending`；
- Session 级失败与正常停止**沿用 A3/A4 既有的 `session_failed` / `session_stopped` 事件与 snapshot 字段**，B2 不新增服务端逻辑，只补移动端呈现（§6）。

### 3.3 取消的解阻塞顺序

`cancel_turn` 分两种情形，Controller 必须区分：

1. **存在 pending action**：先让阻塞中的权限回调返回 `PermissionResultDeny{Interrupt: true}`，读循环随之恢复；再按 Turn 取消收尾（置 `cancelled`、结束 action、发事件）。不额外调用 `Interrupt`；
2. **无 pending action**：按既有路径取消 Turn context 并调用 SDK `Interrupt`。

`action_timeout` 使用 `Deny{Interrupt: false}`：只拒绝本次工具，由 Claude 自行决定后续，Turn 不因超时被强制中断。

### 3.4 旁路通道与死锁不变量

权限回调跑在 SDK 读循环 goroutine，而现有 `internal/session/worker.go` 的 `requests` / `events` / `results` 均为**无缓冲通道**且 Controller 是 `events` 的唯一接收者。若直接在回调中向 `events` 发送，而 Controller 此刻正造访 `requests`，则回调、worker 和 Controller 三方互等死锁。因此：

- pending action 的上行和决议的下行使用**专用旁路通道**，不复用 `RunTurn` 的 `emit` 路径；
- 上行通道对回调侧不得无限阻塞：发送时同时 `select` Turn 取消、Session 停止和超时分支，任一命中则立即返回 Deny；
- 每个 action 的决议下行通道容量为 1 且只投递一次，重复决议在 Controller 侧就被 `action_not_pending` 拦住；
- Controller 侧的 action 处理与 Turn 事件消费处于同一 `select`，不得在持有 action 状态时同步等待 worker；
- 不变量必须有回归测试：在 pending action 阻塞期间提交新输入、取消 Turn、取消排队项和关闭 Session，均不得死锁或丢事件。

## 4. 共用协议变更

### 4.1 版本策略

当前 A4/B1 客户端对未知事件和未知命令采取严格拒绝。B2 将 envelope 的 `version` 升为 2，仍保留原有 envelope 外层字段：

```text
version / type / sessionId / commandId / requestId / seq / payload
```

版本 2 继续使用 JSON、字符串 ID、非负整数 `seq` 和对象 payload。服务端对**每条**消息校验版本（现有 `ValidateEnvelope` 行为），不匹配时返回 `unsupported_version` 且不返回任何 Session 数据。错误 payload 增加 `expectedVersion` 字段，客户端据此提示升级。B1 客户端必须升级到 B2 客户端后才能连接 B2 服务。

### 4.2 结构化内容事件

新增 `event` 的结构化内容类型，事件仍按同一 Session `seq` 单调递增，且只发送当前连接建立后的新增事件：

```json
{
  "kind": "content_block",
  "requestId": "turn-request-id",
  "block": {
    "id": "content-block-id",
    "kind": "thinking | tool_use | tool_result",
    "thinking": "...",
    "toolUseId": "tool-use-id",
    "toolClass": "builtin | mcp",
    "mcpServer": "filesystem",
    "toolName": "Read",
    "isError": false
  }
}
```

字段按 `kind` 严格约束：

- `thinking`：只允许 `id`、`kind`、`thinking`；
- `tool_use`：允许 `id`、`kind`、`toolUseId`、`toolClass`、`toolName`，`toolClass=mcp` 时可带 `mcpServer`，可带服务端白名单抽取的 `preview`；`Task` 工具与其他内置工具同构，不额外带任务字段；
- `tool_result`：允许 `id`、`kind`、`toolUseId`、`isError`；移动端不接收完整结果正文；
- 未知或不完整的 SDK block 不把任意原始对象发给客户端，而是转成 `type=unsupported` 的稳定安全事件；该事件只允许固定 `code`，不是第四种 `content_block.kind`，三端按安全提示处理并继续接收后续事件。

三处刻意的取舍：

- **`tool_use` 不下发原始 `input`**。移动端不展示未筛选的输入参数和文件详情；工具标题可复用同一白名单抽取的 `preview`，但只下发有限长度字符串，减少敏感数据下发面；
- **Tool 结果只下发关联 ID 与成败状态**。不下发结果正文，也不下发任何服务端生成的结果摘要（白名单以外无额外字段）；
- **不做 Task 聚合**。SDK 无 Task 类事件源（§3.1），`taskStatus` / `taskSummary` / 子任务数量 / 耗时只能由 Adapter 自建状态机推导，不属于 B2 范围；子代理内部消息沿用 A2 的 `ParentToolUseID` 过滤，不下发。

Session 终态继续使用现有领域事件，不另造推送专用事件：

- `session_failed`：携带安全的 `error.code` 和 `error.message`，`snapshot.sessionStatus=failed` 与 `snapshot.sessionError` 已由 A3 维护；
- `session_stopped`：表示 Session 已正常结束，`snapshot.sessionStatus=stopped`；
- `turn_completed` 只表示当前 Turn 完成，不能被客户端解释为 Session 结束；
- `turn_failed` 只表示当前 Turn 失败，Session 是否继续由随后 Session 状态决定。

在线手机通过现有 WebSocket 实时收到这些事件；断线后重新连接通过 snapshot 得到最终 Session 状态。B2 只实现事件与页面状态展示，不实现系统通知、后台推送、推送 token 或离线通知队列；后续消息推送阶段复用这些 Session 终态语义。

### 4.3 Pending action

`TurnSnapshot.pendingAction` 从当前空对象扩展为对象或 `null`：

```json
{
  "actionId": "action-id",
  "requestId": "turn-request-id",
  "kind": "tool_approval | ask_user_question",
  "toolName": "Bash",
  "preview": "go test ./...",
  "allowSessionAvailable": true,
  "questions": [
    {
      "questionId": "q1",
      "header": "运行模式",
      "question": "选择运行模式",
      "inputType": "choice",
      "multiSelect": false,
      "options": [
        {"label": "安全", "description": "只读检查"},
        {"label": "完整", "description": "允许修改"}
      ]
    }
  ]
}
```

约束：

- `tool_approval` 必须有 `toolName`；`preview` 按 §3.1 白名单抽取，命中时为已截断的单个字符串，未命中时整个字段缺省；`questions` 为空；工具原始 `input` 对象任何情况下都不下发；
- `ask_user_question` 必须有一个或多个问题，`toolName` 固定为 `AskUserQuestion`，不下发 `preview`（问题已由 `questions` 规范化表达）；自由文本问题标记为 `text` 类型且不要求 `options`（形态已经 §3.1 CLI 实证通过），但仅在 `ServiceConfig.AskUserTextEnabled=true` 时才会生成，默认关闭；
- `allowSessionAvailable` 由服务端根据 SDK `Suggestions` 是否非空计算，只下发布尔值；权限规则原文不下发给客户端；
- 不下发 `toolClass`（MCP 工具自动放行、不会进入审批）与 `toolUseId`（权限回调不提供），客户端只使用项目生成的 `actionId`；
- `header` 是问题的短标题、`question` 是问题正文，两者都可能缺省，服务端保证互相回填后非空；
- `inputType` 为 `choice` 或 `text`；`choice` 必须有非空 `options`，`text` 不需要 `options`；`inputType` 属于单个问题（`questions[]` 元素），不是 `pendingAction` 顶层字段；
- `pendingAction.preview` 与 §4.4 的 `queuedRequests[].preview` 同名但属不同对象，语义一致：都是服务端按上限截断的展示字符串；
- `preview`、问题文本和工具结果只在已认证连接中传输，不写日志、测试快照或文档；
- snapshot 只包含当前一个 pending action，不包含已完成 action 列表或历史。

### 4.4 排队项可见性

`SessionSnapshot` 在保留 `queueDepth` 的同时新增排队列表，使客户端有能力定位取消目标：

```json
{
  "queuedRequests": [
    {"requestId": "req-2", "source": "terminal | mobile", "preview": "重构登录流程"}
  ]
}
```

- `preview` 是服务端按固定上限截断的输入文本，用于列表展示，不是完整输入；
- 列表只含尚未开始的 FIFO 项，不含当前运行的 Turn；
- 该列表同时覆盖 PC 终端与其他手机提交的排队项，且重连后可从 snapshot 恢复，弥补「客户端只记得自己提交过什么」的缺口。

### 4.5 控制命令

新增命令：

```text
resolve_action
  tool_approval:      { actionId, decision: allow_once | allow_session | deny }
  ask_user_question:  { actionId, answers: [{ questionId, values: [string] }] }

cancel_turn
  { requestId }

cancel_queued
  { requestId }
```

规则：

- `resolve_action` 必须精确匹配当前 `actionId` 和 `requestId`；allow/deny 或完整 answers 通过校验后才进入 Controller；
- `allow_session` 只有在 `allowSessionAvailable` 为真时可用，否则移动端不展示该按钮、服务端也拒绝该决议；
- 单选要求恰好一个值，多选允许一个或多个值；选项型问题的值必须命中该问题的 `options[].label`；文本型问题（§3.1 实证通过后才存在）要求 `values` 恰好为 1 个去空白后非空的字符串；所有问题必须回答，禁止部分提交；
- `cancel_turn` 只允许取消当前运行或等待中的 request；`cancel_queued` 只允许取消 FIFO 中尚未开始的 request；
- action 决议、Turn 取消和排队取消均使用 `command_result` 返回接受/拒绝，拒绝使用稳定错误码；
- 同一 action 或 request 的竞态由 Controller 原子判定，后到命令返回 `action_not_pending`、`turn_not_cancellable` 或 `request_not_queued`；
- 不新增服务器端 action 所有权、设备锁或人工仲裁。

新增正式错误码常量（与现有 `Code*` 同一集合，不是建议值）：`invalid_action`、`action_not_pending`、`invalid_answer`、`turn_not_cancellable`、`request_not_queued`、`request_already_finished`、`action_timeout`。

## 5. 状态和时序

```text
Turn running
    ├─ content_block(thinking/tool_use/tool_result) ─> running
    ├─ pending action ─> waiting
    │      ├─ resolve_action ─> running
    │      ├─ cancel_turn ─> Deny{Interrupt:true} 解阻塞 ─> cancelled
    │      ├─ timeout ─> Deny{Interrupt:false} ─> running（由 Claude 决定后续）
    │      ├─ session stop / turn fail ─> action 统一结束
    │      └─ duplicate or late resolve ─> command rejected
    ├─ cancel_turn（无 pending action）─> Interrupt ─> cancelled
    └─ ResultMessage ─> completed/failed
```

- `turn_started` 仍先于同一 `requestId` 的内容事件；`command_result` 仍先于该请求的 `turn_started`；
- `waiting` 不是 Session 终态，Turn 可在 action 决议后继续运行；
- 工具调用和工具结果不结束 Turn；只有 ResultMessage、取消或失败才结束当前 Turn；
- Session 终态事件可以在 Turn 失败之后到达，客户端必须以 `sessionStatus` 为最终输入边界；`session_failed` / `session_stopped` 到达后显示终态横幅并禁用输入；
- pending action 生成后，Controller 先更新 snapshot/发布 waiting 事件，再允许客户端观察到后续状态；
- 重连取得含 pending action 的 snapshot 后，客户端可继续提交决议，不重放此前结构化内容；因此审批卡片必须自包含，不依赖上下文历史；
- 当前 action 只有一个，多个连续工具请求按 SDK 实际顺序逐个出现，避免移动端同时维护多个审批焦点。

## 6. Mobile Web 交互

- B1 的 Thinking/Tool 占位替换为默认折叠的结构化内容块；Thinking、内置 Tool（含 `Task`）和 MCP 工具都以轻量摘要行呈现，展开状态不跨刷新保存；
- 工具项采用精简渲染：展示动作标题、服务端白名单 `preview` 与状态，不展示原始输入参数和文件详情；MCP 只展示 MCP Server/Tool 标题和执行状态；连续工具项归入默认折叠的工具组；
- 工具结果只展示成功/失败状态，不在移动端展开完整结果；`AskUserQuestion` 的回答结果不显示为错误；
- 不做任务卡片：`Task` 只作为一个工具摘要行出现，不展示任务状态、耗时、聚合数量或子任务日志；`session_task.png` 的任务层级属于 B2 之后的能力；
- 结构化内容按 `docs/specs/images/session_detail.png` 的层级组织：Thinking 下可归组工具活动，工具项使用图标、标题和状态，均默认折叠；
- `tool_approval` 按 `docs/specs/images/session_approve.png` 的底部卡片/操作面板呈现：显示权限请求说明和工具名，服务端下发 `preview` 时额外展示该单行预览（未下发则只显示工具名，客户端不自行拼接预览），提供「本次允许」「本会话允许」（仅 `allowSessionAvailable` 为真时）和「拒绝」；决议提交后按钮立即禁用，等待 `command_result` 和后续 Turn 事件；
- `ask_user_question` 使用同一底部交互卡片风格，`header` 作小标题、`question` 作正文两行显示；单选用 Radio、多选用 Checkbox；文本输入框仅在 §3.1 的 CLI 实证通过后提供；未回答完所有问题不可提交；
- Session 错误和正常结束使用明确的终态横幅/状态卡片通知手机；错误显示安全错误信息，结束显示「会话已结束」；两者都保留当前页面已收到的内容，但禁用后续输入；
- waiting 状态只显示当前 action，文本输入和新的 Turn 命令按 Controller 当前规则禁用，避免在交互中再次制造并发 Turn；
- 运行中的 Turn 显示取消按钮；排队项按 snapshot 的 `queuedRequests` 列表展示预览与取消入口，取消只针对对应 request ID；
- 断线时保留当前页面已收到的内存内容；重连后用 snapshot 恢复状态、pending action 和排队列表，不伪造历史结构化消息；
- 多个手机同时在线时，任一手机都可处理 action；失去竞态的手机显示「已由其他客户端处理」。

不引入新的 UI 组件库或原生插件；优先继续使用 antd-mobile 的 Card、Collapse、Radio、Checkbox、Button、Dialog、Popup 和 Toast。结构化内容的精简规则属于移动端展示策略，不改变服务端事件字段。

## 7. 超时、断线和安全边界

- pending action 默认等待 10 分钟，超时按 §3.3 以 `Deny{Interrupt: false}` 结束等待；具体时长作为 PC Service 配置，不进协议负载。该上限同时限定 SDK 读循环被阻塞的最长时间；
- WebSocket 断开不取消 action，其他已认证客户端仍可处理；所有客户端断开后 action 仍按超时规则结束；
- Remote Server 停止、Session 失败或 Turn 取消立即结束 action；
- 工具原始 `input` 只留在 PC 进程内，仅用于回答 SDK；下发到客户端的只有白名单抽取并截断的 `preview` 字符串。`preview` 和 AskUserQuestion 内容可能包含路径、命令或业务数据，只对已认证连接发送，禁止写入日志和验证文档；
- 工具结果在移动端只展示状态；如后续需要查看详情，必须另行设计受限字段，不直接渲染 SDK 原始字符串/JSON、HTML、脚本、外部资源或客户端命令；
- 权限规则原文（SDK `PermissionUpdate`）不下发客户端，客户端只看到 `allowSessionAvailable` 布尔值；
- Session 错误只展示稳定错误码对应的安全文案，不显示 SDK 原始错误、命令行参数、凭证或本机路径；
- cancel 不能撤销已经执行的工具，只能请求中断当前 SDK Turn；UI 必须明确「取消请求」与「已取消」的区别；
- 所有 action 与 cancel 命令仍受 Pair Token、Session Channel、单连接有界队列和消息大小限制保护。

## 8. 三端共用契约核对

按硬约束 11 逐项核对，`结论` 列记录核对依据而非仅标注可行：

| 契约项 | React Web / Capacitor | HarmonyOS NEXT | 核对结论 |
| --- | --- | --- | --- |
| JSON envelope v2 | 标准 JSON 解码与 reducer | ArkTS JSON 解码与状态归并 | 仅新增整数版本值，无端特有依赖 |
| structured content（3 个 kind） | 普通对象、折叠块 | ArkUI 自定义内容卡片 | 字段全为字符串/布尔/整数，无 DOM 依赖 |
| `toolClass` / `mcpServer` | 标题前缀与图标 | 等价 ArkUI 摘要行 | 纯展示用字符串，解析规则在服务端完成 |
| tool approval（3 决议） | Button + action command | Button + action command | 决议为固定枚举字符串 |
| `allowSessionAvailable` | 控制按钮可见性 | 等价控件可见性 | 改为布尔后不再依赖 SDK 结构体，三端一致 |
| `pendingAction.preview` | 单行文本展示 | 等价 ArkUI 文本 | 服务端已抽取并截断的单个字符串，三端无需解析工具 schema |
| AskUserQuestion（单选/多选） | Radio / Checkbox | 等价 ArkUI 控件 | `answers` 为数组，CLI map 转换只在服务端 Adapter 内；文本输入为实证后的可选扩展，三端同时生效 |
| cancel turn / queued | request ID command | request ID command | 仅字符串 ID |
| `queuedRequests` | 列表渲染 + 取消入口 | 等价 ArkUI 列表 | 新增数组字段，元素为三个字符串，无端特有类型 |
| pending action snapshot | 内存对象，不持久化 | 等价易失对象 | 不依赖浏览器存储，符合硬约束 7 |
| Session error/finish | 终态横幅 + snapshot | 等价终态卡片 | 沿用 A4 既有字段，无新增契约 |

B2 不把 React、DOM、WebSocket 实例、Capacitor API 或 antd-mobile 组件写入协议和状态字段。协议只使用 JSON、字符串、布尔值、数组、对象和整数，满足三端共用契约约束。

## 9. 验收范围

### 自动化

- Adapter 能把 Thinking、工具调用（含 `Task` 与 MCP）和工具结果、权限回调映射为稳定领域对象；未知 block/subtype 安全降级为 `unsupported`；
- Adapter 对 `mcp__` 前缀工具自动放行、不生成 pending action；前缀解析失败时回落到审批链路；
- Adapter 的审批预览抽取覆盖白名单内每个工具、`Glob`/`Grep` 的 `path` 拼接、未命中工具不下发 `preview`、字段类型错误不下发，以及按 rune 边界的截断；
- Adapter 的 answers 转换覆盖 key 推导三级回退、单选取 label、多选 `", "` 拼接；自由文本分支仅在 §3.1 实证通过后补测试；
- Adapter 把交互类工具结果的 `is_error` 修正为非错误；
- Adapter 在已有 pending action 时对第二个并发回调返回 Deny 并发 `unsupported` 事件；
- 取消路径：有 pending action 时先 `Deny{Interrupt:true}` 再收尾、不调用 `Interrupt`；无 pending action 时走 `Interrupt`；超时用 `Deny{Interrupt:false}`；
- Controller 保证单 action、action 决议原子性、FIFO 排队取消、当前 Turn 取消和超时结算；
- 旁路通道不变量（§3.4）：pending action 阻塞期间提交新输入、取消 Turn、取消排队项和关闭 Session 均不死锁、不丢事件；
- Controller 的 `queuedRequests` 与 `queueDepth` 始终一致，`preview` 按上限截断；
- Transport 严格校验协议版本（含 `expectedVersion` 错误载荷）、结构化 block、pending action、answers 和 cancel 命令；
- Channel 命令面扩展后仍按 `sessionId` 隔离，`resolve_action` / `cancel_turn` / `cancel_queued` 都经 Controller 单一入口；
- 假 Host 桩能按脚本触发 Thinking、工具调用/结果、工具审批、AskUserQuestion、取消和 Session 终态，不依赖真实 Claude 额度；
- Web reducer 能归并结构化事件、waiting snapshot、排队列表、action 决议、取消和竞态拒绝；
- Go 与 Mobile Web 现有测试全部回归通过（Session 终态沿用 A3/A4，不重写既有测试）。

### 人工浏览器

- 使用假 Host 桩模式触发 Thinking、工具调用/结果、审批和 AskUserQuestion；必要时再用真实 Claude 复测一轮；
- 移动端默认折叠 Thinking、工具项和工具结果；工具/MCP/`Task` 只显示标题与状态，不显示输入参数、命令正文或完整结果，也不出现任务卡片；
- MCP 工具执行时不弹审批卡片，仅出现工具摘要行；
- 多问题覆盖单选、多选和缺少答案拒绝（自由文本仅在实证通过后补验）；`header` 与 `question` 两行正确显示；
- 审批卡片符合参考图层级，覆盖本次允许、可用时的本会话允许和拒绝；`allowSessionAvailable` 为假时不出现该按钮；
- 审批卡片在白名单工具（如 `Bash`）上显示单行 `preview`，在白名单外工具上只显示工具名；
- 手机审批后 Claude Turn 继续，拒绝后 Claude 得到 deny；
- AskUserQuestion 回答后不显示为错误结果；
- 当前 Turn 可取消（含 pending action 期间取消不卡死）；排队项按列表取消，取消后 FIFO 正确；
- 手机在线时能收到 Session error 和 finish 事件；刷新/重连后能从 snapshot 恢复 `failed` 或 `stopped` 终态；
- 两个已认证浏览器同时处理同一 action，只有一个成功；
- 刷新/重连可恢复 pending action 与排队列表，但不恢复完整历史；
- B1 版本客户端连接 B2 服务被拒绝且能提示升级；
- B2 不验证系统通知或后台推送；后续推送功能必须复用本设计的 Session 终态事件语义；
- 未出现真实 token、session 内容、工具原始 `input` 或 action 预览到日志、文档和提交物。

## 10. 后续关系

- B2 设计确认后先把 §3.1/§3.3/§3.4/§4 的协议与安全结论写入 `DECISIONS.md`，再编写实施计划，随后实施 Go Adapter/Session/Transport 与 Mobile Web；
- 后置到 B2 之后的能力：工具输入完整下发与展开、Task 进度聚合与任务卡片（`session_task.png`）、工具结果摘要、系统通知与后台推送；
- B3 在 B2 门禁通过后只做单 Session Remote Server 组件化，不重新设计 action 协议；
- C1 只负责 Capacitor 壳、Web 接入和局域网明文放行，待 Android SDK/JDK 就绪后实施；C2 依赖 B2 与 C1，负责扫码、安全存储、真机和生命周期。
