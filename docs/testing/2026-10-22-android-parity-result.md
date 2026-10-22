# Android 同步 iOS 已验收功能

用户于 2026-10-22 确认 iOS 验证完毕，要求先提交当前代码，再同步 Android 并单独提交；WhaleTerm 留给用户调试样式，不修改、不提交。本轮不推送。

## 提交与功能范围

- iOS/公共实现已保存为 **5e4a0a7**。
- Android 复用当前 React UI 与协议：平铺大触点会话列表、机器名 UTF-8、PC 管理、v4 有界工具结果与旧版协商、图片/相机/文件附件上传与预览、审批/问答、取消/停止、队列、多 Session 与重连状态校正。
- 新增 Android `SessionExperience` 原生插件：SpeechRecognizer 生成可编辑中文草稿；麦克风权限、单录音、超时、停止/取消、退后台/销毁清理；可用时优先设备识别服务，服务不可用会失败，不自动发送。
- 新增在线本地通知：NotificationChannel、Android 13+ 运行时通知权限、固定隐私文案、每 Session 一条；当前可见会话静默，后台可提醒；不可变 PendingIntent 点击传回 `notificationOpened`，支持保留早于 Web 监听的通知事件。退出后不会通过该能力收到新事件，极光仍暂停。
- 新增无数据库私有文件缓存：noBackupFilesDir + SHA256 文件名、串行 I/O、原子替换；共用 TS 白名单、200 项/2 MiB 每会话、20 MiB 总量、30 天保留；不存附件原件、PairToken、审批执行态；重启恢复列表/详情后由 snapshot 校正，删除/清空等价 iOS。系统备份/设备迁移排除本地数据。
- 共用原生端口改为按 iOS/Android 选择，Web 仍为 null；原生扫码、安全存储、Preferences、App、Keyboard 插件保留；Manifest 补语音识别/相机可见性和所需权限，不新增 npm、数据库或推送 SDK。

设计：[Android 同步设计](../specs/2026-10-22-android-parity-design.md)。计划：[实施计划](../plans/2026-10-22-android-parity-plan.md)。

## 验证证据

- 共用 Web 全套 **29 文件 504/504**；包含新增 iOS/Android/Web 端口选择 3 项回归。TypeScript/Vite build、oxlint 通过，oxlint 仅既有 cdp-console.mjs warning。
- 实际使用本机 `/Users/zhangym/.jvms/bin/store/openjdk-21`，不修改全局 Java；`cap sync android` 保留 5 个既有原生插件并使用打包本地资源，无 CAP_SERVER_URL。
- `JAVA_HOME=... ANDROID_HOME=... ./gradlew testDebugUnitTest assembleDebug lintDebug` 成功。新增原生 **6/6**：缓存 4 项（隔离/哈希路径、原子替换/UTF-8 字节上限、过期/总量淘汰/清理、损坏 UTF-8/中断临时文件）；通知序号 2 项（Integer/Long/Double 合法整数、非法范围/非整数拒绝）。另有模板测试随 Gradle 通过。
- 模拟器 pixel7 安装 APK 并启动 App。通过 WebView/CDP 调用真实插件，使用合成 sessionId，无真实 Pair Token：插件注册、中文文件缓存写读、默认通知关闭通过。
- 强制结束 App 进程再启动后，真实原生文件缓存内容仍可读；删除后返回 null，通过。
- 前台当前 Session 不发提醒；退到 Home 后，同 Session 的提醒进入系统通知栏。实际展开通知栏并点击**正文**，App 接收正确 `notificationOpened.sessionId`，通过。首轮测试发现 PluginCall.getLong 不接受 JSON Integer 导致序号为0，已改 Number/安全整数校验并加入回归；最终通知链路全部通过。
- 模拟器语音调用和取消能够结束 Promise，服务缺失时返回不可用；没有使用实际麦克风进行识别质量验收。
- Android lint 无 error；保留依赖版本、旧资源和既定直连明文网络等 warning。新备份规则及权限通过合并 Manifest/构建检查。
- `git diff --check` 通过。APK 为本机构建产物，截图/临时调试脚本/缓存文件均不提交。

## 产物与人工验收

APK：`apps/mobile/android/app/build/outputs/apk/debug/ClaudeMobileRemote.apk`（约 35 MiB）。

macOS 构建：`JAVA_HOME="$HOME/.jvms/bin/store/openjdk-21" bash scripts/macos/4_android_build.sh`（其他机器设置自己的 JDK 21–24）。

安装：`adb install -r apps/mobile/android/app/build/outputs/apk/debug/ClaudeMobileRemote.apk`。

用户真机验收：扫码/VPN 与两个 Session 交叉使用；相册、拍照、文件和大图上传；权限允许/拒绝；语音草稿编辑/停止/取消及后台清理；通知权限和点击定位（含进程不在内存时）；重启离线查看目录和消息、联网校正与清理。厂商通知/语音服务/后台策略、真实相机及物理 Android 设备未由本轮验证，不以模拟器结果代替。

iOS 原生 Swift 未修改；Harmony 后续独立同步。WhaleTerm 本轮没有修改、暂存或提交。
