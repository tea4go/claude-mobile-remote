# C2-A/C2-B 配对二维码与 Web 生命周期实施计划

> **历史说明（2026-09-22）：** 本计划已执行部分采用“切换/返回列表关闭旧连接”的临时方案，该连接生命周期已被 D66 替代；后续多 Session 同步改造使用独立新计划。

## 目标

在不引入原生扫码、安全存储、PNG 文件或 HTTP 二维码接口的前提下，为 Standalone CLI 增加可扫描的终端二维码，并补齐 Mobile Web 的刷新、断线、手动重连和页面生命周期验证。

本计划不接入 WhaleTerm，不接管外部 Claude 进程，不改变 WebSocket envelope、Session 状态模型、Pairing JSON 字段或 `sessionId` 语义。

## 现状与实现边界

- `protocol.Pairing` 已拥有 `version`、`wsUrl`、`sessionId`、`pairToken`、`expiresAt` 五个字段；配对 JSON 由 Go 端产生，Web 端在 `apps/mobile/src/protocol/pairing.ts` 中解析。
- Web 端已经支持 `#pair=<Base64URL(JSON)>` Fragment、内存 Session Store、断线状态和用户触发的 `reconnect()`；实施前先用测试确认现有行为，只有缺口才修改。
- `internal/pairing.Manager` 已支持当前 Session 的 pairing 文件恢复、过期替换、`0600` 文件权限和当前进程撤销；本计划不重写该逻辑，只增加它与二维码编码的组合验证。
- 二维码默认承载纯 `#pair=...` Fragment。它可以被扫码器读取后粘贴到 Mobile Web 地址栏；本阶段不增加完整网页 URL 前缀、原生 Deep Link、PNG 输出或 HTTP 页面。

## 任务 1：建立 Go 与 Mobile Web 共用的 Fragment 编码契约

**文件：**

- 修改：`protocol/pairing.go`
- 新增测试：`protocol/pairing_test.go`
- 参考并补充跨端 fixture：`apps/mobile/src/protocol/pairing.test.ts`

步骤：

1. 先为 `protocol.EncodePairingFragment(Pairing) string` 写测试。`Pairing` 是固定结构（字符串 + int + `time.Time`），`json.Marshal` 不会失败，与 mobile 端 `encodePairingFragment` 同步返回 `string` 保持一致，故函数签名直接返回 `string` 不返回 `error`。测试使用固定 Pairing，断言结果为 `#pair=` 前缀加无填充 Base64URL，并能反解出与原 JSON 字段完全一致的对象。
2. 使用 `json.Marshal` 和 `base64.RawURLEncoding` 实现编码；不得包含换行、`+`、`/` 或 `=`。
3. 保持 JSON 字段顺序和现有 `Pairing` JSON tag 不变；不把 Fragment 编码逻辑放入 `remote` 或移动端专属包。
4. 在 TypeScript 测试增加同一固定 payload 的预期 Fragment fixture，验证 Go 端输出可直接被 `parsePairingFragment` 解析；如 Go 时间格式与 TypeScript fixture 存在差异，以 Go 实际 JSON 字符串作为 fixture 输入，不能修改线上字段格式。

验证：

```bash
GOFLAGS=-buildvcs=false go test ./protocol -count=1
cd apps/mobile && npm test -- src/protocol/pairing.test.ts
```

预期：Go 和 Mobile Web 对同一 Pairing 的 Fragment 编码契约一致，既有 Pairing JSON 测试不回归。

## 任务 2：增加终端二维码渲染

**文件：**

- 修改：`go.mod.base`、本地 `go.mod`（本地文件不提交）
- 新增：`internal/pairing/qr.go`
- 新增测试：`internal/pairing/qr_test.go`
- 修改：`cmd/claude-mobile-remote/run.go`
- 修改测试：`cmd/claude-mobile-remote/run_test.go`

步骤：

1. 采用纯 Go 二维码库 `github.com/skip2/go-qrcode`。该库没有正式 release tag，用最新 pseudo-version 固定：`github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e`；先在本地 `go.mod` 添加依赖并把同一版本同步到 `go.mod.base`；不引入 PNG API或其它图片库。
2. 在 `internal/pairing/qr.go` 提供精确的 CLI 渲染函数：

   ```go
   func RenderTerminalQR(payload protocol.Pairing) (string, error)
   ```

   函数内部先调用 `protocol.EncodePairingFragment`，再用 `qrcode.New(content, qrcode.Medium)` 和 `QRCode.ToString(false)` 生成 UTF-8 终端的 Unicode 半块字符（▀▄█）文本，非 ASCII；非 UTF-8 终端会显示异常，属已知限制。函数不得写文件、不得改变 Pairing Manager 状态；输出只包含二维码图形和必要换行，不另外打印 Pair Token。

3. 为二维码渲染写测试：
   - 二维码库返回的编码错误会原样转换为调用方可识别的错误；
   - 正常 payload 输出非空、多行二维码文本；
   - 单独测试 `protocol.EncodePairingFragment` 的内容等于 `#pair=...`；
   - 断言渲染结果非空且包含多行二维码图形，不额外包含人类可读的 Pair Token 或凭证诊断文本。
4. 在 `run.go` 的首次配对输出位置保留现有 `pairing: <JSON>`，紧接输出 `pairing QR:` 和终端二维码。保留 JSON 是为了兼容现有 A6 手动粘贴流程，二维码是新增展示，不改变已有用户脚本中的 JSON 行格式。
5. CLI 运行时二维码渲染失败必须输出固定错误类别并返回失败码，不打印原始 token 或编码内容。

验证：

```bash
gofmt -w protocol/pairing.go protocol/pairing_test.go internal/pairing/qr.go internal/pairing/qr_test.go cmd/claude-mobile-remote/run.go cmd/claude-mobile-remote/run_test.go
GOFLAGS=-buildvcs=false go test ./protocol ./internal/pairing ./cmd/claude-mobile-remote -count=1
GOFLAGS=-buildvcs=false go build ./cmd/claude-mobile-remote
```

预期：CLI 同时保留可粘贴 JSON 和可扫描终端二维码；不生成文件、不监听新 HTTP 路由、不改变 WebSocket 服务。

## 任务 3：补齐 Mobile Web 页面生命周期边界

**文件：**

- 修改：`apps/mobile/src/App.tsx`、`apps/mobile/src/protocol/client.ts`，增加可注入的页面可见性处理
- 修改测试：`apps/mobile/src/App.test.tsx`、`apps/mobile/src/protocol/client.test.ts`

步骤：

1. 先运行现有“断线与重连”测试，确认已有行为：断线显示 `disconnected`、不自动重连、点击按钮后调用 `reconnect()`、配对失效进入 `needsPairing`。
2. 在 `ProtocolClient` 增加只读的 `connectionState()` 方法，把内部 stage（`connecting`/`authenticating`/`subscribing`/`ready`）映射为导出连接态：`ready` → `connected`；`socket === null && !closedByUser` → `disconnected`；其余按当前 stage 原样返回 `connecting`/`authenticating`/`subscribing`。
3. 在 `App.tsx` 的活动 Session effect 中注册 `visibilitychange` 监听；页面恢复可见时读取 `connectionState()` 并 dispatch `connectionStateChanged`，不得创建新 Socket、调用 `reconnect()` 或修改 Pairing Store。
4. 保证 effect 清理时移除 `visibilitychange` 监听并关闭当前 Client，避免切换 Session 后旧监听器影响新 Session。
5. 增加测试覆盖：
   - 刷新时从 `window.location.hash` 恢复当前 Session；
   - 断线后不会新增 WebSocket 实例；
   - 显式点击“重新连接”只创建一个新 WebSocket；
   - `visibilitychange` 不会自动重连；
   - 切换/返回列表会关闭旧连接；
   - 任何实现仍不调用 `localStorage`、`sessionStorage` 或 IndexedDB。
6. 不改变 `PairingPayload`、`ProtocolClient` 的 hello/subscribe 顺序，不增加协议字段。

验证：

```bash
cd apps/mobile
npm test -- src/App.test.tsx src/protocol/client.test.ts src/session/store.test.ts
npm run lint
npm run build
```

预期：Web 端只在用户操作下重连；刷新、断线和 Session 切换不会产生双连接或凭证持久化。

## 任务 4：更新使用说明与 C2 验证记录

**文件：**

- 修改：`README.md`
- 新增：`docs/testing/C2-result.md`
- 修改：`HANDOFF.md`

步骤：

1. README 的快速开始/A6 章节补充终端二维码输出说明：二维码承载 `#pair=...`，当前无原生扫码时可使用二维码读取工具读取后粘贴到 Mobile Web URL Fragment；继续保留 A6 的 JSON 手动粘贴方式。
2. 写入 `docs/testing/C2-result.md`，记录实际执行的 Go/Web 测试、构建、lint、二维码内容互操作和生命周期测试结果；不得写入真实 Pair Token、二维码截图或 Claude 会话内容。
3. 明确记录暂缓项：原生扫码、安全存储、原生多 Session 持久化、真机、VPN、PNG 和 HTTP 二维码接口。
4. 更新 HANDOFF 的当前执行为 C2-A/C2-B 实施完成或未完成的实际状态，并列出真实验证结果与剩余 C2-C/D 缺口。

验证：

```bash
git diff --check
GOFLAGS=-buildvcs=false go vet ./...
GOFLAGS=-buildvcs=false go test -count=1 ./...
cd apps/mobile && npm test && npm run lint && npm run build
```

如果测试环境禁止本地 loopback 监听，必须在验证记录中区分沙箱限制与实际代码失败，并在允许监听的环境补跑 Transport/Remote 集成测试。

## 通过标准

- CLI 启动时输出原有配对 JSON 和终端二维码；二维码内容与 Mobile Web `#pair=` 解析完全兼容。
- pairing 文件现有恢复、过期、损坏降级、`sessionId` 绑定和权限行为保持通过。
- Mobile Web 刷新、断线、显式重连、配对失效、单连接和生命周期行为通过。
- 未新增 HTTP 端点、PNG 文件、浏览器持久化 Token、原生代码或协议字段。
- Go 格式、vet、测试、构建和 Mobile Web 测试/lint/build 的实际结果均记录；真机、VPN、原生扫码和安全存储明确标记为未验证/暂缓。

本计划不包含 Git 提交步骤；提交由用户另行明确授权。
