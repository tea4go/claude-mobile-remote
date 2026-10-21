# C 阶段 iOS 优先实施验证

日期：2026-10-21。范围：既有 C2 回归与新增 C3 语音草稿、在线事件本地系统提醒。

## 实际实现

- iOS Speech/AVAudioEngine 自定义插件；中文识别，优先本机能力；首次操作请求权限，录音最多 60 秒；停止追加草稿，不自动发送；禁用、切页、后台和音频中断清理麦克风；固定错误文案。
- 设置中显式启用/关闭系统提醒；实时审批、问答、完成、失败、Session 停止触发固定文案，当前正在查看的 Session 不提醒。序号去重，Snapshot 不补发，单 Session 保留最新提醒，删除配对清理提醒。
- 原生启动时注册通知路由；冷启动点击保留到插件及配对恢复完成；点击已有本地配对进入详情，失效配对仍要求重新扫码；不自动执行会话命令。
- 共享 React 端口按平台门控，只有 iOS 接入本轮原生功能；未同步 Android/Harmony 原生工程。

## 本机验证

- `cd apps/mobile && npm test -- --reporter=dot`：26 文件、476/476 通过。覆盖既有配对/消息/审批/取消/多会话及新增草稿追加、不自动发、禁用后迟到结果隔离、卸载取消、权限失败、提醒分类/去重/当前会话抑制/通知设置/点击定位。
- `npm run build`：通过。既有 chunk size 提示仍存在。
- `npm run lint`：通过；仅原有 `scripts/cdp-console.mjs:76` no-unused-expressions 警告。
- `env -u CAP_SERVER_URL npm exec -- cap sync ios`：通过，本地 dist 已同步，未携带开发服务器 URL。
- Xcode Simulator 编译通过；iPhone 17 Pro / iOS 26.3 模拟器安装、启动通过。语音硬件与系统权限不能由编译推断通过。
- `xcodebuild -project apps/mobile/ios/App/App.xcodeproj -scheme App -configuration Debug -sdk iphoneos -destination 'generic/platform=iOS' -derivedDataPath /tmp/cmr-c3-ios-device CODE_SIGNING_ALLOWED=NO build`：真机 arm64 架构通过，无签名，未安装到真机。
- 同工程 `-sdk iphonesimulator -destination 'generic/platform=iOS Simulator' -derivedDataPath /tmp/cmr-c3-ios-build CODE_SIGNING_ALLOWED=NO build`：最终通知路由版本通过，重新安装并启动成功。
- `git diff --check`：通过。
- 模拟器截图复核发现中文显示缺字（方框问号）；当前 iOS 26.3 runtime 的 Fonts 目录未找到 PingFang，已有 antd-mobile 字体回退包含 PingFang SC。尚未完成环境层修复，不将截图计为视觉验收通过；需要真机核对。此问题不影响上述编译/启动结果。

构建日志位于 `/tmp/cmr-c3-ios-build.log`、`/tmp/cmr-c3-ios-device.log`，Web 验证日志 `/tmp/cmr-c3-web-{tests,build,lint}.log`。临时产物不提交。

## 用户真机验收清单（均待用户结果）

1. Xcode 打开 `apps/mobile/ios/App/App.xcodeproj`，选择自己的签名与 iPhone 运行。扫码并允许相机/本地网络权限；关闭重开 App 后，未过期配对恢复且无需再次扫码。
2. 连续两轮文本；PC/手机输入同步；Markdown、Thinking/Tool；工具审批、拒绝、问答（含多选与自由文本）；取消运行/排队项；会话结束禁用输入。
3. 配对多个 Session，前台分别收消息；切换/返回列表不串线；新建/停止会话（仅支持该能力的 Standalone）；WhaleTerm 关闭 Remote 后 PC Claude 继续运行。
4. VPN 直连、断网/恢复、前后台、杀进程重开、凭证过期/移除/重新扫码；网络断开期间事件没有 replay，重连以 Snapshot 为准。
5. 键盘弹出/收起、旋转、安全区、抽屉、边缘返回与滚动位置；系统减少动态效果。
6. 点击语音，允许权限，说话后点击结束；文本追加到原草稿，可编辑后手动发送。拒绝权限可恢复提示；录音时切会话/返回/锁屏/后台/来电后麦克风指示消失，迟到文字不得进入其他 Session。静默识别 60 秒有界结束。
7. 设置启用通知并授权；列表或另一会话等待任务完成/审批/问答/失败，检查提醒不含正文、目录、凭证；点击进入正确会话；当前详情不重复提醒。关闭通知后不再提示，移除配对后提醒清除。测试通知点击冷启动与失效配对。

## 能力边界与未验证

- 系统提醒基于 App 已收到的实时事件；App 被系统挂起/关闭后没有 WebSocket 常驻保证。未实现 APNs provider、离线事件队列、签名推送凭证或 Relay。**不承诺挂起/杀进程后新事件推送，不能据此宣布离线推送完成。** 本地通知 API 与此边界见 [Apple UserNotifications](https://developer.apple.com/documentation/usernotifications)。
- 语音转文本可能由 Apple 语音服务处理，取决于设备本机识别支持情况；App 不保存音频。
- iOS 真机、安全存储、VPN、实际扫码、识别效果、通知横幅与冷启动点击仍待人工验证。检测到的物理 iPhone 状态 unavailable，本轮未安装到真机。
- C2-D 和 C 阶段人工门禁尚未通过；Android/Harmony 原生同步、构建及设备验证在用户完成 iOS 验收后继续。
