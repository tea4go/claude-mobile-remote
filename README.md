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

前提：Go ≥ 1.25 与 Node ≥ 20。新 clone 或 pull 后，从仓库根目录按三步开始：

1. `install_1_base_tools` 安装/准备 Go、Node/npm 和项目依赖；
2. 需要原生 Android 时执行 `install_2_android_sdk`，再用 `check` 检查环境；
3. 选择 Web、Android、iOS 或 CLI 入口开始开发。

### 新 pull 的三步路径

macOS：

```bash
bash scripts/macos/install_1_base_tools.sh
bash scripts/macos/install_2_android_sdk.sh
bash scripts/macos/check.sh
bash scripts/macos/web-dev.sh
```

Windows PowerShell：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/windows/install_1_base_tools.ps1
powershell -ExecutionPolicy Bypass -File scripts/windows/install_2_android_sdk.ps1
powershell -ExecutionPolicy Bypass -File scripts/windows/check.ps1
powershell -ExecutionPolicy Bypass -File scripts/windows/web-dev.ps1
```

`install_2_android_sdk` 会在明确执行时安装 command-line tools、platform-tools、Android platform 36 和 build-tools 36；支持 `--sdk-root`/`-SdkRoot`。不需要 Android 时可跳过该步骤。基础安装只在 `go.mod` 不存在时从 `go.mod.base` 恢复，不覆盖已有本地 `go.mod`；随后执行 `go mod tidy` 和 `npm ci`。

### 平台命令表

| 用途 | macOS | Windows PowerShell |
| --- | --- | --- |
| 环境检查 | `bash scripts/macos/check.sh` | `powershell -ExecutionPolicy Bypass -File scripts/windows/check.ps1` |
| 安装基础工具/依赖 | `bash scripts/macos/install_1_base_tools.sh` | `powershell -ExecutionPolicy Bypass -File scripts/windows/install_1_base_tools.ps1` |
| 安装 Android SDK | `bash scripts/macos/install_2_android_sdk.sh` | `powershell -ExecutionPolicy Bypass -File scripts/windows/install_2_android_sdk.ps1` |
| 检查 Android | `bash scripts/macos/check.sh --android` | `powershell -ExecutionPolicy Bypass -File scripts/windows/check.ps1 -Android` |
| Web 开发 | `bash scripts/macos/web-dev.sh` | `powershell -ExecutionPolicy Bypass -File scripts/windows/web-dev.ps1` |
| Web 构建 | `bash scripts/macos/web-build.sh` | `powershell -ExecutionPolicy Bypass -File scripts/windows/web-build.ps1` |
| Android live | `bash scripts/macos/android-run.sh --live` | `powershell -ExecutionPolicy Bypass -File scripts/windows/android-run.ps1 -Live` |
| Android dist | `bash scripts/macos/android-run.sh --dist` | `powershell -ExecutionPolicy Bypass -File scripts/windows/android-run.ps1 -Dist` |
| Android Debug 构建 | `bash scripts/macos/android-build.sh` | `powershell -ExecutionPolicy Bypass -File scripts/windows/android-build.ps1` |
| CLI 启动 | `bash scripts/macos/cli-run.sh --profile local` | `powershell -ExecutionPolicy Bypass -File scripts/windows/cli-run.ps1 --profile local` |
| iOS Simulator 运行 | `bash scripts/macos/ios-run.sh` | 不提供：Windows 无 Xcode/iOS Simulator 环境 |
| iOS Simulator 构建 | `bash scripts/macos/ios-build.sh` | 不提供：请在 macOS 上执行 |

### 参数与运行模式

Web 开发默认监听 localhost；需要手机或其他机器访问时显式指定地址和端口：

```bash
bash scripts/macos/web-dev.sh --host 0.0.0.0 --port 5173
# Windows：web-dev.ps1 -Host 0.0.0.0 -Port 5173
```

Web 构建默认依次执行 test、lint、build；仅调试某一步时可使用 `--skip-test`/`--skip-lint`，Windows 对应为 `-SkipTest`/`-SkipLint`。

Android 有两种模式，且必须二选一：

- `--live`/`-Live` 使用 Web 开发服务器，脚本把模拟器访问宿主机的地址设为 `http://10.0.2.2:5173`。需同时保持 `web-dev` 运行；可用 `--port`/`-Port` 修改端口，并可用 `--avd`/`-Avd` 指定模拟器。
- `--dist`/`-Dist` 先构建 Web，再使用本地构建资源同步 Capacitor 并安装 Debug 包，不依赖正在运行的 Web server。

Android 脚本只负责检查/启动模拟器、同步和安装，不启动 Claude Service。iOS 脚本仅面向 macOS 的 Xcode Simulator，不处理真机签名；Windows 没有 iOS 入口。

CLI 脚本会把剩余参数原样透传给 `go run ./cmd/claude-mobile-remote`，例如：

```bash
bash scripts/macos/cli-run.sh \
  --profile device \
  --cwd /Users/zhangym/code/work/my-project \
  --permission-mode acceptEdits
```

脚本不会替用户选择 `--profile`、`--cwd`、`--resume` 或配对参数，也不会把配对凭证写入文件或命令日志。

### 安全边界

这些脚本只编排现有 Go、npm、Capacitor、Gradle、Android SDK 和 Xcode 命令：

- 不开放系统防火墙、不扫描端口、不自动暴露局域网服务；需要跨设备访问时由开发者显式选择 `--host` 和 CLI 网络 profile；
- 脚本不保存或写入 token、配对 JSON、会话内容、签名文件或 keystore，也不额外回显 CLI 参数中的 token；CLI 正常启动时仍会按产品流程在终端显示配对 JSON/二维码，请勿将其写入日志、截图或问题单；
- 不覆盖已有 `go.mod`，不执行 `git reset`、`git restore`、`git checkout`、`git add`、`git commit` 或 `git push`；
- 不启动或托管 Claude Service，Session 生命周期仍由 CLI/宿主负责。

`go.mod`/`go.sum` 是本机状态（可能含指向本地 SDK 的 `replace`），不入库；仓库提交的是基准 `go.mod.base`。依赖变更时只在本地修改 `go.mod`，确认后再由开发者决定是否同步更新 `go.mod.base`。

### 故障排查

- 缺少 Go 或 Node/npm：macOS 可按脚本提示使用 Homebrew；Windows 可按提示使用 `winget` 或 Go、Node.js 官方安装包。脚本不会静默修改 PATH。
- 缺少 Android SDK、`adb`、模拟器或平台组件：执行 `install_2_android_sdk`（可用 `--sdk-root`/`-SdkRoot` 指定目录），再执行带 Android 选项的 `check`。先用 `emulator -list-avds` 确认存在 AVD。
- Android live 白屏或连接失败：确认 Web dev server 仍在运行、端口与 `--port`/`-Port` 一致，并确认模拟器使用 `10.0.2.2`；真机不要直接套用该地址，应使用 CLI 的 `device` profile 和手机可达地址。
- Android dist 仍显示旧页面：重新执行 `android-run --dist`/`-Dist`，确认没有残留的 live server 配置；必要时卸载旧 Debug 包后重装。
- iOS 命令失败：仅在安装 Xcode 和命令行工具的 macOS 上运行，并先打开目标 Simulator；如果工程切换为 CocoaPods，再按脚本提示安装 CocoaPods。Windows 请改用 Android 或 Web 入口。
- 脚本检查通过但 CLI 启动失败：保留脚本输出的第一条错误，使用下面的手工命令复现，以便区分 Go、SDK、Claude 配置和网络问题。

脚本不可用时的最小手工诊断命令：

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

## 使用

### 开发版：日常启动

日常使用只需要关注会话、工作目录和 Claude 参数。省略 `--resume` 会新建会话；CLI 自动生成 UUID 并预分配给 Claude，Connect 成功后、首个 Turn 前在终端输出配对 JSON 和二维码。

```bash
go run ./cmd/claude-mobile-remote \
  --cwd /Users/zhangym/code/work/my-project \
  --permission-mode acceptEdits
```

恢复已有会话：

```bash
go run ./cmd/claude-mobile-remote \
  --resume <sessionId> \
  --cwd /Users/zhangym/code/work/my-project \
  --permission-mode acceptEdits
```

常用 Claude 参数包括 `--permission-mode`、`--model` 和 `--max-turns`。不传 `--profile` 时使用 `local`：Remote Server 只监听 `127.0.0.1:8765`，适合本机浏览器。

启动后终端会同时输出可粘贴的配对 JSON 和 `pairing QR:` 终端二维码。二维码承载现有 `#pair=<Base64URL(JSON)>` Fragment；当前没有原生扫码时，可用二维码读取工具读取内容，再把 Fragment 放到 Mobile Web 地址栏后打开。A6 的 JSON 手动粘贴方式继续可用。

`--cwd` 路径会在启动前解析为绝对路径，并且必须已经存在且是目录；不传时使用当前进程工作目录。`--resume <sessionId>` 只能恢复当前没有被其他 Claude 进程活跃占用的会话，不能接管正在运行中的外部 Claude 进程。

**恢复 ≠ 接管活跃会话。** `--resume` 只能接续一个当前没有被其他 Claude 进程活跃占用的会话；它不能接管一个正被另一个 Claude Code 进程运行中的会话。每个活跃 Session 始终只有一个 Host 和一个 SDK Client（见 DECISIONS D2 / D39），对活跃会话再开一个 `--resume` Client 属禁止行为，会引发会话冲突。

### 调试版：模拟器、真机和底层网络排障

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

### A6 局域网跨机器联调

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
