# claude-mobile-remote

通过独立 Standalone CLI 托管一个本地 Claude Session，让用户离开电脑后可在手机上继续查看和控制。

## 环境准备（一次性）

前提：Go ≥ 1.25 与 Node ≥ 20。新 clone 或 pull 后，从仓库根目录按序执行：

macOS：

```bash
bash scripts/macos/1_install_base_tools.sh
bash scripts/macos/1_install_android_sdk.sh   # 需要 Android 时
bash scripts/macos/2_check.sh
```

Windows PowerShell：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/windows/1_install_base_tools.ps1
powershell -ExecutionPolicy Bypass -File scripts/windows/1_install_android_sdk.ps1  # 需要 Android 时
powershell -ExecutionPolicy Bypass -File scripts/windows/2_check.ps1
```

`1_install_base_tools` 安装 Go、Node/npm 与项目依赖；`1_install_android_sdk` 安装 Android SDK（command-line tools、platform-tools、platform/build-tools 36），不需要 Android 时可跳过；`2_check` 只读检查环境，不修改系统。基础安装只在 `go.mod` 不存在时从 `go.mod.base` 恢复，不覆盖已有本地 `go.mod`。

## Server 服务端（Claude 会话服务）

```bash
# macOS
bash scripts/macos/3_cli_run.sh --profile local
# Windows
powershell -ExecutionPolicy Bypass -File scripts/windows/3_cli_run.ps1 --profile local
```

脚本会把剩余参数透传给 `go run ./cmd/claude-mobile-remote`。启动后终端会输出可粘贴的配对 JSON 和二维码，用手机 Web 粘贴 JSON 或在地址栏放二维码 Fragment 即可连接。配对 JSON 含控制凭证，勿写入文件、日志或截图。

### 网络 profile

| profile | 监听地址 | 二维码中的客户端地址 | 适用场景 |
| --- | --- | --- | --- |
| `local`（默认） | `127.0.0.1:8765` | `ws://127.0.0.1:8765/ws` | 本机浏览器调试 |
| `emulator` | `0.0.0.0:8765` | `ws://10.0.2.2:8765/ws` | Android 模拟器，`10.0.2.2` 固定解释为宿主机 |
| `device` | `0.0.0.0:8765` | 自动选一个非 loopback IPv4 | 真机；多网卡时用 `--advertise-host` 覆盖 |
| `debug` | 同 `local` | 需手动指定 | 精确控制网络参数时的调试入口 |

### 常用参数

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `--profile` | `local` | 远程环境：`local`\|`emulator`\|`device`\|`debug` |
| `--listen` | `127.0.0.1:8765` | WebSocket 监听地址 |
| `--advertise-url` | `ws://127.0.0.1:8765/ws` | 写入二维码且客户端实际连接的地址 |
| `--advertise-host` | 空 | `device` profile 指定客户端可达 IPv4（高级参数） |
| `--pair-token-ttl` | `720h`（30 天） | 配对凭证有效期，可收紧 |
| `--allowed-origin` | `http://127.0.0.1:5173`、`http://localhost:5173` | 额外允许的浏览器 Origin，可重复 |
| `--resume` | 空 | 恢复已有会话的 session ID（UUID） |
| `--cwd` | 空 | Claude 会话的工作目录 |
| `--permission-mode` | 空 | Claude 权限模式：`default`\|`acceptEdits`\|`plan`\|`bypassPermissions` |
| `--model` | 空 | Claude 使用的模型 |
| `--max-turns` | `0` | Claude 单次连接允许的最大 Turn 数 |

日常启动指定 `--cwd`（工作目录）和 `--permission-mode`；恢复已有会话用 `--resume <sessionId>`（只能接续当前未被其他 Claude 进程占用的会话，不能接管运行中的会话）。不传 `--profile` 时默认 `local`。`--fake-host`（脚本化假 Host）与 `--version` 用于自动化验证，一般开发用不到。

## Web 端

| 用途 | macOS | Windows PowerShell |
| --- | --- | --- |
| 开发 | `bash scripts/macos/3_web_dev.sh` | `powershell -ExecutionPolicy Bypass -File scripts/windows/3_web_dev.ps1` |
| 构建 | `bash scripts/macos/4_web_build.sh` | `powershell -ExecutionPolicy Bypass -File scripts/windows/4_web_build.ps1` |

开发默认监听 localhost；需要手机或其他机器访问时用 `--host 0.0.0.0`（Windows `-Host`）。构建默认依次 test、lint、build，可用 `--skip-test`/`--skip-lint`（Windows `-SkipTest`/`-SkipLint`）跳过。

## Android 和 iOS

### Android（两条路径，二选一）

| 路径 | 说明 | macOS | Windows PowerShell |
| --- | --- | --- | --- |
| `dist` | 一条命令、自包含（推荐先跑通）：把 Web 构建产物打进 App，装到模拟器自动拉起，不依赖外部服务 | `bash scripts/macos/3_android_run.sh --dist` | `powershell -ExecutionPolicy Bypass -File scripts/windows/3_android_run.ps1 -Dist` |
| `live` | 两条命令、两个终端，改代码即刷新 | `bash scripts/macos/3_android_run.sh --live` | `powershell -ExecutionPolicy Bypass -File scripts/windows/3_android_run.ps1 -Live` |

**live 需要在两个终端分别执行**（Web dev server 常驻 + 装 App）：

```bash
# 终端 1：启动 Web dev server
bash scripts/macos/3_web_dev.sh            # Windows: powershell -ExecutionPolicy Bypass -File scripts/windows/3_web_dev.ps1

# 终端 2：装并自动拉起 App
bash scripts/macos/3_android_run.sh --live # Windows: powershell -ExecutionPolicy Bypass -File scripts/windows/3_android_run.ps1 -Live
```

要点：

- 模拟器通过 `10.0.2.2` 访问宿主机，live 端口需与 `--port`/`-Port` 一致（默认 5173）。
- 本机有多个 AVD 时用 `--avd`/`-Avd` 指定（如 `-Avd Phone_API35`）；只有一个可省略。可用 `emulator -list-avds` 查看。
- 只构建不运行（可选）：`bash scripts/macos/4_android_build.sh` / `powershell -ExecutionPolicy Bypass -File scripts/windows/4_android_build.ps1`。

### iOS（仅 macOS）

| 用途 | 命令 |
| --- | --- |
| Simulator 运行 | `bash scripts/macos/3_ios_run.sh` |
| Simulator 构建 | `bash scripts/macos/4_ios_build.sh` |

Windows 无 Xcode/iOS Simulator 环境，改用 Android 或 Web 入口。

## 常见问题排查

- **缺少 Go 或 Node/npm**：按脚本提示用 Homebrew（macOS）或 `winget`/官方安装包（Windows）。脚本不静默修改 PATH。
- **缺少 Android SDK / adb / 模拟器 / 平台组件**：执行 `1_install_android_sdk`，再跑带 Android 选项的 `2_check`（`bash scripts/macos/2_check.sh --android` / `powershell -ExecutionPolicy Bypass -File scripts/windows/2_check.ps1 -Android`）。先用 `emulator -list-avds` 确认存在 AVD。
- **Android live 白屏或连接失败**：确认 Web dev server 仍在运行、端口与 `--port`/`-Port` 一致，并确认模拟器走 `10.0.2.2`；真机不要套用该地址，改用 CLI 的 `device` profile。
- **Android dist 仍显示旧页面**：重新执行 `3_android_run --dist`/`-Dist`，确认没有残留 live server 配置；必要时卸载旧 Debug 包重装。
- **iOS 命令失败**：仅在有 Xcode 和命令行工具的 macOS 上运行，并先打开目标 Simulator。
- **脚本检查通过但 CLI 启动失败**：保留第一条错误，用 [docs/advanced-usage.md](docs/advanced-usage.md) 里的手工命令复现，区分 Go / SDK / Claude 配置 / 网络问题。

## 高级用法与排障

安全边界、debug profile、A6 局域网跨机器联调、脚本不可用时的手工诊断命令，见 [docs/advanced-usage.md](docs/advanced-usage.md)。

## 仓库

https://github.com/tea4go/claude-mobile-remote
