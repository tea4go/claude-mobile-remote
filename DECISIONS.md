# DECISIONS.md — Claude Mobile Remote 决策日志

> 只追加或替代正式决策，不记录普通开发进度。每条包含日期、决策、理由、影响和状态。

## D1 · 建立独立项目

- **日期**：2026-08-21
- **决策**：项目独立于 WhaleTerm，在 `claude-mobile-remote` 仓库重新设计和开发。
- **理由**：产品定位已变为独立 CLI 托管 Claude，不再是 WhaleTerm 扩展。
- **影响**：WhaleTerm 的旧方案、阶段、记忆和 Remote Agent 代码不继承。
- **状态**：已定

## D2 · 使用 claude-agent-sdk-go 托管 Claude

- **日期**：2026-08-21
- **决策**：使用 `github.com/tea4go/claude-agent-sdk-go` 作为 Claude 接入层。
- **理由**：SDK 由用户自行维护，已在 WhaleTerm 验证使用，覆盖 Claude 完整生命周期（流式回复、Tool、权限审批、问答交互、中止/恢复及管道日志），并可自主修复缺陷；本项目仍需在 A2 验证独立集成及 Claude 上游兼容性。
- **影响**：Service 从会话开始时持有 Claude，不接管任意外部 Claude 进程。
- **状态**：已定

## D3 · 首期只做单会话接力

- **日期**：2026-08-21
- **决策**：首期手机通过扫码绑定一个正在进行的 `sessionId`。
- **理由**：优先验证用户离开电脑后的会话接力价值，减少历史和设备管理复杂度。
- **影响**：多会话和多 PC 后置。
- **状态**：已定

## D4 · 不同步扫码前完整历史

- **日期**：2026-08-21
- **决策**：手机连接时只返回当前状态、待处理事项和配对后的新增消息。
- **理由**：产品不是历史管理器，也可降低首期同步和缓存成本。
- **影响**：状态快照必须足以解释当前任务，但不包含完整 transcript。
- **状态**：已被 D13 澄清

## D5 · 局域网 WebSocket 与 session 通道

- **日期**：2026-08-21
- **决策**：首期手机通过局域网 WebSocket 连接 Service，并订阅指定 `sessionId` 的逻辑消息通道。
- **理由**：WebSocket 支持实时事件和双向操作；操作系统 Pipe 不能作为手机网络协议。
- **影响**：公网中转最低优先级，协议需保留未来更换传输层的边界。
- **状态**：部分替代（网络前提见 D12；Channel 模型见 D10）

## D6 · 简单 CLI 与浏览器优先

- **日期**：2026-08-21
- **决策**：首期使用简单行输入输出和移动浏览器，不复刻完整 Claude Code 终端，也不先做原生 App。
- **理由**：最快验证 Claude、终端和手机的通信闭环。
- **影响**：Capacitor、扫码插件、语音、推送和复杂生命周期后置。
- **状态**：已定

## D7 · 移动端技术路线

- **日期**：2026-08-21
- **决策**：移动 Web 使用 React + TypeScript + antd-mobile + Vite；iOS / Android 使用 Capacitor；HarmonyOS NEXT 使用 ArkTS + ArkUI 独立开发。
- **理由**：iOS / Android 可共享成熟移动 Web UI，HarmonyOS NEXT 按其原生体系独立适配。
- **影响**：所有设计和协议兼顾三端，但实施顺序为 Web → iOS / Android → HarmonyOS NEXT。
- **状态**：已定

## D8 · AI 长期记忆文件

- **日期**：2026-08-21
- **决策**：使用 `AGENTS.md`、`CLAUDE.md`、`HANDOFF.md` 和 `DECISIONS.md` 支持 Codex、Claude 等 AI 长周期协作。
- **理由**：文件作为长期记忆，避免依赖单个对话上下文。
- **影响**：子阶段门禁、目标变化或会话移交时更新 HANDOFF；仅在用户确认决策变化时更新 DECISIONS；CLAUDE 引用 AGENTS，避免规则重复。
- **状态**：已被 D17 补充

## D9 · 使用里程碑与子阶段门禁

- **日期**：2026-08-21
- **决策**：开发规划采用“里程碑 + 可独立验证的子阶段”；每次实施计划只展开当前子阶段。
- **理由**：避免一个阶段同时包含 Service、前端和联调，降低 AI 长会话与问题定位成本。
- **影响**：首期拆为 A1 项目骨架、A2 SDK 托管、A3 Session Core、A4 Channel、A5 Mobile Web、A6 VPN 联调；前一门禁未通过不得进入下一阶段。
- **状态**：已定

## D10 · 显式 Session Channel 订阅

- **日期**：2026-08-21
- **决策**：手机完成 WebSocket 鉴权后，通过 `subscribe(sessionId)` 显式订阅 Session Channel；事件和操作都经该 Channel 路由。
- **理由**：将“订阅消息”落实到架构与协议，并为多会话保留清晰边界。
- **影响**：协议增加 `subscribe`、`subscribed`、`unsubscribe`；Channel 不保存完整会话，也不运行 Claude。
- **状态**：已定

## D11 · AI 禁止执行 Git 操作

- **日期**：2026-08-21
- **决策**：AI 只负责修改文件、构建和测试，不得执行任何 Git 命令；用户负责检查和提交。
- **理由**：由用户掌握版本状态和提交边界。
- **影响**：后续 AI 不运行 `git status`、`git diff`、暂存、提交、推送、分支或恢复命令。
- **状态**：已被 D20 替代

## D12 · 手机通过 VPN 直连 PC

- **日期**：2026-08-21
- **决策**：手机默认连接 VPN，并可直接访问 PC Service；首期不处理局域网环境差异。
- **理由**：VPN 已提供稳定的手机到 PC 网络可达性，无需为局域网发现、防火墙差异或 NAT 穿透增加复杂度。
- **影响**：二维码使用 VPN 内可达的 PC Service 地址；A6 改为 VPN 手机联调；公网中转只解决未来脱离 VPN 的独立访问。本决策替代 D5 的局域网前提。
- **状态**：已定

## D13 · 完整上下文与操作保留在 PC

- **日期**：2026-08-21
- **决策**：Claude 的完整上下文、会话状态和操作执行始终留在 PC；手机只展示 PC 推送的数据并回传消息、审批等操作。
- **理由**：手机是远程操作界面，不是独立 Claude 客户端，无需保存或重建 Claude 上下文。
- **影响**：手机配对时不下载完整历史；snapshot 只用于界面同步与断线恢复，所有操作仍由 PC 根据当前会话状态执行。本决策澄清 D4。
- **状态**：已定

## D14 · A1 只创建 Go 与移动 Web 公共骨架

- **日期**：2026-08-21
- **决策**：A1 只创建 Go CLI、React/Vite 与基础工程配置；Capacitor iOS / Android 工程在 C1 创建，HarmonyOS NEXT 工程在 D1 创建。
- **理由**：首期先验证浏览器通信闭环，避免原生工具链阻塞 A1，并保持 Web → iOS / Android → HarmonyOS NEXT 的实施顺序。
- **影响**：A1 门禁只验证 Go 与移动 Web；React 项目使用 `apps/mobile/`，C1 在其下增加 Capacitor 平台工程，D1 再创建 `apps/harmony/`。
- **状态**：已定

## D15 · Session 与 Turn 分层

- **日期**：2026-08-21
- **决策**：Session 状态与单轮 Turn 状态分开管理；`ResultMessage` 只结束当前 Turn，不结束 Session。
- **理由**：Claude 会话支持多轮交互，一轮输出完成后仍需继续接收终端或手机消息。
- **影响**：Session 使用 `starting / ready / stopped / failed`，Turn 使用 `idle / running / waiting / completed / cancelled / failed`；只有 Session 停止或失败后才拒绝新操作。
- **状态**：已定

## D16 · A 阶段手机只收发文本

- **日期**：2026-08-21
- **决策**：里程碑 A 的手机端只接收实时文本并发送文本消息；审批、回答和取消在 B2 实现。
- **理由**：避免在最小通信闭环中提前引入 Tool、权限和复杂任务控制。
- **影响**：A4 只实现文本命令回传；B2 复用同一命令链路增加 `actionId` 与对应操作类型。
- **状态**：已定

## D17 · 开发记录由 AI 自动维护

- **日期**：2026-08-21
- **决策**：每项开发任务由 AI 在开始前登记 `HANDOFF.md` 的“当前执行”，结束前记录完成项、验证、未验证项和下一步；`docs/testing/` 只用于子阶段门禁和重要联调。
- **理由**：保证多人和多 AI 会话可以接续，同时避免为普通任务编写无人使用的测试流水账。
- **影响**：本决策补充 D8 的更新频率；用户只负责需求与决策确认、体验验收和 Git 操作授权；人工验收由 AI 提供清单并代为记录，用户不需要手工维护记忆文档。Git 执行边界见 D20。
- **状态**：已定

## D18 · go.mod 不入库，提交 go.mod.base 基准

- **日期**：2026-08-21
- **决策**：`.gitignore` 忽略 `go.mod` 与 `go.sum`，仓库提交基准文件 `go.mod.base`；克隆后 `cp go.mod.base go.mod && go mod tidy` 恢复构建。
- **理由**：与 WhaleTerm 同模式。SDK 由用户自维护，开发期需要在 `go.mod` 使用指向本地 SDK 的 `replace`，此类本机改动不应进入仓库。
- **影响**：仓库不锁定传递依赖版本，克隆构建依赖当时 `go mod tidy` 的解析结果；依赖正式变更时由 AI 同步更新 `go.mod.base` 并交用户确认提交范围。README 快速开始与 AGENTS.md 开发规范已同步该规则。
- **状态**：已定

## D19 · 三端共用契约与兼容性核对

- **日期**：2026-08-21
- **决策**：定义三端共用契约（通信协议、状态模型、配对负载、验收用例，清单见总体设计 §8.1）；任何协议或状态变更必须核对三端（含 HarmonyOS NEXT）可实现性，核对结论记入本文件后方可实施。
- **理由**：用户明确要求保证所有规划适用于 HarmonyOS NEXT，防止前两端收尾后发现无法兼容而被迫换方案。原 D7/REQUIREMENTS §七只有意图表述，缺少可执行的契约边界与核对机制。
- **影响**：本决策补充 D7，升格为 AGENTS.md 硬约束 11；协议与状态相关变更的开发流程增加三端核对步骤；UI 框架、路由、推送、生命周期等端自主选择项不在契约内，互不约束。
- **状态**：已定

## D20 · Git 操作按用户明确授权执行

- **日期**：2026-08-21
- **决策**：AI 默认不得主动执行任何 Git 命令；当用户明确要求执行 Git 时，AI 可在该次明确授权范围内运行完成该操作所必需的检查与 Git 命令。一次授权不自动延伸到后续提交、推送、恢复、分支或其他 Git 操作。
- **理由**：用户希望保留对 Git 操作时机和范围的控制，同时允许在明确委托时由 AI 完成具体 Git 工作。
- **影响**：本决策替代 D11；AGENTS.md 硬约束 9、README 与 HANDOFF 的职责表述同步调整。当前用户已进一步明确本次授权全部 Git 操作；实际仍按任务目标只提交 A1，不因授权范围扩大而主动执行无必要的推送、合并、恢复或分支操作。
- **状态**：部分被 D23 替代

## D21 · A2 使用薄 SDK Adapter 验证单 Client 多 Turn

- **日期**：2026-08-21
- **决策**：A2 在 `internal/claude` 建立薄 SDK Adapter，由其持有一个长生命周期 `claude-agent-sdk-go` Client，转换 A2 所需事件并验证真实 `sessionId` 与顺序多 Turn；CLI 不直接处理 SDK Message，A2 不提前实现 A3 Session Controller。
- **理由**：隔离 SDK 生命周期和消息细节，保留可测试边界，同时避免把状态机、输入队列和网络协议提前混入 A2。
- **影响**：SDK 类型限制在 `internal/claude`；A2 CLI 只做同步行输入；A3 在 Adapter 之上增加 Session/Turn 状态与串行队列。本决策不修改三端共用契约。
- **状态**：已定

## D22 · 设计文档先由用户检查再决定提交方

- **日期**：2026-08-21
- **决策**：AI 起草设计文档后直接交用户检查，不执行“AI 自查并自动提交设计文档”。用户检查完成后，只有用户明确要求 AI 执行 Git 提交时才由 AI 提交；否则由用户自行提交。
- **理由**：用户需要在设计进入 Git 历史前掌握内容检查与提交边界，避免 AI 将内部自查等同于用户验收。
- **影响**：本决策补充 D20，并覆盖工具或 Skill 中要求 AI 自动自查、提交设计文档的默认流程；代码与门禁验证仍按既有质量规则执行。A2 设计提交 `e225162` 发生在本决策确认之前，不自动回退。
- **状态**：已定

## D23 · AI 可主动使用只读 git diff

- **日期**：2026-08-21
- **决策**：AI 可在无需逐次授权的情况下主动运行只读 `git diff`（含 `--check`、`--stat` 等 diff 变体）检查工作区改动；暂存、提交、推送、恢复、分支、合并及其他会改变仓库或远端状态的 Git 操作仍必须由用户明确授权。其他未明确允许的 Git 只读命令仍按授权制执行。
- **理由**：`git diff` 是开发与评审中的必要只读检查，不改变仓库状态；用户仍需掌握提交及其他状态变更边界。
- **影响**：本决策替代 D20 中“AI 不得主动运行任何 Git 命令”的绝对限制，并同步修改 AGENTS.md 硬约束 9、README 与 HANDOFF；D22 的设计检查与提交规则不变。
- **状态**：已定

## D24 · A3 使用单 Actor Session Core 与纯数据 snapshot

- **日期**：2026-08-24
- **决策**：A3 的单 `SessionController` 使用一个 Actor 独占 Session/Turn 状态、容量为 32 的等待 FIFO、request ID 和事件 `seq`，由一个 Host Worker 串行调用 A2 Adapter。`Submit` 接受后立即返回服务端生成的 128-bit 随机 request ID，执行结果异步发布。Snapshot 细化为真实 `sessionId`、Session 状态、可空 Session 错误、`seq`、当前或最近 Turn（状态、request ID、来源、消息、可空错误和可空 `pendingAction`）及 `queueDepth`；主动停止可把运行中 Turn 置为 `cancelled`，独立 Turn 取消仍留在 B2。
- **理由**：单写入者模型可以同时保证双端输入 FIFO、SDK Client 不被并发调用、状态快照一致和事件顺序；有界等待队列避免输入无限堆积；原子“订阅 + snapshot”边界可在不保存完整事件历史的前提下避免订阅建立时出现缺口。
- **影响**：A3 新增 `internal/session` 并把 CLI 从同步直连 Host 迁移为异步提交与事件消费；最多同时存在 1 个运行中 Turn 和 32 个等待输入。A4 Channel 必须复用 Controller，并负责把纯数据领域事件映射为 JSON envelope 和组合连接状态。共用字段只使用 JSON 可表达的字符串、非负整数和可空对象，不包含 Go、浏览器或 Capacitor 专属语义；React/TypeScript、Capacitor iOS/Android 与 HarmonyOS ArkTS/ArkUI 均可实现相同状态和归并规则，三端可实现性核对通过。init 前无真实 session ID 的内部失败只供本地 CLI，不得序列化给移动端。
- **状态**：已定

## D25 · A3 保留内部接受握手，A4 保证协议可观察顺序

- **日期**：2026-08-24
- **决策**：A3 保持 `Submit(ctx, input) (requestID, error)` 接口和 Actor 内部无缓冲 response 握手，不增加可执行回调或二阶段确认。该握手只保证接受结果已交给 `Submit` goroutine 后 Actor 才开始派发 Turn，不承诺 Go 方法已经返回或外部调用方代码先获得调度。A4 Channel/Transport 必须在 PC Service 端保证移动命令的 `command_result` 已进入对应连接的有序输出队列后，才让同一 request ID 的 `turn_started` 对该客户端可见；必要时由服务端短暂缓冲提前事件。
- **理由**：该竞态不会造成 SDK 并发写入、请求丢失或 Session 状态损坏，影响限于协议可观察顺序；在 Session Actor 中执行外部回调会引入阻塞、重入、panic 和失败原子性契约，而可观察顺序更适合由实际拥有连接输出队列的 A4 边界处理。
- **影响**：A3 CLI 在单一事件循环内同步 Submit 并登记 request ID，不受该调度窗口影响。A4 的提前事件缓冲必须在内存中、有界，并在命令接受、连接关闭或 Session 终态时释放；具体结构随 A4 设计确定。顺序由服务端统一保证，三端只消费相同的正常顺序 JSON 消息：React/TypeScript、Capacitor iOS/Android 与 HarmonyOS ArkTS/ArkUI 均无需平台特有回调、线程调度或客户端乱序缓存，三端可实现性核对通过。
- **状态**：已定

## D26 · CLI EOF 使用 PC 内部 Controller 结算边界

- **日期**：2026-08-24
- **决策**：A3 Controller 新增进程内 `WaitIdle(ctx)`：仅当 Actor 已处理当前 Worker result，且无运行中 Turn、无等待输入时返回深复制 snapshot；Session 已结束时返回缓存的最终 snapshot。CLI 在 EOF 已观察到全部已接受请求的 Turn 终态后调用该方法，再依据最终 Session 状态决定退出码。`WaitIdle` 不发布事件、不递增 `seq`，不进入 A4 Transport/Channel 或任何移动端协议。
- **理由**：Adapter Turn 终态事件先于 `Host.RunTurn` 的 Worker result 返回；只按终态事件清理 CLI pending 会在随后发生 Session scope 失败时提前成功退出。由 Actor 提供结算边界能复用 Session 真相，避免 CLI 猜测事件时序或新增移动端握手。
- **影响**：改动仅限 PC Service 的 Go Controller 与 CLI 内部调用面，不新增协议消息、JSON 字段、状态枚举或客户端行为。React/TypeScript Web、Capacitor iOS/Android 与 HarmonyOS ArkTS/ArkUI 均无需实现 `WaitIdle` 或平台特有语义，三端共用契约保持不变，可实现性核对通过。
- **状态**：已定

## D27 · A4 使用每连接原子订阅与服务端有界顺序门控

- **日期**：2026-08-24
- **决策**：A4 的每个已订阅 WebSocket 连接各自持有一个 A3 Controller 原子订阅，先发送 `subscribed` 和 snapshot，再推送 seq 大于 snapshot 基准的事件。连接使用单 reader、单 writer 和有界输出队列；提交移动命令期间在该连接短暂缓存提前事件，必须先把同时含客户端 `commandId` 与 PC `requestId` 的 `command_result` 入队，再释放同一 request ID 的 `turn_started`。提前缓存上限为 32 个 envelope/256 KiB，普通输出队列上限为 64 个 envelope/1 MiB，任一溢出只关闭该连接。断线后重新订阅最新 snapshot，不实现事件 replay 或跨连接命令幂等。
- **理由**：每连接复用 A3 的原子 snapshot/事件边界，避免 A4 重建 snapshot reducer 和订阅竞态；服务端顺序门控落实 D25，使所有客户端观察到相同正常顺序；有界连接队列把慢网络与 Controller 和其他客户端隔离。
- **影响**：版本 1 envelope 固定包含 `version`、`type`、`sessionId`、`commandId`、`requestId`、`seq` 和对象 `payload`；A4 只实现 `send_text` 命令。React/TypeScript、Capacitor iOS/Android 与 HarmonyOS ArkTS/ArkUI 均只需标准 WebSocket、JSON 和相同状态归并，不需要浏览器 API、平台线程回调或客户端乱序缓存，三端可实现性核对通过。A4 闭环由自动化 WebSocket 集成测试和 Go CLI 协议测试客户端完成，移动页面留给 A5。
- **状态**：已定

## D28 · A4 使用短期可复用 Pair Token 与显式 VPN 地址

- **日期**：2026-08-24
- **决策**：A4 Pair Token 使用 256-bit 安全随机值和无填充 Base64URL 编码，与精确 `sessionId` 绑定，默认有效期 10 分钟。有效期内可供多个连接重复认证；自然过期只阻止新认证并触发配对信息轮换，不断开已认证连接；用户撤销或 Service 停止立即失效并关闭相关连接。配对负载为纯 JSON `{version, wsUrl, sessionId, pairToken, expiresAt}`；token 在 WebSocket 首条 `hello` 中提交，不放入 WebSocket URL，Web 配对链接使用 URL Fragment。Service 默认监听 `127.0.0.1`，VPN bind 地址和 `advertise-url` 由用户显式配置，不自动探测网卡。
- **理由**：VPN 提供网络可达性但不证明客户端获得了当前 Session 的控制授权；短期 token 是不引入账号体系的最小应用层鉴权。显式地址避免错误选择网卡，Fragment 和日志过滤降低凭证经访问日志、Referer 或错误信息泄露的风险。
- **影响**：认证前不返回 Session 数据；自然过期支持扫码、刷新和短暂重连，撤销保留立即止损能力。无 Origin 的原生客户端仍可认证，浏览器 Origin 必须匹配显式 allowlist。配对与鉴权只依赖 JSON、WebSocket、安全随机字符串和时间戳，React Web、Capacitor 与 HarmonyOS NEXT 均可实现；Web 的 Fragment 仅是承载方式，不改变其他端协议，三端可实现性核对通过。
- **状态**：已定

## D29 · 里程碑 A 的 Mobile Web 功能优先，视觉辅助延后至 B

- **日期**：2026-08-24
- **决策**：A5 Mobile Web 以完成本机浏览器通信闭环和可用的最小功能界面为目标，不启用视觉辅助，不开展视觉方案比较或精细化视觉打磨；视觉辅助在里程碑 B 的移动端 Web 研发中启用。
- **理由**：里程碑 A 的核心价值是验证配对、连接、状态同步、实时文本展示、文本发送和刷新重连是否可用，视觉探索不应延迟功能闭环。
- **影响**：A5 仍使用 React、TypeScript 与 antd-mobile 提供清晰可操作的功能布局、基本状态反馈和移动端可用性，但不把品牌视觉、复杂动效、完整 Markdown/Tool 呈现或精细交互作为门禁。该决策只调整 Web UI 的实施优先级，不修改 D27/D28 的通信协议、状态模型、配对负载或三端验收契约。
- **状态**：已定

## D30 · A5 配对同时支持 URL Fragment 与原始 JSON 粘贴

- **日期**：2026-08-24
- **决策**：A5 Mobile Web 支持从 `#pair=<Base64URL(JSON)>` 自动读取配对负载，也支持用户在页面粘贴 A4 输出的原始配对 JSON。原始 JSON 校验通过后规范化为相同的 Base64URL Fragment 并启动连接；Fragment 在短期凭证有效期内保留，使页面刷新可重新认证和取得最新 snapshot，不使用 localStorage 保存凭证。
- **理由**：Fragment 为后续二维码/配对链接提供直接入口，且不会进入 HTTP 请求、服务端访问日志或 Referer；原始 JSON 粘贴让 A5 在尚未生成二维码图片时仍可完成本机功能验收。保留 Fragment 是不引入持久化身份体系的最小刷新恢复方式。
- **影响**：页面必须严格校验版本、`wsUrl` scheme、`sessionId`、`pairToken` 和 `expiresAt`，解析失败或过期时不得建立 WebSocket；界面不得回显 token。该做法是 Web 的凭证承载方式，不改变 D28 配对负载或其他端的鉴权协议，三端共用契约保持不变。
- **状态**：已定

## D31 · A5 断线后由用户显式重连

- **日期**：2026-08-24
- **决策**：A5 在页面首次打开或刷新且 Fragment 配对负载有效时自动连接；运行中 WebSocket 断开后显示明确的断线状态和“重新连接”按钮，由用户显式发起重新认证与订阅，不执行后台自动重试。
- **理由**：A5 需要验证刷新后重新取得最新 snapshot，但完整自动重试、退避、网络在线状态和结果不确定命令处理属于 B4。显式重连能保持行为可解释，也避免断线边界自动重发旧命令。
- **影响**：重连复用当前 Fragment 中仍有效的短期凭证，重新执行 `hello → subscribe → snapshot`，并清空旧连接的易失事件视图；任何 pending command 在断线后标记为结果未知且不自动重发。该策略不修改服务端协议，后续 Capacitor 与 HarmonyOS 可采用各自生命周期策略。
- **状态**：已定

## D32 · A5 使用本次连接内的易失消息流

- **日期**：2026-08-24
- **决策**：A5 Mobile Web 不维护完整会话历史。首次订阅以最新 snapshot 初始化当前或最近 Turn；连接建立后的 `text_delta` 按 `requestId` 累积为 Claude 回复。手机发送的文本仅在对应 `command_result` 确认接受并返回 `requestId` 后加入消息流；终端发起的 Turn 因服务端事件不包含终端输入正文，只标记为“电脑端发起”，不伪造提示词。刷新或显式重连时清空上一连接的易失消息流，再以最新 snapshot 重新开始，不做事件 replay。
- **理由**：该范围能够展示扫码后的实时对话和当前状态，同时遵守“手机不管理全量历史”的产品边界；A4 协议既不提供历史回放，也不在终端 `turn_started` 中传输输入正文。
- **影响**：前端 reducer 只保存当前页面连接期的内存状态，不使用 localStorage 或持久化 transcript；重连后旧消息消失是预期行为。实现只消费 D27 的 snapshot、事件和命令结果，不增加浏览器专属协议字段，React Web、Capacitor 与 HarmonyOS NEXT 均可采用相同归并语义，三端共用契约保持不变。
- **状态**：已定

## D33 · A5 允许 Turn 运行时继续提交文本

- **日期**：2026-08-24
- **决策**：A5 在已订阅且 Session 未停止或失败时允许发送文本，即使当前已有 Turn 正在运行；输入仍由 PC 端 Session Controller 统一串行排队。Web 页面同一时刻最多保留一个尚未收到 `command_result` 的发送请求，结果返回前禁用再次发送；成功反馈只表示 Service“已接收”，不宣称已执行或已排到特定位置。
- **理由**：A3 已提供终端与手机统一 FIFO，允许运行中提交可以验证真实接力和排队能力；限制客户端命令结果握手为单飞可降低 A5 状态复杂度，并避免把接受结果误解为 Turn 执行完成。
- **影响**：收到接受结果后，页面用返回的 `requestId` 关联手机输入和后续 Turn 事件；命令拒绝则保留输入供用户修改或重试。该规则只约束 Web 交互节奏，不增加协议字段；Capacitor 与 HarmonyOS NEXT 可采用相同行为，三端共用契约保持不变。
- **状态**：已定

## D34 · A5 显式更换配对并清理连接状态

- **日期**：2026-08-24
- **决策**：A5 提供“更换配对”操作；触发后关闭当前 WebSocket、清空 URL Fragment 和本次连接的易失消息/命令状态，并返回原始配对 JSON 输入页。配对负载解析失败或凭证已过期时同样停留在输入页并显示原因，不建立连接；页面不回显 `pairToken`。
- **理由**：用户需要在 token 轮换、地址变化或连接到错误 Session 时有明确的退出与重新配对路径；彻底清理旧连接状态可避免两个 Session 的界面内容混合。
- **影响**：更换配对不写 localStorage，也不向服务端增加注销命令；WebSocket 关闭即解除当前订阅，新的配对按完整 `hello → subscribe → snapshot` 流程建立。Fragment 清理是 Web 路由行为，不改变三端共用配对或通信契约。
- **状态**：已定

## D35 · A5 使用小型协议模块与 reducer 状态机

- **日期**：2026-08-24
- **决策**：A5 将配对负载解析、版本 1 协议类型与解码、WebSocket 连接生命周期、页面状态归并拆为职责单一的小模块；使用 React reducer 维护单连接 UI 状态，`App` 只负责页面编排和副作用接线。不在 A5 引入 Zustand 等状态库，也不提前创建跨端共享包。
- **理由**：该结构能独立测试不可信 JSON、协议顺序、断线和 reducer 归并，同时保持单页面实现轻量；后续 B 阶段增加审批与取消时无需继续膨胀单一 Hook 或 `App`。
- **影响**：TypeScript 协议类型集中维护，组件不得各自复制消息定义；模块只依赖标准 JSON、WebSocket 和 React 状态能力。未来 Capacitor 可复用 Web 实现，HarmonyOS 按同一共用契约独立实现，不受 Web 模块组织约束。
- **状态**：已定

## D36 · A5 不推断实时队列深度

- **日期**：2026-08-24
- **决策**：A5 reducer 保留 snapshot 中的 `queueDepth`，初次连接时若其大于零，可提示“连接时有 N 条等待任务”；状态栏不持续展示或自行推断实时排队数量。当前 Turn 和 Session 状态仍按 event 实时更新。
- **理由**：A4 只有 snapshot 携带 `queueDepth`，后续 event 不包含队列变化；客户端既无法观察终端新提交的等待输入，也无法可靠维护全局实时计数。持续展示推断值会误导用户。
- **影响**：A5 不为队列数字修改 A4 协议或状态模型。若后续产品确需实时队列深度，应作为三端共用契约变更重新设计并核对 React Web、Capacitor 与 HarmonyOS NEXT；当前三端契约保持不变。
- **状态**：已定

## D37 · A5.1 通过 turn_started 实时同步当前输入正文

- **日期**：2026-08-25
- **决策**：在 A6 VPN 联调前增加 A5.1。A3 在既有 `turn_started.text` 中携带规范化后的本轮输入正文，终端和手机来源使用同一字段语义。Pairing 成功并订阅同一 Channel 的手机连接地位相同，不建立设备身份、主从或跨客户端锁；任意连接均可发送，全部强制为 `source=mobile` 并由同一 Controller 按实际接受顺序形成 FIFO。Mobile Web 对不存在的 request ID 创建输入消息，对已由本连接 `command_result` 展示的手机输入按 `requestId`、来源和正文校验后去重。snapshot 不增加输入字段，刷新、重连和新连接不恢复旧输入正文。
- **理由**：用户验证 A5 时确认手机能够同步状态和 Claude 回复，但看不到 PC 发送的正文；该能力属于里程碑 A 的基础双端文本闭环，应在 A6 真机联调前固定。复用既有字段比新增 event kind 更小，且不改变版本 1 JSON 结构；Service 与新客户端仍需协调升级并通过同一门禁。
- **影响**：本决策替代 D32 中“终端 Turn 只显示来源标记”的局部约定，D32 的易失消息、无历史、无 replay 边界继续有效。跨手机不承诺设备优先级、网络到达时间或客户端时钟顺序，只承诺 Controller 接受后的全局 FIFO 与唯一 request ID；每连接 command result 和慢连接隔离仍由 A4 保证。A5.1 不增加设备 ID、抢占或取消，真正中断当前 Turn 仍归 B2。A4 普通 outbox 与提前事件缓存必须容纳最大合法 prompt 加 envelope 开销，同时继续有界；React Web、Capacitor 和 HarmonyOS NEXT 均按相同 `source/requestId/text` 规则实现，不引入平台专属依赖。三端可实现性核对通过。
- **状态**：已定
