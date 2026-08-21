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
- **理由**：已验证可覆盖进程生命周期、流式事件、权限、取消、恢复和管道日志。
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
- **状态**：已定

## D5 · 局域网 WebSocket 与 session 通道

- **日期**：2026-08-21
- **决策**：首期手机通过局域网 WebSocket 连接 Service，并订阅指定 `sessionId` 的逻辑消息通道。
- **理由**：WebSocket 支持实时事件和双向操作；操作系统 Pipe 不能作为手机网络协议。
- **影响**：公网中转最低优先级，协议需保留未来更换传输层的边界。
- **状态**：已定

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
- **影响**：检查点更新 HANDOFF；仅在决策变化时更新 DECISIONS；CLAUDE 引用 AGENTS，避免规则重复。
- **状态**：已定
