# C2 配对、生命周期与原生能力设计

- 日期：2026-08-31
- 状态：已确认（用户确认；范围按精简方案修订）
- 前置：C1、B2、B3
- 关联决策：D41、D44、D49、D50、D51、D54

## 1. 目标与范围

C2 当前只完成后端配对二维码、配对凭证恢复和 Mobile Web 生命周期闭环。Capacitor 原生扫码、安全存储、原生多 Session 持久化、iOS/Android 真机与 VPN 全部暂缓，不纳入本轮实施。

W1 WhaleTerm 接入暂缓。C2 不接入 WhaleTerm，不接管任意已经在终端运行的 Claude 进程，不创建新的 Claude Client，不改变既有 WebSocket 协议、Session 状态模型、Pairing JSON 字段或 `sessionId` 语义。

## 2. 分阶段交付

### 2.1 C2-A：后端终端二维码与配对持久化

现有 `protocol.Pairing` 是唯一配对数据来源。二维码编码与 Mobile Web 已有 Fragment 解析保持一致：

```text
#pair=<Base64URL(JSON Pairing)>
```

JSON 字段仍为 `version`、`wsUrl`、`sessionId`、`pairToken` 和 `expiresAt`。Go 端新增无填充 Base64URL 编码，与 `apps/mobile/src/protocol/pairing.ts` 已有的 `encodePairingFragment` 锁定同一编码契约：两端对同一 payload 必须产出相同字符串，互操作测试见 §5。原生扫码后的解析暂不在本轮实现。

CLI 的默认输出为终端 ANSI/ASCII 二维码，二维码内容本身不改变线上协议。本轮不生成 PNG，不增加 HTTP 二维码接口；二维码库选型与 `go.mod.base` 的依赖变更在实施计划中确定。不把 token 写入普通日志、测试快照或文档。

现有 pairing 小文件机制继续负责当前 Session 的重启恢复：仅恢复相同 `sessionId` 且未过期的 token；过期、损坏或不匹配时生成新 token；主动关闭使当前进程凭证失效。文件读写失败只降级为不恢复/不保存，不阻断 Session 启动。

### 2.2 C2-B：Mobile Web 生命周期

浏览器的配对记录继续只存在内存，不写 `localStorage`、`sessionStorage` 或 IndexedDB。URL Fragment 是刷新后唯一允许的配对输入来源。

页面行为固定为：

1. 首次加载时解析有效 Fragment 并自动连接；
2. 刷新后重新解析 Fragment，仅恢复当前一个 Session，不恢复历史消息；
3. WebSocket 断开后显示断线状态，不进行无限后台重试；
4. 用户点击“重新连接”时重新认证和订阅；
5. token 过期或鉴权失败时标记 `needsPairing`，要求重新扫码/输入；
6. 切换 Session 时关闭旧连接，任一时刻只保留一个活动连接；
7. 页面从后台恢复时只重查连接状态（如检查 WebSocket readyState），不自动重连、不绕过用户的重新连接语义。

### 2.3 暂缓：C2-C 原生扫码与安全存储

本轮不实现相机扫码、原生安全存储、原生多 Session 列表跨重启持久化或原生前后台恢复。后续重新启动 C2-C 时，Pair Token 必须使用 iOS Keychain 或 Android Keystore 支持的安全方案，普通元数据使用原生应用私有存储；不得以浏览器 Web Storage 代替。

### 2.4 暂缓：C2-D 真机与 VPN 验收

设备可用且 C2-C 完成后，再单独执行 iOS/Android 真机验收：扫码、本地网络权限、明文局域网 WebSocket、VPN 直连、后台/前台切换、系统杀进程恢复、过期凭证、多配对切换和安全存储隔离。

## 3. 数据流

```text
CLI Pairing
  ├── Pairing JSON
  ├── terminal QR
  └── Mobile URL Fragment
          ↓
      Pairing parser
          ↓
  validate version / wsUrl / sessionId / token / expiresAt
          ↓
  PairingStorage
          ↓
  one ProtocolClient WebSocket
          ↓
  hello → subscribe(sessionId) → snapshot/events/commands
```

二维码只负责承载已存在的配对负载，不授予额外权限，也不引入新的服务端 HTTP 入口。服务端仍在 `hello` 中校验 Pair Token 与精确 `sessionId`，移动端仍只接收当前 Snapshot 和连接后的事件。

## 4. 安全与失败处理

- Pair Token 只在二维码、配对 Fragment、受保护 pairing 文件和原生系统安全存储中出现；不得进入日志、提交、测试快照或普通错误信息。
- Fragment 解析失败、版本错误、非 `ws/wss` URL、空字段或已过期时，不建立 WebSocket。
- WebSocket 断开不删除有效配对记录；凭证过期或服务端拒绝认证时进入重新配对状态。
- 浏览器刷新不会凭空恢复内存列表；只有 URL Fragment 能恢复当前配对，符合浏览器不持久化 token 的约束。
- Remote Server 重启的 token 恢复继续受 `sessionId` 精确绑定和绝对 `expiresAt` 限制，不做滑动续期。

## 5. 验证策略

无真机时的通过条件：

- Go QR 编码与 Mobile Web Fragment 解码互操作，编码后 JSON 字段逐项一致；Go 端与 Mobile 端 Base64URL 实现对同一 payload 产出相同字符串；
- CLI 默认能输出可被 QR 解码器识别的终端二维码内容；
- pairing 文件重启恢复、过期替换、损坏降级和撤销行为通过测试；
- Mobile Web 刷新、断线、手动重连、过期和单连接约束通过自动化测试；
- 既有 Go/Web 门禁不回归，协议 JSON 与 B2/B3 fixture 不变。

原生扫码、安全存储、原生生命周期和 VPN 属于暂缓项，不纳入本轮 C2 通过结论；将来实施 C2-C/D 后另行记录在 `docs/testing/C2-result.md`，不得用桌面浏览器测试替代真机结论。

## 6. 明确不做

- WhaleTerm W1 接入；
- 任意外部终端 Claude 进程的监听、注入或接管；
- 浏览器端持久化 Pair Token；
- PNG 文件二维码输出和 HTTP 二维码接口；
- 原生扫码、安全存储、原生多 Session 持久化、真机和 VPN 验收；
- 公网 Relay、推送和语音；
- 服务端多 Session Registry；
