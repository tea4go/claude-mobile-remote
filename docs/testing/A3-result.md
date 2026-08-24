# A3 Session Core 门禁记录

- 日期：2026-08-24
- 子阶段：A3 Session Core
- 结论：通过

## 自动验证

| 命令 | 结果 |
| --- | --- |
| `go test -race ./internal/session ./cmd/claude-mobile-remote` | PASS |
| `go test -race ./...` | PASS |
| `gofmt -l .` | PASS（无输出） |
| `GOFLAGS=-buildvcs=false go vet ./...` | PASS（无输出） |
| `GOFLAGS=-buildvcs=false go build ./...` | PASS（无输出） |
| Controller 重点并发用例 `-count=50` | PASS |

全仓 race 门禁实际追加 `-count=1` 强制禁用测试缓存；全部四个 Go package fresh PASS。最终 Important 修复后另对 CLI EOF/Session failure focused race 执行 `-count=20`、对阻塞 Reader 取消执行 `-count=100`，结果均 PASS。

## 设计核对

- Host 最大并发调用数为 1；
- 等待容量为 32，运行中 Turn 不计入；
- Turn scope 失败后继续，Session scope 失败后清队列；
- snapshot、原子订阅和 seq 无缺口；
- Stop 取消当前 Turn，Host 只关闭一次；
- CLI EOF 排空，主动停止立即收敛；
- 未实现 A4 协议、鉴权、配对或 B2 操作。

## Task 7 前回归收口

- 两请求均已接受且 EOF 后，首 Turn 导致 Session scope failure 时，CLI 不执行第二个 Turn，只报告一次 Session 根因；
- Session failure 前已有流式文本时，CLI 正确结束当前输出行；
- 可恢复 Turn failure 仍报告 Turn 错误并正常结束 EOF；
- 1 MiB LF、CRLF、无换行均接受，超限 1 byte 拒绝；
- `WaitIdle` 等待 Worker result 真正结算，不修改 `seq`，调用方 context 超时不会阻塞 Actor。

## 最终审查修复记录

- 最终整体审查发现：若 Reader 尚未被 CLI 观察为 EOF，`turn_failed → session_failed` 仍可能输出两次同一根因。新增 Reader 保持打开、无 sleep 的确定性交错测试，旧实现稳定 RED；修复后 Turn 错误统一延迟到 Worker settlement 或下一条 `turn_started`，Session scope failure 只报告 Session 根因，可恢复 Turn failure 仍报告 Turn 错误；
- 同步补充直接 Session failure 的部分文本换行回归；
- 修复后的首次全仓 race 暴露取消测试可能在 scanner 进入 `Read` 前取消。确认无生产 goroutine 泄漏后，让测试显式等待阻塞 Read 已开始；该用例 race `-count=100` 与随后全部门禁 fresh PASS。

## 三端共用契约复核

- D24 字段仍只包含 JSON 可表达的 `sessionId`、request ID、状态、来源、`seq`、`queueDepth`、message、可空 error 与可空 `pendingAction`；React/TypeScript、Capacitor iOS/Android 和 HarmonyOS ArkTS/ArkUI 均无平台特有依赖；
- D25 未引入可执行回调或客户端乱序兼容；`command_result → turn_started` 的网络可观察顺序仍由 A4 PC Service 边界负责；
- D26 的 `WaitIdle` 只存在于 PC 进程内 Controller/CLI 调用面，不发布事件、不增加协议字段或客户端行为。

## 未执行项

- 未重复运行真实 Claude integration：A3 只在已通过 A2 真实双轮门禁的薄 SDK Adapter 之上增加 Session Core，本阶段计划门禁使用 fake/controlled Host 和非 integration 自动化测试；
- A3 无新增人工体验门禁，移动浏览器体验与 VPN 联调分别留在 A5、A6。
