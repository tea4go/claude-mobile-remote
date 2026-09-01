# D2 HarmonyOS NEXT 与 Android 功能拉齐实施计划

> 执行方式：先补齐列表与导航，再补齐详情交互和消息流，最后用主工作区 `devecocli build --modules entry@default` 复验。Git 写操作仍需用户单独授权。

**Goal:** 让 `apps/harmony` 具备当前 Android 壳所承载的 `apps/mobile` 业务能力，包括会话列表、完整待处理事项交互、取消能力、答案回显和更完整的消息展示。

**Architecture:** 保留 D1 已打通的 ArkTS 协议客户端；在 Harmony 侧补一层页面内存 Session 列表与更接近 Android 的会话 reducer/UI，不复用 React/Capacitor 运行时。

**Tech Stack:** HarmonyOS NEXT Stage、ArkTS、ArkUI、`devecocli`。

## 文件与边界

- Modify: `apps/harmony/entry/src/main/ets/session/{model,reducer,store}.ets`
- Create: `apps/harmony/entry/src/main/ets/session/records.ets`
- Modify: `apps/harmony/entry/src/main/ets/pages/{AppShell,PairingPage,SessionPage}.ets`
- Create: `apps/harmony/entry/src/main/ets/pages/SessionListPage.ets`
- Modify: `apps/harmony/entry/src/main/ets/components/{MessageList,PendingSummary,QueuedSummary,StatusBar}.ets`
- Create: `apps/harmony/entry/src/main/ets/components/MessageComposer.ets`
- Modify: `HANDOFF.md`

## Task 1: 补齐会话列表与页面导航

- [ ] 增加页面内存 Session 记录结构，支持新增、重命名、删除、最近连接时间与本地状态。
- [ ] AppShell 改成 `list / pairing / detail` 三视图，行为对齐 Android：无记录直达配对页，有记录先显示列表，返回列表时关闭当前连接。
- [ ] 支持从已有记录重新进入详情，并保持单连接。

## Task 2: 补齐会话状态与命令门控

- [ ] 状态模型补齐消息流、排队列表、待处理事项、单在途命令、最后通知和错误态。
- [ ] 保持 `send_text / resolve_action / cancel_turn / cancel_queued` 共用单在途门控。
- [ ] `waiting/pendingAction` 期间禁止发送文本，但允许取消。

## Task 3: 补齐详情页交互

- [ ] 实现 `tool_approval` 三决议。
- [ ] 实现 `ask_user_question` 的单选、多选、文本输入。
- [ ] 实现“关闭事项 = 取消当前 Turn”。
- [ ] 实现当前 Turn 取消与排队请求取消。

## Task 4: 补齐消息展示

- [ ] 输入消息显示来源与 `accepted/unknown` 状态。
- [ ] Claude 文本保留换行，代码块与普通段落区分展示。
- [ ] 展示 `thinking` / `tool_use` / `tool_result` 摘要。
- [ ] 展示 `action_resolved.answers` 回显。
- [ ] 展示 session/turn 终态与 unsupported notice。

## Task 5: 主工作区验证与交接

- [ ] 在主工作区执行 `devecocli build --modules entry@default`，必须成功。
- [ ] 更新 `HANDOFF.md`，把“与 Android 仍有功能差距”改成实际状态。
- [ ] 若代码层功能已拉齐，再单独向用户申请本次小需求的 `commit` 与 `push` 授权。
