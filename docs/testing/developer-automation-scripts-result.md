# Developer Automation Scripts 验证记录

日期：2026-09-01

## 已完成

- macOS：`scripts/macos/` 提供 `1_install_base_tools`、`1_install_android_sdk`、`2_check`、`3_web_dev`、`4_web_build`、`3_cli_run`、`3_android_run`、`4_android_build`、`3_ios_run`、`4_ios_build`。
- Windows：`scripts/windows/` 提供与 macOS 对齐的基础、Web、CLI、Android 入口；Windows 不提供 iOS 入口。
- README 已补充新 pull 项目的最短路径、参数、排障、安全边界和手工命令。
- 脚本不执行 Git 写操作、不启动 Claude Service、不保存凭证；Windows CLI helper 隐藏参数日志。

## 实际验证

| 命令 | 结果 |
| --- | --- |
| `bash -n scripts/macos/*.sh` | 通过 |
| 全部 macOS 脚本 `--help` | 通过 |
| `scripts/macos/2_check.sh` | 通过；Go 1.25.3、Node 25.4.0、npm 11.12.1 |
| `scripts/macos/2_check.sh --android` | 通过；Java 21、SDK platform 36、Android 工具和 Gradle wrapper 可用 |
| 新增安装脚本 `--help`、macOS `--check-only` 临时目录检查 | 通过；未修改真实 SDK |
| `scripts/macos/2_check.sh --ios` | 按预期未通过：当前环境 `xcrun simctl` 不可用；工程为 Capacitor SPM，不要求 CocoaPods |
| `cd apps/mobile && npm exec -- cap --version` | 通过；Capacitor CLI 8.5.0 |
| `cd apps/mobile && npm test && npm run lint && npm run build` | 通过；405 tests，lint 0 error（保留 1 个既有 warning），build 通过 |
| `git diff --check` | 通过 |

## Windows 实测（PowerShell 5.1）

- 9 个 `.ps1` 补齐 UTF-8 BOM 并通过 5.1 parser；修复七类问题：缺 BOM（5.1 按 GBK 误读中文）、`_common.ps1` 正则转义、`2_check.ps1` 用 `--version` 检查 go（应为子命令 `go version`）、局部变量 `$home` 与只读 `$HOME` 冲突（改 `$sdkHome`）、`$ErrorActionPreference='Stop'` 下 `2>` 重定向被包装成终止错误（新增 `Invoke-Capture`）、StrictMode 下空 `List[string]` unroll 触发 `PropertyNotFoundStrict`（改 `if ($errors)`）、`Start-AndroidAvd` 冷启动超时 120 秒且 `-WindowStyle Hidden` 隐藏窗口（改 300 秒 + 进度输出）。
- `2_check.ps1` 端到端通过（go/node/npm 检出、退出码 0）；`2_check.ps1 -Android` 通过；`1_install_base_tools.ps1 -ShowHelp` 冒烟通过；`3_android_run.ps1 -Live` 的 `Require-AndroidEnvironment` 用真实 `ANDROID_HOME` 验证返回 gradle wrapper。
- Android 模拟器「卡死」根因：默认 virtio-wifi 拉起宿主 `netsimd.exe`（网络模拟守护进程）在本机 `ntdll.dll` access-violation 崩溃（火绒内核驱动 `sysdiag.sys`/`hrwfpdrv.sys` 仍加载，典型 AV 内核 hook），阻塞 QEMU 主循环——WHPX/AEHD/TCG 三加速、两种图形后端、两个模拟器版本、两个镜像「同样卡死、零内核输出」的统一原因。
- 解决：`_common.ps1` 的 `Start-AndroidAvd` 用 `-feature -Wifi -feature -VirtioWifi` 禁用，android-35（`Phone_API35`）完整启动（`Boot completed in 25844 ms`、`sys.boot_completed=1`、adb `device`、Android 15、netsimd 不再拉起）。
- `toggle-hypervisor.ps1` 保留 `Emulator`/`Whpx`/`Wsl2`/`Status` 四模式，WHPX 为推荐后端。
- Android Gradle 构建需 JDK 21–24（Capacitor 8 生成的 `capacitor.build.gradle` 写死 `source/target=Java 21`，Gradle 8.14.3 最高支持 JDK 24）；`JAVA_HOME` 指向 JDK 26 时报 `Unsupported class file major version 70`。`_common.ps1` 新增 `Get-JavaMajor`/`Get-EffectiveJavaHome`/`Resolve-AndroidJdk`，`Require-AndroidEnvironment` 自动改用兼容 JDK（优先 Android Studio 内置 JBR=JDK 21），`2_check.ps1 -Android` 会提示并自动改选；用 JBR 实测 `assembleDebug` 构建成功。

## 未执行/限制

- macOS 无 `pwsh`，Windows parser 与实机流程已在真实 Windows（PowerShell 5.1）复核通过（见上文「Windows 实测」）；Windows `-Help` 仅在 `1_install_base_tools.ps1` 冒烟通过，未全量执行。
- Android 模拟器已启动并完成 Android 15 引导（见上文），但未实际安装 APK；iOS Simulator 与 Xcode build 未执行。
- `GOFLAGS=-buildvcs=false go test ./...` 触发项目既有测试的环境限制：沙箱禁止 httptest 绑定 `[::1]:0`/`127.0.0.1:0`，失败点在 `cmd/channel-test-client`、`internal/transport`、`remote` 的监听测试，与本次脚本无关。
- 未执行安装脚本，避免在当前工作区重复运行 `go mod tidy`/`npm ci` 或修改真实 SDK；安装脚本的命令路径已做静态核对。

## 安全与产物核对

- 脚本扫描未发现 `git reset`、`git restore`、`git checkout`、`git add`、`git commit`、`git push`、Claude Service 启动或 keystore 读取。
- 未新增 token、配对 JSON、会话内容、APK/IPA、签名文件；`go.mod` 仍按项目本地约定不纳入提交候选。
