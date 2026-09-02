# 高级用法与排障

本文收纳根 `README.md` 之外的运行细节：脚本安全边界、CLI 调试 profile、A6 局域网跨机器联调，以及脚本不可用时的最小手工诊断命令。

## 脚本安全边界

这些脚本只编排现有 Go、npm、Capacitor、Gradle、Android SDK 和 Xcode 命令：

- 不开放系统防火墙、不扫描端口、不自动暴露局域网服务；需要跨设备访问时由开发者显式选择 `--host` 和 CLI 网络 profile；
- 脚本不保存或写入 token、配对 JSON、会话内容、签名文件或 keystore，也不额外回显 CLI 参数中的 token；CLI 正常启动时仍会按产品流程在终端显示配对 JSON/二维码，请勿将其写入日志、截图或问题单；
- 不覆盖已有 `go.mod`，不执行 `git reset`、`git restore`、`git checkout`、`git add`、`git commit` 或 `git push`；
- 不启动或托管 Claude Service，Session 生命周期仍由 CLI/宿主负责。

`go.mod`/`go.sum` 是本机状态（可能含指向本地 SDK 的 `replace`），不入库；仓库提交的是基准 `go.mod.base`。依赖变更时只在本地修改 `go.mod`，确认后再由开发者决定是否同步更新 `go.mod.base`。

## 脚本不可用时的最小手工诊断命令

```bash
cp go.mod.base go.mod  # 仅在 go.mod 不存在时执行
go mod tidy
go run ./cmd/claude-mobile-remote -version
cd apps/mobile
npm ci
npm run test
npm run lint
npm run build
npm run dev
```

构建与测试：`go test ./...`、`gofmt -l .`（无输出即通过）。注意 `go vet` / `go build` 需加 `GOFLAGS=-buildvcs=false`（go.mod 不入库的副作用，见 D18）。

## 调试版 profile：模拟器、真机和底层网络排障

远程环境优先使用 profile：

```bash
# Android 模拟器，固定把 10.0.2.2 解释为宿主机
go run ./cmd/claude-mobile-remote --profile emulator

# 真机，LAN 和 VPN 统一为 device；多网卡时可覆盖自动选择的地址
go run ./cmd/claude-mobile-remote \
  --profile device \
  --advertise-host 10.88.16.45
```

`device` 会选择一个非 loopback IPv4 作为二维码中的地址；如果电脑同时连接多个 LAN/VPN，使用 `--advertise-host` 指定手机实际可达的地址。LAN 与 VPN 在 Remote Server 层没有区别，差异只是所选 IP 和网络可达性。

只有需要精确控制网络参数时才使用 `debug` profile 或底层参数：

```bash
go run ./cmd/claude-mobile-remote \
  --profile debug \
  --listen 0.0.0.0:8765 \
  --advertise-url 'ws://10.0.2.2:8765/ws' \
  --allowed-origin http://localhost
```

`--listen` 是服务绑定地址，`--advertise-url` 是二维码写入且客户端实际连接的地址，`--allowed-origin` 是 WebSocket 页面来源白名单。它们属于调试参数，普通开发不需要填写。

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
