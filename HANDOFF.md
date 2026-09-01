# HANDOFF.md — Claude Mobile Remote 交接文档

> 新会话先读 `AGENTS.md`，再读本文件、`DECISIONS.md`、`REQUIREMENTS.md` 和总体设计。完成阅读后先复述当前状态和下一步，再按“当前执行”继续；涉及尚未确认的设计、计划或产品决策时先等用户确认。
>
> 本文件只记录**当前状态、当前执行、未闭环项和阻塞**。已完成阶段的过程明细见 `docs/HANDOFF-archive.md`（新会话不需要读），阶段门禁证据见 `docs/testing/`，已确认决策见 `DECISIONS.md`。

## 0. 一句话现状

A1–A6 已提交；B1 已完成并通过门禁；**B2 已完成并通过用户验收**（2026-08-28，含三批移动端 UI 打磨；真实 Claude 端到端验证后置，不阻塞）；C1 已完成并通过门禁（Capacitor iOS/Android 工程 + Web 接入 + 局域网明文放行 + Android 模拟器壳内连通）；服务重启后的配对恢复已闭环；**B3 已完成并通过门禁**（协议类型下放到 `protocol/`、远程能力抽取为 `remote.Server`）；**C2-A/C2-B 已完成并通过用户验收**（终端二维码 + Mobile Web 生命周期）；**CLI 产品化配置改造已完成并通过用户验收**（网络 profile + 常用 Claude 参数）。C2-C（原生扫码/安全存储）与 C2-D（真机/VPN）暂缓。下一步：W1（依赖 B3，已就绪）。

## 当前执行（AI 自动维护）

- **子阶段**：开发者自动化脚本设计。
- **本轮任务**：参考 Macro-Deck-Client-App 的 macOS/Windows 脚本体系，为新 pull 项目提供安装、环境检查、Web/原生运行、CLI 调试和构建入口。
- **范围**：新增 `scripts/macos` 与 `scripts/windows` 平台脚本及共享说明；覆盖 `install_1_base_tools`、`install_2_android_sdk`、`check`、`web-dev`、`web-build`、`android-run`、`android-build`，macOS 额外覆盖 `ios-run`、`ios-build`、CLI 启动；不自动开放防火墙、不处理凭证、不改变协议或 WhaleTerm 接入边界。
- **通过条件**：新用户可从 README 的最短路径开始；脚本具备可重复执行的依赖检查、明确失败提示和参数透传；macOS/Windows 入口语义对齐；脚本静态检查与可执行的 dry-run/check 验证通过。
- **当前状态**：平台脚本、README 与验证记录已完成；未执行 Git 写操作。
- **实际完成项**：新增 macOS Bash 与 Windows PowerShell 的共享 helper、环境检查、基础工具安装、Android SDK 安装、Web 开发/构建、CLI、Android live/dist/build 入口；macOS 额外提供 iOS Simulator 运行/构建；README 增加新 pull 三步路径、命令表、排障和安全边界。Android 目标 AVD 通过 serial 绑定，live/dist 恢复环境变量，CLI 日志隐藏参数。
- **实际验证**：macOS `bash -n`、全部 `--help`、基础 `check`、`check --android` 通过；Capacitor CLI 版本核对通过；Mobile Web 405 个测试、lint、build 通过；`git diff --check` 通过。Go 全量测试受当前沙箱禁止 httptest 绑定本地端口影响，Windows PowerShell parser/实机流程、Android 安装和 iOS Simulator 流程未执行。
- **当前结论**：脚本实现与文档完成，验证记录见 `docs/testing/developer-automation-scripts-result.md`；下一步由用户在 Windows 及需要的原生设备环境执行平台验收。

- **子阶段**：开发者自动化脚本安装体系重构。
- **本轮任务**：参照 Macro-Deck-Client-App 的 install_1/install_4 结构，新增基础工具与 Android SDK 实际安装入口，删除重复的 setup 入口并同步 README。
- **范围**：新增 macOS/Windows `install_1_base_tools` 与 `install_2_android_sdk`；删除对应旧 `setup`；保留 check、Web、Android、iOS、CLI 运行/构建入口；Android 安装组件以当前工程 compileSdk 36 为准，不处理签名、凭证、Claude Service 或 Git 写操作。
- **通过条件**：安装脚本可重复执行，明确处理 SDK 根目录、Java、sdkmanager、platform-tools、platform 36、build-tools；Windows 可配置用户环境变量，macOS 给出当前终端和持久化配置提示；旧入口不再出现在 README；静态检查与可执行检查通过。
- **当前状态**：安装脚本已新增，旧 setup 已删除，README/验证记录已同步；正在完成最终静态验证，未执行 Git 写操作。
- **实际完成项**：新增 macOS/Windows `install_1_base_tools` 与 `install_2_android_sdk`；Android 安装包含 command-line tools、platform-tools、platform 36、build-tools 36、Java 检查和环境变量配置；Windows 写入用户级环境变量，macOS 输出 shell 配置；删除两个旧 setup 脚本。
- **实际验证**：macOS 新脚本 `bash -n`、帮助、`--check-only` 临时目录检查通过；未在真实 SDK 上重复安装；Windows PowerShell 和真实 Windows 安装流程未执行。
- **当前结论**：安装体系已按确认方案重构，最终文件清单和限制见 `docs/testing/developer-automation-scripts-result.md`。

- **子阶段**：移动端 Web UI 第二轮可用性修复。
- **本轮任务**：根据真机截图继续调整普通正文/用户输入字号，扩大抽屉关闭按钮触控区，修复权限/问答命令发送失败后按钮失效，以及为空的工具详情提示。
- **范围**：仅修改 `apps/mobile` 展示层与客户端命令交互反馈；不改协议字段、服务端语义、配对凭证或三端共用契约。工具结果正文仍遵守不下发边界。
- **通过条件**：普通文本与用户输入跟随较大的 antd-mobile 字号变量；抽屉关闭按钮具备可靠的移动端触控尺寸；命令只有真正发出后才锁定，发送失败可重试并有提示；工具详情无内容时给出明确说明；测试、lint、build 通过，并完成浏览器/截图复核。
- **实际完成项**：普通正文与用户输入改为使用 `--adm-font-size-9`；抽屉标题栏关闭/返回按钮扩大为 44×44，审批/问答/提交按钮增加 44px 最小触控高度；命令回调返回发送结果，只有真正写入 WebSocket 后才锁定操作，失败时保留按钮并提示重试；工具结果活动无正文时显示“工具结果正文未下发，请在电脑端查看”，空思考内容也有明确占位。
- **实际验证**：`cd apps/mobile && npm test` → 16 个文件、405 个测试通过；`npm run lint` 0 error（保留 1 个既有 warning 在 `scripts/cdp-console.mjs`）；`npm run build` 通过；`git diff --check` 通过。已检查用户提供的四张真机截图，尚未在手机热点环境重新人工复测。
- **当前结论**：本轮仍只修改 Mobile Web 展示与命令失败反馈，不改协议和工具结果脱敏边界；下一步由用户刷新页面后复测普通文本、抽屉按钮和权限/问答操作。

- **子阶段**：移动端 Web UI 可用性修复。
- **本轮任务**：修复手机浏览器中字号偏小，以及会话详情输入栏随页面内容滚动、无法固定在底部的问题。
- **范围**：仅调整 `apps/mobile` 的字号继承/antd-mobile 变量使用与详情页视口布局；不改变协议、状态模型、配对凭证或交互语义。
- **通过条件**：普通正文与抽屉不再被 12–14px 行内样式压缩；字号跟随 antd-mobile 变量；消息区独立滚动且输入栏稳定停留在视口底部；Mobile Web 测试、lint、build 通过。
- **实际完成项**：详情页改为 `100dvh` 视口高度并隐藏页面级溢出，消息区保持独立滚动；底部输入栏不可被消息内容挤下，并适配 `safe-area-inset-bottom`。新增全局基础样式清除浏览器 body 默认边距并继承 antd-mobile 字体变量；应用内正文、Markdown、抽屉和辅助信息的字号覆盖改为 `--adm-font-size-*` 变量。
- **实际验证**：`cd apps/mobile && npm test` → 16 个文件、403 个测试通过；`npm run lint` 0 error（保留 1 个既有 `scripts/cdp-console.mjs` warning）；`npm run build` 通过；`git diff --check` 通过。尚未重新进行手机热点真机人工复测。
- **当前结论**：本轮仅涉及 Mobile Web 展示层，不改变协议、状态、配对或三端共用契约；等待用户在手机浏览器复测字号和底部输入栏。

- **子阶段**：W1 设计。
- **本轮任务**：规划 WhaleTerm 对已有单个 Claude Session 的进程内 Remote Server 接入，实现不中断任务的远程管理。
- **范围**：复用 WhaleTerm 已有 Claude Client、Session Controller 和输入队列；不做 CLI 生产级 PTY，不接管普通终端外部 Claude 进程，不创建第二个 SDK Client，不停止宿主 Session。
- **通过条件**：W1 设计明确宿主适配边界、开启/关闭生命周期、当前 Turn 行为、跨仓库依赖和可执行验收；经用户确认后编写实施计划。
- **当前结论**：W1 架构设计和实施计划已获用户确认；已切换 WhaleTerm `claude_remote_manage` 分支，Task 1 盘点完成，Task 3 开始。WhaleTerm 当前工作区存在一个用户未跟踪的 `WhaleTerm-Claude功能介绍.html`，保留不动。
- **本轮已完成**：WhaleTerm 已接入本地 `claude-mobile-remote` 公共模块；新增 `backend/chat/claude_remote_bridge.go` 及聚焦测试，建立单 Session 协议事件 Hub，覆盖 Snapshot、事件 seq、有序发布和错误 Session 隔离。
- **本轮验证**：`GOFLAGS=-buildvcs=false go test -vet=off ./backend/chat -run 'TestClaudeRemoteEventHub' -count=1` 通过；默认 `go test` 仍被 WhaleTerm 既有 `deepwiki.go`/`gemini.go` 等 vet 错误拦截。
- **当前缺口**：真实 Claude manager 的 stream、权限、队列、取消和 Session 状态尚未接入 Hub；`remote.Server` 尚未由 WhaleTerm 持有；Wails“远程管理”入口、配对展示和跨仓库人工验收尚未实施。

- **子阶段**：CLI 产品化配置改造。
- **本轮任务**：将普通启动参数收敛为 Session/CWD/Claude 参数；用 `local`、`emulator`、`device`、`debug` profile 区分网络环境，并同步开发版/调试版使用文档。
- **范围**：LAN 与 VPN 统一为 `device`；保留底层网络参数作为 debug 能力；不改变 Session Host 唯一性、WebSocket 协议、配对负载或 Remote Server 架构。
- **通过条件**：profile 默认与覆盖规则有测试；`permission-mode` 等常用 Claude 参数传入 SDK；普通/调试命令文档清晰；Go 测试、构建、vet 通过。
- **实际完成项**：新增 `local`、`emulator`、`device`、`debug` profile；新增 `--advertise-host`、`--permission-mode`、`--model`、`--max-turns`；修正 resume/新建 Host 的 SDK 参数装配顺序；README 已拆分开发版与调试版，设计/计划/验证记录位于 `docs/specs/`、`docs/plans/`、`docs/testing/`。
- **实际验证**：`cmd/claude-mobile-remote` 与 `internal/claude` 测试通过；`go vet ./...` 与 CLI build 通过；用户已完成开发版、模拟器和真机 `device` profile 验收；全量 Go 测试仍受当前沙箱 loopback 监听限制，真实 Claude live 测试未执行完成。
- **当前结论**：CLI 日常启动可省略底层网络参数，LAN/VPN 统一由 `device` profile 表达；用户验收通过，准备执行本次提交。

- **子阶段**：C2-A/C2-B 已完成并通过用户验收。
- **本轮任务**：缩小 CLI 终端二维码的显示尺寸，保持 Pairing Fragment、JSON 输出和二维码内容不变。
- **范围**：仅调整 `internal/pairing` 的终端渲染方式；不改变协议、配对凭证、服务启动参数或 Mobile Web 行为。
- **通过条件**：新增回归测试证明终端二维码相较原渲染在宽度和高度上缩小；配对相关测试与 CLI 构建通过。
- **实际完成项**：终端二维码使用 UTF-8 半块字符渲染，并关闭库内置的四模块外边框；新增回归测试锁定宽度和高度均小于旧渲染，且渲染结果与无边框预期一致。
- **实际验证**：`go test ./internal/pairing ./protocol ./cmd/claude-mobile-remote -count=1` 通过；`GOFLAGS=-buildvcs=false go build ./cmd/claude-mobile-remote` 通过；`git diff --check` 通过。
- **当前结论**：CLI 启动参数和二维码承载内容不变，终端显示尺寸进一步缩小；用户已确认 C2 配对、生命周期、重启恢复和二维码验收通过；本次提交尚未执行。

- **子阶段**：C2-A/C2-B 实施完成并通过用户验收。
- **本轮任务**：W1 暂缓，完成 C2-A 后端终端二维码与现有配对持久化回归、C2-B Mobile Web 刷新/断线/手动重连/可见性状态同步；原生扫码、安全存储、多 Session 原生持久化、真机和 VPN 暂缓。
- **范围**：不接入 WhaleTerm，不接管外部 Claude 进程，不改变 WebSocket 协议、Session 状态模型或 Pairing JSON 形状；二维码仅做终端 ANSI/ASCII 输出，不生成 PNG、不增加 HTTP 二维码接口。
- **通过条件**：终端二维码与现有 Pairing Fragment 互操作；pairing 文件行为不回归；Web 刷新/断线/手动重连和可见性边界通过；原生、PNG、HTTP 二维码接口、真机和 VPN 明确记录为暂缓。
- **实际完成项**：新增 `protocol.EncodePairingFragment`、终端二维码渲染与 CLI `pairing QR:` 输出；新增 Go/TypeScript 互操作测试；`ProtocolClient` 增加连接状态读取，App 增加 `visibilitychange` 状态同步；README 与 `docs/testing/C2-result.md` 已更新。
- **实际验证**：Go 聚焦测试、CLI 构建、`go vet ./...`、Mobile Web 403 个测试、lint 和 build 通过；全量 Go 测试仅在当前沙箱的既有 httptest/本地监听用例失败（禁止绑定 `[::1]:0` 与 `127.0.0.1:0`），详见 `docs/testing/C2-result.md`。
- **当前结论**：C2 精简设计已按用户确认提交（`a3e18f2`）；实施计划位于 `docs/plans/2026-08-31-C2-pairing-web-lifecycle-plan.md`，本轮实现与人工验收已完成。

- **子阶段**：仓库配置维护。
- **本轮任务**：将 `claude-mobile-remote` CLI 二进制产物加入 `.gitignore`。
- **通过条件**：`.gitignore` 明确忽略该二进制名称，不影响现有 Go 源码和测试规则。
- **实际完成项**：已在 Go 二进制忽略区新增 `claude-mobile-remote`。
- **实际验证**：`git diff --check` 通过；`rg` 确认 `.gitignore` 第 5 行包含精确规则 `claude-mobile-remote`。

- **子阶段**：B3 已收口。本轮完成协议类型下放与单 Session Remote Server 组件化，门禁全绿，证据见 `docs/testing/B3-result.md`。
- **实际完成项**：
  1. **`protocol/` 包（新增）**：JSON-bearing 协议类型（Snapshot/Turn/Event/Input/ContentBlock/PendingAction/Question/QuestionAnswer/ActionDecision/ServiceConfig/Pairing/TruncatePreview）的唯一 Go 来源，不依赖任何 internal 包、SDK 或 Transport。
  2. **`internal/session`/`claude`/`pairing` 改 alias**：`session` 的 `model.go`/`event.go`/`input.go`/`controller.go` 类型与哨兵改 alias 指向 `protocol`；`Snapshot.setQueuedRequests` 方法改包级函数（Go 不能对非本地 alias 定义方法）；`claude.TruncatePreview` 改 delegate，`pairing.Payload` 改 alias。
  3. **`internal/channel`/`transport` 指向 `protocol`**：Channel 的 `Controller` 接口、Transport 三文件的 payload/snapshot/event 引用改用 `protocol` 类型；**删除 transport 本地 `ActionDecision`/`QuestionAnswer` 副本**，`sessionResolution` 简化为深拷贝。
  4. **`remote/` 包（新增）**：公共 `Controller`/`Config`/`Server`，`Start` 合并原 `Activate`（注册 Channel + 签发凭证，失败回滚），`Close` 注销 Channel 且从不调用宿主 Controller 的 Start/Stop。
  5. **`cmd/claude-mobile-remote` 迁移**：`main.go` 改用 `remote.New` 装配；`run.go` 的 `remoteService` 去掉 `Activate`、`Start()` 返回 Pairing、删除「init 后延迟激活」死分支；删除旧 `cmd/claude-mobile-remote/remote.go`。
- **实际执行的验证及结论**：`gofmt -l .` 无输出；`GOFLAGS=-buildvcs=false go vet ./...` 无输出；`GOFLAGS=-buildvcs=false go test -count=1 ./...` → **10 个包全部 ok**（新增 `protocol`、`remote`）；`git diff --check` 无空白错误。`rg` 核对确认协议 `type X` 定义只出现在 `protocol/`，`internal/session`/`pairing`/`transport` 内均为 alias，transport 本地 `ActionDecision`/`QuestionAnswer` 副本已消除。
- **未验证项**：真实 Claude 端到端（额度受限，不阻塞）；WhaleTerm W1 接入（B3 只交付公共边界，未实现 WhaleTerm 侧适配）；前端/HarmonyOS（B3 只动 Go 侧，wire contract 不变，未重跑 `npm`）。
- **未执行**：任何 Git 写操作（仅用只读 `git diff --check`）。

- **子阶段**：B2 已收口。用户于 2026-08-28 反馈「B2 验证已通过」，本轮工作是提交前重跑完整门禁、做敏感信息与产物核对，并把验收结论写入 `docs/testing/B2-result.md`。
- **提交前门禁（本轮实际执行，全绿）**：`gofmt -l .` 无输出；`GOFLAGS=-buildvcs=false go vet ./...` 无输出；`GOFLAGS=-buildvcs=false go test -count=1 ./...` → 8 个包全部 ok（非缓存）；`cd apps/mobile && npm run test` → 16 文件 **400 passed**；`npm run lint` **0 error**（1 个既有 warning 在 `scripts/cdp-console.mjs`）；`npm run build` 通过（`index-*.js` 490.71 kB / gzip 155.25 kB）。`git diff --check` 无空白错误；`go.mod`/`go.sum`/`apps/mobile/dist` 均未入库；凭证特征扫描仅命中「描述扫描动作的文档文字」与 `fakehost_test.go` 的守卫用例，无真实凭证。
- **提交时刻意排除**：`docs/specs/images/qoder/{sessionDetail,sessionTools,toolDetail}.png` **未入库**。它们是用户提供的 Qoder App 界面参考图，画面内含真实 Claude 会话正文与本机绝对路径，与 AGENTS.md 开发规范「不提交会话内容」冲突；仅作为本地参考保留，`docs/HANDOFF-archive.md` §6 已标注未入库。
- **上一批（第三批两项）实际完成项**（仅移动端样式，未动协议、状态模型与交互语义，Go 侧零改动）：
  1. **表格与底色分离**：页面底色收归到 `ui.ts` 的 `PAGE_BACKGROUND`（`#fafbfd`）与 `SESSION_BACKGROUND`（偏白渐变，原顶部 `#eef2f7` 偏灰蓝），`App.tsx` 两处 `#f5f5f5` 与 `SessionDetail` 硬编码渐变改为引用常量；`MessageMarkdown` 表格改为白底卡片（容器 `#fff` + 8px 圆角 + 轻阴影，网格线 `#e3e6ec`，th `#f5f7fa`）——根因是正文区无气泡背景且单元格透明，页面底色直接透上来；代码块同理补一条 `#e3e6ec` 边框。
  2. **抽屉透亮化**：根因是 `backdropFilter` 会把身后的遮罩一起采样进模糊结果，antd-mobile 默认 `rgba(0,0,0,0.55)` 会把整个抽屉染成灰面。新增 `DRAWER_MASK_STYLE`（`rgba(15,23,42,0.16)`）并传给两个抽屉的 `Popup maskStyle`（遮罩仍在，点击拦截与 `onMaskClick` 语义不变）；`DRAWER_SHEET_STYLE` 不再复用 `GLASS_SURFACE_STYLE`，改为白底 0.6 + `blur(30px) saturate(200%) brightness(1.14)` + 顶部白高光边 + 外投影；`GLASS_PANEL_STYLE` 改用白色高光边（原深色描边在亮玻璃上显脏），preview 底色从 `rgba(0,0,0,0.04)` 改为半透明白。
- **实际执行的验证及结论**：
  - `npx tsc -p tsconfig.app.json --noEmit` 无输出（也回证 antd-mobile `Popup` 确实接受 `maskStyle`）；`npm test` → 16 文件 **400 passed**（+1）；`npm run lint` 0 error（1 个既有 warning 在 `scripts/cdp-console.mjs`）；`npm run build` 通过；
  - 新增护栏用例「抽屉遮罩显著淡于默认档，玻璃靠 brightness 提亮」：在 jsdom 里对**真实渲染的** `.adm-mask` / `.adm-popup-body` 断言遮罩 alpha < 0.3 且玻璃带 `brightness`，锁住设计约束而不锁具体色值（否则改回默认遮罩就会退化成灰面）。
- **视觉观感的证据边界**：三批 UI 打磨的实际观感由**用户人工确认通过**，仓库内没有截图证据（AI 侧浏览器子代理环境不可用：viewport 20px、`visibilityState=hidden`、截图固定报 `NATIVE_BROWSER_VIEWPORT_UNAVAILABLE`）。不得对外表述为「AI 已视觉验证」。
- **仍未验证**：真实 Claude 端到端（工具审批放行/拒绝后 Turn 继续）因额度受限后置；真机、VPN、原生扫码与安全存储属 C2。
- **已执行的 Git 写操作**：按用户本次明确授权完成一次本地 `git commit`（未 push、未改分支）。授权仅限本次提交，后续 Git 写操作需重新授权。
- **上一批（第二批四项）实际完成项**:
  1. **审批与提问改抽屉（D53）**：`PendingActionCard` 从固定卡片改为底部抽屉（`Popup position="bottom" destroyOnClose closeOnMaskClick={false}`），`pendingAction` 非空即自动打开；只有标题栏 ✕ 可关闭，关闭即复用既有 `cancel_turn`（`SessionDetail` 新传 `cancelEnabled`/`onCancel`）；`cancelEnabled=false` 时 ✕ 禁用，取消命令被拒后抽屉自动重开，事项不会无人处理。抽屉视觉常量上移到 `src/components/ui.ts`（`DRAWER_SHEET_STYLE`/`DRAWER_HEADER_STYLE`/`ICON_BUTTON_STYLE`）与 `ActivityGroup` 共用。
  2. **状态并入抽屉标题**：`ActivityGroup` 详情删除独立「状态」区块，状态标签渲染在 `DrawerHeader` 标题行（新增 `status` 入参）。
  3. **答案可见（D52）**：`action_resolved` 新增可选 `answers`（每项 `{questionId, header, question, values}`），数据源是服务端校验后交给 SDK 的同一份答案，**不涉及 Claude 工具结果正文（D46 不变）**、不进 snapshot、重连不重放；Go 侧 `ResolvedAnswer` + `resolvedAnswers`/`cloneResolvedAnswers` + `validateResolvedAnswers`（仅 `action_resolved` 可携带，其它 kind 出现即协议错误）；移动端解码同规则校验，reducer 新增 `AnswerMessage`，`MessageFeed` 渲染「你的回答」（问题 + 选中值 Tag）。
  4. **文档精简**：`HANDOFF.md` 291 行 → 本文件规模，历史执行明细迁入 `docs/HANDOFF-archive.md`。
- **实际执行的验证及结论**：
  - `gofmt -l internal cmd` 无输出；`GOFLAGS=-buildvcs=false go vet ./...` 无输出；`go test ./...` 8 个包全部 ok（本次沙箱下未出现 httptest 绑定失败）；
  - `cd apps/mobile && npm test` → 16 文件 **399 passed**（本轮 +15：ActivityGroup 状态标题、PendingActionCard 抽屉 5 例、decode 答案 8 例、reducer 2 例、MessageFeed 回显 1 例）；`npm run lint` 0 error（1 个既有 warning 在 `scripts/cdp-console.mjs`）；`npm run build` 通过；
  - 浏览器实测（`--fake-host interactive` + Vite dev 5174，390×844）四点**全部通过**：审批抽屉从底部弹出且点遮罩不关闭 → 「本次允许」；提问抽屉单选+多选作答后消息流出现「你的回答」，两个多选值均可见；再次发送后点 ✕ → Claude 回复出现「已取消」且输入框恢复可用；工具详情标题右侧带状态标签且正文无独立「状态」区块。
- **未验证项**：本轮实测由浏览器子代理执行，因其 viewport 受限**未能产出截图**（结论基于 DOM 与计算样式断言）；console 无 error/warn 这一条本轮未单独取证；真实 Claude 端到端仍未验（额度受限，不阻塞）。
- **未执行**：任何 Git 写操作（按硬约束 9 仅可能用到只读 `git diff`）。

## 1. 当前目标

B2 已整体收口（结论见 `docs/testing/B2-result.md` §5）。B3 已完成并通过门禁（结论见 `docs/testing/B3-result.md`）：协议类型下放到 `protocol/` 单一 Go 来源、远程能力抽取为可导入的 `remote.Server`。C2-A/C2-B 已完成并通过用户验收（结论见 `docs/testing/C2-result.md`）：后端终端二维码与 Mobile Web 生命周期闭环。CLI 产品化配置改造已完成并通过用户验收（D55，结论见 `docs/testing/cli-productized-config-result.md`）：网络 profile 与常用 Claude 参数分层。下一步进入 W1 设计（WhaleTerm 为已持有的单个 Claude Session 启动进程内 Remote Server，依赖 B3 公共边界）。

```text
机器 A：PC 输入 ──> Session Controller ──> Claude
                         └──LAN WebSocket──> 机器 B 浏览器
```

## 2. 已确认事项

- 项目仓库：`/Users/zhangym/code/work/utils/claude-mobile-remote`；独立于 WhaleTerm，旧方案、阶段和记忆不继承；W1 只复用本项目新的单 Session Remote Server；
- 使用 `github.com/tea4go/claude-agent-sdk-go` 托管 Claude；一个 Standalone CLI 进程只管理一个 Session，低频并行 Session 用多进程 + 不同端口；
- CLI 保留简单行输入输出作为 PC 输入和调试入口，不产品化 Claude Code 终端；
- WhaleTerm 后续只为其已持有的一个 Session 启动进程内 Remote Server，不创建第二个 SDK Client；
- 扫码绑定真实 `sessionId`；手机认证后显式订阅对应 Session Channel，Channel 只负责消息路由；
- 移动端后续安全保存多个已扫描 Session，一次只连接当前进入的 Session；
- Claude 的完整上下文、会话状态和所有操作执行都留在 PC，手机只展示数据并回传操作；手机不展示扫码前完整历史；
- Claude 一轮输出完成只结束 Turn，不结束 Session；
- Pair Token 默认有效期 30 天（D51）；原生 App 使用系统安全存储，浏览器不写 `localStorage`；服务重启时恢复本地 pairing 文件中的未过期 token（D50 之前的配对决策），过期或删除该文件后重新扫码；服务端不做滑动续期；
- A6 为 LAN 跨机器桌面浏览器门禁；手机默认通过 VPN 直连 PC，C2 做真机验收；
- iOS / Android 使用 Capacitor（C1 已建工程），HarmonyOS NEXT 在 D1 独立开发；
- C2 依赖 C1/B2，W1 依赖 B3；B3 是单 Session Remote Server 组件化，不再建设 B4 持久恢复阶段；
- 后期公网访问使用可信的每 CLI 单出站隧道 Relay，不建设 PC 聚合 daemon、云端 Session 中心或永久 route；心跳、事件 replay、系统推送、单进程多 Session 和云端多 PC 管理后置；
- AI 可主动使用只读 `git diff`；Git 写操作与其他未明确允许的 Git 命令仍需用户授权（D23）。

## 3. 文档状态

- 常驻文档均已用户确认：`REQUIREMENTS.md`、`AGENTS.md`、`CLAUDE.md`、`DECISIONS.md`、总体设计、A5.2 后简化路线设计。
- A1–A6、B1、B2、C1、B3、C2-A/C2-B、CLI 产品化配置的分阶段设计与实施计划均已确认并执行；门禁记录见 `docs/testing/`（A1/A2/A3/A4/A5.1/A6/B1/C1/B3/C2/cli-productized-config 已通过，A5.2 真实 Claude 门禁因额度受限仅部分通过，`A5-result.md` 待补写）。
- 完整逐项状态表（含每个文件路径与当时结论）见 `docs/HANDOFF-archive.md` §8。
- 业务代码：B1 及之前、B2、C1 均已**本地提交未推送**；B3 及后续改动均已本地提交（`110d8e5`、`d9f413d`、`d587ea2`），当前工作区以 `git status` 为准。

## 4. 下一步（仅未闭环项）

1. **进入 W1 设计**（WhaleTerm 为已持有的单个 Claude Session 启动进程内 Remote Server，依赖 B3 公共边界）；
2. **C2-C（暂缓）**：原生扫码、Keychain/Android Keystore 安全存储、原生多 Session 列表跨重启持久化、前后台恢复；重启时需连同硬约束 7、D41 的浏览器存储约束和 `store.test.ts` 的「不写 Storage」护栏一并正面修改，并重新评估是否下发/续期 `expiresAt`（D51 已记原因）；
3. **C2-D（暂缓）**：iOS/Android 真机、VPN、本地网络权限、系统杀进程恢复；
4. C1 遗留待办：Origin 默认值是否加入 `http://localhost`（C1-result §7 发现 2），用户决定留待后续；
5. 额度恢复后重跑真实 Claude 门禁（`TestLiveReservedSessionIDPreTurn`），补验首轮完成、跨 Turn 上下文和 Resume 同 ID，并一并补验 B2 的真实 Claude 工具审批端到端；
6. 按需补做 A5.2 重连/resume 人工步骤并补写 `docs/testing/A5-result.md`；
7. 行内代码当前为 `wordBreak: 'break-all'`，会在任意字符处切断标识符（B1-result §8.3），待确认是否改为 `break-word`；
8. SDK `WithSessionID` 发布正式版本后，把本地 `replace` 换回正式版本并同步 `go.mod.base`。

## 5. 首期关键验证

已验证：SDK 生命周期/流式消息/取消、真实 `sessionId` 与二维码时机、Turn 完成后 Session 可继续、终端与手机输入在单一 Controller 串行、snapshot 最小字段、显式 listen/advertise/Origin 支持跨机器访问且保持默认 loopback 边界、后端终端二维码与 Mobile Web 生命周期闭环、CLI 网络 profile 与常用 Claude 参数。
待验证：手机 VPN、原生扫码、安全存储和前后台恢复（暂缓项 C2-C/C2-D 真机阶段）。

## 6. 环境与已知事项

- 本机具备 Go、Node.js、Xcode；Android SDK 已就绪（platform-tools 37.0.1 / platforms 36 / build-tools 36.0.0 / emulator 37.1.11 / android-36 arm64-v8a，AVD `pixel7`），`ANDROID_HOME=~/Library/Android/sdk` 已写入 `~/.zshrc`，JDK 21 由用户 JVMS 管理；
- 模拟器只用于界面适配与构建验证；其网络经 NAT（宿主机 `10.0.2.2`），不能代替 C2 真机与 VPN 验证；
- Android 壳需 `server.androidScheme: 'http'` 才能连 `ws://` 明文（D49）；dist 加载路径下壳内 Origin 为 `http://localhost`（无端口），live reload 下为 dev server 地址（如 `http://10.0.2.2:5173`），联调需对应的 `--allowed-origin`；
- 桩服务必须以管道方式启动（CLI 读 stdin，`nohup ... &` 会因 EOF 立即退出）：`tail -f /dev/null | /tmp/cmr ... --allowed-origin http://localhost:5174`；
- antd-mobile 5.42.3 部分传递依赖与 React 19 peer 范围不一致，用户已评估可忽略；
- 本地沙箱禁止绑定 `[::1]:0`，`internal/transport` 与 `cmd/channel-test-client` 的部分 httptest 用例可能因此失败，属环境限制而非回归。

## 7. 参考提醒

WhaleTerm 的旧远程控制方案、阶段和记忆不读取也不继承。当前只参考 SDK 已验证用法；进入 W1 后再按已确认的单 Session 边界检查 WhaleTerm 当前 Claude manager/client 接口。
