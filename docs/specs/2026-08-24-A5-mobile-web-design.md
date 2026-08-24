# A5 Mobile Web 分阶段设计

- 日期：2026-08-24
- 子阶段：A5 Mobile Web（里程碑 A）
- 状态：已确认（含 2026-08-24 审查批注：认证失效窗口、非正常 close 统一处理、消息方向表、connectionStatus 忽略、UTF-8 字节上限）
- 依据：`REQUIREMENTS.md` §6、§9.1，总体设计 §4–§9，D27–D36

## 1. 目标

A5 在 `apps/mobile` 的 React + TypeScript + antd-mobile + Vite 骨架上实现首个移动浏览器页面，原样消费 A4 版本 1 WebSocket 契约，完成本机浏览器中的配对、认证、订阅、状态展示、扫码后文本展示、文本发送和刷新式重连闭环。

A5 是功能门禁，不是完整移动产品。页面只维护当前连接期的易失消息流，不重建 Claude 历史；不启用视觉辅助，不实现精细视觉、完整 Markdown、Tool/审批/取消、摄像头扫码、自动重试、Capacitor、HarmonyOS NEXT 或 VPN 手机联调。

## 2. 范围

### 2.1 A5 实现

- 从 `#pair=<Base64URL(JSON)>` 自动加载 A4 配对负载；
- 粘贴 A4 输出的原始 pairing JSON，并规范化为相同 Fragment；
- 严格解析版本 1 pairing 与 WebSocket envelope/payload；
- 执行 `hello → hello_ack → subscribe → subscribed → snapshot`；
- 归并 snapshot 和 event，展示 Session、Turn 及本连接内文本；
- 发送 `send_text`，处理接受、业务拒绝和结果未知；
- 刷新后凭 Fragment 自动重新认证并取得最新 snapshot；
- 运行中断线后显式重连，以及显式更换配对；
- 自动化前端测试与真实 Service 的本机浏览器闭环记录。

### 2.2 A5 不实现

- 二维码图片、摄像头扫码和系统剪贴板权限；
- 完整 transcript、扫码前历史、event replay 或消息持久化；
- 后台自动重试、退避、心跳、跨连接命令幂等或自动重发；
- Markdown、代码高亮、Tool、审批、回答、取消和权限交互；
- 品牌视觉、复杂动效、视觉方案比较或精细响应式设计；
- Capacitor、iOS/Android 配置、HarmonyOS 和 VPN 真机验证；
- A4 服务端协议、状态模型或配对负载变更。

## 3. 方案与模块边界

A5 采用“小型协议模块 + React reducer 状态机”，不把连接和页面状态全部堆入 `App` 或单一 Hook，也不为单页面引入 Zustand 等状态库。

```text
apps/mobile/src/
├── protocol/
│   ├── types.ts       # 版本 1 pairing、envelope、payload 与领域枚举
│   ├── pairing.ts     # Fragment/JSON 解析、校验与规范化
│   ├── decode.ts      # 不可信网络 JSON 的运行时严格解码
│   └── client.ts      # 单 WebSocket 生命周期与协议编排
├── session/
│   └── reducer.ts     # 连接、snapshot、event、消息和命令归并
├── components/
│   ├── PairingForm.tsx
│   ├── SessionStatus.tsx
│   ├── MessageFeed.tsx
│   └── MessageComposer.tsx
└── App.tsx            # Fragment、client 创建/销毁与页面编排
```

文件名可在实施计划中按测试拆分微调，但必须保持：协议类型集中维护，组件不复制消息定义；`client` 不依赖 React 或 DOM；reducer 是可独立测试的纯状态层；`App` 只接线 Fragment、连接副作用和组件；A5 不创建尚无复用方的跨端共享包。

```text
Fragment / 原始 JSON
        ↓
pairing 严格校验
        ↓
单 WebSocket client
        ↓
版本 1 协议解码
        ↓
React reducer
        ↓
状态、消息与发送界面
```

## 4. 配对、Fragment 与凭证安全

### 4.1 两种入口

页面支持 URL Fragment `#pair=<Base64URL(JSON)>` 和原始 JSON `{version, wsUrl, sessionId, pairToken, expiresAt}`。原始 JSON 校验通过后，以 UTF-8 JSON、无填充 Base64URL 编码为规范 Fragment，再启动连接。

刷新时保留 Fragment，使有效期内的 token 可以重新认证；不使用 localStorage、sessionStorage、IndexedDB 或 Cookie。A5 不引入 History API 路由，符合后续 Capacitor 的 hash/内存路由约束。

### 4.2 严格校验

pairing 必须为版本 1 对象；`wsUrl` 必须是绝对 `ws:` 或 `wss:` URL；`sessionId` 和 `pairToken` 必须非空；`expiresAt` 必须是可解析且未到期的时间。错误只返回固定类别与安全文案，不拼接原始 JSON、网络消息或 token。页面、日志和测试快照均不得回显真实 token。

### 4.3 更换配对

“更换配对”会关闭 WebSocket、使当前连接代次失效、清空 Fragment、清空消息/snapshot/未决命令，并返回 JSON 输入页。配对无效或过期时也停留在输入页，不建立连接。关闭 WebSocket 即解除订阅，A5 不新增注销协议。

## 5. 协议客户端与连接状态机

### 5.1 运行时解码

TypeScript 类型不能替代网络校验。decoder 必须验证 envelope 固定字段、`version === 1`、对象 payload、已知消息类型与方向、各 payload 结构、领域枚举，以及对应消息的 ID/seq 基本约束。未知版本、未知类型或错误结构均终止当前连接并显示不含敏感数据的协议错误。

方向校验依据 A4 设计 §5 的消息表：`hello`、`subscribe`、`unsubscribe`、`command` 只能来自客户端，`hello_ack`、`subscribed`、`snapshot`、`event`、`command_result`、`error`、`unsubscribed` 只能来自服务端；收到方向不符的消息按协议错误处理。

### 5.2 正常流程

```text
未配对
  → 连接中
  → 认证中
  → 订阅中
  → 已连接
```

有效 Fragment 首次加载或刷新时自动连接。WebSocket 打开后严格执行：

```text
hello → hello_ack → subscribe → subscribed → snapshot
                                      ↓
                         event / command_result / error
```

收到 snapshot 前不得发送文本。认证、订阅或协议失败都终止当前连接，不跳过阶段继续处理。

每次创建连接分配单调递增的前端连接代次。所有回调携带所属代次；重连、更换配对或组件卸载后，旧连接的迟到回调必须被忽略。

### 5.3 断线与重连

- 首次加载或刷新自动连接；
- 运行期断线后显示“已断开”，不后台重试；
- token 有效时提供“重新连接”和“更换配对”；
- token 过期后禁用重连并引导更换配对；服务端返回 `unauthorized` 拒绝认证时同样将当前配对视为失效，禁用重连并直接引导更换配对，不进入必然失败的显式重连循环（前端时钟校验通过但服务端已过期时会出现该窗口）；
- 重连新建 WebSocket，完整执行认证、订阅和 snapshot；
- 重连清空旧连接消息，以最新 snapshot 重新初始化；
- 断线时的 pending command 标记“结果未知”，不自动重发。

运行期收到非正常 WebSocket close 一律按“已断开 + 结果未知”处理，不区分断线原因：A4 在连接 outbox 或提前事件缓存溢出时只做普通关闭，不发送协议错误，客户端无法从 close 本身区分“网络断开”与“因慢被服务端关闭”；两种情况都以显式重连后的新 snapshot 重建状态。

Session `stopped` 或 `failed` 时保留最终状态和消息并禁用输入，但不伪造网络断线。

## 6. reducer 与易失消息流

### 6.1 snapshot

snapshot 初始化 `sessionId`、Session 状态/错误、基准 `seq`、当前或最近 Turn，以及连接时的 `queueDepth`。

A4 的 snapshot payload 另含 `connectionStatus` 字段，但它是服务端视角的连接订阅状态（收到 snapshot 时恒为 `subscribed`），不含客户端不可获知的信息；A5 以本地连接状态机为准，忽略该字段，不做双向同步。

`turn.message` 是该 Turn 已累计的 Claude 输出，不是用户输入；非空时作为 Claude 纯文本初始化。页面根据 `source` 标记“手机端发起”或“电脑端发起”，不猜测 snapshot 中不存在的用户正文。

A4 event 不含队列深度，客户端无法可靠维护实时值。初始 `queueDepth > 0` 时可一次性提示“连接时有 N 条等待任务”，状态栏不持续展示推断值。

### 6.2 event

- `turn_started`：按 `requestId` 建立或激活 Turn；
- `text_delta`：按 `requestId` 追加到同一条 Claude 回复；
- `turn_completed` / `turn_failed`：更新 Turn 终态和错误；
- Session 事件：更新 Session 状态和发送能力。

event `seq` 必须从 snapshot 基准严格连续递增。重复、倒退或缺口都表示状态不可信，当前连接按协议错误关闭并要求新 snapshot；A5 不做乱序缓存或 replay。

### 6.3 消息内容

内存消息流只保存本连接期内已被 Service 接受的手机输入、无正文的来源标记、按 request ID 累积的 Claude 文本，以及失败/拒绝/结果未知反馈。

终端 `turn_started` 不含终端输入，只显示“电脑端发起”。刷新后即使 snapshot 的来源为 mobile，也只显示“手机端发起”，因为旧命令正文不在 snapshot 中。重连不保留旧消息，也不从浏览器存储恢复 transcript。

服务端文本按纯文本渲染并保留换行；A5 不转换 Markdown，也不把文本注入为 HTML。

## 7. 文本发送

只有已收到当前 snapshot、Session 可接受输入、没有等待 `command_result` 的 Web 命令，且文本去除外层空白后非空、UTF-8 不超过 1 MiB 时才能发送。当前 Turn 正在运行或已有输入排队时仍可提交，跨入口顺序由 PC Session Controller 保证。

1 MiB 上限按 UTF-8 字节数计算，不是 UTF-16 代码单元数：中文等非 ASCII 文本在 JavaScript `string.length` 中每个字符计 1，但实际 UTF-8 编码占 3 字节。客户端必须用 `new TextEncoder().encode(text).length`（或等价字节长度）校验，与服务端 Go 端按字节数的判断保持一致。

发送流程：

1. 由可注入 ID 生成器产生唯一 `commandId`；浏览器实现可使用 Web Crypto；
2. 冻结当前输入为 pending，禁用再次发送；
3. 发送 `command` / `send_text`；
4. 按 `commandId` 等待 `command_result`；
5. 接受成功时取得 `requestId`，把手机正文加入消息流并显示“已接收”，清空输入并允许下一次发送；
6. 业务拒绝时不加入消息流，保留输入并显示安全错误；
7. 断线时标记“结果未知”，不重发。

“已接收”不表示已执行或处于特定队列位置。A4 已保证成功 result 先于同 request ID 的 `turn_started`，客户端不实现乱序补偿。

## 8. 最小功能界面

配对页包含标题、说明、原始 JSON 多行输入、“连接”按钮和就地错误；有效 Fragment 自动连接，无效 Fragment 显示原因并允许输入新 JSON。

会话页包含：

- 连接、Session 和当前 Turn 状态；
- 可选的“连接时有 N 条等待任务”提示；
- 断线时的“重新连接”，以及始终可访问的“更换配对”；
- 手机文本、来源标记、Claude 流式纯文本和错误反馈；
- 底部固定多行输入与“发送”；
- 等待 command result 时禁用输入，Turn 运行时不禁用；
- 用户接近底部时自动滚动，向上阅读时不强制拉回。

使用 antd-mobile 提供清晰、可触控的基础布局，只保证窄屏和桌面本机调试可用；不启用视觉辅助，不以品牌视觉或精细样式为门禁。

## 9. 错误边界

- pairing 错误：不连接，停留配对页；
- 建连、认证或订阅错误：终止连接，token 有效时允许显式重连；
- 协议或 seq 错误：状态不可信，终止连接并重新取 snapshot；
- command 业务拒绝：保持连接，保留输入；
- Turn 失败：结束当前 Turn，不擅自结束仍可用的 Session；
- Session 失败或停止：禁用发送并保留最终展示。

任何错误都不得包含 pair token、完整 pairing JSON 或原始 hello。测试只使用显然虚构的占位凭证，页面不把网络正文写入控制台。

## 10. 测试与门禁

### 10.1 自动化

1. pairing：Fragment、原始 JSON、Base64URL、过期、非法 scheme、错误版本和 token 不泄露；
2. decoder：合法消息及未知版本、类型、字段、枚举、ID 和 payload；
3. reducer：snapshot、严格 seq、流式累积、Turn/Session 终态、连接时队列提示、命令接受/拒绝/未知和重连清空；
4. client：完整握手、snapshot 前禁发、显式关闭/重连、服务端溢出关闭与网络断开同表现（均为“已断开 + 结果未知”，可显式重连）和旧代次隔离；
5. React Testing Library：配对、自动连接、状态、发送禁用、更换配对和安全错误文案。

WebSocket、时钟、ID 和 location 边界应可注入或替换；测试不依赖真实 sleep、真实 token 或外网。

```bash
cd apps/mobile
npm test
npm run lint
npm run build
cd ../..
git diff --check
```

### 10.2 本机浏览器闭环

1. 启动真实 CLI Service、真实 Claude 和 Mobile Web；
2. 粘贴 Service 输出的 pairing JSON 并连接；
3. 验证 snapshot、连接状态和 Claude 流式文本；
4. 浏览器发送文本，确认“已接收”、Turn 事件和 Claude 输出；
5. 终端发送下一条文本，浏览器显示“电脑端发起”和输出；
6. 刷新页面，确认 Fragment 自动连接并取得最新 snapshot；
7. 制造运行期断线，确认不自动重试且可显式重连；
8. 验证“更换配对”清除 Fragment 和旧连接内容。

结果写入 `docs/testing/A5-result.md`，记录实际命令、自动门禁、人工结论、未验证项和下一步。A5 不以 VPN 手机或视觉验收为通过条件。

## 11. 三端共用契约核对

| 契约 | A5 使用方式 | 三端可实现性 |
| --- | --- | --- |
| 通信协议 | 原样消费 A4 版本 1 JSON envelope、握手与命令顺序 | React/TypeScript、Capacitor 与 ArkTS 均支持标准 JSON/WebSocket |
| 状态模型 | snapshot 初始化，event 按连续 seq 归并；不推断实时队列 | reducer 是 Web 私有实现，相同纯数据规则可在其他端重写 |
| 配对负载 | 原样解析 D28 JSON；Fragment 仅为 Web 承载方式 | 原生端可使用二维码或深链，不依赖 Fragment |
| 验收用例 | 认证、订阅、snapshot、命令、result 先于 Turn、重连 snapshot | C/D 可复用相同行为断言 |

A5 不把 DOM、存储、React、Web Crypto 或路由语义加入共享协议。命令 ID 的平台实现可以不同，只需满足 A4 的非空唯一字符串契约。三端可实现性核对通过。

## 12. 实施前检查点

本文经用户检查确认后，下一步使用 writing-plans 编写 A5 实施计划，并按 TDD 拆分 pairing/decoder、reducer、WebSocket client、React 页面和闭环门禁。实施计划获确认前不修改 A5 代码、配置或依赖。
