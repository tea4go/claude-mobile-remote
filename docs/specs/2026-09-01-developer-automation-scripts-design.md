# 开发者自动化脚本设计

## 目标

为新 clone/pull 的开发者提供按平台可发现、可重复执行的安装、环境检查、开发、调试和构建入口，减少手工切换目录、拼接环境变量和记忆 Capacitor/Android 命令的成本。

## 现状与约束

- Go CLI 位于仓库根目录，依赖本地 `go.mod`；仓库只提交 `go.mod.base`。
- Mobile Web 位于 `apps/mobile`，底层任务由其 `package.json` 提供。
- Capacitor 原生工程位于 `apps/mobile/android` 与 `apps/mobile/ios`。
- Android 模拟器 live reload 需要 `10.0.2.2:5173` 和对应的 Service Origin；dist 模式的壳内 Origin 是 `http://localhost`。
- 参考 Macro-Deck 的平台脚本、共享日志函数、依赖检查和可重复执行风格。
- 脚本不自动开放系统防火墙，不接触配对凭证，不写入 Git，不引入新的运行时任务框架。

## 方案

采用平台原生脚本：

- `scripts/macos/*.sh` 使用 Bash；
- `scripts/windows/*.ps1` 使用 PowerShell；
- 两端保持同名或语义对应的入口和参数；
- 共享函数负责定位仓库根目录、彩色日志、命令检查、版本检查和 npm 依赖准备；
- 脚本只编排现有 npm、Go、Capacitor、Gradle、Xcode 和 Android SDK 命令。

不采用 Node.js 单一启动器或 Make/Taskfile：前者无法自然覆盖 Xcode/PowerShell/Android SDK 差异，后者仍要求 Windows 用户先准备额外任务工具。

## 命令目录

### macOS

| 入口 | 行为 |
| --- | --- |
| `scripts/macos/check.sh` | 检查 Go、Node/npm、Java、Android SDK、adb、Xcode/CocoaPods 条件；不修改系统 |
| `scripts/macos/setup.sh` | 检查并按需通过 Homebrew 安装基础工具，恢复 `go.mod`，执行 `go mod tidy` 与 `npm ci`；Android SDK 安装由显式选项触发 |
| `scripts/macos/web-dev.sh` | 启动 Web Vite 开发服务器，可选 `--host`、`--port` |
| `scripts/macos/web-build.sh` | 执行 Web test、lint、build，可选跳过测试或 lint 以便快速调试 |
| `scripts/macos/android-run.sh` | 检查/启动 AVD，选择 live reload 或 dist，执行 Capacitor sync 和 debug 安装 |
| `scripts/macos/android-build.sh` | 执行 Android debug 构建；release 只构建未签名产物，不读取或生成凭证 |
| `scripts/macos/ios-run.sh` | Web build/sync 后运行 iOS 工程到指定 Simulator |
| `scripts/macos/ios-build.sh` | 执行 Web build、Capacitor sync 和 Xcode archive 前置检查 |
| `scripts/macos/cli-run.sh` | 恢复 `go.mod` 后启动 CLI，向 CLI 透传参数 |

### Windows

提供 `scripts/windows/check.ps1`、`setup.ps1`、`web-dev.ps1`、`web-build.ps1`、`android-run.ps1`、`android-build.ps1` 和 `cli-run.ps1`。iOS 入口不提供，并在 README 中明确说明仅 macOS 支持。

## 初始化策略

`setup` 分为安全的默认步骤和显式的重型步骤：

1. 检查 Go、Node/npm；缺失时给出安装命令，macOS 可用 Homebrew 自动安装，Windows 优先使用已存在的 `winget`，否则输出官方安装提示。
2. 复制 `go.mod.base` 到 `go.mod` 仅当 `go.mod` 不存在；已有本地 `go.mod` 不覆盖。
3. 执行 `go mod tidy` 和 `npm ci`，保证新 pull 项目有可编译的 Go 依赖与前端依赖。
4. Android SDK 不在默认 setup 中静默下载；使用 `setup --android` 或 Windows 对应开关时才安装/检查 command-line tools、platform-tools、项目所需 platform/build-tools，并提示接受 license。
5. iOS 的 Xcode/CocoaPods 只做检查，不自动修改系统 Ruby 或签名环境。

## 运行与调试策略

- `web-dev` 默认监听 localhost；传 `--host` 才允许局域网/模拟器访问。
- `android-run --live` 自动使用 `CAP_SERVER_URL=http://10.0.2.2:5173`、Android sync 和 debug 安装；脚本不启动 Service，避免凭证和 Claude 生命周期被脚本接管。
- `android-run --dist` 先执行生产 Web build，再无 `CAP_SERVER_URL` 地 sync，避免旧 live reload 配置残留。
- `cli-run` 只负责 Go CLI，不替用户决定 `--profile`、`--cwd`、`--resume` 或配对参数；所有剩余参数原样透传。
- 需要 Service + Mobile Web 的联调仍由 README 提供安全示例，脚本只打印下一步，不保存或回显配对 token。

## 错误处理与安全

- 所有脚本启用失败即停；每一步打印实际执行的命令和下一步修复建议。
- 每个入口支持 `--help`；`check` 不安装、不写配置、不启动服务。
- 缺少 SDK、模拟器、设备、Xcode 或签名条件时提前失败，不执行半截 sync/build。
- 脚本不会执行 `git reset/restore/checkout`，不会暂存/提交/推送，也不会覆盖已有 `go.mod`、本地签名文件或配对文件。
- 除 Android SDK 安装这种显式重型操作外，默认步骤只在仓库目录生成允许的依赖/构建产物。

## 验收

- macOS/Windows 的 `--help` 和 `check` 可运行并返回清晰结果。
- 缺少依赖时错误信息包含安装来源和下一步命令。
- 新 clone 后按 README 最短路径可完成 setup、Web 开发、Web 构建和 Android debug 运行。
- `android-run --live` 与 `android-run --dist` 不会互相遗留 `CAP_SERVER_URL`。
- Web test/lint/build 与 Go `GOFLAGS=-buildvcs=false go test ./...` 仍可独立运行。
- 脚本静态检查通过，且不把 token、签名配置、`go.mod`、`go.sum`、dist 或 Android/iOS 构建产物加入版本控制。
