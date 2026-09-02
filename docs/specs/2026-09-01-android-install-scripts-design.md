# Android 环境安装脚本重构设计

日期：2026-09-01

## 背景

当前项目的 `setup` 只恢复 Go module、执行 `go mod tidy` 和 `npm ci`，`check --android` 只检查 Android 环境。参考 `Macro-Deck-Client-App` 的 `install_1_base_tools` 与 `install_4_android_sdk` 后，需要把“项目依赖准备”和“Android SDK 安装”拆成可发现、可重复执行的安装入口，并按执行顺序加数字前缀统一命名。

## 目标

- 为 macOS 和 Windows 提供真正安装 Android command-line tools 与 SDK 组件的入口。
- 保持安装行为显式，不在普通 `check` 或运行脚本中静默下载、接受 license 或修改系统配置。
- 让新 pull 项目可以按基础工具 → Android SDK → 检查 → 运行的顺序启动。

## 结构

### macOS

- `1_install_base_tools.sh`：检查/提示 Go、Node/npm，必要时通过 Homebrew 安装；恢复缺失的 `go.mod`，执行 `go mod tidy` 和 `npm ci`。
- `1_install_android_sdk.sh`：默认使用 `~/Library/Android/sdk`，优先使用已有 sdkmanager，否则通过 Homebrew 安装 command-line tools；安装 `platform-tools`、`platforms;android-36`、`build-tools`；输出当前终端环境变量和 shell 持久化配置提示。

### Windows

- `1_install_base_tools.ps1`：检查/提示 Go、Node/npm，优先使用 winget；恢复缺失的 `go.mod`，执行 `go mod tidy` 和 `npm ci`。
- `1_install_android_sdk.ps1`：定位或下载 command-line tools，检查 Java 17+，通过 sdkmanager 安装 `platform-tools`、`platforms;android-36`、`build-tools`，接受 SDK license，并设置当前会话及用户级 `ANDROID_HOME`/PATH。

### 保留和删除

保留现有 `2_check`、`3_web_dev`、`4_web_build`、`3_android_run`、`4_android_build`、`3_ios_run`、`4_ios_build` 和 `3_cli_run`，因为它们分别承担检查、开发、构建或调试职责。安装入口由 `1_install_base_tools` 与 `1_install_android_sdk` 承担，命名中的数字前缀 1/2/3/4 表示脚本执行顺序（1 安装 / 2 检查 / 3 运行 / 4 构建）。

## 安全边界

- 安装脚本只修改明确的 SDK 目录、项目本地依赖和 Windows 用户环境变量；macOS 默认不自动写入 shell 配置，持久化配置由用户复制脚本输出完成。
- Android 安装会在用户明确运行 `1_install_android_sdk` 时下载组件并接受 license；`2_check` 保持只读。
- 不读取或生成 keystore，不保存或打印 token/配对凭证，不启动 Claude Service，不执行 Git 写操作。
- SDK 目录通过显式参数或环境变量确定；不会递归删除已有 SDK。安装 command-line tools 时只在目标 `cmdline-tools/latest` 明确属于脚本管理的路径内更新。

## 验证

- macOS：`bash -n`、全部帮助、基础工具检查、Android SDK 检查；使用临时 SDK 根目录做参数/组件命令 dry-run，避免修改真实 SDK。
- Windows：在 Windows PowerShell 5.1/7 中执行 parser、帮助和安装后检查；当前 macOS 无法替代 Windows 实机验证。
- 项目回归：Mobile Web test/lint/build、Go 测试和 `git diff --check`；已知本地沙箱端口限制单独记录。
