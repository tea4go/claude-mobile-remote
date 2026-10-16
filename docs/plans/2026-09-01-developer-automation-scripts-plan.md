# 开发者自动化脚本 Implementation Plan

> 本计划按项目自身的开发流程逐项执行。步骤使用复选框（`- [ ]`）跟踪。

**Goal:** 为 macOS 和 Windows 提供可发现、可重复执行的项目安装、环境检查、Web/原生调试、CLI 启动和构建入口，让新 pull 的开发者能从 README 的最短路径开始。

**Architecture:** 使用平台原生脚本：macOS 为 Bash，Windows 为 PowerShell。每个平台有一个共享 helper 负责仓库定位、日志、命令检查、版本检查和项目依赖准备；各入口只编排现有的 Go、npm、Capacitor、Gradle、Xcode 和 Android SDK 命令，不引入新的任务运行时。

**Tech Stack:** Bash 3+/zsh-compatible shell, PowerShell 5.1+, Go 1.25+, Node.js 20+, npm, Vite, Capacitor CLI, Gradle wrapper, Xcode Simulator, Android SDK/adb/emulator。

---

## 文件清单与职责

### 新增

- `scripts/macos/_common.sh`：macOS 脚本共享函数；定位仓库、打印日志、检查命令/版本、初始化 `go.mod`、检查前端依赖和 Android/iOS 工具。
- `scripts/macos/1_install_base_tools.sh`：准备 Go/Node/npm 项目依赖。
- `scripts/macos/1_install_android_sdk.sh`：安装/检查 Android SDK（command-line tools、platform-tools、platform/build-tools 36）；可选 `--sdk-root`、`--check-only`。
- `scripts/macos/2_check.sh`：只读环境检查，支持 `--android`、`--ios` 和 `--help`。
- `scripts/macos/3_web_dev.sh`：启动 Vite，默认 localhost，透传 `--host` 与 `--port`。
- `scripts/macos/4_web_build.sh`：按顺序运行 Mobile Web test、lint、build；支持 `--skip-test` 与 `--skip-lint`。
- `scripts/macos/3_android_run.sh`：选择 `--live` 或 `--dist`，检查/启动 AVD，sync 并安装 debug APK。
- `scripts/macos/4_android_build.sh`：运行 Web build、Capacitor sync 和 Android debug/release 构建；不处理签名。
- `scripts/macos/3_ios_run.sh`：运行 Web build、Capacitor sync，并将 iOS 工程运行到指定 Simulator。
- `scripts/macos/4_ios_build.sh`：运行 Web build、Capacitor sync，并检查 Xcode archive 前置条件后构建 iOS 工程。
- `scripts/macos/3_cli_run.sh`：准备 Go module 后运行根目录 CLI，原样透传剩余参数。
- `scripts/windows/_common.ps1`：Windows 脚本共享函数，使用 PowerShell 原生命令处理路径和依赖检查。
- `scripts/windows/1_install_base_tools.ps1`：准备 Go/Node/npm 项目依赖；通过 winget 给出缺失组件安装提示。
- `scripts/windows/1_install_android_sdk.ps1`：安装/检查 Android SDK（command-line tools、platform-tools、platform/build-tools 36）；可选 `-SdkRoot`。
- `scripts/windows/2_check.ps1`：只读环境检查，支持 `-Android`、`-Help`。
- `scripts/windows/3_web_dev.ps1`：启动 Vite，默认 localhost，支持 `-Host`、`-Port` 和剩余 npm 参数。
- `scripts/windows/4_web_build.ps1`：运行 test、lint、build，支持 `-SkipTest` 与 `-SkipLint`。
- `scripts/windows/3_android_run.ps1`：选择 `-Live` 或 `-Dist`，检查/启动 AVD，sync 并安装 debug APK。
- `scripts/windows/4_android_build.ps1`：运行 Web build、Capacitor sync 和 Android debug/release 构建，不处理签名。
- `scripts/windows/3_cli_run.ps1`：准备 Go module 后运行根目录 CLI，原样透传剩余参数。

### 修改

- `README.md`：增加“新 pull 项目三步开始”、平台脚本命令表、Web/Android/iOS/CLI 示例、Windows 不支持 iOS 的说明和脚本安全边界。
- `HANDOFF.md`：在开发前登记脚本实施；开发中记录脚本接口与验证结论；收尾记录实际完成项和未验证平台。

## Task 1: 建立 macOS 共享 helper 与只读检查

**Files:**
- Create: `scripts/macos/_common.sh`
- Create: `scripts/macos/2_check.sh`

- [ ] **Step 1: 编写共享 helper 的基础函数**

在 `_common.sh` 开启 `set -euo pipefail`，计算 `SCRIPT_DIR` 和 `REPO_ROOT`，并提供以下函数：`log_info`、`log_warn`、`die`、`need_cmd`、`version_at_least`、`ensure_go_module`、`ensure_mobile_dir`、`run_mobile_npm`。`ensure_go_module` 只在根目录不存在 `go.mod` 时执行 `cp go.mod.base go.mod`，不得覆盖已有文件；`run_mobile_npm` 在 `apps/mobile/package-lock.json` 存在时使用 `npm ci`，否则明确失败并提示锁文件缺失。

- [ ] **Step 2: 添加 macOS 环境检查入口**

`2_check.sh` 解析 `--help`、`--android`、`--ios`，默认检查 `go`、`node`、`npm` 及 Go 1.25+/Node 20+；`--android` 额外检查 `ANDROID_HOME` 或 `ANDROID_SDK_ROOT`、`adb`、`emulator` 和项目所需 platform/build-tools；`--ios` 检查 `xcodebuild`、`xcrun simctl` 和 `pod`。缺失项逐项输出安装建议后以非零状态退出；检查脚本不能安装、写配置或启动进程。

- [ ] **Step 3: 运行静态 shell 检查**

运行 `bash -n scripts/macos/_common.sh scripts/macos/2_check.sh`，并运行 `scripts/macos/2_check.sh --help`。预期无语法错误，帮助文本包含所有选项。

## Task 2: 实现 macOS 安装、Web 和 CLI 入口

**Files:**
- Modify: `scripts/macos/_common.sh`
- Create: `scripts/macos/1_install_base_tools.sh`
- Create: `scripts/macos/1_install_android_sdk.sh`
- Create: `scripts/macos/3_web_dev.sh`
- Create: `scripts/macos/4_web_build.sh`
- Create: `scripts/macos/3_cli_run.sh`

- [ ] **Step 1: 添加基础工具安装流程**

`1_install_base_tools.sh` 支持 `--help`。先检查 Go/Node/npm，再调用 `ensure_go_module`、`go mod tidy` 和 `npm ci`；缺少命令时优先使用已存在的 `brew` 给出或执行对应安装步骤，未安装 Homebrew 时输出 brew 官方安装提示并失败。

- [ ] **Step 2: 添加 Android SDK 安装入口**

`1_install_android_sdk.sh` 支持 `--sdk-root PATH`、`--check-only` 和 `--help`，默认 SDK 根目录为 `$ANDROID_HOME`/`$ANDROID_SDK_ROOT`/`~/Library/Android/sdk`。显式运行时才安装 command-line tools、`platform-tools`、`platforms;android-36`、`build-tools` 并接受 license；`--check-only` 只检查 sdkmanager/adb，不下载组件、不静默修改系统配置。

- [ ] **Step 3: 添加 Web 开发与构建入口**

`3_web_dev.sh` 支持 `--host VALUE`、`--port VALUE` 和 `--help`，默认执行 `npm run dev -- --host 127.0.0.1`，指定参数时只覆盖对应 Vite 参数。`4_web_build.sh` 支持 `--skip-test`、`--skip-lint`；在 `apps/mobile` 中按 test、lint、build 顺序执行，任何一步失败立即退出。

- [ ] **Step 4: 添加 CLI 透传入口**

`3_cli_run.sh` 先调用 `ensure_go_module`，再执行 `go run ./cmd/claude-mobile-remote "$@"`；不替用户增加 `--profile`、`--cwd`、`--resume` 或配对参数，不回显参数中的 token。

本轮补充：核对 CLI 全包源码在仓库中可见，将 `.gitignore` 的根目录 CLI 二进制规则锚定到仓库根，避免忽略 `cmd/claude-mobile-remote/` 内新源码；macOS 启动说明标明必须按包运行。用 CLI 包测试和 `darwin/amd64` 构建验证。

- [ ] **Step 5: 验证 macOS 基础入口**

运行 `bash -n scripts/macos/*.sh`、每个脚本的 `--help`，并运行 `scripts/macos/2_check.sh`。预期帮助均成功，check 对本机现有工具给出明确结果；不得生成 token、修改 Git 状态或覆盖已有 `go.mod`。

## Task 3: 实现 macOS Android/iOS 原生入口

**Files:**
- Modify: `scripts/macos/_common.sh`
- Create: `scripts/macos/3_android_run.sh`
- Create: `scripts/macos/4_android_build.sh`
- Create: `scripts/macos/3_ios_run.sh`
- Create: `scripts/macos/4_ios_build.sh`

- [ ] **Step 1: 添加 Android SDK、AVD 和 Gradle helper**

共享函数必须解析 SDK 根目录、检查 `adb`/`emulator`/`sdkmanager`，并检查 `apps/mobile/android/gradlew`。AVD 选择优先使用 `--avd`，否则使用环境变量 `AVD`，最后回退到 `pixel7`；找不到 AVD 时失败并列出 `emulator -list-avds` 的修复命令。启动模拟器后使用 `adb shell getprop sys.boot_completed` 等待 ready，并设置有限重试次数。

- [ ] **Step 2: 实现 Android live/dist 互斥流程**

`3_android_run.sh` 必须要求且只接受一种模式：`--live` 或 `--dist`。live 模式执行 `CAP_SERVER_URL=http://10.0.2.2:5173 npm exec cap sync android` 后安装 debug；dist 模式先执行 Web build，再在未设置 `CAP_SERVER_URL` 的环境中执行 `npm exec cap sync android`，随后安装 debug。支持 `--avd` 和 `--port`，live 模式的端口会同步到 Vite server URL；脚本不启动 Service、不打印配对凭证。

- [ ] **Step 3: 实现 Android 构建流程**

`4_android_build.sh` 默认运行 Web build、Capacitor sync 和 `./gradlew assembleDebug`；`--release` 只运行 `assembleRelease` 并提示产物可能需要外部签名，不读取 keystore、不生成签名配置。

本轮补充：macOS/Windows 构建入口的 sync 均需隔离继承的 `CAP_SERVER_URL`，完成后检查对应 APK 文件并打印绝对路径；验证 debug APK 内的 Web 资源与本轮 `dist` 一致，且 Capacitor 配置不含开发服务器 URL。

debug 包名调整：在 Android Gradle 配置中将 debug 输出文件名设为 `ClaudeMobileRemote.apk`，macOS/Windows 构建脚本与根目录 README 使用该路径；`assembleRelease` 的未签名输出文件名保持不变。运行 `clean assembleDebug` 确认真实产物，并核对文档、脚本及 APK 内容。

- [ ] **Step 4: 实现 iOS 运行和构建流程**

`3_ios_run.sh` 支持 `--simulator NAME`，默认使用当前可用的 iPhone Simulator；先运行 Web build 和 `npm exec cap sync ios`，再调用 `npx cap run ios --target NAME`。`4_ios_build.sh` 运行 Web build 和 sync，再调用 `xcodebuild -workspace App.xcworkspace -scheme App -configuration Debug -sdk iphonesimulator -destination 'generic/platform=iOS Simulator' build`；缺少 CocoaPods/Xcode 时提前失败，不触碰签名。

- [ ] **Step 5: 验证 macOS 原生入口的 dry-run 条件**

运行所有 macOS 原生脚本的 `--help`，在本机运行 `2_check.sh --android --ios`；使用 shell tracing 或受控临时目录确认 live 命令含 `10.0.2.2`、dist 命令不含 `CAP_SERVER_URL`。不能因无连接设备而把 `3_android_run` 或 `3_ios_run` 的失败误报成脚本通过。

## Task 4: 建立 Windows 共享 helper、检查和安装

**Files:**
- Create: `scripts/windows/_common.ps1`
- Create: `scripts/windows/2_check.ps1`
- Create: `scripts/windows/1_install_base_tools.ps1`
- Create: `scripts/windows/1_install_android_sdk.ps1`

- [ ] **Step 1: 编写 PowerShell helper**

`_common.ps1` 使用 `Set-StrictMode -Version Latest` 和 `$ErrorActionPreference = 'Stop'`，计算脚本目录和仓库根目录，提供 `Write-Info`、`Write-Warn`、`Fail`、`Require-Command`、`Ensure-GoModule`、`Invoke-MobileNpm` 和 `Invoke-MobileCommand`。路径通过 `Join-Path` 处理；`Ensure-GoModule` 只在 `go.mod` 不存在时复制 `go.mod.base`。

- [ ] **Step 2: 实现 Windows check**

`2_check.ps1` 支持 `-Help`、`-Android`，检查 Go 1.25+、Node 20+、npm；Android 选项检查 `ANDROID_HOME`/`ANDROID_SDK_ROOT`、`adb`、`emulator` 和 SDK 组件。缺失项输出 winget 或 Android Studio/官方安装建议并以非零状态结束，不安装和不写配置。

- [ ] **Step 3: 实现 Windows 基础工具安装**

`1_install_base_tools.ps1` 基础流程执行依赖检查、`Ensure-GoModule`、`go mod tidy`、`npm ci`；缺少命令时优先提示可用 `winget install`，不自动改变 PATH。

- [ ] **Step 4: 实现 Windows Android SDK 安装**

`1_install_android_sdk.ps1` 支持 `-SdkRoot 路径`，默认 `ANDROID_HOME`/`%LOCALAPPDATA%\Android\Sdk`；定位或下载 command-line tools，检查 Java 17+，通过 sdkmanager 安装 `platform-tools`、`platforms;android-36`、`build-tools` 并接受 license，设置当前会话及用户级 `ANDROID_HOME`/PATH。

- [ ] **Step 5: 验证 PowerShell 语法和帮助**

在 Windows 环境运行 `Get-Command` 检查脚本文件可解析，并执行 `2_check.ps1 -Help`、`1_install_base_tools.ps1 -Help`。在 macOS 只能做文本级参数和路径检查，不能宣称 PowerShell 已执行通过。

## Task 5: 实现 Windows Web、CLI 和 Android 入口

**Files:**
- Create: `scripts/windows/3_web_dev.ps1`
- Create: `scripts/windows/4_web_build.ps1`
- Create: `scripts/windows/3_cli_run.ps1`
- Create: `scripts/windows/3_android_run.ps1`
- Create: `scripts/windows/4_android_build.ps1`

- [ ] **Step 1: 实现 Windows Web 与 CLI 入口**

`3_web_dev.ps1` 支持 `-Host`、`-Port`、`-Help`，调用 `npm run dev -- --host ... --port ...`；`4_web_build.ps1` 支持 `-SkipTest`、`-SkipLint` 并按 test/lint/build 顺序运行；`3_cli_run.ps1` 调用 `Ensure-GoModule` 后执行 `go run ./cmd/claude-mobile-remote @Args`，原样透传剩余参数。

- [ ] **Step 2: 实现 Windows Android live/dist**

`3_android_run.ps1` 支持且只接受 `-Live` 或 `-Dist`，另有 `-Avd`、`-Port`。live 使用 `$env:CAP_SERVER_URL = "http://10.0.2.2:$Port"` 后 sync android；dist 先 Web build，再移除当前进程环境的 `CAP_SERVER_URL` 后 sync android；两者都调用 Gradle wrapper 的 `installDebug`。脚本必须在退出前恢复原有 `CAP_SERVER_URL`。

- [ ] **Step 3: 实现 Windows Android build**

`4_android_build.ps1` 默认运行 Web build、sync 和 `gradlew assembleDebug`；`-Release` 运行 `assembleRelease` 并提示外部签名要求，不访问 keystore。

- [ ] **Step 4: 做跨平台接口对照检查**

逐项核对 macOS/Windows 的 `2_check`、`1_install_base_tools`、`1_install_android_sdk`、`3_web_dev`、`4_web_build`、`3_android_run`、`4_android_build`、`3_cli_run` 语义和帮助说明。Windows 不提供 iOS 脚本，README 必须明确原因和替代入口。

## Task 6: 更新 README 新手路径与安全说明

**Files:**
- Modify: `README.md`

- [ ] **Step 1: 添加新 pull 最短路径**

在 README 顶部开发说明增加：macOS 执行 `bash scripts/macos/1_install_base_tools.sh`、`bash scripts/macos/2_check.sh`，Windows 执行 `powershell -ExecutionPolicy Bypass -File scripts/windows/1_install_base_tools.ps1`、`...2_check.ps1`；需要 Android 时追加 `1_install_android_sdk.sh`/`1_install_android_sdk.ps1`。然后分别给出 Web 开发、Web 构建、Android live/dist、CLI 启动命令。

- [ ] **Step 2: 添加命令表和参数示例**

说明 `--live` 需要同时启动 Web dev server，Android 模拟器通过 `10.0.2.2` 访问宿主机；`--dist` 使用本地构建资源；macOS 的 iOS 命令仅面向 Xcode Simulator；Windows 没有 iOS 入口。注明所有脚本都会向下透传 CLI 参数但不会替用户选择 profile 或写凭证。

- [ ] **Step 3: 添加故障排查和安全边界**

明确缺少 Go/Node/Android SDK/Xcode 时的安装来源；说明脚本不开放防火墙、不保存 token、不覆盖已有 `go.mod`、不执行 Git 写操作，也不启动 Claude Service。保留手工命令作为脚本失败时的诊断依据。

## Task 7: 验证、交接和门禁记录

**Files:**
- Modify: `HANDOFF.md`
- Create: `docs/testing/developer-automation-scripts-result.md`

- [ ] **Step 1: 执行脚本静态验证**

macOS 运行 `bash -n scripts/macos/*.sh`、所有 `--help`、`2_check.sh`；Windows 环境运行 PowerShell parser/check/help；若当前环境无法执行 Windows，则在验证记录中明确标为未执行，不用静态检查冒充平台运行结果。

- [ ] **Step 2: 执行项目回归验证**

运行 `GOFLAGS=-buildvcs=false go test ./...`、`cd apps/mobile && npm test && npm run lint && npm run build` 和 `git diff --check`。预期既有 Go 与 Mobile Web 门禁不回归；lint 的既有 warning 单独记录。

- [ ] **Step 3: 核对安全与产物边界**

用 `git diff --check`、`git status --short` 和文件扫描确认没有 token、`go.mod`、`go.sum`、`apps/mobile/dist`、APK/IPA 或签名文件被新增到版本控制候选中；确认脚本没有 `git reset`、`git restore`、`git checkout`、`git add`、`git commit`、`git push`。

- [ ] **Step 4: 写验证记录并收尾 HANDOFF**

在 `docs/testing/developer-automation-scripts-result.md` 记录实际运行的命令、macOS/Windows 各自结果、未执行项和已知限制；更新 `HANDOFF.md` 的当前状态、实际完成项、验证结论和下一步。不得在没有用户明确授权时执行 Git 暂存、提交或推送。

## 计划自查

- 设计覆盖：脚本目录、跨平台同语义入口、安装/检查、Web/CLI、Android live/dist、macOS iOS、README 新手路径、安全边界和验证记录均有对应任务。
- 占位符扫描：本计划没有未决标记或“实现后再决定”等模糊步骤；所有脚本入口、选项、关键命令和预期验证均已写明。
- 接口一致性：macOS 使用长选项，Windows 使用 PowerShell 参数；对应入口的行为在 Task 4/5 对照，Android live/dist 的环境变量清理策略已分别定义。
