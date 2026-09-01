# Developer Automation Scripts 验证记录

日期：2026-09-01

## 已完成

- macOS：`scripts/macos/` 提供 `install_1_base_tools`、`install_2_android_sdk`、`check`、`web-dev`、`web-build`、`cli-run`、`android-run`、`android-build`、`ios-run`、`ios-build`。
- Windows：`scripts/windows/` 提供与 macOS 对齐的基础、Web、CLI、Android 入口；Windows 不提供 iOS 入口。
- README 已补充新 pull 项目的最短路径、参数、排障、安全边界和手工命令。
- 脚本不执行 Git 写操作、不启动 Claude Service、不保存凭证；Windows CLI helper 隐藏参数日志。

## 实际验证

| 命令 | 结果 |
| --- | --- |
| `bash -n scripts/macos/*.sh` | 通过 |
| 全部 macOS 脚本 `--help` | 通过 |
| `scripts/macos/check.sh` | 通过；Go 1.25.3、Node 25.4.0、npm 11.12.1 |
| `scripts/macos/check.sh --android` | 通过；Java 21、SDK platform 36、Android 工具和 Gradle wrapper 可用 |
| 新增安装脚本 `--help`、macOS `--check-only` 临时目录检查 | 通过；未修改真实 SDK |
| `scripts/macos/check.sh --ios` | 按预期未通过：当前环境 `xcrun simctl` 不可用；工程为 Capacitor SPM，不要求 CocoaPods |
| `cd apps/mobile && npm exec -- cap --version` | 通过；Capacitor CLI 8.5.0 |
| `cd apps/mobile && npm test && npm run lint && npm run build` | 通过；405 tests，lint 0 error（保留 1 个既有 warning），build 通过 |
| `git diff --check` | 通过 |

## 未执行/限制

- 当前 macOS 没有 `pwsh` 或 Windows PowerShell，因此未执行 Windows parser、`-Help` 和 Windows 实机流程；需在 Windows 环境复核。
- 未实际启动 Android 模拟器、安装 APK，未运行 iOS Simulator 或 Xcode build。
- `GOFLAGS=-buildvcs=false go test ./...` 触发项目既有测试的环境限制：沙箱禁止 httptest 绑定 `[::1]:0`/`127.0.0.1:0`，失败点在 `cmd/channel-test-client`、`internal/transport`、`remote` 的监听测试，与本次脚本无关。
- 未执行安装脚本，避免在当前工作区重复运行 `go mod tidy`/`npm ci` 或修改真实 SDK；安装脚本的命令路径已做静态核对。

## 安全与产物核对

- 脚本扫描未发现 `git reset`、`git restore`、`git checkout`、`git add`、`git commit`、`git push`、Claude Service 启动或 keystore 读取。
- 未新增 token、配对 JSON、会话内容、APK/IPA、签名文件；`go.mod` 仍按项目本地约定不纳入提交候选。
