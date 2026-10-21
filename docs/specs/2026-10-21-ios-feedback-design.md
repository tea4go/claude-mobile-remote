# iOS 人工反馈修复

依据 C-ios-result.md 与 D80，iOS 优先自主实施，用户最终真机验收。

1. 修复 WhaleTerm 将一段回复拆为每 32 字突发事件导致有界订阅溢出的风险；增大单段而不增加无界缓冲。EventStream 按 text_delta 累积当前 Turn 正文，宿主刷新快照不得抹掉同一 Turn 或刚结束 Turn 的正文。重连恢复最新 Turn 文本；不承诺完整离线历史与跨 Turn replay。
2. v4 tool_result 增加可选 output（最多 16 KiB UTF-8）与 truncated 标志；仅抽取 SDK 返回的文本块，不传图片 base64 或未知对象。v2/v3 剥离新增字段；缺省保持旧端表现。三端均可使用字符串/布尔值实现，Harmony 后续同步解析。
3. 本机移动输入保存本次进程内附件预览，不持久消息；WhaleTerm 图片展示需复用已有图片组件与上传缓存，不把 SDK 执行提示当成最终用户输入展示。
4. 抽屉限高，标题固定，正文可收缩并内部滚动；运行中用停止替换发送，输入框上方显示加载状态；删去常驻实现提示。首页按机器、项目背景、会话白卡分层。
5. WhaleTerm 配对补稳定 PC ID 和系统显示名；旧配对需重新扫码以更新缺失元数据。
6. 用户指定极光 JPush：iOS 使用 APNs 通知而非自定义消息；Master Secret 仅服务端。先实现独立提供者与配置契约，设备注册及 Relay 路由需明确鉴权、撤销与通知点击定位，缺配置不得伪称已启用远程推送。

## 实施细化

- 上传后断开的直接归并缺陷：本机 accepted 输入为附件名称，Host turn_started 是含临时路径的执行提示。只对已经受理且 requestId/source 匹配、并携带本机附件元数据的输入允许正文不同，保留用户草稿/预览；普通文本仍严格比较。
- 工具结果扩展使用协议 v4，服务端接受 v2/v3/v4；新手机可降级 v3/v2；v2/v3 不发送 output/truncated，避免旧版本严格解码器断连。三端仍使用 JSON/WebSocket，无平台字段。
- 可信 InputDisplay 从上传校验后沿同一 Submit 队列传入 Host，WhaleTerm Wails 的 turn start 仅将用户说明与已验证的图片 data URL 交现有图片组件展示，SDK 仍执行原路径提示。无需解析用户任意文件路径；手机本次进程内保存自己的附件预览，不落盘。
- 极光使用 JPush 6.2.2 + JCore noidfa 5.5.1，下载依赖由导入脚本复制到被忽略的 Binaries；SPM C 桥接独立于 Capacitor 自动生成包，包含厂商隐私资源。未使用 NotificationServiceExtension。
- 设备 tags = cmr_ + SHA256("cmr-push-v1" + NUL + sessionId + NUL + PairToken) 的前 16 字节十六进制，36 字符，不上传原始 Token。它仅授权接收无正文提醒，不能认证 Remote；配对轮换改变 tag。App 关闭通知调用 turnOffPush，删除配对覆盖 tags，AppKey 未配置时不启动 SDK。
- PC 在独立有界订阅上向极光 REST v3/push 发送 iOS notification，手机退后台/退出不依赖 WebSocket 常驻；Host 仍运行且联网。PC 出站访问极光可先于 Relay 工作，点击通知恢复公网会话仍依赖 Relay/VPN。Master Secret 仅 PC 配置，通知不含正文、路径、工具输出或 PairToken。推送队列满时合并提醒，无持久重试队列，不保证每条消息送达。
- 公共 remote.Config 显式 EnablePush 时才读取本机配置；WhaleTerm/真实 CLI 启用，Fake Host 不读取配置。配置缺失保持关闭，错误配置明确报错。当前不修改 Harmony/Android 原生能力。
- 最新 Turn snapshot 正文有 1 MiB 上限，超限有显式提示；不建设跨轮次离线历史。

官方依据：[iOS 集成指南](https://docs.jiguang.cn/jpush/client/iOS/ios_guide_new)、[创建推送 API](https://docs.jiguang.cn/jpush/server/push/rest_api_v3_push)。
