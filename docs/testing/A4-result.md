# A4 Channel 通信门禁记录

- 日期：2026-08-24
- 子阶段：A4 Channel 通信
- 结论：自动化门禁通过，等待用户统一 diff 与人工协议客户端验收
- Git：未执行暂存、提交或其他 Git 写操作

## 1. 实际完成范围

- Task 1：短期 Pair Token、纯 JSON 配对负载、过期轮换、失效、撤销和连接 lease；服务端仅保存凭证摘要。
- Task 2：版本 1 JSON envelope、严格字段与 payload 校验、稳定错误码和 decoder fuzz。
- Task 3：精确 `sessionId` Channel、Controller 原子订阅、递增 seq 校验、Registry 隔离及手机输入来源强制。
- Task 4：每连接数量/字节双重有界 outbox，以及保证 `command_result` 先于提交期间事件的顺序门控。
- Task 5：真实 WebSocket Transport，覆盖 Origin、5 秒鉴权窗口、订阅状态机、命令闭环、多客户端、重连、撤销与有界停机。
- Task 6：CLI 网络配置、Remote Runtime、Session 激活后配对输出、自然轮换和 `BeginShutdown → Controller.Stop → Remote.Close` 生命周期。
- Task 7：最小 Go CLI 协议客户端，覆盖严格配对解析、单 writer、认证/订阅/snapshot、命令结果/事件和重连不重发。

A4 未实现 React 移动 Web、二维码图片、真实 Claude、VPN 真机联调、审批或取消；这些仍按 A5、A6 和 B2 的阶段边界推进。

## 2. 最终门禁证据

所有 Go 命令使用 `/private/tmp/claude-mobile-remote-gocache`，避免沙箱无法访问默认 macOS Go cache；需要真实回环 WebSocket 的测试使用本机临时 listener。

| 门禁 | 真实结果 |
| --- | --- |
| `gofmt -w internal/pairing internal/channel internal/transport cmd/claude-mobile-remote cmd/channel-test-client` | PASS，无格式化错误 |
| `go test -race -count=1 ./internal/pairing ./internal/channel ./internal/transport` | PASS：3/3 packages，无 race |
| `go test -race -count=1 ./cmd/claude-mobile-remote ./cmd/channel-test-client` | PASS：2/2 packages，无 race |
| `go test -count=1 ./...` | PASS：8/8 packages |
| `go test -race -count=1 ./...` | PASS：8/8 packages，无 race |
| `GOFLAGS=-buildvcs=false go vet ./...` | PASS，退出码 0 |
| `GOFLAGS=-buildvcs=false go build ./...` | PASS，退出码 0，仓库内无构建产物 |
| `GOFLAGS=-buildvcs=false go run ./cmd/claude-mobile-remote -version` | PASS，精确输出 `0.1.0-dev`；单元测试确认不构造 Runtime |
| `GOFLAGS=-buildvcs=false go build -o /private/tmp/claude-mobile-remote-channel-test-client ./cmd/channel-test-client` | PASS，产物仅在 `/private/tmp` |
| `go test ./internal/transport -run '^$' -fuzz '^FuzzDecodeEnvelope$' -fuzztime=2s` | PASS：109398 次执行，无 panic |

额外并发回归：

- 客户端“最后 event 后立即正常关闭”在修复后以 race 模式执行 100 次，全部通过；
- Transport 正常最终事件、取消 context 强制关闭全部连接和凭证撤销三类路径以 race 模式执行 20 次，全部通过；
- A4 实现期间各 Task 均按 RED/GREEN 完成 focused 测试，最终证据以上述 fresh 全量门禁为准。

## 3. 协议闭环与安全结果

自动化真实 WebSocket fixture 已覆盖：

1. `hello → hello_ack → subscribe → subscribed → snapshot`；
2. 文本 command 强制映射为 `InputMobile`，成功结果携带 `commandId` 和 `requestId`；
3. 提交期间产生的同 request ID `turn_started` 在 `command_result` 之后可见；
4. 多连接订阅同一 Session、取消后重订阅、断线后只取得最新 snapshot；
5. 无效凭证、鉴权超时、非法状态、Origin 拒绝、消息超限、重复 command ID、关闭期间命令拒绝；
6. Pair Token 撤销关闭已认证连接，正常停机尝试写出最终 Session 事件，超时/取消后强制关闭全部连接并撤销 lease。

凭证扫描命令：

```text
rg -n 'pairToken|PairToken|fixture-credential' --glob '!docs/specs/2026-08-24-A4-channel-transport-design.md' --glob '!docs/plans/2026-08-24-A4-channel-transport.md' .
rg -n '"[A-Za-z0-9_-]{43}"' internal/pairing internal/channel internal/transport cmd/claude-mobile-remote cmd/channel-test-client
```

字段扫描只包含协议字段、安全实现、设计/决策说明和明确的虚构 fixture；43 字符 Base64URL token 字符串字面量扫描无输出。未发现真实 token、用户会话、Claude 配置或凭证快照。配对凭证仅由交互终端输出供用户传给测试客户端，服务端响应、普通错误和日志不回显凭证。

## 4. 三端共用契约复核

| 契约 | 实现核对 | 结论 |
| --- | --- | --- |
| 通信协议 | envelope 精确包含 D27 的 `version/type/sessionId/commandId/requestId/seq/payload`；payload 只使用 JSON object、字符串、布尔、整数和可空对象 | React/TypeScript、Capacitor、HarmonyOS ArkTS 均可实现 |
| 状态模型 | snapshot/event 直接封装 A3 纯数据模型；Session/Turn 状态与 seq 不由 Transport 重解释 | 无 Go、DOM、线程或 UI 生命周期语义 |
| 配对负载 | 与 D28 一致：`version/wsUrl/sessionId/pairToken/expiresAt`；鉴权使用首条 hello，WebSocket URL 不携带 token | URL Fragment 仍仅是 A5 Web 承载建议，不污染原生端协议 |
| 验收行为 | Go 客户端与 WebSocket 集成测试固定认证、订阅、snapshot、result-before-event、重连最新 snapshot | 三端可复用同一行为断言 |

代码中没有客户端乱序缓存、localStorage、DOM、浏览器线程或平台回调依赖。本轮修复只调整客户端内部读取顺序与 PC Service 超时停机收敛，不修改公共字段、状态或三端行为，因此无需新增或修改 `DECISIONS.md`。

## 5. 最终审查修复与已知限制

本地最终审查发现并按 TDD 关闭两个 Important 问题：

- reader 的消息与关闭错误原先走两个 channel，可能在最后一个已读 event 输出前先观察正常关闭；现改为单一有序读取结果流；
- Remote Runtime 等待轮换退出时若 context 已取消，原先可能跳过 Transport Close 与 revoke；现即使等待超时也执行强制清理，Transport 同时保证关闭全部连接。

未关闭 Critical 或 Important。已知非阻塞限制：真实慢 socket 的内核写阻塞受平台缓冲大小影响，A4 没有以不稳定的网络压测模拟它；当前用确定性的 outbox/提前缓存双重溢出测试验证关闭路径，并用真实 WebSocket 多客户端测试验证连接独立性。A5/A6 联调仍需观察实际浏览器和 VPN 慢网行为。

## 6. 人工验收与后续

用户统一检查差异后，可启动真实 Service 并用 `channel-test-client --pair '<payload>'` 验证：authenticated、subscribed、snapshot、command accepted、相同 request ID 的 Turn event，以及重连后的最新 snapshot。未授权凭证不得取得 Session 数据。

用户反馈 A4 人工验收通过前不进入 A5；真实 Claude、移动 Web 和 VPN 结果不得由本记录推断。
