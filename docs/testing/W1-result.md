# W1 WhaleTerm 进程内远程托管验证记录

- 日期：2026-09-18
- 状态：代码与自动化门禁通过，真实会话人工验收待完成

## 已完成实现

- WhaleTerm 使用本项目公共 `protocol` / `remote` 包，不导入 `internal/`；
- 远程输入进入 WhaleTerm 现有 Claude Turn FIFO；
- Snapshot/事件、工具审批、AskUserQuestion、当前 Turn 取消和排队取消映射到现有 Manager；
- 一个进程只允许一个 Session 开启 Remote Server；重复开启复用，跨 Session 开启被拒绝；
- 关闭远程管理只关闭 Server 与事件桥，不停止 Claude Client/Session；
- 聊天页提供“远程管理”入口、可达 WebSocket 地址输入和 Pairing Fragment 复制。
- 首轮前本地 Claude 草稿也显示远程入口；开启时由 WhaleTerm 预分配稳定 UUID，返回 `starting/idle` Snapshot，PC 或手机均可发起首个 Turn；
- Remote Host 保存开启时的完整 Claude 配置，远程首条消息只替换 Session/Turn/输入字段，不再使用稀疏配置创建 Client；
- 首轮 SDK 选项已从 `WithExtraArgs({"session-id": ...})` 迁移到正式 `WithSessionID`。

## 自动化证据

- WhaleTerm：W1 Host、正式 SessionID option 与 Remote Service 定向测试通过；
- WhaleTerm：`go test -race -vet=off ./backend/chat -run 'TestClaudeRemote|TestClaudeNewSessionIDArg|TestBuildSDKOptionsUsesFormalSessionIDOption'` 与 `go test -race -vet=off ./backend/remote` 通过；
- WhaleTerm 前端：`npm run build` 通过；输出仅含仓库既有 CSS `:deep()`、动态导入和大 chunk warning；
- 本项目：`GOFLAGS=-buildvcs=false go test -race -count=1 ./protocol ./remote` 通过；
- 本项目：`GOFLAGS=-buildvcs=false go vet ./protocol ./remote` 通过。

WhaleTerm `backend/chat` 的默认 vet 仍会命中与 W1 无关的既有 Go 1.25 格式化告警；本轮没有把定向 `-vet=off` 表述为全仓 vet 通过。

## 用户人工验收清单

1. 在 WhaleTerm 新建一个本地 Claude 会话，先不要发送消息，直接点击“远程管理”；
2. 连接 Mobile Web 后从手机发送第一条消息，确认 WhaleTerm 开始输出，且配对 ID、WhaleTerm Session ID、Claude init/result ID 一致；
3. 观察只创建一个 Claude SDK Client；随后再在 Claude 正在输出或等待审批时验证远程管理；
4. 输入手机可访问的地址，例如 `ws://<PC-VPN或局域网IP>:7331/ws`；
5. 复制 Pairing Fragment，在现有 Mobile Web 粘贴连接；
6. 确认手机先收到当前状态，再能收到新增文本；
7. 从手机发送一条消息，确认它与 PC 输入按同一队列顺序执行；
8. 至少验证一次工具审批；如任务触发 AskUserQuestion，再验证回答；
9. 验证取消当前 Turn 或取消一条排队输入；
10. 点击“关闭远程管理”，确认 WhaleTerm 中 Claude Session 仍可继续对话；
11. 再次开启并使用新配对内容连接，观察全流程没有启动第二个 Claude SDK Client。

开发态 WhaleTerm `go.mod` 当前通过本地 `replace` 使用含正式 `WithSessionID` 的 SDK 源码；`go.mod.base` 仍固定 v0.1.1。发布/提交前需先发布含该 API 的 SDK 版本，再更新 WhaleTerm 依赖基准并移除本地 replace。

配对内容含短期凭证，不要粘贴到日志、Issue、截图或提交记录。验收只需反馈“通过”，或说明失败步骤与界面错误。
