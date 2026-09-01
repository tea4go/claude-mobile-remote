# Android 环境安装脚本实施计划

## 目标

参照 `Macro-Deck-Client-App` 的安装脚本，为当前项目提供基础工具安装和 Android SDK 实际安装入口，并删除职责重复的旧 `setup` 脚本。

## 实施项

1. 新增 macOS `install_1_base_tools.sh`：检查/提示 Go、Node/npm，恢复缺失的 `go.mod`，执行 `go mod tidy` 与 `npm ci`。
2. 新增 Windows `install_1_base_tools.ps1`：通过 winget 给出基础工具安装入口，恢复缺失的 `go.mod`，执行 Go/前端依赖安装。
3. 新增 macOS `install_2_android_sdk.sh`：支持自定义 SDK 根目录，安装 command-line tools、platform-tools、platform 36、build-tools 36，并输出环境变量配置。
4. 新增 Windows `install_2_android_sdk.ps1`：支持自定义 SDK 根目录，必要时下载 command-line tools，安装 SDK 组件，接受 license，并设置用户级环境变量。
5. 删除 `scripts/macos/setup.sh` 与 `scripts/windows/setup.ps1`，更新 README 和验证记录中的命令。
6. 保留 `check`、Web、Android、iOS、CLI 运行/构建入口。

## 验证

- macOS 运行 Bash 语法、帮助、`--check-only` 和现有 SDK 检查。
- Windows 在当前 macOS 无法运行 PowerShell 时进行文本级安全核对，并记录待 Windows 实机验证项。
- 运行 Mobile Web test/lint/build 和 `git diff --check`。
