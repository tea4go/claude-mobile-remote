# claude-mobile-remote

通过独立 Standalone CLI 托管一个本地 Claude Session，让用户离开电脑后可在手机上继续查看和控制。移动端后续可保存多个已扫描 Session；WhaleTerm 可复用同一单 Session Remote Server，但每个活跃 Session 始终只有一个 Host 和一个 SDK Client。

## 如何开始

无论首次加入还是中途接手，都从项目根目录按以下顺序开始：

1. 启动开发工具，并按“文档地图”完整阅读项目文档；
2. 以 `HANDOFF.md` 作为当前进度的唯一来源，重点核对“当前执行”、未验证项和下一步；
3. 若上一任务已通过子阶段门禁或完成重要联调，再查看 `docs/testing/` 中的验证记录；
4. AI 先向工作人员复述当前状态、待处理问题和下一步，确认后再修改文件；
5. 按当前子阶段的实施计划执行；尚无实施计划时，先编写计划并确认，不直接开始编码；
6. 阶段设计文档由 AI 起草后交工作人员检查；检查完成后，只有工作人员明确要求时 AI 才执行 Git 提交，否则由工作人员自行提交；
7. 任务结束前，由 AI 按 `AGENTS.md` 更新交接、决策和必要的验证记录。

如果文档之间不一致、已有结果无法复核或任务边界不清楚，应先停止并报告，不自行推断后继续。工作人员负责范围确认、决策、体验验收和 Git 写操作授权；AI 负责开发、验证和文档维护，可主动使用只读 `git diff` 检查改动，但暂存、提交、推送、恢复、分支、合并等操作仍需工作人员明确授权。

## 文档地图

新会话按以下顺序阅读：

1. `AGENTS.md`：长期背景、技术约束和协作规则；
2. `HANDOFF.md`：当前进度、下一步和已知问题；
3. `DECISIONS.md`：已确认的产品与技术决策；
4. `REQUIREMENTS.md`：完整产品需求与阶段范围；
5. `docs/specs/2026-08-21-claude-mobile-remote-design.md`：总体技术设计；
6. `docs/specs/2026-08-25-simplified-roadmap-and-session-remote-design.md`：A5.2 后的单 Session、移动端多配对、阶段依赖与可信 Relay 路线。

`CLAUDE.md` 用于引导 Claude Code 读取同一套项目记忆，避免维护重复规则。

## 快速开始

前提：Go ≥ 1.25 与 Node ≥ 20。

`go.mod`/`go.sum` 是本机状态（可能含指向本地 SDK 的 `replace`），不入库；仓库提交的是基准 `go.mod.base`。克隆后先恢复：

```bash
cp go.mod.base go.mod && go mod tidy
```

```bash
# Go CLI
go run ./cmd/claude-mobile-remote -version

# 移动 Web
cd apps/mobile && npm install && npm run dev
```

构建与测试：`go test ./...`、`gofmt -l .`（无输出即通过）、`cd apps/mobile && npm run test && npm run build`。注意 `go vet` / `go build` 需加 `GOFLAGS=-buildvcs=false`（go.mod 不入库的副作用，见 D18）。依赖变更时：本地改 `go.mod`，确认后同步更新 `go.mod.base` 提交。

## 使用

### 新建会话（默认）

不传 `--resume` 直接启动即新建一个 Claude 会话：CLI 自动生成 UUID 并预分配给 Claude，Connect 成功后、首个 Turn 前在终端输出配对 JSON（二维码数据）。默认只监听 `127.0.0.1:8765`，跨机器时按下方「A6 局域网跨机器联调」补充 `--listen` / `--advertise-url`。

```bash
go run ./cmd/claude-mobile-remote
```

如需让 Claude 在指定项目目录中运行，可使用 `--cwd`。路径会在启动前解析为绝对路径，并且必须已经存在且是目录；不传时保持使用当前进程工作目录。

```bash
go run ./cmd/claude-mobile-remote --cwd /Users/zhangym/code/work/my-project
```

### 恢复已有会话

用 `--resume <sessionId>` 接续电脑上已经存在的 Claude 会话。`sessionId` 必须是合法 UUID（8-4-4-4-12），Claude 端需存在该会话历史，否则 init 时按 Session 失败处理。恢复后手机配对、订阅与消息收发和新建会话完全一致。

```bash
go run ./cmd/claude-mobile-remote --resume <sessionId>
```

恢复时也可以显式指定原项目目录：

```bash
go run ./cmd/claude-mobile-remote \
  --resume <sessionId> \
  --cwd /Users/zhangym/code/work/my-project
```

**恢复 ≠ 接管活跃会话。** `--resume` 只能接续一个当前没有被其他 Claude 进程活跃占用的会话；它不能接管一个正被另一个 Claude Code 进程运行中的会话。每个活跃 Session 始终只有一个 Host 和一个 SDK Client（见 DECISIONS D2 / D39），对活跃会话再开一个 `--resume` Client 属禁止行为，会引发会话冲突。

## A6 局域网跨机器联调

A6 只验证桌面浏览器跨机器闭环：机器 A 运行 Claude 和 Remote Server，Windows 机器 B 只运行 Mobile Web。两台机器应位于用户信任的同一局域网。

机器 A 先人工确认自己的私有 IPv4 地址，并确保现有 `go.mod` 已配置可用的 SDK；如果本机依赖 SDK `replace`，不要用 `go.mod.base` 覆盖它。把下列 `<A-LAN-IP>` 替换为实际地址后，从仓库根目录启动：

```bash
GOFLAGS=-buildvcs=false go run ./cmd/claude-mobile-remote \
  --listen 0.0.0.0:8765 \
  --advertise-url 'ws://<A-LAN-IP>:8765/ws'
```

输出的配对 JSON 中，`wsUrl` 必须使用同一 LAN 地址，不能是 `127.0.0.1` 或 `localhost`。配对 JSON 含控制凭证，只能临时传给机器 B 的配对输入框，不要放进命令行、日志、截图、问题单或验证记录。

Windows 机器 B 从 Git 仓库取得项目后，只执行：

```powershell
cd apps/mobile
npm ci
npm run dev
```

在 B 本机打开 Vite 输出的 `http://localhost:5173`，粘贴机器 A 当前输出的配对 JSON。标准流程不运行 Go、不使用 `vite --host`，也不需要增加 `--allowed-origin`。

如果无法连接：

- 连接拒绝或超时：检查机器 A 的 `--listen`、LAN IP，以及机器 A 的主机防火墙是否允许私有网络 TCP 8765；
- 页面提示配对失效：使用当前 Service 最新输出的配对负载，旧 Service 或已修改的凭证不能复用；
- Origin 被拒绝：确认页面确实从 B 的 `http://localhost:5173` 打开。通过 B 的 LAN IP 暴露 Vite 不属于 A6 标准流程；若主动这样做，必须同时显式配置 Vite host 和精确的 `--allowed-origin`。

`--listen 0.0.0.0:8765` 会把端口暴露到机器 A 的全部网络接口。联调结束后在 CLI 输入 `:quit` 停止 Service；项目不会自动探测网卡、开放防火墙或扫描端口。

## 仓库

https://github.com/tea4go/claude-mobile-remote
