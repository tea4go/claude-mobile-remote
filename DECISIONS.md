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
- **状态**：主体有效；会话所有权边界由 D39 补充

## D3 · 首期只做单会话接力

- **日期**：2026-08-21
- **决策**：首期手机通过扫码绑定一个正在进行的 `sessionId`。
- **理由**：优先验证用户离开电脑后的会话接力价值，减少历史和设备管理复杂度。
- **影响**：多会话和多 PC 后置。
- **状态**：首期有效；后续 PC 与移动端的多 Session 边界由 D39、D41 替代原笼统表述

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
- **状态**：里程碑与独立门禁有效；全局串行关系由 D40 替代

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
- **状态**：VPN 产品前提有效；A6 门禁与后期 Relay 边界由 D40、D43 补充

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
- **状态**：除默认 TTL 外有效；TTL 与移动端安全保存由 D41 替代和补充

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
- **状态**：部分被 D64 替代；不持久化、不 replay 继续有效，“重连必清空已见消息”不再有效

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
- **状态**：A5 单连接阶段有效；多 Session App 级状态库选型由 D67 替代，纯 reducer 与模块拆分继续有效

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

## D38 · A5.2 预分配真实 Claude session ID 并在首轮前配对

- **日期**：2026-08-25
- **决策**：在 A6 前增加 A5.2。新会话由 Service 生成合法随机 UUID，并通过 `claude-agent-sdk-go` 新增的 `WithSessionID` 映射为 Claude CLI `--session-id`，从而在首个 Query 前取得该会话的真实 `sessionId`；恢复会话使用已有真实 ID和 `WithResume`，两者互斥。Controller 内部区分 ID 已预留与 Claude init 已确认；SDK Connect 成功后立即注册同一 ID的 Channel、签发 Pair Token 并允许 PC 或任意已配对手机提交第一条真实消息。首个 init 必须返回预期 ID，之后才发布 `session_started`，并保持其早于首个 `turn_started`。
- **理由**：现有流程只有 PC 先发起首轮后才能扫码，阻止手机成为新会话第一条消息的来源，与正常接力流程冲突。Claude CLI 已支持 `--session-id <uuid>`，预分配方案不需要空对话、不污染上下文，也不需要引入本地 `channelId`、临时会话身份或协议版本升级。
- **影响**：D28 的 token、配对负载和精确 Session 绑定结构保持不变，但 A4 总体设计 §10 和原总体设计中“收到 session_started 后才激活配对”的时机约定由本决策替代。现有版本 1 envelope、snapshot 和 Mobile Web 状态模型不增加字段；首轮前 snapshot 为已知 `sessionId`、Session=`starting`、Turn=`idle`，三端均可在该状态发送文本。PC 与多个手机近乎同时提交仍由 Controller 按实际接受顺序形成 FIFO，第一条触发 Query，其余排队。若 Claude 返回不同 ID，Session 必须失败并撤销凭证，禁止动态迁移 Channel。React Web、Capacitor 和 HarmonyOS NEXT 均复用相同协议与行为，三端可实现性核对通过。
- **状态**：已定

## D39 · 单 Session Host 与单 Session Remote Server

- **日期**：2026-08-26
- **决策**：每个活跃 Claude Session 同时只有一个 Session Host 和一个 SDK Client。一个 Standalone CLI 进程只创建或恢复一个 Session；低频并行 Session 使用多个 CLI 进程和不同端口。WhaleTerm 后续只为其已持有的一个 Session 在进程内启动 Remote Server，不创建第二个 Client；关闭远程控制不停止 Claude Session。禁止通过 JSONL 或第二个 `--resume` Client 接管活跃 Session。
- **理由**：同时远程控制多个 Session 的场景少，单进程多 Session daemon 会引入不必要的生命周期、管理 API、持久化和故障隔离复杂度；现有 A2–A5.2 已是单 Session 组合。
- **影响**：保留现有 SDK Adapter、Session Controller、FIFO、Channel、Pairing、Transport 和版本 1 协议；B3 只把远程组合包装为可导入的单 Session 组件，不实现 Runtime Registry、Local Control API、PC TUI 或跨 Session 管理。本决策替代 D2“只能由 Service 启动”的绝对影响表述及 D3 对后续多会话的笼统边界。Session Host/Remote Server 是 PC 进程内概念，不进入客户端协议；React Web、Capacitor 和 HarmonyOS NEXT 均无需区分 CLI 与 WhaleTerm，三端可实现性核对通过。
- **状态**：进程级限制已分别被 D63、D68 替代；每 Session 唯一 Host/Client 继续有效

## D40 · A6 改为跨机器浏览器门禁并采用依赖式并行路线

- **日期**：2026-08-26
- **决策**：A6 改为局域网跨机器桌面浏览器联调：机器 A 运行 Go Service 和 Claude，Windows 机器 B 只运行 `apps/mobile` Vite 并在 `http://localhost:5173` 打开页面，通过 A 配对负载中的显式 LAN `wsUrl` 连接。手机浏览器、VPN 和移动生命周期真机验证移入 C2。C1 在 A6 后可与 B 并行；C2 依赖 C1 和 B2；W1 依赖 B3。
- **理由**：移动测试机尚未到位，跨机器桌面浏览器足以先验证地址、监听、Origin、鉴权和双向通信；Capacitor 工程与 B 阶段服务端能力没有实现冲突，不必全局串行等待。
- **影响**：A6 标准流程使用 `--listen 0.0.0.0:8765` 和 `--advertise-url ws://<A-LAN-IP>:8765/ws`；B 浏览器 Origin 仍是默认允许的 localhost，不要求额外 Origin。默认配置继续只监听 loopback，不增加自动 IP 发现或网络诊断系统。D12 的 VPN 产品前提保持不变，但“A6 为 VPN 手机联调”被本决策替代；D9 的里程碑和子阶段门禁保留，但“所有后续阶段全局串行”改为依赖图门禁。该变更不修改三端协议、状态或配对负载，三端可实现性核对通过。
- **状态**：已定

## D41 · 移动端多配对列表与 12 小时 Pair Token

- **日期**：2026-08-26
- **决策**：Pair Token 默认有效期由 10 分钟调整为 12 小时，用于覆盖典型远程接力窗口，仍可通过 `--pair-token-ttl` 收紧。移动端后续以 `sessionId` 为逻辑身份保存多个已扫描配对记录；原生 App 使用系统安全存储保存未过期 Token，重启后可直接重连，一次只连接当前进入的 Session。浏览器继续使用 Fragment/内存，不把 Token 写入 `localStorage`。过期、撤销或 Remote Server 重启后保留元数据并要求重新扫码；删除本地记录不撤销服务器共享 Token。
- **理由**：用户离开 PC 后，锁屏、切后台和换网导致断线是正常场景，10 分钟凭证不足以覆盖典型接力窗口；移动端列表需要管理多个独立 CLI/Session，但不应反向引入 PC 聚合服务或长期设备身份。
- **影响**：配对负载仍为 `{version, wsUrl, sessionId, pairToken, expiresAt}`，有效期内重新认证后只取得最新 snapshot，不做 replay；12 小时不保证覆盖任意长度的 Session。D28 除默认 TTL 外继续有效，D30–D34 仍约束 A5 浏览器单连接行为。本地列表属于端内 UI/存储能力，不是三端通信协议或服务器端 Session 中心；Capacitor 与 HarmonyOS 使用各自系统安全存储，React Web 不持久化凭证，三端可实现性核对通过。
- **状态**：Pair Token 与安全存储部分继续有效；“一次只连接当前进入的 Session”已被 D66 替代

## D42 · B2 交互范围与 B3 组件化替代多会话阶段

- **日期**：2026-08-26
- **决策**：B2 覆盖 Tool 权限、AskUserQuestion（单选、多选、自由文本和一次 callback 的多个问题）、取消当前 Turn 与按 request ID 取消排队输入。Session 只维护一个可空 pending action；多入口同时回答时第一位合法解决者成功，其余返回 `stale_action`。B3 不再实现单进程多会话，而是提取单 Session Remote Server 和薄宿主接口；不再设置 B4 持久授权/Journal 阶段。
- **理由**：审批、问答和取消是完整远程控制的必要能力；单 pending action 与第一解决者规则能覆盖多手机和 WhaleTerm UI，又不引入设备竞态模型。组件化只服务 Standalone 与 WhaleTerm 两种 Host，无需多 Runtime 管理平台。
- **影响**：B2 的具体 JSON payload、SDK 映射和 callback 释放规则必须在 B2 分阶段设计中再固定并重新完成三端共用契约核对；当前只确认范围和竞态语义，不提前增加协议字段。B3 在 B2 后确定最小公共 Go API，关闭 Remote Server 与停止 Claude Session 保持不同语义。
- **状态**：部分被 D46、D63 替代；B2 交互与 B3 单 Session Server 组件化继续有效，WhaleTerm 可组合多个独立实例

## D43 · 后期公网访问使用可信的每 CLI 单隧道 Relay

- **日期**：2026-08-26
- **决策**：后期公网访问由每个 CLI 为其唯一 Session 建立一条出站 WSS 隧道。可信 Relay 只维护易失的 `routeId → CLI tunnel` 并转发协议流量；公网地址直接写入现有配对负载的 `wsUrl`。CLI 到 Relay 的宿主凭证与手机到 Session 的 Pair Token 分离，Pair Token 仍由 Session Host 验证。CLI 或隧道重启后旧 route 和 Token 失效并重新扫码。
- **理由**：单隧道与单 CLI 单 Session 模型自然一致，可绕过入站 NAT，又不需要 PC 聚合 daemon、统一 endpoint 或云端 Claude 状态。
- **影响**：Relay 可以看到转发流量但不得存储消息、snapshot、上下文或历史；首版不提供永久公网地址、跨重启恢复、云端 Session 列表或账号驱动的多 Session 管理。若未来要求 Relay 无法看到内容，再独立设计端到端加密。现有客户端仍执行 `connect(wsUrl) → hello(pairToken) → subscribe(sessionId)`，配对负载和版本 1 envelope 不变；LAN/VPN 与 Relay 地址可共存于移动端列表，React Web、Capacitor 和 HarmonyOS NEXT 三端可实现性核对通过。
- **状态**：已定

## D44 · C1 范围收敛为原生壳与局域网明文放行，扫码与安全存储移入 C2

- **日期**：2026-08-26
- **决策**：C1 只交付三项：（1）在 `apps/mobile/` 下创建 Capacitor iOS/Android 工程；（2）现有 React/Vite Web UI 通过 `dist → npx cap sync` 接入两个原生壳；（3）局域网明文网络放行与仅开发用 `server.url` live reload。iOS 使用 `NSAllowsLocalNetworking` 与 `NSLocalNetworkUsageDescription`，Android 使用 cleartext 放行（Manifest 属性或 `network_security_config.xml`）。原生扫码、系统安全存储、NativeBridge、生命周期和推送移入 C2，不设 C1.1 子阶段。Android SDK/JDK 在 C1 开始前安装，Android 原生构建纳入 C1 门禁。
- **理由**：不放行局域网明文时，WebView 无法连接 `ws://` LAN 地址，壳子只能展示空布局，消息流、Markdown、状态栏均无真实数据，达不到 C1 尽早验证 Web 界面在两端 WebView 适配程度的目的；REQUIREMENTS 风险表原本已把平台网络配置归在 C1。扫码与安全存储与真机、多配对持久化强耦合，与 C2 一起验收更经济。Android WebView（Chromium）与 iOS WKWebView（WebKit）在 antd-mobile 上表现有差异，只验一端无法得出适配结论。
- **影响**：`REQUIREMENTS.md` 开发步骤图、§9.2 阶段表和 WebView 套壳兼容风险行按本决策更新；C1.1 不作为阶段存在。只放行局域网明文，不使用 `NSAllowsArbitraryLoads`，不放行公网明文；`server.url` 仅用于开发，不得进入发布构建。C1 不新增协议字段、状态枚举、配对负载或凭证存储，Pair Token 仍只在 Fragment/内存；D41 的原生安全存储能力推迟到 C2 实现。HarmonyOS NEXT 不加入 Capacitor 工程，只复用共用契约并自行实现等价明文策略，三端可实现性核对通过。
- **状态**：已定

## D45 · 原生应用标识

- **日期**：2026-08-26
- **决策**：iOS/Android App name 固定为 `Claude Mobile Remote`，App ID / bundle ID 固定为 `com.tea4go.claudemobileremote`，Capacitor `webDir` 固定为 `dist`，Web asset scheme 沿用 Capacitor 默认配置。
- **理由**：App ID 影响签名、商店上架、Keychain group 和后续 HarmonyOS 包名规划，属于长期不可逆标识，必须在创建平台工程前固定；沿用 SDK 所属的 `tea4go` 命名空间保持一致。
- **影响**：标识只用于原生工程与商店分发，不进入 WebSocket envelope、配对负载或三端共用协议；变更 App ID 需重新签名并视为新应用。HarmonyOS NEXT 可使用等价包名，但不复用 Capacitor 工程。
- **状态**：已定

## D46 · B2 结构化内容、权限交互与取消协议

- **日期**：2026-08-26（用户 2026-08-27 复核收敛范围）
- **决策**：B2 将协议 envelope 版本升为 2，保留 `version`、`type`、`sessionId`、`commandId`、`requestId`、`seq` 和对象 `payload`。结构化内容事件只有 `thinking`、`tool_use`、`tool_result` 三类；MCP 工具沿用 `tool_use`，工具名符合 `mcp__<server>__<tool>` 时按 WhaleTerm 已验证策略自动放行，不产生 pending action。**`Task` 按普通工具展示**：SDK 无 Task 类 `SystemMessage` subtype（全包仅 `SubtypeInitialize`），B2 不做任务聚合，不下发 `taskId`/`taskStatus`/`taskSummary`/子任务数量/耗时，子代理消息沿用 A2 的 `ParentToolUseID` 过滤不下发。移动端 Tool/MCP/`Task` 只展示标题与状态，Tool 结果只下发关联 ID 与成败状态（**不下发结果正文，也不下发服务端摘要**）；Thinking 与工具项默认折叠。
- **决策**：工具审批**不下发原始 `input` 对象**，而是由服务端按工具白名单抽取单个字符串 `pendingAction.preview`：`Bash`←`command`，`Read`/`Write`/`Edit`/`NotebookEdit`←`file_path`，`Glob`/`Grep`←`pattern`（有 `path` 时拼为 `<pattern> @ <path>`），`WebFetch`←`url`，`WebSearch`←`query`；未命中白名单、字段缺失或类型不符时不下发 `preview`，客户端只显示工具名；`preview` 按 PC Service 配置上限（默认 200 字符）按 rune 边界截断。回答 SDK 时仍使用服务端保留的未修改原始 `input`。
- **决策**：`AskUserQuestion` 与工具权限共用 SDK `CanUseTool` 回调，协议支持单选、多选和一次多个问题；Adapter 将客户端答案转换为 SDK `UpdatedInput["answers"]` 所需的扁平 map。**自由文本为条件能力**：WhaleTerm 的 `buildAskUserAnswerInput` 只产出 `options[].label`，无自由文本分支；实施阶段先用真实 CLI 实证非 `label` 字符串能否被接受，通过则启用 `inputType: text`，不通过则 B2 只支持单选/多选、`inputType` 固定 `choice` 作为扩展位。工具审批支持本次允许、本会话允许（仅 SDK Suggestions 非空时）和拒绝；所有 action 使用项目生成的 `actionId`，多个已认证客户端竞争时第一条合法决议成功。
- **决策**：SDK 权限回调阻塞期间不得调用 `Interrupt`。pending action 的取消先返回 `PermissionResultDeny{Interrupt:true}` 解阻塞，再完成 Turn 取消；无 pending action 时才调用 SDK `Interrupt`。审批超时返回 `Deny{Interrupt:false}`，由 Claude 决定后续行为。当前 Turn 和 FIFO 排队输入分别支持按 `requestId` 取消。权限回调运行在 SDK 读循环 goroutine 而非 `hostWorker`，`internal/session/worker.go` 的无缓冲通道不得用作 action 通路：pending action 上行与决议下行必须走专用旁路通道，并有死锁回归测试。
- **决策**：Session error/finish 沿用 A3/A4 的 `session_failed` / `session_stopped` 事件与 snapshot，B2 补移动端终态展示；在线客户端实时接收，重连从 snapshot 恢复。系统通知、后台推送和推送 Token 后置，不新增推送专用协议。
- **决策**：B2 新增**假 Host 桩模式**（实现 `session.Host` 窄接口）按脚本触发 Thinking、工具调用/结果、工具审批、AskUserQuestion、取消和 Session 终态，使自动化测试与人工浏览器门禁不依赖真实 Claude 额度；桩模式不得影响默认运行路径。
- **决策**：未知或不完整的 SDK block 使用独立 `type=unsupported` 安全事件，仅携带固定 `code`，不作为第四种 `content_block.kind`，不得透传原始 SDK 数据。`ServiceConfig.ToolPreviewMaxRunes` 默认 200；`ServiceConfig.AskUserTextEnabled` 默认关闭，只有 CLI 实证通过后才允许开启自由文本。
- **理由**：复用 SDK 已验证的权限和消息模型，解决 SDK 单读循环阻塞时的取消竞态；MCP 自动放行与 WhaleTerm 现有行为一致；砍掉无 SDK 数据源的 Task 聚合与无必要的结果摘要、把原始 `input` 换成白名单 `preview`，既压缩 B2 范围又缩小敏感数据下发面；假 Host 桩使门禁可重复执行。
- **影响**：A4/B1 客户端必须升级到版本 2 才能连接 B2 服务；React/Capacitor 与 HarmonyOS NEXT 均按相同 JSON、状态、action 和取消语义实现，不复制 SDK 私有控制协议；工具结果正文、工具原始 `input`、权限规则原文不进入移动端展示或日志；白名单需随新工具人工维护（未列入则无预览）。Task 进度聚合与任务卡片、工具输入完整下发、结果摘要后置到 B2 之后。B3 复用 B2 action 协议，不重新设计多 Session 或 Remote Server 管理。D46 明确替代 D42 中“自由文本已确认纳入 B2”及 `stale_action` 命名的旧表述，统一以本决策的条件能力和 `action_not_pending` 为准。
- **状态**：已定

## D48 · B2 Web 结构化消息顺序与工具活动展示

- **日期**：2026-08-27
- **决策**：移动端 reducer 按事件到达顺序维护 Claude 文本片段与结构化 block 的同一消息流；文本跨越 Thinking/Tool block 时拆成新的文本片段，不再把一个 Turn 的全部文本聚合到工具之后。`tool_use` 与对应 `tool_result` 按 `toolUseId` 合并为一个工具活动，最终状态由结果决定；连续工具活动在展示层归并为一个默认折叠的工具组，文本、Thinking 或输入消息会打断分组。
- **决策**：`content_block.tool_use` 可携带服务端按既有工具白名单抽取、按 `ToolPreviewMaxRunes` 截断的 `preview`；仍禁止下发原始 `input`、结果正文和服务端摘要。工具标题使用 `工具：<name> ｜ <preview>`，无 preview 时回退为工具名。
- **理由**：保持 Claude 实际阅读顺序，避免工具执行过程与回复文本割裂；合并调用和结果减少重复信息；连续工具折叠降低长工具链对阅读流的干扰，同时沿用既有敏感数据最小化边界。
- **影响**：B2 共用协议新增可选 `tool_use.preview`，React Web、Capacitor 与 HarmonyOS NEXT 需按同一字段解码；不新增 SDK 私有字段或完整工具输入。既有 B2 的三类 `content_block.kind`、MCP 自动放行、Task 普通工具展示和结果脱敏规则保持不变。
- **状态**：已定

## D47 · pending action 的事件对与 FIFO 队列快照字段

- **日期**：2026-08-27
- **决策**：pending action 的出现与清除使用一对具名事件而非快照差异比较：`action_required` 携带完整 `pendingAction` 且 `turnStatus` 必为 `waiting`，`action_resolved` 不再携带 `pendingAction` 且 `turnStatus` 回到 `running`；两者与 `snapshot.turn.pendingAction` 并存，前者给在线客户端提供发生时刻，后者给重连客户端提供当前状态。
- **决策**：`snapshot` 在 D46 的 `queueDepth` 之外新增 `queuedRequests` 数组，元素只含 `requestId` 与按 `ToolPreviewMaxRunes` 截断的输入预览；两个字段构成同一份 FIFO 视图，必须同时写入，不允许只更新其中之一。
- **理由**：只靠快照差异比较会丢掉审批发生的时间点，客户端无法区分“新审批到达”与“重连后看到既有审批”，也无法在 action 被其他客户端抢先决议时给出明确反馈；`cancel_queued` 需要客户端先看到可取消的 `requestId`，仅有深度数字无法定位目标。
- **影响（三端可实现性核对结论）**：两个事件与新增快照字段只使用字符串、布尔、数组和对象，走与其它事件同一条按 `seq` 单调有序的管道，不依赖任何浏览器特有能力，React / Capacitor iOS·Android / HarmonyOS ArkTS 均可实现。`queuedRequests` 只下发截断预览，不下发完整输入正文。本条在 D46 之下细化，不替代 D46 任何决策。
- **状态**：已定

## D49 · Android 壳使用 http scheme 以连接局域网明文 WebSocket

- **日期**：2026-08-27
- **决策**：Capacitor Android 工程在 `capacitor.config.ts` 设 `server.androidScheme: 'http'`，使壳内页面以 `http://localhost` 加载本地 `dist`，而非 Capacitor 8 默认的 `https://localhost`。
- **理由**：Capacitor 8 Android WebView 默认 `androidScheme = https`（`CapConfig.java` 源码确认），页面为 `https://localhost` 时浏览器混合内容策略会拦截一切 `ws://` 明文连接，配对后 `new WebSocket('ws://…')` 抛 `SecurityError`，React 组件树崩溃为空白。D44 只放行局域网明文（cleartext），未预见 `https` scheme 额外触发混合内容拦截这一层。`http` 是 Capacitor 官方 `validateScheme` 允许的合法值，与产品「局域网明文直连」定位一致；`http://localhost` 加载的是本地 `dist` 资源（无中间人风险），唯一明文是到 PC 的 `ws://`，本就属 D44 放行范围。
- **影响**：iOS 默认 `capacitor://` scheme 不触发此拦截，无需改动，仅 Android 调整。协议、状态、配对负载与 `sessionId`/request ID/`seq` 语义均不变，三端共用契约不变；属端内平台配置，HarmonyOS NEXT 可自行实现等价策略。本条补充 D44 的平台网络策略，不替代 D44。遗留关联：原生壳页面 Origin 为 `http://localhost`（无端口），是否加入 Service 默认 Origin 白名单待 C2 决定（见 `docs/testing/C1-result.md` §7 发现 2）。
- **状态**：已定

## D50 · Pair Token 通过本地小文件跨 Remote Server 重启恢复

- **日期**：2026-08-27
- **决策**：移动端列表中的 `sessionId` 继续作为会话逻辑身份，但不得单独作为授权依据。每个 Standalone CLI Session 在 PC 本地使用一个仅当前用户可读写的小型 pairing 文件保存当前 Pair Token、绑定的 `sessionId` 和 `expiresAt`；Remote Server 重启后，若 Session ID 相同且凭证未过期，则恢复原 Pair Token。过期、主动撤销或配对文件损坏时才生成新 token 并要求重新扫码。无需数据库、服务器端 Session 中心或新协议字段。
- **理由**：当前列表按 `sessionId` 管理记录，但 Remote Server 重启会让内存中的 Pair Token 丢失，导致列表记录无法继续连接，违背“扫码一次后可继续使用”的预期。只比较 `sessionId` 会把身份识别误当成授权，不能接受；本地文件能以最小复杂度保持 token 授权边界。
- **影响**：配对负载、`hello` 鉴权、Channel、WebSocket 和 `sessionId` 订阅语义不变；Relay 仍只维护易失的隧道映射并透明转发，不保存 pairing 状态。永久设备授权不在本阶段实现。Standalone CLI 的每个 Session 使用独立文件路径，退出时不删除配对文件；下次启动时清理同目录下已过期的记录。过期、Session 不匹配或文件损坏时以新 token 覆盖该文件；pairing 文件的读写 I/O 失败只降级为「本次不恢复/不保存」，不得阻断 Session 启动。本阶段不提供主动撤销入口（`Invalidate`/`Revoke` 只影响当前进程内存中的凭证与已建立连接，不删除文件）；需要彻底失效时由用户删除对应文件。文件不得进入日志、Git 或测试快照。
- **状态**：已定；替代 D41 中“Remote Server 重启后重新扫码”的旧行为，不改变 D41 的移动端安全存储和浏览器不写 `localStorage` 约束；默认 TTL 另见 D51。

## D51 · Pair Token 默认有效期调为 30 天，不引入续期协议

- **日期**：2026-08-27
- **决策**：Pair Token 默认有效期由 12 小时调为 30 天，仍可用 `--pair-token-ttl` 收紧。不引入服务端滑动续期，也不新增下发新 `expiresAt` 的协议字段；若后续确定需要续期，在 C2 随移动端缓存一并设计。
- **理由**：D50 让凭证能跨重启恢复，但 12 小时 TTL 仍使「隔几天接着用」必然重扫，使恢复能力失去意义。`expiresAt` 是写入配对负载、三端共同持有的绝对时间，服务端单方面续期会使客户端那份 `expiresAt` 失真，并被客户端的本地过期校验（`apps/mobile/src/protocol/pairing.ts`）在建连前拦下；要消除分歧必须要么放弃客户端过期校验（正面修改 D30/D34），要么新增下发字段（触发三端契约改动），均超出本轮修复范围。调大默认值只改一个默认参数，三端对绝对过期时间的理解保持一致。
- **影响**：单个 token 的绝对寿命延长到 30 天，属已接受的内部使用权衡；到期仍会自然轮换，不变成永不过期。配对负载字段、`hello` 鉴权、Channel、Relay 和移动端代码均不变，无需三端同步实现；三端可实现性核对通过。
- **状态**：已定；替代 D41 与 D28 的 12 小时默认 TTL，其余约束不变。

## D52 · `action_resolved` 携带服务端已校验的决议答案

- **日期**：2026-08-28
- **决策**：`action_resolved` 事件在 D47 既有字段之外新增一个可选字段，携带**服务端自己校验后回传给 SDK 的那份答案**：每项为 `{questionId, header, question, values}`，`values` 为字符串数组（自由文本为单元素）。该字段只允许出现在 `action_resolved`，其它事件出现即视为协议错误；仅权限类决议（允许/拒绝）时缺省。答案**不写入 `snapshot`**，只随事件实时下发；重连后不重放。
- **理由**：AskUserQuestion 作答后移动端看不到自己选了什么，作答记录必须可见。可选的替代路径是像 WhaleTerm 那样从 `ToolResultBlock.Content` 抽取工具结果正文回显（`docs/testing/B2-result.md` §1 已实证 AskUserQuestion 的结果正文确实回显答案），但这会正面推翻 D46「工具结果只下发关联 ID 与成败状态，不下发结果正文，也不下发服务端摘要」，并把 Bash/Read 等任意工具的输出一并暴露到移动端，安全边界代价过大。本决策的数据源不是 Claude 的工具输出，而是服务端在 `resolve_action` 中已按题目选项白名单校验过、随后交给 SDK 的同一份结构化答案；它本就是客户端自己提交的内容，回显不新增任何信息暴露。纯客户端本地记忆的方案被否，因为它无法证明服务端真正采纳了哪份答案，也在换设备/重连后失真。
- **影响（三端可实现性核对结论）**：新增字段只使用字符串与字符串数组，走与其它事件同一条按 `seq` 单调有序的管道，不依赖任何浏览器特有能力，React / Capacitor iOS·Android / HarmonyOS ArkTS 均可实现。字段可选，旧客户端忽略即退回现状，不改变 D47 的「`action_resolved` 不携带 `pendingAction`、`turnStatus` 回到 `running`」语义。本条在 D47 之下细化，**不触碰也不替代 D46 关于工具结果正文与工具原始 `input` 不下发的任何约束**；服务端仍不持久化答案。
- **状态**：已定

## D53 · 待处理事项改用底部抽屉，显式关闭即取消当前 Turn

- **日期**：2026-08-28
- **决策**：权限审批与 AskUserQuestion 在移动端不再用常驻卡片呈现，改为待处理事项出现即自动弹出的底部抽屉；抽屉只能由显式关闭按钮关闭，点击遮罩与下滑手势不关闭。显式关闭的语义是**取消当前 Turn**，复用既有 `cancel_turn` 命令，不新增协议字段或命令。
- **理由**：审批与提问是阻塞整个 Turn 的必答项，常驻卡片在长表单下挤占正文且容易被忽略。用户要求与 WhaleTerm 一致：关掉审批窗口等价于放弃这一轮。允许遮罩点击或手势关闭会让一次误触销毁整个 Turn，因此收窄为只认显式关闭。
- **影响**：不改协议、状态模型与配对负载，`resolve_action` / `cancel_turn` 语义不变（服务端在有 pending action 时的取消路径见 D46）。属交互契约，三端需保持相同语义：待处理事项必须以强呈现方式出现，放弃入口必须显式且等价于取消 Turn，各端可用自身平台的等价控件实现。
- **状态**：已定

## D54 · C2 分拆为二维码/Web 生命周期/原生实现/真机验收

- **日期**：2026-08-31
- **决策**：W1 WhaleTerm 接入暂缓，优先推进精简 C2。当前只实施 C2-A 后端终端二维码与现有配对持久化、C2-B Mobile Web 生命周期；C2-C 原生扫码/安全存储/原生持久化与 C2-D 真机/VPN 验收暂缓，不纳入本轮实施门禁。
- **理由**：后端二维码和 Web 刷新/断线/手动重连可以在现有 CLI、桌面浏览器和自动化测试中尽快验证；没有真机时不引入原生实现和额外平台复杂度。安全底线仍遵守硬约束 7，不因简化而把 Token 写入日志、浏览器存储或不受保护文件。
- **影响**：二维码编码现有 `Pairing` 负载的 `#pair=<Base64URL(JSON)>` Fragment，不新增协议字段或 WebSocket API。CLI 只显示 ANSI/ASCII 二维码，不生成 PNG、不增加 HTTP 二维码接口。浏览器继续不得写 `localStorage`、`sessionStorage` 或 IndexedDB；现有 pairing 文件恢复、过期、`sessionId` 绑定和权限保护保持不变。C2 不接入 WhaleTerm、不接管外部 Claude 进程、不改变三端协议形状。
- **状态**：已定
## D55 · CLI 网络 profile 与开发/调试参数分层

- **日期**：2026-08-31
- **决策**：CLI 普通使用只暴露 Session、CWD 和常用 Claude 参数；Remote Server 网络配置通过 `local`、`emulator`、`device`、`debug` profile 管理。LAN 与 VPN 统一为 `device`，不作为两个产品概念；底层 `--listen`、`--advertise-url`、`--allowed-origin` 保留为 debug/高级覆盖参数。
- **理由**：LAN 与 VPN 在 Remote Server 层都是客户端直连 PC，只是可达地址和网络前提不同；将二者拆成 profile 会把部署细节暴露给普通用户。默认 `local` 保持 loopback，模拟器使用固定的 `10.0.2.2`，真机 `device` 使用 PC 可达 IPv4 并允许多网卡时显式覆盖。
- **影响**：README 分为开发版和调试版；CLI 新增 `--profile`、`--advertise-host`、`--permission-mode`、`--model`、`--max-turns`。不自动建立公网 Relay、不创建 daemon、不改变 Session/协议/配对负载；未知 Claude 参数不静默透传。
- **状态**：已定

## D56 · emulator profile 自动放行 live reload 默认端口 Origin

- **日期**：2026-09-02
- **决策**：`emulator` profile 在已有 `http://localhost`（dist 壳内 Origin）之外，自动把 `http://10.0.2.2:5173`（live reload 页面 Origin，默认 dev server 端口）追加进 `allowedOrigins`；自定义 Vite 端口（≠5173）时仍需显式 `--allowed-origin http://10.0.2.2:<端口>`。
- **理由**：live reload 下壳内页面从 dev server（`CAP_SERVER_URL=http://10.0.2.2:5173`）加载，Origin 为 `http://10.0.2.2:5173`，精确匹配的 CheckOrigin 此前会拒绝对应的 WebSocket 握手（403），用户须手工补 `--allowed-origin`，命令复杂。`10.0.2.2` 是 Android 模拟器内部 NAT 别名，只指向宿主回环，该 Origin 的页面只可能存在于模拟器内，与 profile 已内置的 `http://localhost` 暴露面同级；不改全局默认白名单，`local`/`device`/`debug` 不受影响。
- **影响**：`cmd/claude-mobile-remote/config.go` emulator profile 追加一条 Origin；`TestParseConfigProfiles` 断言该 Origin；README 与 HANDOFF 的 live 联调说明简化；`--allowed-origin` 保留为自定义端口逃生门（符合 D55 参数分层）。不改变协议、监听、二维码地址或配对负载。

## D57 · CLI 不产品化 PTY，优先 WhaleTerm 进程内无缝托管

- **日期**：2026-08-31
- **决策**：Standalone CLI 保持单 Session Remote Host 定位，不实现生产级 PC PTY/终端模拟器，不以浅层 PTY 替代用户熟悉的原生 Claude CLI 交互。后续优先实现 W1：WhaleTerm 在同一进程内复用其已有 Claude Client、Session Controller 和输入队列，按当前 Session 启动 `remote.Server`，实现不中断任务的远程管理。
- **理由**：完整 PTY 需要处理终端尺寸、ANSI、信号、进程组、全屏界面、中文宽字符、粘贴、断线恢复和跨平台差异，复杂度高且仍不能解决外部接管已运行终端 Claude 的问题。普通终端 Claude 的 Client 不对外暴露，Standalone 无法在运行中安全共享；强行增强 PTY 会损害原生 PC 体验并偏离“重点是托管”的产品定位。
- **影响**：Standalone CLI 只支持从创建时托管，或对已停止的 Session 执行 `--resume` 后托管；普通终端中已运行的 Claude 不支持事后无缝接管。W1 必须使用 WhaleTerm 现有 Client，不得创建第二个 SDK Client，不得通过第二个 `--resume` 进程或 stdout/JSONL 监听实现接管。README、W1 设计和验收文档应明确区分“Standalone 远程优先托管”与“WhaleTerm 已有会话无缝开启远程管理”。
- **状态**：已定

## D58 · WhaleTerm W1 支持首轮前预分配 Session 并配对

- **日期**：2026-09-18
- **决策**：W1 的“WhaleTerm 已持有 Session”包含已经由 WhaleTerm 预分配稳定 UUID、但尚未创建 SDK Client 或发送首条消息的本地 Claude 会话。用户可在首轮前开启远程管理；WhaleTerm 以当前完整 Claude 配置预备该 Session，Remote Host 返回 `starting/idle` Snapshot，PC 或手机任一端提交的首条输入都进入同一 FIFO，并由既有 Manager 创建唯一 SDK Client。SDK 首轮启动使用正式 `WithSessionID`，不再通过 `WithExtraArgs` 注入 `session-id`。
- **理由**：SDK 和 WhaleTerm 均已验证 Claude CLI 支持在进程启动时通过 `--session-id` 绑定调用方预分配的 UUID；A5.2/D38 已证明首轮前配对可行。以“尚未收到 Claude init”为由隐藏 W1 入口会错误地把 Session 身份与 SDK Client 是否已启动绑定，并阻止手机发起首个 Turn。
- **影响**：W1 前端入口不再依赖非空 `sessionId`；开启远程管理时若当前草稿尚无 ID，由 WhaleTerm 预分配并回传后写入当前会话。Remote Host 必须保存开启时的完整宿主配置，远程首条消息只替换输入正文、Turn ID 与已预分配 Session ID，不能用稀疏配置创建另一种 Client。既有已启动 Session、单 Host/单 Client、统一 FIFO、版本 2 wire contract、配对负载和三端状态模型不变；首轮前 `starting/idle` 已由 React Web、Capacitor 和 HarmonyOS NEXT 支持，三端可实现性核对通过。
- **状态**：已定；替代 W1 原设计中“Session ID 为空则拒绝开启”与“仅已运行 Session 可开启”的限制

## D59 · Origin 判定由枚举白名单改为规则判定

- **日期**：2026-09-18
- **决策**：`internal/transport` 的 `CheckOrigin` 由「精确字符串查表」改为「显式白名单 ∪ 三条规则」的并集判定。规则一律忽略端口，放行三类 Origin 主机：**(1) loopback**（`localhost`，或 `net.ParseIP(host).IsLoopback()`，覆盖整个 `127.0.0.0/8` 与 `::1`）；**(2) Android 模拟器 NAT 别名 `10.0.2.2`**；**(3) 等于 `AdvertiseURL` 的主机名**——即用户已经声明手机从哪个主机连过来。其余一律拒绝。显式 `AllowedOrigins`（CLI `--allowed-origin`）保留为规则之外的逃生门。`transport.Config` 新增 `AdvertiseURL` 字段，由 `remote.New` 从 `remote.Config.AdvertiseURL` 透传。
- **理由**：枚举白名单永远追不上浏览器壳真实的 Origin，且每漏一条都是一次**不可诊断**的失败——`CheckOrigin` 不匹配由 gorilla 直接返回 HTTP 403，而浏览器对握手 403 只能观察到 `close` code `1006`，与网络不通、认证超时、服务端关闭同值，服务端又不发送任何 WebSocket close 帧，客户端无从区分。已知会漏的至少三类：Android 壳 `--dist` 的 `http://localhost`、`--live` 的 `http://10.0.2.2:5173`、真机 live reload 的 `http://<LAN IP>:5173`；自定义 Vite 端口还会让端口变化。D56 已为 CLI `emulator` profile 单独补过两条，WhaleTerm 进程内路径又漏了一次，说明枚举方式本身不可维护。规则 (3) 把「用户已声明的可达主机」变成信任依据，规则 (1) 顺带覆盖 iOS Capacitor 的 `capacitor://localhost`。
- **影响**：**替代 A4 §9「浏览器带 Origin 时必须精确匹配允许列表」的表述**。该条的实质约束是「不允许任意 Origin 通配」，规则判定仍满足——不匹配任何规则的 Origin 依旧被拒；改变的是「允许」由枚举改为按主机类别推导。DNS rebinding 防线不变：攻击页把 `evil.com` 解析到 `127.0.0.1` 后连 `ws://evil.com:7331`，Origin 头仍是 `http://evil.com`，三条规则都不匹配；advertise 主机按**相等**比较而非后缀，`192.168.1.10.evil.com` 仍被拒。D56 为 `emulator` profile 追加的两条 Origin 与 `http://localhost` 在新规则下已冗余，但**保留不动**（不改既有 profile 配置，`--allowed-origin` 语义不变）。WhaleTerm 前端删除硬编码的 `CLAUDE_REMOTE_ALLOWED_ORIGINS` 与其传参，不再维护猜出来的清单。不改变协议、配对负载、监听地址、鉴权与 wire contract。
- **三端可实现性**：这是**纯服务端**放行策略，客户端只是照常发送自身运行时 Origin，不需要任何改动或配合。React Web（dev server 为 loopback，命中规则 1）、Capacitor（Android `http://localhost` 命中规则 1；iOS `capacitor://localhost` 命中规则 1；live reload 命中规则 1 或 3）、HarmonyOS NEXT（Origin 主机若为回环或与 `AdvertiseURL` 同主机即命中）均无需适配；规则判定对三端未知的具体 Origin 取值比枚举更宽容。核对通过。
- **状态**：已定；替代 A4 §9 中「精确匹配允许列表」的表述，A4 该节其余约束（无 Origin 客户端仍须鉴权、Origin 不替代应用层鉴权、日志脱敏）不变
## D60 · 订阅事件必须连续；慢消费者以断开并重新获取 Snapshot 收敛

- **日期**：2026-09-21
- **决定**：Controller 的原子 `Subscribe` 返回 `snapshot.Seq` 后，后续事件必须从 `snapshot.Seq + 1` 严格连续。Channel 发现重复、倒退或跳号都终止该订阅并报告 `ErrInvalidSequence`。宿主内部发布不得阻塞 Claude/SDK 事件线程；单个订阅者缓冲区满时关闭该慢订阅，不静默丢弃事件，由 Transport 断开连接，客户端重连后重新鉴权、订阅并获取最新 Snapshot。
- **原因**：静默丢事件后继续转发更高 `seq` 会让客户端持有无法证明完整的增量状态；阻塞发布线程又会反向影响唯一 Session Host。断开并通过最新 Snapshot 恢复符合现有“重连不 replay、恢复当前状态”的产品边界。
- **三端可实现性**：不改变 JSON envelope、字段或客户端平台依赖；React/Capacitor 与 HarmonyOS 客户端均沿用既有 WebSocket 重连和 Snapshot 恢复语义。
- **状态**：已定

## D61 · 移动端可读写 Session 权限模式，PC 侧为唯一执行者

- **日期**：2026-09-21
- **决策**：移动端通过当前 Snapshot 查看 Session 的 Claude 权限模式，并可发送 `set_permission_mode` 请求；权限模式修改必须由 PC 侧 Session Host/Manager 校验并调用 Claude Client 执行，移动端不得本地生效或绕过 PC 权限链路。权限模式变更成功后，服务端更新 Snapshot 并广播 `permission_mode_changed`，配对、重连和多客户端均以 PC 当前值为准。
- **运行中行为**：允许在 Turn 运行中修改；SDK 当前 Client 使用新模式处理后续权限判断。已经产生的 `pendingAction` 不因模式切换自动决议，仍须显式审批或拒绝。
- **默认与并发**：未配置时使用最低权限模式 `default`；多个已认证客户端的修改请求进入 PC Session Controller 串行处理，失败不更新对外状态，成功值才广播。
- **理由**：WhaleTerm 已有 `UpdateClaudeCodePermissionMode`，能够更新活动 Client、排队 Turn 和后续 Client；复用该能力比在移动端复制权限逻辑更安全。Snapshot 负责首连/重连基线，变更事件负责运行中收敛，Pairing payload 继续只承载身份与凭证。
- **三端可实现性**：新增字段和命令只使用枚举字符串及现有 command/event/seq 管道，React/Capacitor/HarmonyOS 均可展示和发起同一请求；不新增平台专属依赖。
- **状态**：已定

## D62 · WhaleTerm Remote Control 采用独立功能模块与窄宿主接口

- **日期**：2026-09-22
- **决策**：WhaleTerm 内的 Remote Control 作为独立功能组织：Remote Session Host、事件/协议适配和 Server 生命周期归入 `backend/remote/`；Wails 入口使用独立 `TClaudeRemoteService`；前端归入 `features/claudeRemote/`。`backend/chat` 只暴露与网络协议无关的窄宿主能力，会话页只保留最小挂载和状态回调。
- **理由**：当前 Remote Host 直接访问 Claude Manager 私有状态，Remote 生命周期挂在 Claude 主业务服务上，前端管理与同步逻辑散落在会话页面，导致独立功能与 WhaleTerm 核心业务双向耦合，后续测试和演进风险较高。
- **影响**：只调整 WhaleTerm 内部包、服务和前端目录边界；不改变公共 `remote.Controller`、wire protocol、配对负载、Session 状态、权限语义或移动端代码。PC、React/Capacitor 与 HarmonyOS NEXT 的三端共用契约均不变，无需客户端迁移。
- **状态**：已定

## D63 · WhaleTerm 按 Session 持有独立 Remote Server

- **日期**：2026-09-22
- **决策**：WhaleTerm 可为多个由其持有的 Claude Session 同时开启远程管理；每个 Session 分别持有唯一 Session Host、唯一 SDK Client 和一个单 Session `remote.Server`，由 WhaleTerm Remote 模块按 `sessionId` 管理实例、监听端口、配对信息和启停生命周期。同一 Session 重复开启复用自身实例，关闭某一实例不影响其他 Session，WhaleTerm 退出时关闭全部 Remote 实例。
- **理由**：“一个 Remote Server 只服务一个 Session”是隔离与所有权约束，不等于“整个 WhaleTerm 进程只能运行一个 Remote Server”。此前把 Standalone CLI 的单进程单 Session 产品边界错误扩大到已有多 Session 管理能力的 WhaleTerm，导致第二个 Session 开启远程管理被进程级单例拒绝。
- **影响**：替代 D39、D42、总体设计及 W1 设计中“WhaleTerm 同时只允许一个 Session 开启 Remote”的局部表述；Standalone CLI 仍保持一进程一 Session，公共 `remote.Server` 仍保持单 Session，不增加服务器端跨 Session 路由或第二个 SDK Client。Start/Get/Stop 必须以 `sessionId` 为作用域，不改变 wire protocol 或配对负载。移动端连接生命周期后由 D66 替代。
- **状态**：已定

## D64 · 移动端在 App 生命周期内按 Session 保留已见消息

- **日期**：2026-09-22
- **决策**：移动端按 `sessionId` 保存本次 App/页面生命周期内已经接收并展示的消息状态。返回 Session 列表只关闭当前 WebSocket，不删除该 Session 的内存消息；再次进入时先恢复已见消息，再用最新 Snapshot 校正 Session、权限、队列、待处理事项和 Snapshot 可表达的当前 Turn。页面刷新、App 销毁或用户删除配对记录时可清空对应消息，不持久化 transcript。
- **理由**：Session 列表与详情切换是同一 App 内的导航，不应等同于销毁会话界面。此前只有配对记录 store，返回列表会重置唯一 reducer，导致用户刚刚看到的实时消息立即丢失；这不是“不管理完整历史”所必需的行为。
- **影响**：替代 D32 中“任何重连都清空旧易失消息”的绝对表述；显式网络重连和重新进入详情均以最新 Snapshot 收敛，但允许保留同一 App 生命周期内的已见消息。服务端仍不 replay，离开期间已经结束且 Snapshot 不再表达的 Turn 不补造；浏览器仍不把消息或 Pair Token 写入 `localStorage`、`sessionStorage` 或 IndexedDB。状态缓存仅依赖端内内存 Map，React Web、Capacitor 与 HarmonyOS NEXT 均可等价实现，不新增平台专属协议，三端可实现性核对通过。
- **状态**：按 Session 保留内存消息继续有效；“返回列表关闭当前 WebSocket”已被 D66 替代

## D65 · 公共 EventStream 承担嵌入式宿主的单 Session 广播

- **日期**：2026-09-22
- **决策**：`claude-mobile-remote/remote` 提供绑定单个 `sessionId` 的公共 `EventStream`。嵌入式宿主使用它维护 Snapshot、分配事件 `seq`，并把一条事件广播到每个订阅者的独立有界 channel；慢订阅者只关闭自身。WhaleTerm 不再实现自己的 Remote Hub，只把所属 Session Host 的事件映射后发布到该 Session 的 `EventStream`。
- **理由**：现有 `remote.Controller.Subscribe` 要求宿主提供可多订阅的事件源，但此前该广播能力只存在于 Standalone 的内部 Session Controller，导致 WhaleTerm 重复实现 Hub。把同一语义提升为公共组件，才能让嵌入式宿主真正复用 Remote 广播实现，并避免多套 seq、Snapshot 和慢消费者逻辑。
- **影响**：新增 Go 公共嵌入 API，不改变 wire protocol、配对负载、Session/Turn 状态或客户端行为。`EventStream` 永久绑定一个 Session，拒绝不同 `sessionId` 的 Snapshot/事件；不同 WhaleTerm Session 仍各自持有独立 `remote.Server` 和 `EventStream`。React、Capacitor 与 HarmonyOS NEXT 继续使用原协议，无需修改，三端可实现性不变。
- **状态**：已定

## D66 · 移动端在 App 前台生命周期内同时管理全部有效配对 Session

- **日期**：2026-09-22
- **决策**：移动端一次只展示一个 Session 详情，但在 App 前台且 JavaScript 运行的生命周期内，为所有有效配对 Session 分别维护独立的 `ProtocolClient`、WebSocket 连接和按 `sessionId` 隔离的内存状态。配对成功即纳入同步管理；返回列表或进入其他 Session 只改变展示目标，不关闭原连接、不清空状态。删除配对、凭证失效、对应 Remote 停止或 App 销毁时才释放该 Session 的连接与内存状态。列表从各 Session 状态派生连接、运行、等待决策、完成、错误和未读提示。
- **理由**：用户同时配对多个 Claude Session，代表要同时观察和控制这些会话；若只有进入详情才连接，列表只是配对书签，用户离开详情期间会漏掉回复、Turn 完成和待决策事件，无法实现多 Session 管理。2026-08-26 的单连接方案是为快速推进 B1、复用单 Session reducer 而采用的临时实现，并未堵死协议或服务器架构，现在以 App 级多连接和分桶状态替代。
- **影响**：替代 D41、D63、D64、REQUIREMENTS、总体设计及 B1/C2/W1/Harmony 设计中的“只连接当前详情 Session”“返回列表关闭 WebSocket”表述。每条连接仍只认证并订阅一个 `sessionId`，每个 PC 端 `remote.Server` 仍只服务一个 Session；不增加服务器端 Session Registry、跨 Session 路由、数据库、事件 replay 或配对字段。当前无 replay 的边界继续存在：App 进程存活且连接连续时可完整接收配对后的事件；操作系统挂起、杀进程或网络断开期间可能产生缺口，恢复时只能由最新 Snapshot 校正其可表达的状态，完整离线补偿与系统推送后续单独设计。
- **三端可实现性**：React Web/Capacitor 使用 `sessionId → ProtocolClient` 与 `sessionId → MobileState` 的 App 级容器；HarmonyOS NEXT 使用等价的连接表与状态表。三端均只依赖现有 Pairing、WebSocket 和 wire contract，不需要平台专属协议。浏览器仍不持久化 Pair Token 或 transcript；原生端后续继续使用系统安全存储保存配对凭证。核对通过。
- **状态**：已定

## D67 · 移动端多 Session 状态使用 Zustand，连接副作用由独立 Runtime 管理

- **日期**：2026-09-22
- **决策**：React Web/Capacitor 的 App 级多 Session 状态采用 Zustand。Store 保存配对记录、`sessionId → MobileState`、未读标记、当前视图和当前展示 Session；独立 `SessionRuntimeManager` 保存 `sessionId → ProtocolClient`、连接 generation 与网络副作用，并把收到的事件派发到 Zustand。现有单 Session 纯 reducer 继续作为状态归并核心，不在组件或 Runtime 中复制状态机。
- **理由**：旧 B1 只有一个详情 Session，`useReducer + useRef` 是合理的最小实现；D66 后多个 Session 会同时更新，列表和详情需要按 Session 精确订阅，继续把多个 Map、Client 和 reducer 编排堆在 `App.tsx` 会重新耦合连接、导航与渲染。Zustand 提供轻量的外部 Store、selector 和组件外更新能力，适合该状态形态；Happy App 也采用“独立 Sync 对象负责网络、Zustand 负责可观察状态”的职责分离。
- **影响**：`apps/mobile` 新增 Zustand 运行时依赖；不使用 `persist` middleware，不写 `localStorage`、`sessionStorage` 或 IndexedDB，不持久化 Pair Token 或消息正文。`ProtocolClient` 不作为供 UI 订阅的响应式数据；组件只通过 selector 订阅所需 Session 或列表摘要，避免其他 Session 流式事件触发无关详情重渲染。HarmonyOS NEXT 不复用 npm 库，但使用等价的 App 级状态容器和 Runtime/Store 职责边界；wire protocol、Pairing、Go Server 和状态语义不变，三端可实现性核对通过。
- **状态**：已定

## D68 · Standalone CLI 支持进程内多 Session 管理

- **日期**：2026-09-23
- **决策**：Standalone CLI 要在同一进程中同时管理多个独立 Claude Session。已配对移动端可在该 CLI 的工作目录发起新建，旧 Session 保持运行；此需求不考虑 WhaleTerm 接入场景。每个 Session 的 Host、SDK Client、输入队列和状态继续独立。
- **理由**：仅靠多个 CLI 进程和不同端口，已配对手机无法直接在当前工作目录新建并管理并行会话。
- **影响**：替代 D39 及此前需求/总体设计中“Standalone 单进程只能有一个 Session”的限制；不改变“每个活跃 Session 只有一个 Host 和一个 SDK Client”、不接管外部活跃会话、手机不保存完整上下文等约束。原 D43 每 CLI 单隧道 Relay 方案需后续单独修订。CLI 管理器、协议与 UI 的具体方案见 `docs/specs/2026-09-23-standalone-cli-multi-session-design.md`，后续确认见 D69–D71。
- **状态**：已定；具体契约和产品语义由 D69–D71 补充

## D69 · CLI 多 Session 使用共享监听与可选管理协议

- **日期**：2026-09-23
- **决策**：用户检查多 Session 设计和实施计划后同意进入实现。Standalone CLI 在一个 WebSocket 监听上按 `sessionId` 分发鉴权、Channel 与配对；公共 `remote.Server` 保持单 Session。wire protocol v2 增加可选的 `create_session`、`create_session_result`、`stop_session`、`stop_session_result`，Pairing v1 可选增加纯数据 `capabilities` 字符串数组。管理结果仅回复请求连接，创建结果中的新 Pair Token 不进入旧 Session 广播；无能力字段的既有配对仍按旧行为使用。创建授权范围、默认容量与停止后入口保留语义另待用户对具体选项确认，不由本条决定。
- **理由**：共享端口让同一 CLI 的新 Session 可由已配对移动端直接连接，同时保留各 Session 独立凭证和现有单 Session 嵌入 API。可选扩展避免旧配对强制迁移。
- **三端可实现性**：消息仅使用现有 WebSocket/JSON envelope、字符串、布尔、对象和可选字符串数组；React Web/Capacitor 共用 TypeScript 协议层，HarmonyOS NEXT 在 ArkTS 协议层独立实现同样解析和请求关联。新增结果只在请求方收到，不要求任一端使用浏览器、原生安全存储或 SDK 专属协议；相同创建、失败、幂等、停止与隔离用例可跨端执行。核对通过。
- **状态**：架构与可选契约已定；三项产品语义由 D70、D71 确认

## D70 · 任一有效已订阅 Session 凭证可新建 CLI 会话

- **日期**：2026-09-23
- **决策**：用户确认持有本 CLI 任一有效 Session 配对凭证、完成 `hello` 鉴权并订阅该 Session 的客户端，可以请求该 CLI 在固定工作目录新建独立会话。凭证仍只能读取和控制自身 Session；新 Session 的独立凭证只返回创建请求所在连接，不向其他订阅者广播。
- **理由**：移动端不依赖 PC 终端，即可从任一已配对会话继续创建新会话；跨 Session 数据权限并未随创建权限扩大。
- **三端可实现性**：授权依据是服务端已有的 Session 凭证与连接状态，创建请求/结果为 WebSocket JSON；React Web、Capacitor 与 HarmonyOS NEXT 均可按同一流程发送和接收，不依赖平台私有能力。核对通过。
- **状态**：已定

## D71 · CLI 默认四个活跃会话，停止后保留配对入口

- **日期**：2026-09-23
- **决策**：CLI 默认同时最多托管四个活跃 Claude Session；创建中和停止中计入容量，已停止/失败的不计入。手机停止某 Session 仅结束其 Claude Runtime，保留认证、终态 Snapshot 和创建新 Session 的配对入口；本 Session 的执行命令继续被拒绝。彻底撤销入口另设操作，不由“停止会话”隐式触发。
- **理由**：上限限制有效凭证被滥用时的进程资源消耗；保留入口避免停止后移动端无法查看结果或继续工作。
- **三端可实现性**：容量和停止结果通过固定错误码及既有终态 Snapshot 表达，不增加平台字段；React Web、Capacitor 和 HarmonyOS NEXT 均可在原连接/重连后展示终态并继续发送创建请求。核对通过。
- **状态**：已定

## D72 · 移动端 Claude 会话按工作目录分组并使用稳定标题

- **日期**：2026-09-23
- **决策**：用户确认 Standalone CLI 与 WhaleTerm Remote 的已配对 Claude 会话均按实际 `cwd` 分组，按最近有意义的活动从近到远排序；自动标题取宿主观察到的首条非空用户输入首行，生成后保持稳定，用户手动名称优先。React/Capacitor 列表使用 antd-mobile `List`，HarmonyOS NEXT 提供等价 ArkUI 列表；会话从详情返回列表时应易于定位。无可靠目录或标题的旧会话使用明确回退展示。
- **理由**：ID 摘要无法辨认会话，当前“最近连接时间”排序让重连后的会话跳位；工作目录、稳定标题和活动摘要能让用户在多个 Claude Session 之间直接定位。
- **影响**：替代 D66 和移动端多 Session 设计中“本阶段不按项目分组”的局部范围限制，但不引入服务器端全量 Session 列表；同名路径跨机器只视为路径相同，不推断机器身份。目录和展示信息须在配对鉴权后按 `sessionId` 授权提供。协议方案见 `docs/specs/2026-09-23-mobile-project-session-list-design.md`：v2 wire 保持原样，v3 在已授权 Snapshot 可选增加 `projectCwd`、`sessionTitle`、`createdAt`、`lastActivityAt`，有意义活动事件可选增加 `lastActivityAt`，首次标题事件可选增加 `sessionTitle`；新客户端在旧服务 `unsupported_version` 后重新连接 v2，配对负载 v1 不变。时间使用 RFC3339 UTC，未知值省略。
- **三端可实现性**：版本协商、纯 JSON 可选字符串和时间戳只需要 WebSocket/JSON；React Web/Capacitor 与 ArkTS/ArkUI 均可独立解码、回退、分组和展示。antd-mobile 只用于 React 展示层，不进入协议或 Harmony 工程。旧版缺字段时有回退组和标题；认证/订阅之前不返回新元数据，保持跨 Session 隔离。三端契约核对通过；互操作须由实施测试验证。
- **状态**：产品语义与技术契约已由用户确认；本阶段用户已反馈验证通过，自动化与验收记录见 `docs/testing/2026-09-23-mobile-project-session-list-result.md`；Harmony DevEco 工程构建/真机仍待补

## D73 · C2-C 先实施无设备依赖的原生配对逻辑

- **日期**：2026-09-24
- **决策**：用户确认暂停原定自动化测试补强，先实施 C2-C 的纯逻辑准备：安全 Pairing 与普通元数据的分离端口、恢复与过期筛选、扫码结果解析入口和前台恢复策略。原生插件接线、系统安全存储实证、相机扫码及 C2-D 真机/VPN 验收继续后置。
- **理由**：没有真机时仍可固定可测试的数据和生命周期规则，同时避免把内存测试端口误当成正式安全存储。
- **影响**：细化 D54 的暂缓范围；浏览器继续只使用 Fragment 和内存，不将 Pair Token 写入 Web Storage。通信协议、Pairing 字段、Session 状态和三端共用验收语义不变；HarmonyOS NEXT 可用其独立安全存储与状态事件实现相同业务规则。
- **状态**：已定；本轮只完成逻辑与自动化门禁，C2-C 原生接线和 C2-D 仍未完成

## D74 · 继续实施 C2-C 的 Capacitor 原生接线

- **日期**：2026-09-24
- **决策**：用户确认 C2-C 纯逻辑结果后要求继续；本轮接入 iOS/Android 安全存储、普通元数据、扫码和 App 前台事件，并连接现有多 Session UI/Runtime。C2-D 真机、VPN 和真实相机验收仍后置。
- **影响**：替代 D73 中“原生接线继续后置”的阶段范围限制。浏览器仍只用 Fragment/内存；通信协议、Pairing 字段和三端共用状态不变，Harmony 可独立实现等价行为。
- **状态**：已定；Android 本机门禁与模拟器假凭证验证通过，iOS 工程编译和 C2-D 待补

## D75 · 优先修复 iOS 页面切换体验

- **日期**：2026-10-08
- **决策**：用户在 iOS 真机反馈列表、详情和配对页切换缺少动画，要求将导航体验改进提前到 C2-D 门禁之前实施。先在共用 React/Capacitor UI 实现方向一致的页面转场，并处理快速切换、滚动位置与系统“减少动态效果”；随后由用户在 iOS 真机验收。原生手势返回和长列表性能问题按真机反馈再决定范围。
- **理由**：当前 App 按状态直接替换页面，`antd-mobile` 导航栏不管理页面栈或转场，因此切换时瞬间跳变。转场可解决这个具体问题；整体流畅度还受 WebView 绘制、页面内容与设备状态影响，需分别观察。
- **影响**：C2-C 增加导航体验收尾并优先于 C2-D；只修改本地 UI 导航呈现，不改变通信协议、配对负载、Session 状态模型或三端共用验收契约。HarmonyOS NEXT 保持独立 ArkUI 导航实现。
- **状态**：已定；待实现与 iOS 真机验收

## D76 · 本地 PC 稳定编码与移动端设备管理

- **日期**：2026-10-09
- **决策**：用户要求在首页增加设置入口，用于管理不同 PC，并将 PC 机器编码纳入配对。Standalone CLI 在本机配置目录生成并持久保存随机 PC 编码；配对负载增加可选 `pcId`，同一 PC 的多个 Session 共享该编码。旧服务和未提供编码的嵌入宿主按连接地址作为兼容分组，不能宣称它是稳定机器身份。设置页按设备查看、删除本地配对；删除设备等同删除其本地会话配对，不影响 PC 上 Claude Session。
- **理由**：`sessionId` 仅标识 Claude Session，`wsUrl` 会随网络和端口变化；随机编码可稳定分组且不暴露硬件序列号。
- **三端核对**：`pcId` 是平台中立的可选配对字段。Go Pairing 生产与旧负载解码、React/Capacitor 和 HarmonyOS NEXT 都可按相同格式解析；旧记录和未升级宿主继续有效。设置页与本地存储由各端独立实现，不加入浏览器专属字段。此字段只用于展示分组，不作为认证或授权依据。
- **影响**：将此前后置的机器稳定身份提前到本轮；不建立服务器端设备中心、永久设备授权或跨 PC 控制。WhaleTerm 嵌入端可经公共 Remote Server 配置提供自己的稳定编码；未提供时保持兼容行为。
- **状态**：已定；Standalone/React/Capacitor 与三端配对契约接线已实现，本机门禁通过，iOS 真机体验待验收

## D77 · PC 名称与移动端设备层级

- **日期**：2026-10-09
- **决策**：用户将设置改为通用设置列表，PC 管理下钻且仅展示设备；首页会话列表按 PC 再按工作目录分组，保留首页新增配对与单会话操作。PC 管理显示可选主机名和从已配对 Session 连接推导的状态，不引入设备级心跳。详情抽屉采用较短确定时长并提高面板位置。
- **理由**：设备身份、连接状态和会话入口各有不同职责，原单层设置混在一起且看不到主机名；抽屉内部弹簧时长也与页面节奏不一致。
- **三端核对**：Pairing v1 可选 `pcName` 为纯文本显示元数据，Go、React/Capacitor 与 HarmonyOS NEXT 均可解析，旧负载可继续解码。`pcId` 仍是稳定分组键；`pcName`、连接地址和连接状态均不参与鉴权。UI 层级由各端独立实现，协议中没有平台依赖。
- **影响**：新 CLI 配对携带机器名；既有配对需要重新扫码才可获得机器名。WhaleTerm 可选传入 `PCName`；本地 PC 状态不是设备级网络探测。
- **状态**：已实施，本机门禁通过；iOS 真机体验待验收

## D78 · C2-C 完整体验先同步 Android，Harmony 延期

- **日期**：2026-10-13
- **决策**：用户要求当前完整 C2-C 功能与体验先同步 Android；HarmonyOS NEXT 本轮暂不同步代码，仅保证功能具备平台等价实现路径。
- **三端核对**：配对字段、WebSocket 消息和 Session 状态为纯数据，Harmony 既有协议层已支持相关可选 PC 字段。PC→目录分组、设备移除、固定状态、图标返回、页面转场、滚动恢复、抽屉、聚焦与禁用反馈可由 ArkUI 状态和组件实现；扫码、安全存储及前台生命周期需独立平台适配。键盘避让使用各端窗口布局能力，不要求 Harmony 复用 iOS keyboardLayoutGuide、DOM 或 Capacitor 插件。后续 Harmony 工程构建与真机验收仍需补齐。
- **影响**：Android 复用当前 React UI 和原生端口；Harmony 完整 C2-C UI 与原生能力接线延期，保留同一业务及安全语义。
- **状态**：已定；Android 同步和 debug 构建通过，真机验收待补。

## D79 · C2-D 先验证 iOS 真机与 WhaleTerm Remote Control

- **日期**：2026-10-13
- **决策**：用户当前没有 Android 设备，C2-D 先以 iOS 真机验证 WhaleTerm 已植入的 Remote Control 完整功能；Android 与 HarmonyOS NEXT 后续对照 iOS 验收结果补齐。测试使用 `/Users/zhangym/code/work/whale-term` 仓库实现，不以已安装桌面 App 的存在替代源码版本核对。
- **影响**：优先验证 WhaleTerm 已持有 Session 的配对、消息与状态同步、审批与问答、取消、关闭远程管理、恢复和多 Session 隔离。协议及状态模型不变；Android/Harmony 的设备门禁保持未通过，不以 iOS 结果代替。
- **状态**：已定；iOS 真机联调进行中。
