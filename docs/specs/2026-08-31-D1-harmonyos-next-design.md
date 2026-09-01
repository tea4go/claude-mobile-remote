# D1 HarmonyOS NEXT 工程与协议接入设计

- 日期：2026-08-31
- 状态：草案，待用户确认
- 前置阶段：B2、C1、B3 已完成并通过门禁；C2 已确认当前精简范围，但 D1 仍遵守总路线中的 `C2 -> D`
- 关联决策：D7、D19、D46、D47、D48、D50、D51、D54

## 1. 目标

D1 的目标不是把现有 iOS / Android 的全部能力一次性搬到 HarmonyOS NEXT，而是先建立一个**独立于 Capacitor 的 ArkTS / ArkUI 工程**，并验证它能稳定消费当前项目已经定下来的三端共用契约：

1. 建立 `apps/harmony/` 独立工程，不复用 `apps/mobile/ios`、`apps/mobile/android` 或 React 组件二进制；
2. 在 ArkTS 侧完整实现当前版本的配对解析、协议解码、连接编排和基础状态归并；
3. 提供一个最小可运行的 Harmony 调试壳，能够手工导入配对信息、连接当前 Session、展示 snapshot / event、发送文本命令并手动重连；
4. 为后续 D2 的功能迁移提供稳定工程边界，避免到 D2 才发现协议或状态模型在 Harmony 上不可落地。

D1 的核心产出是**工程边界与协议边界的落地**，不是视觉对齐，也不是原生能力完备。

## 2. 范围

### 2.1 包含

- 创建 `apps/harmony/` 独立 HarmonyOS NEXT 工程；
- 建立 ArkTS 版协议模块：配对 payload、envelope、snapshot、event、command 的类型、严格解码与编码；
- 建立单 Session 的连接与状态模块：`hello -> subscribe -> snapshot`、事件流归并、断开、手动重连；
- 提供最小页面壳：
  - 配对输入页；
  - 当前 Session 详情页；
  - 基础状态栏、消息流、待处理事项摘要、排队请求摘要；
- 提供至少一个可用的文本发送入口，用于启动或继续当前 Turn；
- 保持 Pair Token 只在当前页面内存中存在，不写入普通日志或不受保护存储。

### 2.2 不包含

- 不把 `apps/mobile` React Web UI 套进 WebView 作为 Harmony 方案；
- 不追求与 iOS / Android 当前 UI 的视觉一致或组件级复用；
- 不实现扫码、系统安全存储、推送、前后台恢复、系统杀进程恢复；
- 不实现完整 B2 交互能力的最终 UI，包括：
  - Thinking / Tool 的正式卡片样式；
  - `resolve_action` 的审批与问答控件；
  - `cancel_queued`、`cancel_turn` 的完整交互收口；
- 不实现多 Session 持久化列表；
- 不宣称 VPN、真机、发布或商店分发可用；
- 不改变既有 Go 服务协议、Pairing JSON 字段、`sessionId` 语义和 `seq` 顺序规则。

### 2.3 阶段定位

本设计建议把 D 阶段拆成：

1. **D1：工程与协议接入**
2. **D2：功能迁移**
3. **D3：真机、推送与发布验证**

其中 D1 只解决“能不能在 HarmonyOS NEXT 上以独立原生工程吃下当前契约”，不解决“是否已经达到日常使用完成度”。

## 3. 总体方案

HarmonyOS NEXT 端沿用现有项目的一 Session 模型和服务端契约，但完全独立实现客户端：

```text
Go Session Host / Remote Server
        │
        │ WebSocket + JSON envelope v2
        ▼
apps/harmony/
├── Pairing parser
├── Protocol decoder / encoder
├── ProtocolClient
├── Session reducer / store
└── ArkUI pages / components
```

设计原则：

- **协议复用，UI 不复用**：复用相同字段、状态和事件语义，不复用 React/Capacitor 运行时；
- **单连接、单 Session**：D1 与当前 Web 客户端一致，一次只连接一个当前 Session；
- **严格解码，不偷宽容**：ArkTS 解码规则与 Go / TypeScript 契约保持一致，避免 Harmony 端自行发明兼容分支；
- **原生能力后置**：扫码、安全存储、推送、生命周期恢复都留到 D2/D3，再按 Harmony 官方能力补齐。

## 4. 工程结构

D1 建议使用 HarmonyOS NEXT 标准 Stage 工程，目录只放 Harmony 自己的实现，不复制 `apps/mobile/src`：

```text
apps/harmony/
├── AppScope/
├── entry/
│   └── src/main/ets/
│       ├── entryability/
│       ├── pages/
│       │   ├── PairingPage.ets
│       │   └── SessionPage.ets
│       ├── protocol/
│       │   ├── types.ets
│       │   ├── pairing.ets
│       │   ├── decode.ets
│       │   └── client.ets
│       ├── session/
│       │   ├── model.ets
│       │   ├── reducer.ets
│       │   └── store.ets
│       └── components/
│           ├── StatusBar.ets
│           ├── MessageList.ets
│           └── PendingSummary.ets
├── oh-package.json5
└── build-profile.json5
```

约束：

- `protocol/` 只承载三端共用契约的 ArkTS 表达，不混入 ArkUI 视图代码；
- `session/` 只承载本地状态归并，不直接依赖页面；
- 页面只消费 `session/` 的稳定状态，不自行解析原始 WebSocket 消息；
- D1 不为 Harmony 预埋第二套服务端协议，也不把 Harmony 特有字段加回 Go 端。

## 5. 协议与数据模型

### 5.1 配对负载

D1 继续复用当前配对字段：

```text
version / wsUrl / sessionId / pairToken / expiresAt
```

行为约束保持一致：

- 只接受 `ws://` 或 `wss://`；
- `sessionId`、`pairToken`、`expiresAt` 不能为空；
- 已过期 payload 不建立连接；
- `sessionId` 仍只是逻辑身份，真正授权依然由 `sessionId + pairToken` 共同决定；
- 不增加 Harmony 专属字段，不新增二维码承载格式。

D1 的配对导入方式只要求**手工粘贴**现有配对字符串或等价文本；扫码属于后续阶段。

### 5.2 Envelope 版本

D1 必须直接消费当前版本 2 协议，不再降回版本 1：

```text
version / type / sessionId / commandId / requestId / seq / payload
```

ArkTS 解码器必须与现有 B2 契约对齐：

- `snapshot`、`event`、`unsupported`、`command_result`、`error` 的字段校验严格执行；
- 未知 `type`、未知 `kind`、字段缺失、字段类型错误时，视为协议错误；
- `unsupported` 是独立 envelope，不是第四种 `content_block.kind`；
- `error.code=unauthorized` 进入“需要重新配对”，而不是普通断线。

### 5.3 首轮前与空枚举约束

D1 必须显式兼容 A5.2 已验证的“首轮前已可配对”语义，避免重犯移动 Web 之前的解码收窄问题：

- `turn.source` 在首轮前允许为空串；
- `event.source` 在不适用时允许为空串；
- 只有当 `requestId` 非空且语义要求来源存在时，才要求其值为 `terminal` 或 `mobile`；
- 不允许把空串一律当成非法枚举后直接断连。

### 5.4 结构化内容与待处理事项

D1 的协议层必须完整理解当前 B2 已确认的数据：

- `content_block.kind`：`thinking`、`tool_use`、`tool_result`
- `pendingAction.kind`：`tool_approval`、`ask_user_question`
- `action_resolved.answers`
- `queuedRequests`
- `unsupported.code`

但 D1 页面只需要做**最小展示**：

- `thinking`：显示为只读摘要块；
- `tool_use` / `tool_result`：显示工具名、预览和最终状态，不做完整视觉打磨；
- `pendingAction`：显示当前有待处理事项及其摘要，不强制在 D1 就做完整审批表单；
- `queuedRequests`：显示 request ID、来源与预览，证明 FIFO 视图在 Harmony 可见。

### 5.5 命令面

为避免 D2 再重写一遍底层协议，D1 的 ArkTS `ProtocolClient` 直接保留完整命令面：

- `send_text`
- `resolve_action`
- `cancel_turn`
- `cancel_queued`

但 D1 页面只要求实际接通 `send_text`；其他命令是否在 D1 暴露按钮，由实施计划按最小验证需要决定。这样可以同时满足：

- D1 已经验证 Harmony 端命令编码与连接编排无误；
- D2 迁移交互能力时不必返工底层协议客户端。

## 6. 本地状态与连接模型

### 6.1 单 Session、单连接

D1 延续当前客户端边界：

- 任一时刻只保持一个活动 WebSocket；
- 只连接一个当前 Session；
- 不建立后台多连接；
- 不做事件 replay，只重取最新 snapshot。

### 6.2 内存态

D1 的本地状态只保存在内存：

```text
HarmonySessionState
├── pairing
├── connection
├── snapshot
├── timeline
├── pendingCommandId
└── protocolError / pairingInvalid
```

约束：

- Pair Token 不写入普通存储；
- 页面退出或进程结束后状态可丢失；
- 断线后由用户手动触发重连；
- 若 Pair Token 过期或被拒绝，进入重新配对态而不是自动反复重试。

### 6.3 ProtocolClient 语义

ArkTS `ProtocolClient` 应复用当前 Web 客户端的核心语义：

1. 建连后先发 `hello`；
2. `hello_ack` 后发 `subscribe(sessionId)`；
3. 收到 `subscribed` 后等待 `snapshot`；
4. 进入 ready 后才允许命令发送；
5. 同一时刻最多一个在途 `commandId`；
6. `command_result` 只表示命令被接受，不代表 Turn 完成；
7. 非正常 close 统一进入断线态，由页面决定是否手动重连。

这部分属于三端共用行为，不允许因为 ArkTS 页面写法不同而改变。

## 7. 页面与交互

### 7.1 PairingPage

目标：完成最小导入和连接准备。

包含：

- 文本输入框，用于粘贴配对字符串；
- 解析与校验结果提示；
- “连接当前 Session”按钮；
- 对过期、字段错误、非 `ws/wss`、版本不匹配的明确错误提示。

不包含：

- 相机扫码；
- 本地已配对列表；
- 历史记录；
- 后台自动恢复。

### 7.2 SessionPage

目标：证明 Harmony 端能看懂当前协议并参与一个 Session。

包含：

- 连接状态、Session 状态、Turn 状态、`seq`、`sessionId` 摘要；
- 文本消息流；
- 结构化内容的简化块；
- 当前 `pendingAction` 摘要；
- `queuedRequests` 摘要；
- 文本输入框；
- 断开与手动重连入口。

建议：

- D1 优先保证“信息可见、状态可判断、连接可控”；
- 视觉样式不对齐 B1/B2 的现有 React 页面；
- 若 pending action 已出现但 D1 未暴露完整决议控件，页面必须明确提示“该交互能力留待 D2”。

## 8. Harmony 原生边界

### 8.1 不复用 Capacitor

HarmonyOS NEXT 不加入 `apps/mobile` 的 Capacitor 工程，也不运行 React 页面。D1 只能复用以下内容：

- 配对负载字段；
- 版本 2 协议；
- Session / Turn 状态语义；
- 事件和命令的 wire contract；
- 已确认的验收行为。

以下内容不得视为 Harmony 依赖：

- antd-mobile 组件；
- React reducer 实现本身；
- Capacitor 生命周期或平台配置；
- Android / iOS 的 WebView 网络策略。

### 8.2 原生能力后置

以下事项不在 D1 落地，但必须在设计上留出位置：

- 相机扫码；
- 系统安全存储；
- 剪贴板或深链导入优化；
- 前后台恢复；
- 推送与通知；
- 真机 VPN 验证。

其中网络权限、明文 `ws://`、`wss://`、应用权限声明等平台工程项，实施时必须以 HarmonyOS 官方文档和 DevEco 实际工程为准；在未核实前，不把任何推测字段写成三端共用契约。

## 9. 验证范围

D1 的通过条件应围绕“工程建立 + 协议打通 + 最小壳验证”定义。

### 9.1 自动化与构建

至少包括：

- Harmony 工程可被 DevEco / `devecocli` 识别；
- ArkTS 协议与配对模块有单测或等价验证；
- 构建成功；
- 不引入与三端契约冲突的新字段或新版本分支。

协议测试应覆盖：

- 配对 payload 正常解析；
- 过期 payload、空字段、非法 URL、版本不匹配拒绝连接；
- `snapshot.turn.source == ''` 的首轮前场景；
- `content_block` / `pendingAction` / `queuedRequests` / `unsupported` 的严格解码；
- `error.code=unauthorized` 与普通断线的区别处理；
- 单在途 `commandId` 门控。

### 9.2 联调

至少做以下联调：

- 使用现有 Go 服务与假 Host 桩；
- 手工导入配对信息；
- 完成 `hello -> subscribe -> snapshot`；
- 显示文本消息；
- 发送一条文本命令并收到 `command_result` 与后续事件；
- 断开后手动重连并恢复最新 snapshot；
- 错 token / 过期 token 明确进入重新配对态。

### 9.3 D1 不得宣称

- 真机可用；
- VPN 可用；
- 扫码已完成；
- 安全存储已完成；
- 完整 B2 交互已完成；
- 发布与上架可用。

这些都属于 D2 或 D3。

## 10. 与 D2 / D3 的衔接

### 10.1 建议的 D2 范围

D2 在 D1 工程与协议稳定后，迁移当前 Web 客户端已经具备的业务能力：

- 多 Session 列表与切换；
- Markdown / 代码块 / Thinking / Tool 的正式展示；
- `resolve_action` 的审批与问答交互；
- `cancel_turn` / `cancel_queued` 的完整 UI；
- 会话错误、终态与答案回显的完整呈现；
- 需要时纳入 Harmony 的扫码与安全存储设计。

### 10.2 建议的 D3 范围

D3 再处理设备与发布层面的验证：

- 真机；
- VPN；
- 前后台切换；
- 系统杀进程恢复；
- 推送；
- 打包、签名、发布验证。

## 11. 待确认点

本草案默认以下假设；若用户不接受，应先改设计再进入实施计划：

1. D1 只做独立工程与协议接入，不要求第一版就具备 B2 完整交互闭环；
2. Harmony 不走 WebView 套壳，而是 ArkTS / ArkUI 独立实现；
3. D1 本地状态只保存在内存，扫码与安全存储不提前塞进本阶段；
4. D2 才追求对当前 `apps/mobile` 功能面的真正迁移，而不是在 D1 里混做。
