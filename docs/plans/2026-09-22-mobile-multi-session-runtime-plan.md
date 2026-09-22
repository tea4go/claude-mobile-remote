# 移动端 App 级多 Session 同步实施计划

> 状态：自动化实施与验证已完成，待双 Session 人工验收。Git 写操作仍需用户单独授权。

**Goal:** 用独立 Session Runtime Manager + Zustand App Store 实现 App 级多连接与按 `sessionId` 分桶的内存状态，不修改服务端和协议。

## Task 0：依赖与 Store 基线

- 在 `apps/mobile` 增加固定兼容版本的 Zustand 运行时依赖并同步 lockfile。
- 增加 Store 测试，固定配对记录、Session 状态、未读和导航的初始结构。
- 增加 selector 隔离测试：更新 A 不通知只订阅 B 的消费者。
- 禁止引入或配置 Zustand persist middleware。

## Task 1：先固定回归测试

- 修改 `apps/mobile/src/App.test.tsx`：把“返回列表关闭连接/切换只保留一个连接”旧断言替换为多连接生命周期断言。
- 增加 A/B 同时连接、列表期间收事件、查看 A 时 B 完成、删除单个 Session、重新配对 generation 隔离和 App 卸载关闭全部连接测试。
- 修改 `apps/mobile/src/session/reducer.test.ts`：增加终态 Snapshot 清除旧 streaming/停止状态且保留正文测试。

## Task 2：实现 Zustand Store 与 App 级 Session Runtime

- 新建 Zustand Store，管理配对记录、`sessionId → MobileState`、未读、当前视图与当前 Session。
- 新建职责单一的 Runtime Manager，只管理 `sessionId → ProtocolClient`、连接 generation 和网络副作用。
- Runtime 复用现有 reducer 和协议客户端；所有 action 显式路由到目标 `sessionId`。
- 加入 client generation，忽略已替换或已删除 Client 的延迟回调。
- 不使用 Zustand persist，不引入数据库或协议字段。

## Task 3：改造 App 与列表/详情

- `App.tsx` 不再以 `activePairing` effect 持有唯一 Client；导航状态进入 Zustand，App 只负责根级生命周期接线。
- 配对成功确保对应 Runtime 启动；返回列表不关闭连接；删除、失效和卸载按 Session 清理。
- 详情从 Runtime 读取当前 Session 状态，命令发送到对应 Client。
- 列表增加从 Runtime 派生的运行、等待、完成、错误和未读提示；不增加机器分组。

## Task 4：Snapshot 终态收敛

- 修正 reducer：终态 Snapshot 更新同 request ID 的流式消息状态并保留已有正文。
- Snapshot 无活动 Turn 时清除旧连接遗留的可停止推导状态。
- 保持无 replay、无完整历史边界。

## Task 5：验证与交接

- 运行 Zustand Store/Runtime 定向测试、全量 Vitest、lint、TypeScript/Vite build。
- 运行 `git diff --check` 并检查无 Pair Token、真实会话内容或构建产物。
- 人工验收两个 WhaleTerm Claude Session 同时运行：列表期间同步、跨详情同步、终态清除停止、权限/审批/取消隔离。
- 验证完成后更新 `HANDOFF.md`；若达到重要联调门禁，再更新 `docs/testing/`。
