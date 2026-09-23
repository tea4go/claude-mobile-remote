# HANDOFF.md — Claude Mobile Remote 交接文档

> 新会话先读 `AGENTS.md`，再读本文件、`DECISIONS.md`、`REQUIREMENTS.md` 和总体设计。历史过程见 `docs/HANDOFF-archive.md`，阶段门禁见 `docs/testing/`。

## 当前状态

- 当前执行：移动端 App 级多 Session 同步与 Zustand 改造已完成自动化验证和双 Session 人工验收，W1 WhaleTerm 真实会话人工验收也已由用户确认通过，待提交本轮实现。Zustand 按 `sessionId` 分桶保存配对、会话状态、未读和导航；独立 `SessionRuntimeManager` 持有每个 Session 的 `ProtocolClient`、连接 generation 和命令路由。返回列表或切换详情不再关闭连接，删除单个记录只关闭对应连接，App 卸载关闭全部连接；Snapshot 终态会校正旧 streaming/停止状态并保留已收正文。未修改协议、Go Remote Server、机器分组、数据库/replay、系统后台推送或浏览器持久化策略。

- 最近完成：W1 WhaleTerm 真实会话人工验收已由用户确认通过；此前修复 Android WebView 中权限 SVG 顶部被裁剪的问题；`permission.svg` 扩展视口至完整路径范围，并移除外部 DTD/编辑器元数据。

- A1–A6、B1、B2、B3、C1、C2-A/C2-B、D2、CLI 产品化配置均已完成；C2-C/C2-D 暂缓。
- W1 移动端输入框 UI 已完成用户验收并提交：`ddaeb7d feat: refine mobile composer and permission controls`。
- 本次提交包含移动端输入框、附件/权限 Popover、停止按钮、权限模式共享契约及配套测试；未包含用户测试截图和无关脚本权限位变化。

## 最近验证

- 本轮移动端多 Session Runtime：新增固定依赖 Zustand 5.0.8（未使用 persist）、App Store/selector 隔离测试和多连接生命周期回归；移动端全量 Vitest **425/425**、`npm run build`、`npm run lint` 通过。lint 仅有既有 `scripts/cdp-console.mjs` unused-expression warning，build 仅有既有大 chunk 提示。自动化覆盖列表期间持续连接、查看 B 时 A 接收并保留消息、未读/运行摘要、同 Session 换凭证替换旧 generation、删除单个 Session 隔离、App 卸载关闭全部连接，以及 Snapshot 终态清除旧 streaming 状态并保留正文。用户已完成两个 WhaleTerm Session 同时配对、运行、切换和状态隔离人工验收，结果通过。
- 本次提交前核对：架构决策、需求、设计与实施计划、移动端既有状态/权限修复，以及公共 `remote.EventStream` 广播实现与测试均纳入范围；新增 D66/D67 及 `docs/specs/2026-09-22-mobile-multi-session-runtime-design.md` 与对应计划，三端可实现性核对通过。定向文档检查无新增空白错误；全仓 `git diff --check` 仍只被用户已有的 `apps/mobile/src/assets/icons/svgs/permission.svg` 行尾空白阻塞。代码测试沿用下方最近验证结果，本次未因提交重复运行。
- 本轮 Remote 空闲态修复：WhaleTerm `node --test tests/claudeRemoteManagement.test.mjs` **20/20** 通过，`frontend npm run build` 通过；构建仅有既有 Vue `:deep`、CSS `:deep()`、动态/静态 import 和大 chunk 警告。前端全量 `node --test tests/*.test.mjs` 中本轮 Remote 用例全部通过，但仍有 4 个既有无关失败：Claude MCP 对话框静态断言、Claude MCP 路径别名加载、颜色主题导出不匹配、Project Cloud 默认字段断言。
- W1 WhaleTerm 真实会话人工验收：用户已确认通过；验收记录已同步到 `docs/testing/W1-result.md`。本次未补写用户未逐项报告的具体验收细节。
- 本轮多 Session 串线修复：新增公共 `remote.EventStream` 广播、跨 Session 拒绝和慢订阅者隔离测试；`go test -race -count=1 ./remote -run 'EventStream|PublicSurface|Server'` 通过。WhaleTerm 新增两个真实 `ClaudeRemoteSessionHost` 的 `chatKey` 隔离测试；`go test -race -vet=off ./backend/chat -run 'ClaudeRemote|PermissionMode' -count=1`、`go test -vet=off ./backend/remote -count=1`、`go test -vet=off ./backend/services -run '^TestRemotePairingResponse$' -count=1` 通过。本轮文件定向 `git diff --check` 通过。公共 `internal/channel` race 测试通过；`internal/transport` 全量 race 仍只有既有两个失败：慢客户端会关闭快速连接、Server Close 未刷出最终 Session 事件，与本轮修改无代码交集。
- 本轮仅修改 W1 设计、实施计划和交接记录，未修改业务代码；三份本轮文档的定向 `git diff --check` 通过。全仓 `git diff --check` 仍只被用户已有的 `apps/mobile/src/assets/icons/svgs/permission.svg` 行尾空白阻塞。
- 本轮 Remote 恢复修复：WhaleTerm Remote 前端定向测试 **19/19** 通过，`frontend npm run build` 通过；构建仅有既有 Vue `:deep`、CSS `:deep()`、动态/静态 import 与大 chunk 警告。相关文件定向 `git diff --check` 通过。
- 本轮多 Session/移动端缓存验证：WhaleTerm `go test -vet=off ./backend/remote -count=1`、`go test -vet=off ./backend/services -run '^TestRemotePairingResponse$' -count=1`、`go test -vet=off ./backend/chat -run 'ClaudeRemote|PermissionMode' -count=1` 通过；Remote 前端测试 18/18、前端 `npm run build` 通过。移动端全量 Vitest **417/417**、`npm run build`、lint 通过；lint 只有既有 `scripts/cdp-console.mjs` warning，两个前端构建只有既有 Vue/CSS、动态 import 和大 chunk 警告。新增测试覆盖两个 Session 独立启停、首选端口占用后自动顺延，以及返回列表再进入并接收 Snapshot 后保留已见消息。误跑 WhaleTerm `backend/services` 全包仍触发既有无关 `TestGetRemoteConn` 类型转换 panic；收窄到本轮服务测试后通过。两仓库本轮文件定向 `git diff --check` 通过；本项目全量检查仍受用户已有 `permission.svg` 行尾空白影响。
- 本轮 WhaleTerm 模块边界验证：`go test -vet=off ./backend/services -run '^TestRemotePairingResponse$' -count=1`、`go test -vet=off ./backend/remote -count=1`、`go test -vet=off ./backend/chat -run 'ClaudeRemote|PermissionMode' -count=1` 通过；前端 Remote 测试 18/18、`npm run build`、WhaleTerm `git diff --check` 通过。前端构建只有既有 Vue `:deep`、CSS 语法、动态/静态 import 和大 chunk 警告。全仓 `go build ./...` 仍被既有 `backend/market` 缺失嵌入文件/错误模块路径及 Windows-only webview2 包阻塞；误用宽泛 `-run 'Remote|PermissionMode'` 时触发既有无关 `TestGetRemoteConn` panic，收窄到本轮测试后全部通过。本项目 `git diff --check` 被用户已有 `permission.svg` 行尾空白阻塞，本轮文档自身无新增格式错误。
- 权限模式双向同步已由用户完成真实双端验收，并已提交此前 WhaleTerm 改动。
- 本轮 WhaleTerm 验证：新增“空闲态权威权限不被旧请求覆盖”和“移动端空闲切换通知 PC”回归测试；权限相关定向测试、`go test -vet=off ./backend/remote -count=1`、前端 `npm run build`、两仓库 `git diff --check` 通过。`backend/chat` 全量测试运行到既有 `TestClaudeCodeCancelNotifiesLocalStreamBeforeAsyncAbort` 时因全局 `isEventsEmit=true` 配合测试用无效 Wails context 退出；本轮定向测试均通过。普通 `go test` 仍受既有 `deepwiki`/`gemini` vet 格式错误影响；frontend lint 仍缺生成文件 `.eslintrc-auto-import.json`。
- 移动端 Vitest：**415/415**；TypeScript、build、`git diff --check` 通过。尚未在本轮重新运行 lint；构建仅有既有大 chunk 提示。
- Go 非网络包及 Remote 通过；`internal/transport` 仍有两个生命周期测试缺口：慢客户端隔离会同时关闭快速连接，服务关闭未刷出最终事件。该问题与本轮 UI/权限字段接线无直接交集，待单独处理。
- lint 仅有既有 `scripts/cdp-console.mjs` warning；build 仅有既有大 chunk 提示。

## 当前工作区

- 本次提交排除：`scripts/macos/3_web_dev.sh` 权限位变化、`docs/testing/images/` 用户测试截图，以及 `apps/mobile/src/assets/icons/svgs/permission.svg` 的用户已有行尾空白变化；以上内容继续保留在工作区供后续决定。

## 下一步

1. 提交已完成人工验收的本轮移动端实现（需用户单独授权 Git 写操作）。
2. 机器稳定身份、列表机器分组、断线 replay、系统后台推送继续作为后续独立设计，不混入本次改造。
3. 单独修复并验证 `internal/transport` 两个生命周期测试；后续按需推进 C2-C/C2-D。

## 约束提醒

- Session Host 是唯一会话管理者；输入必须经单一队列；协议消息必须带 `sessionId`。
- 手机不保存完整 Claude 历史；浏览器不持久化 Pair Token。
- 任何协议/状态变更需核对三端可实现性并记录到 `DECISIONS.md`。
- Git 写操作必须由用户明确授权。
