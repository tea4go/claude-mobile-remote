# Android / Harmony 同步验证

日期：2026-10-27。基准：当前 iOS 已验收功能及 mobile-parity 设计。用户已授权实施与提交，不推送。

## 差异闭合

| 能力 | Android | Harmony |
| --- | --- | --- |
| 配对目标、分阶段超时、有效快照后配对 | 共用 React 实现，同步资源 | 新增阶段期限与安全目标展示；有效快照后保存并接续同一连接 |
| 状态进度、耗时、token | 共用实现 | 新增 v5 解码、状态归约、中文阶段/耗时/token，结束清除进度 |
| WebSocket 静默断网 | 共用 v6 心跳 | 新增10秒探活/5秒响应、超期关闭、系统网络监听、前台复查与后台释放 |
| 旧宿主兼容 | 沿用 v2–v5 降级 | 补齐 v6→v5/v4/v3/v2；旧协议不宣称应用层心跳 |
| 调试面板 | 默认开启、右侧入口、倒序、紧凑操作 | 新增同等入口与200条内存日志、设置开关、清空/复制/临时文件分享 |
| 宿主异常同步 | 共用安全诊断处理 | 新增固定诊断码与受限数值记录；不显示原始后台日志 |
| 审核离线演示 | 配对/设置入口共用实现 | 新增独立内存控制器、配对/设置入口、持续模拟标识、流式/进度、审批、问答、停止、重置/退出 |
| 附件、语音、通知、安全存储、缓存 | 沿用既有 Android 原生适配 | 沿用既有 Harmony 原生适配；回归保留凭证/消息隔离与缓存限制 |
| 安全区域与键盘 | 共用安全区 CSS 与既有 Android 原生布局 | 保留系统安全区域、RESIZE 键盘避让；演示复用原生详情，不扩展到状态栏区域 |

Harmony 演示不创建 ProtocolClient、不生成配对凭证、不调用缓存/文件/原生权限端口；真实会话仍遵循原有生命周期。模拟模式并不代表整个 App 中已配对的真实连接都停止。

## 执行证据

- `bash scripts/macos/4_android_build.sh`：移动37文件542测试通过；lint仅既有 cdp-console 警告；Web build 与 cap sync android 通过。首次 Gradle 被沙箱阻止访问用户缓存，获工具审批后续办原生编译。
- `./gradlew assembleDebug assembleRelease`：BUILD SUCCESSFUL。Debug为测试签名；Release未签名，不视为正式分发包。
- 两个APK内11个Web文件逐字节匹配dist；无开发server.url，桥接完整日志关闭，Android远程检查开关为true。
- `npm test --prefix apps/harmony`：53项测试通过，16个逻辑文件类型检查通过。新增覆盖 v5/v6 严格解码、降级、各阶段超时、心跳成功/失败、跨Session拒绝、断网后操作未知、日志上限/脱敏/开关、演示审批/问答/取消/清理。
- `bash scripts/macos/4_harmony_build.sh`：API24真实ArkTS/ArkUI编译与HAP压缩结构检查通过；存在既有弃用API/异常处理告警，未配置签名。最终日志 `/tmp/cmr-harmony-build.log`。
- 既有待提交发布准备源码：`GOFLAGS=-buildvcs=false go test ./cmd/review-server`通过（临时端口需沙箱外执行）；TestFlight脚本5项测试通过；新增shell脚本语法检查通过。
- `git diff --check`通过。安装包、工具缓存、签名材料、实际配对凭证和私人截图不提交。

## 产物

- Android测试APK：`apps/mobile/android/app/build/outputs/apk/debug/ClaudeMobileRemote.apk`，SHA256 `1da51cf380765aded8f12d3d1f26973edca64b933b49fa8df2284ed5f349baab`。
- Android未签名Release：`apps/mobile/android/app/build/outputs/apk/release/app-release-unsigned.apk`，SHA256 `52d2c6635d8f6c2db20523d3c1d07d29c8d210c7360f4b188b5de7babdaa8169`。
- Harmony未签名HAP：`apps/harmony/entry/build/default/outputs/default/entry-default-unsigned.hap`；仅编译验证，设备安装与AppTest发布另需签名。

## 未验证与人工验收

本轮未安装Android/Harmony真机，未完成新平台审核或发布，不以编译代替体验验收；iOS已提交的TestFlight构建未替换。

1. 离线从配对页/设置进入演示，确认顶部标识、审批允许/拒绝、问答、停止后继续、退出重进均正常。
2. 真实扫码检查目标与阶段；错误网络停留配对页，成功后进入详情并显示进度。
3. 前台切换/断开网络，确认无需发送消息即可更新连接状态；恢复后手动重连，草稿/消息保留且不自动重发。
4. 查看默认右侧调试入口、倒序日志、关闭/开启、复制/导出；确认键盘弹出时输入区及操作可用。
