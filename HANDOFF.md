# HANDOFF.md — Claude Mobile Remote 交接文档

> 新会话先读 `AGENTS.md`，再读本文件、`DECISIONS.md`、`REQUIREMENTS.md` 和总体设计。完成阅读后先复述当前状态和下一步，再按“当前执行”继续；涉及尚未确认的设计、计划或产品决策时先等用户确认。
>
> 本文件只记录**当前状态、当前执行、未闭环项和阻塞**。已完成阶段的过程明细见 `docs/HANDOFF-archive.md`（新会话不需要读），阶段门禁证据见 `docs/testing/`，已确认决策见 `DECISIONS.md`。

## 0. 一句话现状

A1–A6 已提交；B1 已完成并通过门禁；**B2 已完成并通过用户验收**（2026-08-28，含三批移动端 UI 打磨；真实 Claude 端到端验证后置，不阻塞）；C1 已完成并通过门禁；服务重启后的配对恢复已闭环；**B3 已完成并通过门禁**；**D2 已完成代码层功能拉齐并通过 Harmony 编译门禁**；**C2-A/C2-B 已完成并通过用户验收**；**CLI 产品化配置改造已完成并通过用户验收**。**W1 代码与自动化门禁已完成，等待用户在 WhaleTerm + Mobile Web 做真实会话验收。** C2-C（原生扫码/安全存储）与 C2-D（真机/VPN）暂缓。

## 当前执行（AI 自动维护）

- **子阶段**：W1 · 移动端输入框 UI 验收与提交。
- **当前状态**：**用户已验收通过，正在提交**（2026-09-21）。提交范围包含输入框 UI、其依赖的权限模式共享契约/移动端接线、配套测试、SVG 与项目记忆；排除用户提供的 `docs/testing/images/` 截图和无关的 `scripts/macos/3_web_dev.sh` 权限位变化。
- **提交前门禁**：移动端 Vitest **415/415**、TypeScript、lint、build 和 `git diff --check` 通过；Go 的 `cmd/claude-mobile-remote`、Channel、Claude、Pairing、Session、Version、Protocol、Remote 通过，首轮前 Snapshot 权限字段契约测试已更新并通过。`internal/transport` 仍有两个与本轮 UI/权限字段接线无直接代码交集的生命周期用例失败：慢客户端隔离会同时关闭快速连接、服务关闭未刷出最终事件；已单独复跑确认，后续作为传输层缺口处理。lint 仅有既有 warning，build 仅有既有大 chunk 提示。

- **子阶段**：W1 · 移动端停止按钮布局收敛。
- **本轮任务**：将运行中“取消当前 Turn”入口移动到输入框上方，缩小按钮并简化可见文案。
- **范围**：仅修改移动端 `MessageComposer` 布局、样式和对应测试；不修改取消命令、运行态同步、协议或宿主逻辑。
- **通过条件**：停止按钮相对输入框定位在右上方；尺寸明显小于当前版本；正常态显示“停止”、请求后显示“停止中”；点击仍携带当前 `requestId`，移动端门禁不回归。
- **当前状态**：**代码完成，等待用户真机视觉验收**（2026-09-21）。停止入口已从屏幕 fixed 改为输入框容器右上方 absolute 定位；按钮缩至 26px 高、12px 字号，正常态显示“停止”，取消命令在途时显示“停止中”并禁用重复点击；无障碍名称与取消 requestId 行为保持不变。
- **实际验证**：`MessageComposer` 定向测试 **10/10**、移动端全量 Vitest **415/415**、TypeScript、lint、build 和 `git diff --check` 通过；lint 仅有既有 `scripts/cdp-console.mjs` warning，build 仅有既有大 chunk 提示。
- **未验证项 / 下一步**：需用户真机确认按钮与输入框上沿的 8px 间距、尺寸及运行中/停止中视觉。未执行 Git 写操作。

- **子阶段**：W1 · 移动端工具按钮居中与 Popover 二次收敛。
- **本轮任务**：修复权限/语音图标与 hover 背景的视觉偏移，继续压缩附件和权限 Popover，并重做权限当前项的勾选样式。
- **范围**：仅修改移动端 `MessageComposer` 样式、渲染和对应测试；不修改 SVG 图形数据、权限同步、协议或宿主代码。
- **通过条件**：四个工具按钮的内容盒严格水平/垂直居中；Popover 比上一版更窄、更低、更少留白；权限项文字左对齐、勾选标记位于右侧且不挤压文字；失焦关闭及既有交互测试不回归。
- **当前状态**：**代码完成，等待用户真机视觉验收**（2026-09-21）。四个工具按钮统一增加外层与 `.adm-button-content` 双层 flex 居中，消除权限/语音图标与 hover 背景的中心偏移；Popover 最小宽度收至 132px（权限 140px）、行高收至 36px、字体 14px；权限勾选改为当前项右侧 15px 主色细勾，不再占用左侧图标列。
- **实际验证**：`MessageComposer` 定向测试 **9/9**、移动端全量 Vitest **414/414**、TypeScript、lint、build 和 `git diff --check` 通过；lint 仅有既有 `scripts/cdp-console.mjs` warning，build 仅有既有大 chunk 提示。
- **未验证项 / 下一步**：需用户真机确认 hover/tap 背景居中、Popover 密度及右侧勾选视觉。未执行 Git 写操作。

- **子阶段**：W1 · 移动端输入工具栏视觉与弹层交互修正。
- **本轮任务**：根据 `mobile_chat_input3.png`、`mobile_chat_input_plus.png` 和 `mobile_chat_input_permission.png` 的真机反馈，统一左右工具图标线条重量；将附件/权限弹层收敛为统一、紧凑的 antd-mobile Popover 风格，并修复点击外部无法关闭。
- **范围**：仅修改移动端 `MessageComposer`、相关 SVG 和对应测试；不修改权限同步、协议、WhaleTerm、附件上传或语音能力边界。
- **通过条件**：左侧添加/权限图标与右侧语音/发送视觉线宽协调；两个 Popover 的圆角、内边距、行高和项目间距一致且更紧凑；点击触发器或弹层外部均可关闭；现有附件选择、权限切换和移动端门禁不回归。
- **当前状态**：**代码完成，等待用户真机视觉验收**（2026-09-21）。添加入口已改用 antd-mobile `AddOutline`，权限 SVG 移除额外描边并缩至 19px；附件与权限统一改用紧凑的 `Popover.Menu`，权限当前项使用左侧勾选和主色文字；显隐交回 `trigger="click"` / `onVisibleChange`，点击触发器或弹层外部均会关闭。
- **实际验证**：`MessageComposer` 定向测试 **9/9**、移动端全量 Vitest **414/414**、TypeScript、lint、build 和 `git diff --check` 通过；lint 仅有既有 `scripts/cdp-console.mjs` warning，build 仅有既有大 chunk 提示。
- **未验证项 / 下一步**：当前环境未做真机截图，需用户确认四个工具图标的视觉重量、两个紧凑菜单的尺寸和失焦关闭手感。未执行 Git 写操作。

- **子阶段**：W1 · 移动端权限入口交互修正。
- **本轮任务**：按用户验收反馈将底部权限入口改为纯图标；权限文字只在点击后的 antd-mobile 选择弹层内展示，当前 active 权限显示选中勾。
- **范围**：仅修改移动端 `MessageComposer` 和对应测试；不修改权限同步、协议、WhaleTerm 或其他输入工具能力。
- **通过条件**：工具栏权限入口无可见文字和下拉箭头；仍具备无障碍名称；弹层列出全部权限，当前项 active 并打勾；切换回调和移动端门禁通过。
- **当前状态**：**代码完成，等待用户视觉验收**（2026-09-21）。权限入口已使用 `permission.svg` 纯图标按钮，不显示模式文字或下拉箭头；弹层使用 antd-mobile `Selector` 展示全部权限，当前模式保持 active 并显示组件自带勾选标记。
- **实际验证**：`MessageComposer` 定向测试 9/9、移动端全量 Vitest **414/414**、TypeScript、lint、build 和 `git diff --check` 通过；lint 仅有既有 `scripts/cdp-console.mjs` warning。
- **未验证项 / 下一步**：需用户在真机确认权限图标尺寸、弹层位置及 active 勾选视觉。未执行 Git 写操作。

- **子阶段**：W1 · 移动端输入工具栏视觉返工。
- **本轮任务**：根据 `mobile_chat_input2.png` 的验收反馈和 `codex-remote.png` 参考图重排输入区；最大限度复用 antd-mobile，把权限模式移到左下角 `+` 右侧并缩小发送按钮。
- **范围**：仅修改移动端 `MessageComposer` 及对应测试和样式；不修改协议、权限同步逻辑、附件/语音能力边界或 WhaleTerm。
- **通过条件**：输入区采用“上方文本、下方工具栏”的协调布局；附件菜单和权限选择使用 antd-mobile 组件；权限入口位于 `+` 右侧；发送按钮明显缩小且禁用/可发送状态清晰；移动端测试、类型检查、lint、build 与 `git diff --check` 通过。
- **当前状态**：**代码完成，等待用户视觉验收**（2026-09-21）。输入区已改为参考图的上下两层结构；附件和权限入口分别使用 antd-mobile `Popover`，附件项使用 `Button`、权限项使用 `Selector`；权限入口位于左下方 `+` 右侧；语音与发送位于右侧，发送按钮由 42px 高亮圆形收敛为 32px 透明按钮承载 28px SVG。
- **实际验证**：移动端定向测试 9/9、全量 Vitest **414/414**、TypeScript、lint、build 通过；lint 仅有既有 `scripts/cdp-console.mjs` warning；`git diff --check` 通过。
- **未验证项 / 下一步**：当前环境没有可用浏览器表面，无法生成真实窄屏截图；需用户在手机或现有调试环境验收输入区整体比例、两个 Popover 和键盘弹起状态。未执行 Git 写操作。

- **子阶段**：W1 · 修复权限模式配对/双向同步并补充诊断日志。
- **本轮任务**：修复 WhaleTerm 空闲状态下 PC 权限变更无法广播、无轮次事件被 Host 丢弃导致的移动端不同步；补充配对 Snapshot、权限命令发送/结果、PC 执行/广播及移动端归并日志。
- **范围**：CMR 协议保持现有字段和 version 2；WhaleTerm 仅增加按 agent 的权限变化订阅与远程 Host 处理，不改变 Claude Client 单实例和 Session 输入队列；新增同步回归测试。
- **通过条件**：配对 Snapshot 展示当前 PC 权限；PC 空闲/运行中切换均能同步移动端；移动端切换能同步 PC 和其他移动端；失败回滚可诊断；相关 Go/移动端测试通过。
- **当前状态**：**已完成修复**（2026-09-21）。WhaleTerm 新增按 agent 的权限变化订阅，权限事件不再依赖活动 Turn；远程 Host 接收无 requestId 的权限事件并更新 Snapshot/广播；移动端补充命令发送失败和事件归并日志。
- **实际验证**：WhaleTerm `backend/chat`/`backend/remote` 权限与远程定向测试通过；移动端 Vitest **413/413**、TypeScript、lint、build 通过；CMR Go 协议/Channel/Session/Claude/Transport 编译测试通过；两仓库 `git diff --check` 通过。
- **未验证项 / 下一步**：需用户重新配对确认：配对首个 Snapshot、PC 空闲切换、PC 运行中切换、移动端切换，以及第二个移动端是否同时收到变更。未执行 Git 写操作。

- **子阶段**：W1 · 移动端活动块与输入工具栏视觉修复。
- **本轮任务**：暂停权限同步；修复 WhaleTerm 远程 Write 工具无结构化内容时把 `${toolKey}` 占位文本发到移动端的问题；参考 `codex-remote.png` 和新增 SVG 重做底部输入工具栏与附件菜单。
- **范围**：只改 WhaleTerm 远程事件过滤、移动端 `MessageComposer`/活动渲染及对应测试；不扩展附件上传协议、不恢复权限同步开发。
- **通过条件**：Write 占位符不再出现在移动端；结构化工具块仍可展示；输入栏窄屏下固定、按钮可用、附件菜单具备相机/照片/文件入口并复用 SVG；移动端测试、lint、build 通过。
- **当前状态**：**已完成**（2026-09-21）。WhaleTerm 过滤 `${toolKey}` 内部占位符；移动端 reducer 增加旧 Host 兼容过滤；输入栏改为底部胶囊工具栏，附件菜单使用新增相机/照片/文件 SVG，语音和发送入口使用对应 SVG。
- **实际验证**：移动端 Vitest **414/414**、TypeScript、lint、build 通过；WhaleTerm 远程 Host 占位符定向测试通过；两仓库 `git diff --check` 通过。
- **未验证项 / 下一步**：需用户实际查看窄屏输入栏、附件弹出菜单，以及 Write 工具调用的移动端显示。权限同步按用户要求暂缓。

- **子阶段**：移动端输入区专项优化。
- **本轮任务**：参考 `docs/testing/images/chatInput/`，重做会话底部输入区；保留现有文本发送闭环，增加照片/文件、相机、权限模式、语音控制和发送/终止状态入口。照片/文件、相机、语音先以不误导用户的原生能力预留为主；权限模式显示并复用当前 Session 已同步的远程权限状态，不新增协议字段或第二套权限模型。
- **范围**：仅改 `apps/mobile/src/components/MessageComposer.tsx`、`SessionDetail.tsx` 及对应移动端测试/视觉常量；不改变 WebSocket 协议、Session Controller、配对负载和现有权限决议语义。浏览器端不伪造原生文件、相机或麦克风成功结果。
- **通过条件**：输入区在窄屏下可用且不遮挡消息；文本输入、发送、超长校验和运行中终止不回归；照片/文件、相机、语音入口有明确的预留状态；权限模式能反映当前会话权限状态；相关移动端测试、lint、build 通过。
- **当前状态**：**第一版已完成**（2026-09-21）。输入区已重做为胶囊式底栏：支持 Enter 发送、Shift+Enter 换行、附件/相机入口、语音入口、运行中取消和权限同步提示；附件/相机/语音当前只做浏览器可安全呈现的预留，不伪造已发送或已录音结果。
- **实际验证**：移动端全量 Vitest 16 个文件 **412/412** 通过；`npx tsc -p tsconfig.app.json --noEmit` 通过；`npm run lint` 通过（仅既有 `scripts/cdp-console.mjs` warning）；`npm run build` 通过。
- **已知边界 / 下一步**：当前 wire contract 没有“PC 全局权限模式”字段，移动端不能切换或读取一个未下发的全局模式；本轮只同步已有 `pendingAction` 的 PC 权限请求，并显示“权限：跟随 PC”。若要支持“读取/切换 PC 权限模式”，需另立协议与三端设计任务。原生 Capacitor 文件选择、相机、麦克风权限和附件/语音上传作为后续任务。
- **需求修正（用户已确认）**：移动端需要展示当前 Session 的权限模式，并可在 Turn 运行中发起修改；修改最终必须由 PC 侧调用 Claude Client 执行，成功后同步给所有客户端。当前执行已切换为权限模式同步与运行时修改。

- **子阶段**：W1 · 移动端权限模式只读展示与 PC 侧运行时修改。
- **本轮任务**：在共享协议中增加当前 `permissionMode`、权限模式变更事件和 `set_permission_mode` 命令；PC 侧串行校验并执行 SDK 权限模式切换；移动端通过 Snapshot 恢复、事件实时同步并提供选择控件。
- **范围**：不改变既有工具审批、AskUserQuestion、Session Host 单 Client 和输入队列；已产生的 `pendingAction` 不因模式切换自动决议；不接入原生附件/语音。
- **通过条件**：默认无配置时为最低权限 `default`；配对/重连 Snapshot 展示 PC 当前模式；移动端修改请求由 PC 成功执行后才更新并广播；运行中修改、失败回滚、当前审批不被自动绕过均有测试覆盖；Go/Node/Mobile 门禁通过。
- **当前状态**：**权限模式双向同步代码已完成**（2026-09-21）。共享协议新增 `permissionMode`、`permission_mode_changed` 和 `set_permission_mode`；Standalone Session Controller/Host 串行执行 SDK 运行时切换；WhaleTerm Host 复用既有 `UpdateClaudeCodePermissionMode`，PC 侧 hook 与移动端请求均广播变更；移动端 Snapshot/事件归并并提供选择控件。未执行 Git 写操作。
- **实际验证**：移动端 `tsc`、Vitest **413/413**、lint、build 通过（lint 仅既有 warning）；CMR Go 非网络包通过，transport/remote 网络测试在当前沙箱因 `httptest` 监听权限失败；WhaleTerm `backend/remote` 与 `backend/chat` 权限/远程定向测试通过，已 gofmt。
- **已知边界 / 下一步**：协议仍保持 version 2，旧 Snapshot 缺少 `permissionMode` 时移动端兼容回退 `default`；`bypassPermissions` 当前仅作为权限模式选项，未额外增加二次确认。下一步继续优化移动端输入区：权限选择器的反馈态、附件/相机/语音原生占位和窄屏布局，并补权限模式专门测试及真实 WhaleTerm + Mobile Web 验收。

- **子阶段**：W1 · 修复 WhaleTerm/移动端运行态同步与远程文本流粒度。
- **本轮任务**：让 PC 与移动端对同一 Turn 进入/退出运行态一致；PC 和移动端运行中均提供可见的终止入口；远程文本增量保持顺序并避免单个 SDK 文本块过大导致移动端成片渲染。
- **范围**：WhaleTerm 仅改远程 Host/聊天视图及对应测试；移动端仅改取消入口/必要状态测试；不改 wire protocol、字段、seq 语义、Channel 订阅模型或 Session Host 单写入边界。
- **通过条件**：手机发起 Turn 时 PC 进入运行态并显示终止入口；PC 发起 Turn 时移动端显示运行态及终止入口；任一端终止后两端都进入取消终态；远程文本内容不变、seq 连续且单次增量受限；相关 Go/Node/移动端测试通过。
- **当前状态**：**代码与自动化验证已完成**（2026-09-21）。PC 远程轮次会进入现有 Footer 加载态；移动端取消入口固定悬浮；远程文本块按 32 个 Unicode 字符切分并保持内容顺序。
- **实际验证**：WhaleTerm 远程 Host/订阅相关 race 测试通过；移动端全量 Vitest 409/409 通过；本项目 Channel/remote/CLI race 测试通过；两仓库 `git diff --check` 通过。WhaleTerm `backend/chat` 全包测试仍有既有 Wails invalid context 日志导致失败，远程相关定向测试通过。
- **未验证项 / 下一步**：尚未完成真实 WhaleTerm + 手机设备闭环，需人工确认双向运行态、悬浮终止和文本流观感。

- **子阶段**：W1 · 收敛 WhaleTerm Channel 订阅语义与前端状态恢复。
- **本轮任务**：把 WhaleTerm 内部事件桥接补齐为可注销、可检测背压的订阅模式；消除 Manager 全局回调覆盖竞态；让公共 Channel 拒绝不连续事件；恢复前端已开启状态并修正 session/listen key 回退；复用公共配对 TTL 默认值。
- **范围**：本项目修改 `remote` 默认配对 TTL、`internal/channel` 序号连续性及测试，并记录订阅连续性决策；WhaleTerm 修改 `backend/chat` Manager 事件订阅与 Host/Hub、`backend/remote` TTL 配置、远程管理前端及对应测试。不改 wire protocol、协议字段、Session/Turn 状态模型、配对负载，不创建第二个 SDK Client，不执行 Git 写操作。
- **通过条件**：Manager 多订阅者互不覆盖且可独立注销；订阅 context 取消后 Hub 无残留；Hub 溢出关闭慢订阅而非静默丢事件；Channel 遇到 seq 缺口返回 `ErrInvalidSequence`；前端可从 `GetClaudeRemoteManagement` 恢复当前状态且不以空 prop 覆盖已解析 ID；相关 Go/Node 测试、race 测试、格式检查和 `git diff --check` 通过。
- **当前状态**：**代码与自动化验证已完成**（2026-09-21），等待 WhaleTerm + Mobile Web/Android 模拟器真实会话验收。
- **实际完成项**：
  1. WhaleTerm Manager 保留既有测试钩子，同时新增可独立注销的事件/原始消息订阅集合；Host 不再替换全局回调，关闭失败或未赢得 Service 所有权的 Host 不会清掉正在工作的订阅。Service 复用同一 Session 时会关闭未采用的新 Host。
  2. `claudeRemoteEventHub` 在订阅 context 取消时删除并关闭对应 channel；单个订阅缓冲区满时只关闭该慢订阅，不阻塞 SDK 事件线程、不静默跳过后继续发送。
  3. 公共 Channel 要求 `event.Seq == lastSeq + 1`；跳号、重复、倒退或订阅源异常关闭都会进入错误通道，使 Transport 关闭连接并由客户端重连获取最新 Snapshot。该语义记录为 D60。
  4. 公共 `remote.DefaultPairTokenTTL` 成为 CLI 与 WhaleTerm 共用的 30 天默认值，删除 WhaleTerm 重复常量。
  5. WhaleTerm 前端挂载时通过 `GetClaudeRemoteManagement` 恢复属于当前 Session 的已开启状态；历史回写使用 `listenKey || chatKey`；启动参数不再以空 `props.sessionId` 覆盖 `makeChatConfig` 已解析的 `session_id`。
- **实际验证**：本项目 `go test -race ./internal/channel ./remote ./cmd/claude-mobile-remote` 通过；WhaleTerm `backend/chat` 远程 Host/Hub/订阅针对性 race 测试通过；`backend/remote` race 测试通过；远程管理前端专项测试 18/18 通过；Vite production build 通过（仅既有 Vue `:deep`、CSS 与 chunk warning）；两仓库 `git diff --check` 通过。WhaleTerm 全量 Node 测试为 89 项中 85 通过、4 项既有失败（Claude MCP 布局/模块别名、颜色主题导出、Project Cloud 默认字段），本次专项用例无新增失败。
- **未验证项 / 下一步**：尚未做真实设备闭环。人工验证：刷新 WhaleTerm 后仍显示当前远程管理状态；手机连接后连续收发；断开/重连恢复最新 Snapshot；关闭后重开无需重复配对；快速重复点击启动不会造成事件停止或重复。未执行任何 Git 写操作。

- **子阶段**：W1 · 修复 WhaleTerm 进程内 Host 发布非法 `turn_started` 导致移动端掉线。
- **本轮任务**：WhaleTerm 的 `ClaudeRemoteSessionHost.handleManagerEvent` 在 `onturnstart` 分支从不设置 `event.Text`，产出空正文 `turn_started`。共享传输层 `internal/transport` 的 `validateEvent` 要求该事件携带非空正文（正文即用户输入，移动端据此渲染输入气泡），校验失败即判为出站非法事件并 `connection.shutdown()` 关闭 WebSocket。现象：配对成功后任意一方开始一轮会话，移动端立即掉线，PC 侧照常执行（PC 走 Wails 事件，不经过该传输层）。本轮让 Host 用本轮 turn 的 prompt 填充 `event.Text`。
- **范围**：只改 WhaleTerm `backend/chat`（`claude_code_cli_new.go` 的 `claudeTurnLifecycle` 增一个字段、`claude_code_cli_new_lifecycle.go` 的 `registerClaudeTurn` / `runClaudeTurnQueue` / `resolveSessionForRequest` 与新增正文访问器、`claude_remote_session_host.go` 的 `onturnstart` 分支）及对应测试。**不改** `claude_code_cli.go` 的 `promptPayload`、`claude_code_cli_new_prompt.go` 的 `buildPromptPayloadNew` / `extractPromptsFromMessagesNew`——按用户要求「remote control 相关改动集中，非必要不动原有文件」（这两个文件功能牵扯大）。不改 `internal/transport` 校验策略（该契约同时被独立 CLI host 依赖）、不改 wire protocol、不改配对负载、不改移动端、不改 `BindSession`、不创建第二个 SDK client、不执行 Git 写操作。
- **通过条件**：新增 Host 用例先观察 RED——`turn_started` 必须携带非空且不超过 `protocol.MaxPromptBytes` 的正文、`Source` 合法、`TurnStatus == running`（即共享传输层的 `turn_started` 合法性谓词）；实现后 GREEN。既有 `TestClaudeRemoteSessionHostMapsEventsAndPermission` / `TestClaudeRemoteSessionHostReservesSessionBeforeFirstTurn` 等不回归；`go test ./backend/chat/...` 通过；`gofmt -l` 无输出；`git diff --check` 通过。
- **当前状态**：**已完成**（2026-09-20）。两项修复均已实现并通过自动化验证，等待用户在 WhaleTerm + Android 模拟器做真实会话验收。
- **实际完成项**：
  1. **非法 `turn_started`（移动端掉线）**：`claudeTurnLifecycle` 增 `prompt` 字段，由 `registerClaudeTurn` 在 `c.mu` 内写入（避免与 `activeTurns` 读取竞争）；新增 `claudeTurnPrompt(chatKey, turnID)` 访问器（轮次 ID 必须匹配，否则返回空串——`activeTurns` 可能已被下一轮覆盖）；`ClaudeRemoteSessionHost.handleManagerEvent` 的 `onturnstart` 分支现在填充 `event.Text`，按 `protocol.MaxPromptBytes` 在 UTF-8 边界截断，取不到正文时**不发布并告警**而非发出非法事件。正文取本轮提交给 Claude 的 prompt（`turn.payload.prompt`）——Host 只拿得到 `chatKey` + `turnID`、看不到 payload，故必须由 manager 侧携带。**注**：曾一度引入 `promptPayload.userText` 以让手机气泡显示用户原话（prompt 已被 `preparePromptForExec` 转义并拼上技能前缀与附件说明），经用户评审后按「不过度修改原有后台代码」原则**已回退**；代价是气泡显示的是含技能前缀与附件说明的 prompt，Windows 上换行显示为字面 `\n`。
  2. **`resolveSessionForRequest` 铸造第二个身份**：请求未带 `session_id` 且本地元数据丢失时，先查 `autonomy.SessionID(agentID)`，已绑定则复用 canonical session，仅在无绑定时才生成新 UUID。此前该路径凭空铸新 UUID，紧随其后的 `BindSession` 必然以 `session identity mismatch` 拒绝本轮。
- **实际验证**：TDD 先 RED 再 GREEN。
  - 非法 `turn_started` 的 RED 是**行为性**失败（测试可编译、运行时失败）：`invalid turn_started event: {... Text: ...}` —— 空正文，正是传输层判非法并 shutdown 的条件。
  - 身份铸造的 RED 同样是行为性失败：`resolved session = "0a6f30f4-…", want the bound canonical session`。
  - 新增/加强用例：`TestClaudeRemoteSessionHostTurnStartedIsTransportValid`（复刻 `internal/transport.validateEvent` 的 `turn_started` 合法性谓词——该谓词不可导出，故在测试内镜像）、`TestClaudeRemoteSessionHostSuppressesTurnStartedWithoutInputText`、`TestClaudeTurnPromptIsScopedToTheActiveTurn`、`TestResolveSessionForRequestKeepsAutonomyBoundSession`。
  - 既有 `TestClaudeRemoteSessionHostMapsEventsAndPermission` 的 fixture 补上本轮正文并把断言从「只看 Kind/RequestID」加强为同时校验 `Text`——原断言正是漏掉该缺陷的原因。
  - **回归对照（决定性）**：把两项修复在 `/tmp` 副本中还原（基线）后跑同一套用例。口径取**具体失败用例名**——分批边界随测试增删位移，按批次名不可比。当前树与基线的失败用例**完全一致**（8 个，全部为既有失败：`TestDefaultClaudeMcpGetRunnerUsesCommandContext` 的 `command type = *execplus.CmdPlus` 断言失败、`TestListTools` / `TestStartMcpTool` 需外部 MCP 服务、`TestSetClaudeCodeMcpDisabledWritesScopeAndIsIdempotent`、`TestDispatchResultMessageStopsOnSuccess`、`TestClaudeToolKeysAreUniqueWithinOneAssistantMessage`、`TestClaudeToolResultKeepsTheOriginalToolKey`、`TestResolveSkillInvocationsNewSupportsMultipleSkillsInOrder`）。基线中两项修复的用例为 RED（`turn_started text = "", want the turn's user input`；`resolved session = "d559654f-…", want the bound canonical session`），当前树 GREEN。即本次改动**未引入任何新失败**。
  - `gofmt -l backend/chat/` 无输出。
- **未验证项 / 已知限制**：
  - `backend/chat` 整包 `go test` 在当前仓库**本来就无法一次跑完**（与本次改动无关，已用还原副本证明）：部分用例会留下 `go runClaudeTurnQueue` 协程，其在测试结束后调用 `emitClaudeEvent` → Wails `runtime.EventsEmit`，而测试用的 `ctx` 不是 Wails 生命周期 ctx，Wails 直接 `log.Fatal` 终止整个测试进程（报 `cannot call '…runtime.EventsEmit': An invalid context was passed`）。另有 `TestDefaultClaudeMcpGetRunnerUsesCommandContext` 等既有的真实断言失败。**均未修复**（超出本次范围）。
  - `go test` 默认 vet 会因既有的 `deepwiki.go` / `gemini.go` / `deepwiki_file.go` 报错而 build failed，需 `-vet=off`。
  - WhaleTerm 目录不是 Git 仓库，无法产出 `git diff --check`。
  - 真实设备端到端（模拟器配对 → 任一方发起会话不掉线 → 两端共享上下文 → 无第二个 Claude 进程）未复验，需用户验收。
- **未做（仍待用户决定）**：WhaleTerm 前端 `ClaudeRemoteManagement.vue` 两处回退缺陷——`:76` 的 `updateHistoryConfig(props.listenKey, …)` 在 `listenKey === ''` 时静默失效（`:117` 有 `|| props.chatKey` 回退，`:76` 没有），以及 `:119` 的 `claudeConfig.session_id = props.sessionId` 会用可能为空的 prop 覆盖 `makeChatConfig` 已解析出的 ID。后端修复后这两处已不再导致 identity mismatch，但仍会让配对使用/回写过期或空的 session id。
- **下一步**：用户在 WhaleTerm + Android 模拟器复验；并决定是否处理上述前端回退缺陷。

- **子阶段**：W1 · 将 Origin 判定由「枚举白名单」改为「规则判定」。
- **本轮任务**：把共享 `internal/transport` 的 `CheckOrigin` 从精确字符串查表改为「显式白名单 ∪ 三条规则」的并集判定，使 loopback（任意端口）、Android 模拟器别名 `10.0.2.2`（任意端口）、以及 `AdvertiseURL` 的主机名（任意端口）一律放行；据此移除 WhaleTerm 前端那份猜出来的硬编码 Origin 清单。
- **范围**：改 `internal/transport/server.go`（`Config` 增 `AdvertiseURL` 字段 + `CheckOrigin` 判定逻辑）、`remote/remote.go`（透传 `config.AdvertiseURL` 给 transport）、`internal/transport/server_test.go`（新增规则用例）；WhaleTerm 侧删除 `CLAUDE_REMOTE_ALLOWED_ORIGINS` 常量与组件传参、更新对应测试。不改协议、配对负载、`listenAddress`、鉴权与配对 wire contract；保留 `AllowedOrigins` 机制本身供 CLI `--allowed-origin` 使用；不执行 Git 写操作。
- **通过条件**：三条规则各自的用例放行、公网 Origin 与 DNS rebinding 场景仍返回 403、显式白名单仍生效、既有 `TestWebSocketOriginPolicy` 不回归；WhaleTerm 前端测试与构建通过；`DECISIONS.md` 记录新决策并复核 D56 与 A4 §9。
- **当前状态**：代码与自动化验证已完成；等待用户在模拟器/真机复验连接。失败可观测性一项仍待决策（见下条记录）。
- **根因结论**：`CheckOrigin` 用精确字符串查表，任何未枚举到的 Origin 都被 gorilla 直接返回 HTTP 403，WebSocket 不建立；而浏览器对握手 403 只能观察到 `1006`，与网络不通、认证超时、服务端关闭同值，服务端又从不发送 WebSocket close 帧，因此该失败在客户端**不可诊断**。枚举方式本身不可维护：D56 已为 CLI `emulator` profile 补过两条，WhaleTerm 进程内路径仍漏了 `--dist` 的 `http://localhost` 与 `--live` 的 `http://10.0.2.2:5173`；真机 LAN IP 与自定义 Vite 端口还会继续漏。
- **实际完成项**：
  1. `internal/transport/server.go`：`Config` 新增 `AdvertiseURL` 字段；`CheckOrigin` 改为「显式白名单 ∪ 规则判定」；新增 `advertiseHost` 与 `isTrustedOriginHost`。规则忽略端口，放行 loopback（`localhost` / `net.ParseIP().IsLoopback()`，覆盖 `127.0.0.0/8` 与 `::1`）、`10.0.2.2`、以及等于 `AdvertiseURL` 主机名三类，其余拒绝。
  2. `remote/remote.go`：`tcfg.AdvertiseURL = config.AdvertiseURL`（该值此前只传给 `pairing.NewManager`，未进 transport）。
  3. WhaleTerm `frontend/src/utils/gpt/claudeRemotePairing.js` 删除 `CLAUDE_REMOTE_ALLOWED_ORIGINS`；`ClaudeRemoteManagement.vue` 删除该 import 与 `allowedOrigins:` 传参。`AllowedOrigins` 机制本身保留给 CLI `--allowed-origin`。
  4. `DECISIONS.md` 新增 **D59**，并显式声明替代 A4 §9「精确匹配允许列表」的表述（其余约束不变）。
- **实际验证**：TDD 先 RED——先观察到编译失败（`config.AdvertiseURL undefined`），随后**临时禁用规则**重跑，实测每条应放行的 Origin 都返回 **403 Forbidden / websocket: bad handshake**（`http://localhost`、`http://localhost:5174`、`http://127.0.0.1:8080`、`http://127.0.0.2:5173`、`http://[::1]:3000`、`capacitor://localhost`、`http://10.0.2.2:5174`、`http://192.168.1.10:5173`），确认 RED 是行为性失败而非仅编译失败；恢复规则后全部 GREEN。`go test ./internal/transport/... ./remote/... ./protocol/...` 全通过（含既有 `TestWebSocketOriginPolicy` 不回归）；新增 5 条用例覆盖三条规则、DNS rebinding 拒绝面（`http://evil.com`、`http://192.168.1.10.evil.com`、`http://10.0.2.2.evil.com`、`http://localhost.evil.com` 均 403）与显式白名单逃生门。`go vet` 通过、`gofmt -l` 无输出。WhaleTerm：`node --test frontend/tests/claudeRemoteManagement.test.mjs` → 10 pass / 0 fail；全量 `node --test frontend/tests/*.test.mjs` → **81 tests / 77 pass / 4 fail**，4 条失败与改动前基线完全一致，无新增回归；`frontend` 下 `npm run build` 通过（仅既有 chunk-size warning）。
- **未验证项**：模拟器与真机内的真实连接未复验，需用户重启 WhaleTerm 后重试；`AdvertiseURL` 的主机名规则依赖用户填对地址（填 `127.0.0.1` 时真机本就拨不通，属配置问题而非放行问题）。
- **三端可实现性**：纯服务端策略，客户端无需改动；React Web / Capacitor（Android `http://localhost`、iOS `capacitor://localhost`）/ HarmonyOS NEXT 均命中规则，核对通过（详见 D59）。
- **下一步**：用户重启 WhaleTerm，在模拟器内用 `ws://10.0.2.2:7331/ws` 复验配对；并决定是否补服务端拒绝路径日志（可观测性）。

- **子阶段**：W1 · 修复 Android 模拟器配对被 Origin 403 拒绝。
- **本轮任务**：修复 WhaleTerm 进程内 Remote Server 因 `allowedOrigins` 未覆盖 Android 壳页面 Origin，导致模拟器内 WebSocket 握手一律被 HTTP 403 拒绝的问题；并对「失败原因不可观测」做处置决策。
- **范围**：只改 WhaleTerm 前端组件传入的 `allowedOrigins` 取值与其纯函数常量；不改 `internal/transport` 的 `DefaultConfig()` 全局默认白名单（遵守 D56「不改全局默认白名单」）；不改协议、配对负载、`listenAddress`、鉴权与配对 wire contract；不执行 Git 写操作。
- **通过条件**：模拟器 `--live`（`http://10.0.2.2:5173`）与 `--dist`（`http://localhost`）两种 Origin 均被放行；桌面 Web dev（`http://127.0.0.1:5173`/`http://localhost:5173`）行为不回归；前端测试与构建通过。
- **当前状态**：Origin 修复已完成并通过自动化验证；真实模拟器连接待用户复验。失败可观测性一项待用户决策（见下）。
- **根因结论**：服务端用 gorilla `CheckOrigin` 对 `Origin` 头做**精确字符串匹配**（`internal/transport/server.go:75-81`），不匹配由 gorilla 直接返回 **HTTP 403**，WebSocket 不建立。WhaleTerm 组件此前硬编码 transport 默认值 `['http://127.0.0.1:5173','http://localhost:5173']`，而 Android 壳两种模式的页面 Origin 都不在其中：`--live` 经 `CAP_SERVER_URL=http://10.0.2.2:5173` 加载，Origin 为 `http://10.0.2.2:5173`；`--dist` 因 `androidScheme: 'http'` 加载本地 dist，Origin 为 `http://localhost`（无端口）。已用探针实测（未改产品代码）：前两条 101，后两条 403。D56 已为 CLI `emulator` profile 修过同一问题，WhaleTerm 进程内路径未对齐；D49 亦已把 `http://localhost` 是否入白名单标记为待 C2 决定。
- **实际完成项**：`frontend/src/utils/gpt/claudeRemotePairing.js` 新增导出 `CLAUDE_REMOTE_ALLOWED_ORIGINS`（4 条：桌面 dev 两条 + `http://localhost` + `http://10.0.2.2:5173`）并注明依据；`ClaudeRemoteManagement.vue` 改为引用该常量，不再内联数组。改动仅此两处 + 测试。
- **实际验证**：新增测试「放行 Android 壳两种模式的 Origin」先观察 RED（模块未导出该常量，`SyntaxError: does not provide an export named`）再实现转 GREEN；`node --test frontend/tests/claudeRemoteManagement.test.mjs` → 10 pass / 0 fail；全量 `node --test frontend/tests/*.test.mjs` → **81 tests / 77 pass / 4 fail**，4 条失败与改动前基线完全一致，无新增回归；`frontend` 下 `npm run build` 通过（仅既有 chunk-size warning）；`git diff --check` 通过。
- **未验证项**：模拟器内真实连接未复验（需用户重启 WhaleTerm 使前端改动生效后重试）；`10.0.2.2:5173` 之外的**自定义 Vite 端口**仍会 403，当前无 `--allowed-origin` 等价逃生门；真机 live reload 用 LAN IP 时同样不在白名单（`--dist` 的真机路径已由 `http://localhost` 覆盖）。
- **失败可观测性结论（重要）**：用户原选「客户端读 close code」经查证**无效**——服务端全模块从不发送 WebSocket close 帧（无 `CloseMessage`/`WriteControl`，仅裸 `socket.Close()`），且浏览器对握手 403 只报 `1006`，与网络不通、认证超时、服务端关闭同值，读 code 不增加任何信息。唯一能区分 403 的手段是服务端在拒绝时留日志；该模块当前**全程零日志**。待用户决定是否加。
- **下一步**：用户重启 WhaleTerm 重试模拟器连接并反馈；同时决定失败可观测性处置方式。

- **子阶段**：W1 · WhaleTerm 配对入口真实环境修正。
- **本轮任务**：修复 WhaleTerm Wails 应用内“远程管理”因使用 `window.prompt`/`window.alert` 无响应的问题；让移动端手工输入同时接受原始 Pairing JSON 与 WhaleTerm 展示的 `#pair=...` Fragment；明确并优化“手机可访问的 WebSocket 地址”的用途与模拟器填写提示。
- **范围**：WhaleTerm 使用现有 Naive UI 组件完成地址输入、错误提示和配对展示，不再调用浏览器原生 prompt/alert；移动端不改变 Pairing wire contract，只扩展手工输入解析入口；保留 `AdvertiseURL` 作为写入 Pairing payload 的客户端连接地址，继续与服务端 `0.0.0.0:7331` 监听地址分离；补充对应测试与 W1 文档，不执行 Git 写操作。
- **通过条件**：Wails 应用点击“远程管理”可见应用内配置界面并能启动；粘贴完整 `#pair=...` 可成功解析；Android 模拟器可按界面提示使用 `ws://10.0.2.2:7331/ws`，真机/LAN/VPN 可填写 PC 可达 IP；错误通过应用内消息展示；前端测试、构建及相关 Go 回归通过。
- **当前状态**：代码与自动化验证已完成；真实 WhaleTerm + Mobile Web 人工验收待用户执行，完成前不宣称 W1 人工门禁通过。
- **根因结论**：移动端手工配对表单当前只调用 `parsePairingJson`，因此把 WhaleTerm 输出的 `#pair=...` 当 JSON 必然报“配对信息无效”；WhaleTerm 入口直接调用 `window.prompt`/`window.alert`，Wails WebView 内该交互不可靠且无应用内反馈；`AdvertiseURL` 不是服务端绑定地址，而是写入 Pairing payload、供手机/模拟器主动连接 PC 的地址，无法一律使用 `127.0.0.1`（Android 模拟器需用 `10.0.2.2`，真机需用 PC 的 LAN/VPN 地址）。
- **实际完成项**：
  1. **移动端手工输入统一入口**：`apps/mobile/src/protocol/pairing.ts` 新增 `parsePairingInput(text, now)`，`#pair=` 开头走既有 `parsePairingFragment`，其余走既有 `parsePairingJson`，两种输入共用同一套 `validatePayload`；`App.tsx` 手工提交改调该入口；`PairingForm` 文案改为“配对 JSON 或完整 #pair Fragment”。未改 Pairing wire contract、校验规则与 token 处理，未写入任何浏览器存储。
  2. **WhaleTerm 地址校验抽为纯函数**：新增 `frontend/src/utils/gpt/claudeRemotePairing.js`，`normalizeClaudeRemoteAdvertiseUrl` 接受 `ws://`/`wss://`、拒绝空值与 `0.0.0.0`（服务端监听地址不可拨号）并给出可操作提示；默认值 `CLAUDE_REMOTE_DEFAULT_ADVERTISE_URL = 'ws://127.0.0.1:7331/ws'` 保持既有本机开发语义。
  3. **WhaleTerm 远程管理拆为独立组件**：新增 `frontend/src/views/chat/components/ClaudeRemoteManagement.vue`，把入口按钮、配对内容展示与复制、启动弹框（`n-modal` + `n-input`）、全部 `$message` 反馈与相关样式整体迁入；组件内部用 `isLocalClaude(chatKey)` 判定是否渲染，父组件一行即可接入。`index.vue` 的净改动为 **+5 行**（`v-model:session-id` / `:chat-key` / `:listen-key`），删除原有 5 个 ref、4 个函数、3 段样式与 2 处 import；`window.prompt`/`window.alert` 已彻底移除。
  4. **会话 ID 回写语义不变**：组件内 `onChatSession` 与 chat 视图原实现逐字一致——仅在 sessionId 变化时 emit `update:sessionId` 并调用 `updateHistoryConfig(listenKey, { session_id })`；`listenKey`/`sessionId` 由父组件传入（二者是 chat 视图局部状态，不在 store）。
  5. **接口保留**：`StartClaudeRemoteManagement`/`StopClaudeRemoteManagement`、`listenAddress: '0.0.0.0:7331'`、`allowedOrigins: ['http://127.0.0.1:5173', 'http://localhost:5173']`、`agentId = listenKey || chatKey`、`claudeConfig` 构造与 `stream/messages` 语义均未改动。
- **实际验证**：WhaleTerm 新增 `frontend/tests/claudeRemoteManagement.test.mjs`（9 条：地址校验行为 + 组件源码断言，先观察失败再实现）通过；`node --test frontend/tests/*.test.mjs` → **80 tests / 76 pass / 4 fail**，4 条失败与改动前基线完全一致（`claudeMcpDialogLayout`、`claudeMcpManagement`（`@/assets` 别名导致模块找不到）、`colorThemeGenerator`、`projectCloudSetting`），无新增回归；`npm run build` 通过（保留仓库既有 CSS `:deep()` 与 chunk-size warning）。本项目 `apps/mobile`：`npm test` → **16 文件 408 passed**；`npm run lint` 0 error（仅既有 `scripts/cdp-console.mjs:76` warning）；`npm run build`（`tsc -b && vite build`）通过。两个仓库 `git diff --check` 均通过。
- **未验证项**：WhaleTerm `npm run lint` 在当前仓库**改动前即不可用**（eslint 9.39 需 flat config，仓库只有 `.eslintrc.cjs`，且其引用的 `.eslintrc-auto-import.json` 未被 `unplugin-auto-import` 生成）——本轮未修复该既有问题，WhaleTerm 侧仅以 `node --test` 与 `vite build` 作为自动化门禁；WhaleTerm 组件在真实 Wails WebView 内的观感与交互、`10.0.2.2` 提示文案的实际可用性、以及真实 Claude 会话的端到端配对未做人工验收。
- **下一步**：用户在 WhaleTerm 中打开本地 Claude 会话 → 点“远程管理” → 在应用内弹框按提示填地址（模拟器 `ws://10.0.2.2:7331/ws`，真机填 PC 可达地址）→ 复制配对 Fragment → 在 Mobile Web 手工粘贴完整 `#pair=...` 连接，反馈“通过”或具体问题；AI 再记录人工结论并关闭 W1。

- **子阶段**：W1 · WhaleTerm 首轮前远程配对修正。
- **本轮任务**：推翻“只有已运行且已有 `sessionId` 的 WhaleTerm 会话才能开启远程管理”的错误限制，使新建但尚未发送首条消息的本地 Claude 会话可预分配真实 UUID、先配对，再由 PC 或手机首条输入启动同一个 Claude Session；同时将 WhaleTerm 首轮 `session-id` 参数接入从 `WithExtraArgs` 迁移到 SDK 正式的 `WithSessionID`。
- **范围**：更新已确认的 W1 设计/计划/决策；调整 WhaleTerm 前端入口、会话预分配与 Remote Host 配置复用；保持唯一 Host/唯一 SDK Client、PC/手机统一 FIFO、既有 wire contract 和关闭远程不停止 Session 的约束；不执行 Git 写操作。
- **通过条件**：未发首条消息的本地 Claude 会话可打开远程管理并取得稳定真实 `sessionId`；PC 或手机任一端可提交首条消息且只创建一个 SDK Client；已有会话行为不回归；首轮使用 `WithSessionID`；新增/更新 Go 与前端测试、构建和 W1 回归门禁通过。
- **当前状态**：实现与自动化门禁已完成，等待用户执行首轮前真实 Claude + Mobile Web 验收；完成前不宣称 W1 总门禁通过。
- **实际完成项**：新增 D58 并修订 W1 设计/计划；WhaleTerm 远程入口不再依赖已有 `sessionId`，开启时可通过现有 Manager 预分配稳定 UUID；Remote Host 接受 `IsStarted=false` 的预备 Session、返回 `starting/idle`，并保存完整 `ClaudeCodeChat` 模板供手机首条输入复用；配对返回 ID 会写回当前前端会话。首轮 SDK 参数改用正式 `WithSessionID`，不再把 `session-id` 放进 `WithExtraArgs`。
- **实际验证**：WhaleTerm W1 Host/SessionID option 定向测试通过；对应 `go test -race -vet=off` 通过；`backend/remote` race 测试通过；`backend/services` 编译通过；前端 `npm run build` 通过（仅既有 `:deep()` warning）；本项目 `GOFLAGS=-buildvcs=false go test -race -count=1 ./protocol ./remote`、`go vet ./protocol ./remote` 和 `git diff --check` 通过。公共 Remote 首次在受限沙箱内因禁止绑定临时端口失败，沙箱外重跑通过。
- **依赖边界**：WhaleTerm 提交基准仍依赖 SDK v0.1.1（无 `WithSessionID`）；本地 `go.mod` 已增加指向 `/Users/zhangym/code/work/utils/claude-agent-sdk-go` 的开发态 replace 以完成验证。发布/提交前需发布含正式 API 的 SDK 版本、更新 WhaleTerm 依赖基准并移除本地 replace。
- **下一步**：用户按 `docs/testing/W1-result.md` 先验证“新建后不发消息 → 开启远程 → 手机发送首条消息”，再验证运行中接力；反馈“通过”或具体问题，AI 记录人工结论并关闭 W1。

- **子阶段**：W1 · WhaleTerm 进程内 Remote Server，代码完成，等待人工验收。
- **本轮任务**：完成 Task 3–6：真实 Session Host、Remote Server 生命周期、配对展示、自动化测试。
- **范围**：WhaleTerm 复用现有 SDK Client、Turn FIFO、权限与取消逻辑；提供当前会话“远程管理”入口和可复制 Pairing Fragment；不修改 wire contract、不创建第二个 Client、不引入 PTY/JSONL 接管、不执行 Git 写操作。
- **当前状态**：W1 代码与自动化验证完成；Task 7 真实 WhaleTerm + Mobile Web 人工验收待用户执行，完成前不宣称 W1 总门禁通过。
- **实际完成项**：WhaleTerm `backend/chat/claude_remote_session_host.go` 将 Submit/Snapshot/Subscribe/审批/AskUserQuestion/当前 Turn 取消/排队取消连接到现有 Manager；Manager 事件同时发送给 Wails 与 Remote Hub。`backend/remote` 增加固定 Session Controller 与进程级单实例 Service，重复开启复用、跨 Session 拒绝、关闭只释放 Remote Server/Host。`TClaudeCodeService` 暴露启停/查询 API；聊天页增加远程管理入口、可达 WebSocket 地址输入、Pairing Fragment 展示与复制。开发态依赖继续使用 WhaleTerm 本地 `go.mod` replace，未修改 `go.mod.base`。
- **实际验证**：WhaleTerm Host/Adapter/生命周期定向测试通过；对应 `go test -race -vet=off` 通过；前端 `npm run build` 通过（保留仓库既有 CSS `:deep()` 与 chunk-size warning）；本项目 `GOFLAGS=-buildvcs=false go test -race -count=1 ./protocol ./remote` 与 `go vet ./protocol ./remote` 通过。详细证据见 `docs/testing/W1-result.md`。
- **已知验证边界**：WhaleTerm `backend/chat` 默认 vet 仍被既有 Go 1.25 告警阻断，因此定向测试使用 `-vet=off`；真实 Claude 会话、扫码/粘贴配对、手机输入/审批/问答、关闭重开与单 Client 进程观察由用户验收。
- **下一步**：用户按 `docs/testing/W1-result.md` 的清单验收并反馈“通过”或具体问题；AI 再记录结果并决定是否关闭 W1。

- **子阶段**：交接文档维护。
- **本轮任务**：解决远程拉取后 `HANDOFF.md` 中 Windows 与 macOS 开发者脚本进度产生的冲突。
- **范围**：仅合并 `HANDOFF.md` 的冲突内容；保留两侧已完成记录，不删除工作区其他改动，不执行任何 Git 写操作。
- **通过条件**：`HANDOFF.md` 不再包含冲突标记；Windows 与 macOS 两条脚本维护记录均完整保留；`git diff --check` 无空白错误。
- **当前状态**：冲突已按确认方案合并；只读检查通过，未发现残留冲突标记或空白错误。

- **子阶段**：Windows 开发者脚本维护。
- **本轮任务**：修复 `scripts/windows/4_web_build.ps1` 调用 `apps/mobile` 测试时，Vitest 在 Windows 下找不到 `src/setupTests.ts`、导致 16 个测试套件全部未执行的问题。
- **范围**：仅调整 `apps/mobile` 的测试配置与必要说明；不改移动端业务行为、不跳过测试、不放宽构建门禁。
- **通过条件**：`npm run test` 在 `apps/mobile` 下恢复正常加载 `setupTests.ts` 并执行现有测试；`4_web_build.ps1` 或其等价命令链能继续跑过测试、lint、build。
- **当前状态**：已完成修复并通过完整门禁；`4_web_build.ps1` 在 Windows 工作区入口路径下恢复可用。
- **实际完成项**：`apps/mobile/vite.config.ts` 新增 `resolve.preserveSymlinks = true`，显式保留当前工作区的 `C:` 入口路径，避免 Vite/Vitest 在 Windows junction 环境下把根目录归一到真实 `D:` 路径；`test.setupFiles` 调整为项目根相对路径 `/src/setupTests.ts`。同时将 `apps/mobile/package.json` 的 `vite` 版本从 `^8.2.0` 收紧为精确 `8.1.5`，并通过 `npm install -D vite@8.1.5` 同步更新 `package-lock.json` 与本地依赖，避免后续 `npm install` 再次抬回存在回归的 8.2.x。
- **根因结论**：当前仓库通过 `C:\MyWork\...` 入口访问，但真实目录位于 `D:\MyWork\...`；原来的 Vite 8.2.2 + Vitest 4.1.11 在此类 Windows symlink/junction 场景下会把测试根目录与 `setupFiles` 解析到错误路径，先后表现为 `/src/setupTests.ts` 与 `/@fs/D:/.../setupTests.ts` 无法加载。
- **实际验证**：`npm run test` 已恢复通过，结果为 **16 个测试文件、405 个测试全部通过**；随后两次执行 `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/windows/4_web_build.ps1`，均完整跑过 `npm run test`、`npm run lint`、`npm run build`。其中 lint 仅保留 1 个既有 warning（`scripts/cdp-console.mjs:76` 的 `eslint(no-unused-expressions)`），无 error；生产构建通过，产物体积为 `dist/assets/index-CksCmMak.js` 492.41 kB / gzip 155.69 kB。
- **未处理项**：测试输出仍存在既有运行期 warning，包括 antd-mobile 的 `Please make sure you have imported antd-mobile/es/global` 和若干 React `act(...)` 提示；这些 warning 未阻塞本轮脚本修复，但后续若要收敛测试输出可单独处理。

- **子阶段**：Windows 开发者脚本维护。
- **本轮任务**：修复 `scripts/windows/1_install_base_tools.ps1` 在 `go mod tidy` 阶段因外部 `GOPROXY/GOSUMDB` 指向不稳定镜像而失败的问题，为基础工具安装补齐可回退的 Go 依赖下载策略。
- **范围**：仅调整 `scripts/windows` 下与 Go module 安装相关的脚本和说明；优先保证当前 PowerShell 进程内自恢复，不改仓库 Go 依赖、不改业务代码、不引入 Git 写操作。
- **通过条件**：Windows 基础安装脚本在存在异常 `GOPROXY/GOSUMDB` 环境变量时，仍能使用脚本内回退策略完成 `go mod tidy`，或至少明确输出已尝试的回退链与最终失败原因；帮助命令和轻量验证可证明新逻辑生效。
- **当前状态**：已完成脚本修复并通过受控验证；真实联网环境下的 `go mod tidy` 尚未在本轮重跑。
- **实际完成项**：在 `scripts/windows/_common.ps1` 新增 `Set-ProcessEnvironment` / `Restore-ProcessEnvironment` / `Get-GoEnvironmentProfiles` / `Invoke-GoWithFallback` 等 helper；`1_install_base_tools.ps1` 的 `go mod tidy` 改为经由回退链执行。新逻辑先继承当前 Go 环境尝试一次，失败后自动切到 `GOPROXY=https://proxy.golang.org,direct`、`GOSUMDB=sum.golang.org`，并在每次尝试后恢复原进程环境变量。
- **实际验证**：`powershell -File scripts/windows/1_install_base_tools.ps1 -ShowHelp` 正常输出用法；另在 `%TEMP%/cmr-go-fallback-test` 生成 mock `go.cmd` 做受控验证，模拟“继承环境使用 `https://goproxy.cn,direct` 失败、官方代理成功”的场景，`Invoke-GoWithFallback` 已按预期完成两次调用、第二次成功，且 `GOPROXY/GOSUMDB` 在函数返回后恢复原值。
- **未验证项**：未在真实外网环境中完整重跑 `go mod tidy + npm ci`；若用户网络对 `proxy.golang.org` 不可达，仍需根据实际环境再补充额外镜像策略。

- **子阶段**：开发者自动化脚本 Bug 修复。
- **本轮任务**：修复 macOS `scripts/macos/3_android_run.sh` 执行 `installDebug` 后不启动 App 的问题。
- **范围**：仅改 `scripts/macos/3_android_run.sh`，在 `installDebug` 后追加 `adb shell am start -n com.tea4go.claudemobileremote/.MainActivity`，对齐 Windows `3_android_run.ps1` 的 `Start-AndroidApp` 步骤；README 无需改动（其本就声明「自动拉起」，改的是脚本行为对齐文档）。
- **通过条件**：`bash -n` 静态检查通过；`--dist` / `--live` 执行后 App 被拉起、页面显示；`git diff --check` 无空白错误。
- **当前状态**：已完成修复并实测验证通过。
- **实际完成项**：`scripts/macos/3_android_run.sh` 在 `installDebug` 后追加 `"$adb_bin" shell am start -n com.tea4go.claudemobileremote/.MainActivity`（`adb_bin` 已在脚本第 25 行赋值，`ANDROID_SERIAL` 由 `start_android_avd` export，live/dist 两个模式共用该行）。
- **实际验证**：`bash -n` 通过；`git diff --check` 无空白错误；对运行中的 `emulator-5554` 实测 `am start` → logcat 出现 `Displayed com.tea4go.claudemobileremote/.MainActivity`，进程存活无 FATAL，`topResumedActivity` 为该 Activity，页面正常显示。
- **未执行**：任何 Git 写操作。

- **子阶段**：开发者自动化脚本 + 文档重构（README 与脚本命名）。
- **本轮任务**：为 `scripts/windows` 下 8 个主入口 `ps1` 脚本补齐同名 `.bat` 包装脚本，便于 Windows 用户双击或直接通过 cmd 启动；实现参考 `wireguard-harmony/build-app.bat`，保持参数透传与暂停查看输出行为。
- **范围**：新增 `1_install_android_sdk.bat`、`1_install_base_tools.bat`、`2_check.bat`、`3_android_run.bat`、`3_cli_run.bat`、`3_web_dev.bat`、`4_android_build.bat`、`4_web_build.bat`；不改动现有 `.ps1` 逻辑，不为 `_common.ps1` 和 `toggle-hypervisor.ps1` 增加 `.bat` 包装层，不改变脚本编号、CLI 参数语义或协议/网络行为。
- **通过条件**：8 个主入口 `.bat` 与同名 `.ps1` 一一对应；批处理可正确定位同目录 `ps1`、透传 `%*` 参数并保留 `pause`；帮助命令或等价轻量调用可证明 `.bat -> .ps1` 调用链可用；`git diff --check` 通过。
- **当前状态**：8 个 `.bat` 包装脚本已新增并完成帮助命令验证；`scripts/windows` 的主入口现在同时支持 `.ps1` 与 `.bat` 调用；未执行 Git 写操作。
- **实际完成项**：新增 `1_install_android_sdk.bat`、`1_install_base_tools.bat`、`2_check.bat`、`3_android_run.bat`、`3_cli_run.bat`、`3_web_dev.bat`、`4_android_build.bat`、`4_web_build.bat`。8 个文件均采用与 `wireguard-harmony/build-app.bat` 一致的薄包装模式：`powershell.exe -ExecutionPolicy Bypass -File "%~dp0<同名>.ps1" %*`，并保留 `pause` 便于双击查看输出。
- **实际验证**：先用 `rg --files ... -g "*.bat"` 确认新增前目录中不存在 `.bat`；新增后用 `Get-ChildItem scripts/windows -Filter *.bat` 确认 8 个文件全部到位。`rg -n --glob "*.bat" "powershell\\.exe -ExecutionPolicy Bypass -File|pause" scripts/windows` 核对 8 个批处理都包含统一包装逻辑；`rg -n --glob "*.ps1" "Alias\\('Help'\\)|ShowHelp|用法:" scripts/windows` 确认 8 个主入口 `ps1` 都支持帮助参数。随后实际执行 8 个 `.bat` 的 `-Help` 轻量调用，全部成功输出对应 `ps1` 用法并进入 `Press any key to continue . . .`；`git diff --check -- HANDOFF.md scripts/windows` 仅提示 `HANDOFF.md` 的 LF/CRLF 工作区转换 warning，无空白错误。
- **实际完成项**：新增 macOS Bash 与 Windows PowerShell 的共享 helper、环境检查、基础工具安装、Android SDK 安装、Web 开发/构建、CLI、Android live/dist/build 入口；macOS 额外提供 iOS Simulator 运行/构建；README 增加新 pull 三步路径、Server 参数表、命令表、排障和安全边界。Android 目标 AVD 通过 serial 绑定，live/dist 恢复环境变量，CLI 日志隐藏参数。脚本文件名统一为 `N_role` 前缀（1=安装、2=检查、3=运行、4=构建），`_common`、`toggle-hypervisor` 为共享/辅助脚本不参与排序；文档（README、advanced-usage、三份 plan/spec、验证记录）中的旧脚本名引用全部更新。
- **实际验证**：macOS `bash -n`、全部 `--help`、基础 `check`、`check --android` 通过；Capacitor CLI 版本核对通过；Mobile Web 405 个测试、lint、build 通过；`git diff --check` 通过。Go 全量测试受当前沙箱禁止 httptest 绑定本地端口影响，Windows PowerShell parser/实机流程、Android 安装和 iOS Simulator 流程未执行。
- **当前结论**：脚本实现与文档完成，验证记录见 `docs/testing/developer-automation-scripts-result.md`。Windows 侧已实测闭环（9 个 `.ps1` 补 UTF-8 BOM 并修复七类 PowerShell 5.1 解析/StrictMode/启动超时问题，`2_check.ps1` 端到端通过）；Android 模拟器「卡死」根因已定位并解决（默认 virtio-wifi 拉起宿主 `netsimd.exe` 崩溃阻塞 QEMU，WHPX/AEHD/TCG 一律卡死），`_common.ps1` 的 `Start-AndroidAvd` 已用 `-feature -Wifi -feature -VirtioWifi` 禁用，android-35 完整启动（adb `device`、`sys.boot_completed=1`）。WHPX 为推荐后端，`toggle-hypervisor.ps1` 保留四模式。Android 构建需 JDK 21–24，`_common.ps1` 的 `Require-AndroidEnvironment` 会自动选用兼容 JDK（优先 Android Studio JBR=JDK 21）。

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
- Android 壳需 `server.androidScheme: 'http'` 才能连 `ws://` 明文（D49）；dist 加载路径下壳内 Origin 为 `http://localhost`（无端口），live reload 下为 dev server 地址（如 `http://10.0.2.2:5173`）；`emulator` profile 已内置 `http://localhost` 与 `http://10.0.2.2:5173`（live 默认端口，D56），自定义 Vite 端口才需 `--allowed-origin`；
- 桩服务必须以管道方式启动（CLI 读 stdin，`nohup ... &` 会因 EOF 立即退出）：`tail -f /dev/null | /tmp/cmr ... --allowed-origin http://localhost:5174`；
- antd-mobile 5.42.3 部分传递依赖与 React 19 peer 范围不一致，用户已评估可忽略；
- 本地沙箱禁止绑定 `[::1]:0`，`internal/transport` 与 `cmd/channel-test-client` 的部分 httptest 用例可能因此失败，属环境限制而非回归。

## 7. 参考提醒

WhaleTerm 的旧远程控制方案、阶段和记忆不读取也不继承。当前只参考 SDK 已验证用法；进入 W1 后再按已确认的单 Session 边界检查 WhaleTerm 当前 Claude manager/client 接口。
