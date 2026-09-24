# HANDOFF.md — Claude Mobile Remote 交接文档

> 新会话先读 `AGENTS.md`，再读本文件、`DECISIONS.md`、`REQUIREMENTS.md` 和总体设计。历史过程见 `docs/HANDOFF-archive.md`，阶段门禁见 `docs/testing/`。

## 当前状态

- 当前执行：`internal/transport` 两个既有生命周期缺口已修复，用户反馈测试通过并授权提交本轮修复。全量 Go race 与构建门禁通过，详见 `docs/testing/2026-09-24-transport-lifecycle-result.md`。Harmony DevEco 工程构建/真机仍待补。

- 上一轮多会话新建范围仅 Standalone，不含 WhaleTerm 或 Relay。用户确认的语义记录于 D70/D71：任一有效已订阅 Session 凭证可新建，默认最多四个活跃会话，停止后保留配对与终态入口。本轮列表分组覆盖 CLI 与 WhaleTerm，产品语义记录于 D72。

- 最近完成：共享监听与按 Session 鉴权的 CLI Runtime Manager、v2 可选管理消息及配对能力字段、终端 `:sessions/:use/:new/:stop/:pair`、React/Capacitor 与 Harmony 新建/停止入口。新会话从 CLI 固定工作目录启动；停止释放活跃容量但保留终态快照和配对入口。管理结果仅回请求连接，创建使用 `commandId` 幂等与容量/频率控制。已有公共单 Session Remote Server API 不变。设计和计划已按用户审阅意见更新；三端共用契约可实现性记录于 DECISIONS。

- A1–A6、B1、B2、B3、C1、C2-A/C2-B、D2、CLI 产品化配置均已完成；C2-C/C2-D 暂缓。
- W1 移动端输入框 UI 已完成用户验收并提交：`ddaeb7d feat: refine mobile composer and permission controls`。
- 本次提交包含移动端输入框、附件/权限 Popover、停止按钮、权限模式共享契约及配套测试；未包含用户测试截图和无关脚本权限位变化。

## 最近验证

- 本轮 Transport 生命周期修复：三个真实 WebSocket 生命周期用例以 race 模式连续运行 10 轮通过；`go test -race ./internal/transport`、`go test -race ./...`、`GOFLAGS=-buildvcs=false go build ./...`、`git diff --check` 通过。用户另反馈测试通过，未提供额外命令或设备细节。快连接测试逐事件读取，慢连接独立溢出退出；订阅源正常关闭时先排空事件，Server 停机等待 writer 刷出最终事件。未做真实慢网或移动真机验证；协议与三端状态模型未变。
- 用户已反馈本阶段验证通过，未逐项报告具体设备和操作场景；验收结果已记入 `docs/testing/2026-09-23-mobile-project-session-list-result.md`。提交前复跑 WhaleTerm `go test -vet=off ./backend/chat -run 'ClaudeRemote|PermissionMode' -count=1` 通过；两仓库暂存差异 `git diff --cached --check` 通过。Harmony DevEco 工程构建/真机仍未执行。
- 本轮 L3 列表紧凑化：React/Capacitor Vitest **438/438**、`npm run lint`、`npm run build` 通过；lint 仅既有脚本 warning，构建仅大 chunk 提示。测试已随整行进入、“更多”菜单和未读点更新，并验证删除菜单不误进详情。`git diff --check` 通过；新增 CSS 单独检查无尾随空白。自动化未覆盖真实窄屏视觉/触控；用户随后反馈本阶段验证通过，未报告具体设备和逐项场景。详见 `docs/testing/2026-09-23-mobile-project-session-list-result.md`。
- 本轮项目分组：Go 新增协议、CLI 双会话元数据、EventStream、v2/v3 Snapshot/Event 隔离的 `go test -race` 定向通过，`GOFLAGS=-buildvcs=false go build ./...` 通过；Go 全量仍仅有既有 `internal/transport` 两个生命周期用例失败。WhaleTerm `go test -vet=off ./backend/chat -run 'ClaudeRemote|PermissionMode' -count=1` 通过。React/Capacitor Vitest **438/438**、build、lint 通过；lint 仅既有脚本 warning，build 仅大 chunk 提示。Harmony 7 个非 UI `.ets` 完成语法解析，内存转译执行的 8 项 self-check 通过；本机缺 DevEco/hvigor/ohpm，ArkUI 工程构建与真机未验证。`git diff --check` 与 WhaleTerm 改动文件 `gofmt -d` 通过。详见 `docs/testing/2026-09-23-mobile-project-session-list-result.md`。
- 本轮仅完成设计与记忆文档，未修改业务代码、未运行代码测试。核对现有 CLI 固定 `cwd`、WhaleTerm Session `ProjectPath`/`CreatedAt`、React 与 Harmony 列表和协议严格解码；发现旧 Transport 遇未知消息即关闭连接，因此草案采用 v2/v3 双版本协商与回退，具体契约待用户检查。`git diff --check` 通过；新增设计/计划文件另查行尾空白。Harmony 当前机器缺 DevEco/hvigor/ohpm 的既有限制仍在。
- 本轮入口优化：React/Capacitor Vitest **433/433**、build、lint、`git diff --check` 通过；新增测试覆盖 CLI/WhaleTerm 能力门控、终态仍可新建、创建中禁用及附件图标。lint 只有既有脚本 warning，build 只有既有大 chunk 提示。Harmony UI 已同步调整；当前机器缺 DevEco/hvigor/ohpm，工程构建及真机体验未验证。验证记录见 `docs/testing/standalone-cli-multi-session-automated-result.md`。
- 本轮 UI 收尾：React/Capacitor Vitest **431/431**、build、lint 及 `git diff --check` 通过；lint 只有既有脚本 warning，build 只有既有大 chunk 提示。新增测试覆盖详情底部新建、终止操作说明和终态仍可新建。Harmony UI 已对应调整，但本机缺 DevEco/hvigor/ohpm，未验证工程构建。用户反馈实际多会话测试整体通过、确认本阶段完成；未逐项回报具体验收场景。记录见 `docs/testing/standalone-cli-multi-session-automated-result.md`。
- 本轮 Standalone 多会话：CLI/Claude/协议包 `go test -race` 定向通过；补充 Stop 调用方取消后的状态回退测试后，CLI `TestManagedCLI` race 测试再次通过。新增 loopback WebSocket 从 A 创建 B、停止 B、B 终态重连与再新建的自动化联调通过；`GOFLAGS=-buildvcs=false go build ./...` 通过。移动端 Vitest **429/429**、lint、build 通过；lint 只有既有 `scripts/cdp-console.mjs` warning，build 只有既有大 chunk 提示。Harmony 非 UI `.ets` 文件完成语法解析和内建 self-check 用例扩展；当前环境无 DevEco/hvigor/ohpm，不能宣称 Harmony 工程构建通过。全量 `go test ./... -count=1` 仅 `internal/transport` 的两个既有生命周期测试失败（慢客户端隔离导致快连接关闭、Close 最终事件未刷出）；新逻辑测试均通过。`git diff --check` 通过，新增未跟踪文件另做行尾空白检查通过。真实 SDK 同目录双 Session、手机/真机操作及容量门禁尚未人工验收。
- 此前设计阶段仅完成设计文档与项目记忆更新，未修改业务代码；新增设计和计划为未提交文件，`git diff --stat` 不统计它们。其后已按用户确认进入代码实施，结果见上。
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

- 本轮 Transport 生命周期修复包含 `internal/transport` 实现与测试、A4 实施计划、此交接记录，以及 `docs/testing/2026-09-24-transport-lifecycle-result.md`；用户已授权提交这些文件。
- 本轮按用户授权将项目分组、展示元数据和列表紧凑化相关代码/文档分别提交至本项目与 WhaleTerm；`docs/testing/images/` 截图及 WhaleTerm `frontend/tests/claudeRemoteManagement.test.mjs` 未纳入提交，保持原样。

## 下一步

1. 本阶段用户已反馈验收通过。后续在 DevEco 环境补 Harmony 工程构建与真机验收；若用户提出具体体验问题，再按反馈迭代。
2. Transport 生命周期缺口已关闭；机器稳定身份、断线 replay、系统后台推送及 Relay 多 Session 映射继续作为后续独立设计。后续 Git 写操作需另行明确授权。

## 约束提醒

- Session Host 是唯一会话管理者；输入必须经单一队列；协议消息必须带 `sessionId`。
- 手机不保存完整 Claude 历史；浏览器不持久化 Pair Token。
- 任何协议/状态变更需核对三端可实现性并记录到 `DECISIONS.md`。
- Git 写操作必须由用户明确授权。
