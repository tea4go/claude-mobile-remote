# C3 iOS 语音与系统通知

用户授权直接实施，本机验证后统一真机验收；Android/Harmony 原生同步延后。C2 真机门禁待人工验收，C3 实施并行准备不等于 C2/C 阶段门禁通过。

## 语音

iOS 自定义 Capacitor 插件使用 Speech/AVAudioEngine，首次点击才请求语音识别与麦克风权限。默认中文，优先本机识别；设备不支持时遵循 Apple 系统识别服务（可能需要网络），启动前界面提示。音频不存文件、不进 Remote 协议或日志。识别至多 60 秒，点击停止可提前结束，切页、后台、禁用或卸载取消并释放麦克风。结果追加至当前输入草稿，可编辑后手动发送；禁止自动提交或覆盖已有草稿。权限拒绝/不可用/中断显示固定错误，不能输出原始插件错误。

## 通知

iOS UserNotifications 本地通知，由已鉴权且已订阅的实时领域事件触发：审批/问答、Turn 完成/失败、Session 失败/停止。Snapshot 和重连不补造通知；按 Session + seq 去重。默认关闭，通过设置显式启用并请求系统授权，偏好仅保存布尔值。当前前台详情正在查看的 Session 不提醒；其他 Session 或后台仍收到的事件提醒。系统提醒只含固定文案和 Session ID 定位，不含 token、URL、目录、标题、正文或错误内容。点击后仅打开仍存在的本地配对；失效配对走重新配对，不自动执行命令。移除配对清理关联通知。

这是在线事件系统提醒；系统挂起/杀进程后没有 WebSocket 接收保证，无 APNs provider、推送凭证或离线事件服务，不能保证新事件离线送达。离线推送仍是后续独立设计，验收表明确该缺口。

## 契约与平台

不修改协议、Snapshot、Pairing 或 Session 状态。识别仅产出普通文本，经已有串行输入链路发送。通知使用已有领域事件，平台端口只含固定提醒分类、sessionId、seq。Android 可用 SpeechRecognizer/NotificationManager，Harmony 可用系统语音与 notificationManager 等价实现；本轮不接入其他平台。浏览器保持无持久凭证，不调用原生端口。

## 验证

测试语音草稿、停止/取消/切页隔离、失败及无自动发送；通知分类、seq 去重、Snapshot 不通知、当前会话抑制、设置授权与点击定位。Web 全量测试/lint/build、cap sync ios、Xcode Simulator 和 device 无签名编译；真机单独验证权限、麦克风释放、前后台、VPN、安全存储和通知点击。

依据：[Apple Speech](https://developer.apple.com/documentation/speech/sfspeechrecognizer)、[Apple UserNotifications](https://developer.apple.com/documentation/usernotifications)、[Capacitor 原生插件](https://capacitorjs.com/docs/ios/custom-code)。
