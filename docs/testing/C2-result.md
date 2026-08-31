# C2-A/C2-B 配对二维码与 Web 生命周期 · 验证记录

- 日期：2026-08-31
- 关联设计：`docs/specs/2026-08-31-C2-pairing-lifecycle-design.md`
- 关联计划：`docs/plans/2026-08-31-C2-pairing-web-lifecycle-plan.md`
- 关联决策：D54

## 1. 本轮范围

本轮只实施 C2-A 后端终端二维码与现有 pairing 持久化回归、C2-B Mobile Web 生命周期。二维码使用 `#pair=<Base64URL(JSON)>` Fragment，不新增 WebSocket 字段或 HTTP API；CLI 保留原有配对 JSON 输出并追加终端二维码。

未实施：原生扫码、安全存储、原生多 Session 跨重启持久化、PNG、HTTP 二维码接口、真机、VPN、WhaleTerm W1 和外部 Claude 进程接管。

## 2. 自动验证

### Go

```text
gofmt -l protocol internal/pairing cmd/claude-mobile-remote
```

通过：无输出。

```text
GOCACHE=/private/tmp/claude-mobile-remote-gocache GOFLAGS=-buildvcs=false go test ./protocol ./internal/pairing ./cmd/claude-mobile-remote -count=1
```

通过：`protocol`、`internal/pairing`、`cmd/claude-mobile-remote` 均通过。

```text
GOFLAGS=-buildvcs=false go build ./cmd/claude-mobile-remote
```

通过：CLI 构建成功。

新增测试覆盖：Go Pairing Fragment 编码与解码字段一致、Base64URL 字符集、终端二维码非空多行输出、二维码不额外显示人类可读 Token、CLI 保留 JSON 并输出 `pairing QR:`；终端二维码使用半块字符并关闭额外外边框以缩小显示尺寸。

```text
GOCACHE=/private/tmp/claude-mobile-remote-gocache GOFLAGS=-buildvcs=false go vet ./...
```

通过：无输出。

```text
GOCACHE=/private/tmp/claude-mobile-remote-gocache GOFLAGS=-buildvcs=false go test -count=1 ./...
```

部分通过：`cmd/claude-mobile-remote`、`internal/channel`、`internal/claude`、`internal/pairing`、`internal/session`、`protocol` 和其它非监听包通过；`cmd/channel-test-client`、`internal/transport`、`remote` 的既有本地监听/httptest 用例在当前沙箱分别因禁止绑定 `[::1]:0` 或 `127.0.0.1:0` 失败。该限制不属于二维码或 Web 生命周期实现回归，需在允许 loopback 监听的环境重跑。

### Mobile Web

```text
npm test -- src/protocol/pairing.test.ts
```

通过：22 个测试通过；新增 Go fixture 可由 Mobile Web `parsePairingFragment` 解析。

```text
npm test -- src/App.test.tsx src/protocol/client.test.ts src/protocol/pairing.test.ts src/session/store.test.ts
```

通过：4 个测试文件、98 个测试通过；覆盖页面 Fragment 恢复、断线不自动重连、显式重连、单连接、`visibilitychange` 状态同步和协议客户端连接阶段。

```text
npm run lint
```

通过：0 error；存在 1 个既有 warning，位置为 `scripts/cdp-console.mjs:76` 的 `no-unused-expressions`。

```text
npm run build
```

通过：TypeScript 编译和 Vite production build 成功。

```text
npm test
```

通过：16 个测试文件、403 个测试通过。

## 3. 行为结论

- CLI 继续输出原有 pairing JSON，并追加终端二维码；Pairing JSON 字段和 WebSocket 协议不变。
- Go 端 Fragment 与 Mobile Web 端 Fragment 解码互操作，`version`、`wsUrl`、`sessionId`、`pairToken`、`expiresAt` 保持一致。
- 现有 pairing 文件恢复、过期替换、损坏降级、`sessionId` 绑定和文件权限逻辑未重写，聚焦 Go 回归通过。
- Web 刷新从 URL Fragment 恢复当前 Session；断线不后台无限重试；只有用户点击“重新连接”才建立新连接。
- 页面恢复可见时只同步当前 Client 连接状态，不自动重连，不修改内存配对列表。

### 用户人工验收

用户已确认以下 4 项通过：

1. 扫码得到的 Pairing Fragment 可用于 Mobile Web 配对完整链路；
2. 页面刷新、断线、手动重连和可见性变化行为符合预期；
3. 服务重启后配对凭证仍可恢复使用；
4. 二维码可被微信识别，且缩小后的终端显示尺寸符合预期。

## 4. 未验证与暂缓

- 终端二维码已通过微信人工识别，并验证缩小后的显示尺寸；本记录不保存二维码截图或真实 Pair Token。
- 原生扫码、Keychain/Android Keystore、安全存储、原生多 Session 列表、移动前后台恢复、真机网络权限和 VPN 未实施/未验证。
- 本轮未运行真实 Claude 端到端，不影响二维码与 Web 生命周期自动测试结论。
