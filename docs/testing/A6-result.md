# A6 局域网跨机器桌面浏览器联调门禁记录

- 日期：2026-08-26
- 子阶段：A6 局域网跨机器桌面浏览器联调
- 依据：`docs/plans/2026-08-26-A6-lan-cross-machine-browser.md`、D40、D41
- 结论：**自动门禁通过；用户确认跨机器人工门禁通过；A6 完整通过**

## 1. 验证拓扑

| 角色 | 验证内容 |
| --- | --- |
| 机器 A | 运行 Standalone CLI、Claude Session 和 Remote Server，显式监听 LAN 地址 |
| 机器 B | Windows 机器，只运行 `apps/mobile` 的 Vite Mobile Web，并从 `http://localhost:5173` 打开浏览器 |
| 网络 | A 与 B 位于同一信任局域网；B 浏览器通过配对负载中的 A 的显式 LAN `wsUrl` 连接 |

真实 LAN 地址、`sessionId`、Pair Token、配对 JSON 和用户会话内容不写入本记录。

## 2. 自动门禁证据

| 门禁 | 结果 |
| --- | :---: |
| `GOCACHE=/private/tmp/claude-mobile-remote-gocache GOFLAGS=-buildvcs=false go test -race -count=1 ./...` | PASS：8/8 Go 包，无 race |
| `GOCACHE=/private/tmp/claude-mobile-remote-gocache GOFLAGS=-buildvcs=false go vet ./...` | PASS |
| `GOCACHE=/private/tmp/claude-mobile-remote-gocache GOFLAGS=-buildvcs=false go build ./...` | PASS |
| `cd apps/mobile && npm ci` | PASS |
| `cd apps/mobile && npm test` | PASS：167/167 |
| `cd apps/mobile && npm run lint` | PASS |
| `cd apps/mobile && npm run build` | PASS |
| `gofmt -l cmd/claude-mobile-remote internal` | PASS：无输出 |
| lockfile、凭证字面量和 `git diff --check` 检查 | PASS |

自动门禁确认默认配置仍保持 loopback，LAN 暴露需要显式 `--listen 0.0.0.0:8765`；Pair Token 默认有效期为 12 小时且仍可通过参数覆盖。`npm ci` 使用已修复的可选平台包元数据完成。

## 3. 用户人工验收

用户于 2026-08-26 反馈 A6 测试通过，覆盖本阶段清单：

| 验收项 | 结果 |
| --- | :---: |
| 错误 Pair Token 无法获得会话数据 | PASS |
| 首轮前配对 | PASS |
| 手机端发起首条消息并收到回复 | PASS |
| PC 输入同步到机器 B，来源显示正确 | PASS |
| 机器 B 输入与 Claude 回复同步且不重复 | PASS |
| 两个浏览器入口观察到一致的 FIFO 顺序 | PASS |
| 页面刷新后在有效期内重新连接并获取当前 snapshot | PASS |
| 默认 loopback 与显式 LAN 监听边界 | PASS |

以上结论依据用户人工验收反馈记录；未记录任何真实凭证、地址、Session ID 或会话文本。

## 4. 共用契约与结论边界

- 版本 1 JSON envelope、`sessionId`、request ID、`seq`、状态模型、配对负载字段和 snapshot/reconnect 行为未改变。
- 12 小时只改变 Pair Token 默认配置，已由 D41 确认；本阶段不新增 `DECISIONS.md` 决策。
- A6 结论仅为“局域网跨机器桌面浏览器闭环通过”。
- A6 不宣称手机真机、VPN、Capacitor、扫码、安全存储、移动生命周期、HarmonyOS 或公网 Relay 已验证；这些能力仍按路线在 C1/C2/D 阶段处理。

## 5. 后续

- B1：消息渲染与已配对 Session 列表设计/实施。
- C1：Capacitor iOS/Android 工程、Web 接入和局域网明文放行；C1 可与 B1 并行准备。原生扫码和安全存储移入 C2。
- B2 仍依赖 B1，C2 仍依赖 C1 与 B2。
- A5 正式门禁记录和 A5.2 真实 Claude 额度受限项仍是历史遗留记录，不阻断本次 A6 的跨机器浏览器结论。
