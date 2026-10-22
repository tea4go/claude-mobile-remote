# Android 同步已验收 iOS 功能

用户确认 iOS 验证完毕，要求先提交 iOS，再全部同步 Android 后单独提交；WhaleTerm 本轮不提交、不修改。首个提交为 5e4a0a7。

共用 React UI、v4/v3/v2 协议、附件上传/图片预览、问答审批、取消、队列、多 Session、PC 元数据/平铺列表及状态修复随 cap sync android 复用。补 Android 原生 SessionExperience 插件实现当前 iOS 全部有效能力：

- SpeechRecognizer 生成可编辑文字草稿，用户显式发送；RECORD_AUDIO 运行时权限，单录音、超时、停止、取消、退后台/销毁清理；服务缺失或授权拒绝明确失败，不伪造成功。
- NotificationChannel 与 Android 13+ POST_NOTIFICATIONS 权限；默认关闭，允许审批/问题/完成/失败/停止本地提醒；通知正文不包含会话内容，每 Session 只保留一条。点击使用不可变 PendingIntent，将 sessionId 交给共用 notificationOpened 事件；支持冷启动保留与热启动，前台正在查看的 Session 不提醒。
- 私有 noBackupFilesDir 内每 Session 一个 SHA256 命名 JSON；串行 I/O、原子替换，复用 200 项/2 MiB/20 MiB/30 天限制及恢复规则，不引入数据库。沿用 TypeScript 白名单投影和系统安全配对；关闭系统备份避免敏感本地数据迁移。
- 本地插件注册在 MainActivity；现有 Capacitor 扫码、安全存储、Preferences、App 生命周期与 Keyboard 原生依赖保留。检查相机/相册/文件选择的 WebChromeClient 路径和 Android Manifest 合并。
- 通知与缓存桥接按原生平台选择，浏览器仍为空端口；iOS 插件实现保持不变。极光离线推送按当前 iOS 暂停状态同步，不接入新的推送商、Relay 或历史协议。Harmony 保持后续独立实现。

不修改 wire contract/配对字段，Android/Harmony 都可按同一权限和文件语义实现。官方 API 依据：[Capacitor Android 插件](https://capacitorjs.com/docs/plugins/android)、[SpeechRecognizer](https://developer.android.com/reference/android/speech/SpeechRecognizer)、[通知权限](https://developer.android.com/develop/ui/views/notifications/notification-permission)。

通过条件：共用 Web 全套测试/build/lint，文件缓存原生单元测试、Android debug APK 构建；模拟器可用时验证插件/缓存/通知与拍照选择器。实际语音识别、VPN、厂商后台策略及物理手机相机由用户真机验收，不能以构建通过替代。
