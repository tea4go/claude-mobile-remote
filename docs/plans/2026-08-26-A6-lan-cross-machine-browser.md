# A6 局域网跨机器桌面浏览器联调实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 Windows 机器 B 只运行 Mobile Web，即可通过显式 LAN WebSocket 地址控制机器 A 托管的单个 Claude Session，并完成里程碑 A 的跨机器门禁。

**Architecture:** 机器 A 继续运行现有 Standalone CLI、Session Controller、Pairing、Channel 和 Transport；只在启动时显式监听 LAN，并把同一地址写入配对负载。机器 B 在自己的 `localhost:5173` 运行 Vite，浏览器直接连接配对负载中的机器 A `wsUrl`。不增加发现服务、代理、服务器端多 Session、网络诊断或新的协议字段。

**Tech Stack:** Go 1.25、`claude-agent-sdk-go`、Gorilla WebSocket、React 19、TypeScript、Vite 8、Vitest、Windows 桌面浏览器。

---

## 文件与边界

- 配置与测试：`cmd/claude-mobile-remote/config.go`、`cmd/claude-mobile-remote/config_test.go`；只调整 Pair Token 默认有效期并固化既有 LAN 参数行为。
- 操作说明：`README.md`、`apps/mobile/README.md`；分别说明机器 A、机器 B 的最小启动步骤和有限的故障定位。
- 验证记录：实施并完成双机联调后创建 `docs/testing/A6-result.md`，并更新 `HANDOFF.md`。
- 标准拓扑中的 `<A-LAN-IP>` 是实施现场替换的机器 A 私有 IPv4 地址，不是待设计项；它必须同时用于 `--advertise-url` 和机器 B 实际连接。
- 机器 B 不运行 Go、不启动 Remote Server，也不使用 `vite --host`；页面 Origin 保持 `http://localhost:5173`，因此不需要新增 `--allowed-origin`。
- 机器 A 沿用开发者现有的本地 `go.mod`/SDK `replace` 环境；A6 不复制 `go.mod.base` 覆盖本地 SDK 配置，也不修改 SDK 仓库、协议、状态模型、配对负载结构或前端业务代码。
- 默认启动仍只监听 `127.0.0.1:8765`；只有用户显式传入 LAN 参数时才暴露到局域网。绑定 `0.0.0.0` 只用于可信网络和本次门禁。
- 手机、VPN、Capacitor、扫码、安全存储、多配对列表、B2 操作、B3 组件化和 Relay 均不属于 A6。

### Task 1: 以 TDD 调整 Pair Token 默认有效期并锁定网络配置

**Files:**
- Modify: `cmd/claude-mobile-remote/config_test.go`
- Modify: `cmd/claude-mobile-remote/config.go`

- [ ] **Step 1: 先修改默认配置断言**：将 `TestParseConfig` 的默认 Pair Token 期望改为 `12*time.Hour`，同时明确断言默认监听仍是 `127.0.0.1:8765`、默认公布地址仍是 `ws://127.0.0.1:8765/ws`、默认 Origin 恰好包含 `http://127.0.0.1:5173` 与 `http://localhost:5173`。保留现有显式 LAN 参数和 `--pair-token-ttl 15m` 覆盖值断言。
- [ ] **Step 2: 运行定向测试确认 RED**：执行 `GOCACHE=/private/tmp/claude-mobile-remote-gocache GOFLAGS=-buildvcs=false go test ./cmd/claude-mobile-remote -run '^TestParseConfig$' -count=1`，预期只因实现仍返回 `10m` 而失败；如果还有其他失败，先定位原因，不扩大实现范围。
- [ ] **Step 3: 实现最小改动**：只把 `parseConfig` 中 `pairTokenTTL` 默认值从 `10 * time.Minute` 改为 `12 * time.Hour`；不改变 `pairing.Manager` 的过期、自然轮换、撤销或已认证连接语义。
- [ ] **Step 4: 格式化并确认 GREEN**：执行 `gofmt -w cmd/claude-mobile-remote/config.go cmd/claude-mobile-remote/config_test.go`，再运行 `GOCACHE=/private/tmp/claude-mobile-remote-gocache GOFLAGS=-buildvcs=false go test ./cmd/claude-mobile-remote -run 'TestParseConfig' -count=1`，预期默认值、显式 LAN 配置、TTL 覆盖值和非法配置测试全部通过。
- [ ] **Step 5: 回归配对生命周期**：运行 `GOCACHE=/private/tmp/claude-mobile-remote-gocache GOFLAGS=-buildvcs=false go test -race -count=1 ./internal/pairing ./cmd/claude-mobile-remote`，预期通过且无 race；确认本轮没有把 12 小时硬编码进 `pairing.Manager` 或 wire payload。

### Task 2: 写清双机启动与有限故障定位

**Files:**
- Modify: `README.md`
- Modify: `apps/mobile/README.md`

- [ ] **Step 1: 增加机器 A 启动说明**：在根 README 写明先人工确认机器 A 的私有 IPv4 地址，再从仓库根目录启动：

  ```bash
  GOFLAGS=-buildvcs=false go run ./cmd/claude-mobile-remote \
    --listen 0.0.0.0:8765 \
    --advertise-url 'ws://<A-LAN-IP>:8765/ws'
  ```

  说明配对 JSON 中的 `wsUrl` 必须是同一 LAN 地址，不能是 `127.0.0.1` 或 `localhost`；Pair Token 不得放进命令行、截图、问题单或验证记录。
- [ ] **Step 2: 增加 Windows 机器 B 启动说明**：把 `apps/mobile/README.md` 的 Vite 模板文案替换为项目说明，并在根 README 给出相同的最小命令：

  ```powershell
  cd apps/mobile
  npm ci
  npm run dev
  ```

  B 只打开 Vite 输出的 `http://localhost:5173`，粘贴机器 A 当前输出的配对 JSON；不运行 Go，不把 Vite 暴露给 LAN。
- [ ] **Step 3: 限定网络暴露**：说明 A、B 应位于用户信任的同一局域网；只在需要时允许机器 A 的主机防火墙接收私有网络 TCP 8765，测试结束后停止 Service。不要自动修改防火墙，也不要加入网卡探测、端口扫描或诊断程序。
- [ ] **Step 4: 增加三类故障定位**：连接拒绝/超时先检查 A 的监听参数、LAN IP 和主机防火墙；页面提示配对失效时使用当前 Service 最新配对负载；Origin 被拒绝时确认页面确实从 B 的 `localhost:5173` 打开。只有用户主动通过 B 的 LAN IP 打开 Vite 时，才需要同时显式配置 Vite host 和精确 `--allowed-origin`，该路径不是 A6 标准门禁。
- [ ] **Step 5: 检查文档示例安全性**：所有示例只使用 `<A-LAN-IP>`、字段名和无凭证命令；不写真实 LAN 地址、`sessionId`、Pair Token 或用户会话内容。

### Task 3: 运行单机自动门禁

**Files:**
- Verify: `cmd/claude-mobile-remote/`, `internal/`, `apps/mobile/`

- [ ] **Step 1: 运行 Go 全量 race 测试**：执行 `GOCACHE=/private/tmp/claude-mobile-remote-gocache GOFLAGS=-buildvcs=false go test -race -count=1 ./...`，预期全部 Go 包通过且无 race。
- [ ] **Step 2: 运行 Go 静态检查与构建**：分别执行 `GOCACHE=/private/tmp/claude-mobile-remote-gocache GOFLAGS=-buildvcs=false go vet ./...` 和 `GOCACHE=/private/tmp/claude-mobile-remote-gocache GOFLAGS=-buildvcs=false go build ./...`，预期退出码为 0 且仓库内不产生构建产物。
- [ ] **Step 3: 运行 Mobile Web 门禁**：在 `apps/mobile` 依次执行 `npm ci`、`npm test`、`npm run lint`、`npm run build`，预期测试、lint、TypeScript 和 Vite 构建全部通过；A6 不以修改前端代码解决网络问题。
- [ ] **Step 4: 检查格式与凭证**：执行 `gofmt -l cmd/claude-mobile-remote internal`，预期无输出；执行 `rg -n '"[A-Za-z0-9_-]{43}"' internal/pairing internal/channel internal/transport cmd/claude-mobile-remote cmd/channel-test-client apps/mobile/src docs/testing README.md apps/mobile/README.md`，预期没有疑似真实 Pair Token 字符串字面量；如有输出必须逐项确认并移除真实凭证。
- [ ] **Step 5: 检查差异**：执行 `git diff --check`，预期无空白错误；核对差异只包含 Task 1/2 的配置、测试和说明，以及当前执行所需的 HANDOFF 记录。

### Task 4: 完成 A/B 双机人工门禁

**Files:**
- Verify: machine A Standalone CLI
- Verify: machine B `apps/mobile` in two browser tabs/windows

- [ ] **Step 1: 建立标准拓扑**：机器 B 先按 Task 2 启动本地 Vite；机器 A 使用显式 LAN 参数启动 Service。确认 A 输出的配对负载使用 `ws://<A-LAN-IP>:8765/ws`，`expiresAt` 约为签发后 12 小时；只观察结果，不复制凭证到文件。
- [ ] **Step 2: 验证鉴权隔离**：在 B 的临时浏览器窗口中把 Pair Token 任意一个字符替换后再粘贴，预期页面显示配对失效且看不到 snapshot、状态或消息；随后关闭该临时窗口。验证过程中不在日志或记录中写出原始/修改后的 token。
- [ ] **Step 3: 验证首轮与双向来源同步**：在 A 尚未提交任何 PC 输入时，用未修改的配对负载连接 B，预期进入 `starting/idle`；由 B 发送第一条测试消息并收到「手机端 · 已接收」，A 终端和 B 都应显示对应 Claude 回复；再由 A 终端发送一条测试消息，B 应显示「电脑端」正文，A 与 B 再次显示对应回复，两个入口都没有重复消息。
- [ ] **Step 4: 验证多入口 FIFO**：用同一有效配对负载再连接 B 的第二个浏览器窗口；从 A 终端、B 窗口 1、B 窗口 2 在短时间内各提交一条带不同测试标记的文本。实际先后由 Controller 接受顺序决定，不预设某个入口优先；两个 B 窗口必须观察到相同的全局 Turn 顺序，每条只执行一次，Claude 不并发写乱，任一窗口不会获得特殊所有权。
- [ ] **Step 5: 验证有效期内重连**：保持 Service 运行并刷新 B 的一个有效页面，预期 Fragment 中的未过期凭证自动重新完成 `hello → subscribe → snapshot`；页面取得最新 snapshot，但不恢复扫码前完整历史，也不重发断线前命令。另一个窗口应保持正常连接。
- [ ] **Step 6: 验证默认安全边界的组合证据**：人工门禁确认 LAN 访问只在显式 `--listen 0.0.0.0:8765` 时进行；Task 1 自动测试确认无参数默认值仍是 loopback。A6 不通过自动发现 IP、自动开放防火墙或改变 Vite host 来达成连接。
- [ ] **Step 7: 用户验收检查点**：实施 AI 汇总上述 5 个可见行为，让用户只反馈通过或具体问题。若网络、Claude 额度或浏览器环境阻止任何主门禁，记录为未验证并继续定位，不能把单机自动测试推断成跨机器通过。

### Task 5: 记录 A6 结果并交接

**Files:**
- Create: `docs/testing/A6-result.md`
- Modify: `HANDOFF.md`

- [ ] **Step 1: 创建门禁记录**：记录日期、A/B 职责、匿名化网络拓扑、实际自动命令与结果，以及首轮前配对、双向来源、FIFO、错误凭证、重连和默认 loopback 的逐项结果。命令中的实际 IP 统一写成 `<A-LAN-IP>`，不记录 token、真实 `sessionId` 或用户会话内容。
- [ ] **Step 2: 明确结论边界**：A6 结果只能声明“局域网跨机器桌面浏览器闭环”；不得据此声明手机、VPN、Capacitor、扫码、安全存储、移动生命周期或公网 Relay 已验证。自动通过但人工未完成时，结论必须是部分通过。
- [ ] **Step 3: 复核三端共用契约**：确认版本 1 envelope、状态枚举、`version/wsUrl/sessionId/pairToken/expiresAt` 配对负载和 snapshot/reconnect 行为均未改变；12 小时只改变默认配置且已由 D41 确认，因此不新增 `DECISIONS.md`。若实施中出现任何公共契约变更，停止 A6 并先完成三端核对与用户决策。
- [ ] **Step 4: 更新 HANDOFF**：写入实际完成项、实际门禁、未验证项和下一阶段候选；A6 完整通过后只把 B1 与 C1 标记为可开始，不在本任务提前实施。
- [ ] **Step 5: 最终文档与差异检查**：执行 `git diff --check`，并检查 `docs/testing/A6-result.md` 不含凭证、真实 Session 内容或未经验证的结论。
- [ ] **Step 6: 停止在 Git 写操作前**：只向用户展示差异与验证摘要；不执行 add、commit、push、分支或其他 Git 写操作，除非用户在届时明确授权具体操作。

## Self-review checklist

- Spec coverage: 覆盖 D40/D41、机器 A/B 职责、12 小时默认 TTL、显式 LAN、首轮前配对、双向来源、多个平等客户端 FIFO、错误凭证、有效期内重连、默认 loopback、验证记录和 C2 边界。
- Simplicity: 复用现有 CLI、Controller、Pairing、Channel、Transport 与 Mobile Web；必要业务代码改动仅一处默认值，不新增服务、协议、页面、自动发现或诊断能力。
- Placeholder scan: 计划没有 TBD、TODO 或未定义处理；`<A-LAN-IP>` 是每次联调时由操作者替换的明确运行参数。
- Type consistency: 保持 `sessionId`、`wsUrl`、Pair Token、`expiresAt`、request ID 和 `seq` 既有命名与语义；不引入浏览器特有的公共契约。
- Security: 默认 loopback 不变，LAN 暴露必须显式开启；凭证只在终端到浏览器的临时传递中出现，不写入源码、文档、日志、截图或 Git。

计划完成后先交用户检查。用户确认后，实施阶段再按用户选择使用 `superpowers:subagent-driven-development` 或 `superpowers:executing-plans`；本计划阶段不修改业务代码、不运行构建/测试、不执行 Git 写操作。
