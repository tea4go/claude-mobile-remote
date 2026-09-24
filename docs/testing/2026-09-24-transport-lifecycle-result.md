# Transport 生命周期回归验证

- 日期：2026-09-24
- 范围：慢客户端隔离、订阅源关闭时的最终事件、Server 关闭刷出最终事件
- 结论：自动化门禁通过；本次未执行真机网络联调
- 人工反馈：用户确认测试通过，未提供额外命令或设备细节

## 修复与验证

- 慢客户端用例逐条等待快连接消费，避免测试流量同时压满两个独立 outbox；慢连接仍不读取，测试确认其订阅退出后快连接可收到后续事件。
- Transport 收到订阅源正常关闭通知时先排空已缓冲事件。Server 停机期间由 `Close` 等待 writer 刷出；单独关闭订阅源时，连接在有界时间内刷出后关闭。
- 新增订阅源单独关闭的真实 WebSocket 回归用例，验证终态事件先于连接关闭到达。

| 命令 | 结果 |
| --- | --- |
| 三个生命周期用例 `go test -race ./internal/transport -run 'TestWebSocket(SlowClientIsolation\|CloseFlushesFinalSessionEvent\|SubscriptionCloseFlushesBufferedEvent)$' -count=10` | 通过，10 轮 |
| `go test -race ./internal/transport` | 通过 |
| `go test -race ./...` | 通过，所有 Go 包 |
| `GOFLAGS=-buildvcs=false go build ./...` | 通过 |
| `git diff --check` | 通过 |

测试使用本机回环 WebSocket 和受控 Controller；未覆盖真实慢网、移动真机或 Harmony DevEco 构建。没有更改协议字段、状态模型、配对负载或三端验收语义。
