# Android 同步 iOS 验证记录（2026-11-10）

- 范围：复用 iOS 1.0(9) 的共享界面和功能，包括 WhaleTerm 双向队列、电脑选择、近期详情与长按复制修复；核对 Android 现有原生桥接和系统配置。通知推送维持既有暂停范围，旧会话缓存问题暂缓。
- 核查：Android 已实现长按系统触觉、文本及调试日志复制、语音和缓存桥接；扫码、附件和系统返回使用既有实现。本轮未发现可据此确认的原生代码缺口，未改共享协议。
- 资源：复用 11 月 9 日 iOS 构建的 dist，执行 `cap copy android`；APK 内全部 11 个 dist 文件逐项比较一致，iOS 归档 index 与 dist 摘要一致。源码文件时间均早于 dist index；本轮没有重新生成 Web 构建。
- 构建：离线 `./gradlew --offline assembleDebug` 成功，存在 Gradle/SDK 兼容提示；Debug APK 为 `apps/mobile/android/app/build/outputs/apk/debug/ClaudeMobileRemote.apk`，SHA-256 `aadaf9c917c7c98f3107fdce59a05b3102f421afb9a96629d4ff2c1111955e7a`。首次因沙箱限制 Gradle 缓存访问失败，授权后通过。
- 未验证：按用户要求未运行 npm lint/build/test；ADB 在沙箱内无法启动，没有模拟器或真机结果。请在 Android 上验收队列双向操作和断线恢复、电脑选择持久化、长按复制及触觉、扫码/附件/语音、键盘和系统返回。构建通过不代表设备验收通过；Debug 包不用于正式发布。
