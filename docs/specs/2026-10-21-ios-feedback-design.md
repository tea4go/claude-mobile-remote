# iOS 人工反馈修复

> 2026-10-27 替代说明：下文 PC 直连极光、PC 持有 Master Secret、点击后依赖 Relay 的推送路径是 D82 当时的实现方案；D100 已取消 Relay，并将后续离线通知改为 WhaleTerm→edo HTTPS→极光等平台。当前 iOS 接收端仍按 D83 暂停，E3 需另做专项设计和真机验收；下文历史方案不作为新的实施计划。

依据 C-ios-result.md 与 D80，iOS 优先自主实施，用户最终真机验收。

1. 修复 WhaleTerm 将一段回复拆为每 32 字突发事件导致有界订阅溢出的风险；增大单段而不增加无界缓冲。EventStream 按 text_delta 累积当前 Turn 正文，宿主刷新快照不得抹掉同一 Turn 或刚结束 Turn 的正文。重连恢复最新 Turn 文本；不承诺完整离线历史与跨 Turn replay。
2. v4 tool_result 增加可选 output（最多 16 KiB UTF-8）与 truncated 标志；仅抽取 SDK 返回的文本块，不传图片 base64 或未知对象。v2/v3 剥离新增字段；缺省保持旧端表现。三端均可使用字符串/布尔值实现，Harmony 后续同步解析。
3. 本机移动输入保存本次进程内附件预览，不持久消息；WhaleTerm 图片展示需复用已有图片组件与上传缓存，不把 SDK 执行提示当成最终用户输入展示。
4. 抽屉限高，标题固定，正文可收缩并内部滚动；运行中用停止替换发送，输入框上方显示加载状态；删去常驻实现提示。首页按机器、项目背景、会话白卡分层。
5. WhaleTerm 配对补稳定 PC ID 和系统显示名；旧配对需重新扫码以更新缺失元数据。
6. 用户指定极光 JPush：iOS 使用 APNs 通知而非自定义消息；Master Secret 仅服务端。先实现独立提供者与配置契约，设备注册及 Relay 路由需明确鉴权、撤销与通知点击定位，缺配置不得伪称已启用远程推送。

## 实施细化

- 会话列表导航栏随安全区固定，内容使用独立滚动区域并保留返回位置；用户输入和 Claude 回复提供正文复制入口，显式反馈成功或失败。iOS/Android 使用各自系统剪贴板，浏览器使用 Clipboard API；复制仅由用户主动点击触发。iOS 外层 WebView 已配置禁滚动和回弹，真机需验证。

- 2026-11-02 反馈：列表状态移至标题行右侧；输入法组合期间 Enter 不发送。问答抽屉文本允许原生选择，iOS 在 Info.plist 声明系统常见本地化并允许混合本地化，系统菜单由 iOS 按设备偏好选择，其他语言回退开发语言；需真机复验。待发送附件使用缩略图及右上角移除按钮，图片点按打开 antd-mobile 现有 ImageViewer，非图片显示文件名；沿用原上传与草稿保留逻辑。设置页统一使用列表行及通知开关，缓存清除反馈放入同一说明区；通知权限拒绝时保持关闭。运行中保留停止按钮，草稿非空时同时显示发送到既有 FIFO 队列的按钮；队列沿用已存在的列表和逐项取消；运行中本机发送被受理后按 requestId 暂列队列，开始执行或取消时移除，重连快照以宿主为准，不虚构引导能力。WhaleTerm 配对快照复用现有 sessionTitle 字段和 PC 的轻量元数据读取，按自定义标题、AI 标题、首条输入优先级提供标题并使用既有限长；空草稿不伪造标题。PC 对话收尾按现有消息 UUID 归并末条显示记录，重复收尾不追加第二份；不同 UUID 的相邻消息保留。手机权限切换成功后通过 WhaleTerm 本机事件通知应用级监听，按会话和目录复用现有项目权限存储；事件只含身份与模式，不改变共享协议。双端队列引导须先定位现有数据链路，若需要更改共享契约则先进行三端核对。

- 2026-10-30 功能图标交互修复：输入栏按钮与附件菜单中的装饰图片禁止系统长按图片菜单、选择与拖动命中，点击由原按钮/菜单承接；消息图片、附件缩略图和正文保留既有交互。不改变三端协议或状态模型。

- 上传后断开的直接归并缺陷：本机 accepted 输入为附件名称，Host turn_started 是含临时路径的执行提示。只对已经受理且 requestId/source 匹配、并携带本机附件元数据的输入允许正文不同，保留用户草稿/预览；普通文本仍严格比较。
- 工具结果扩展使用协议 v4，服务端接受 v2/v3/v4；新手机可降级 v3/v2；v2/v3 不发送 output/truncated，避免旧版本严格解码器断连。三端仍使用 JSON/WebSocket，无平台字段。
- 可信 InputDisplay 从上传校验后沿同一 Submit 队列传入 Host，WhaleTerm Wails 的 turn start 仅将用户说明与已验证的图片 data URL 交现有图片组件展示，SDK 仍执行原路径提示。无需解析用户任意文件路径；手机本次进程内保存自己的附件预览，不落盘。
- 极光使用 JPush 6.2.2 + JCore noidfa 5.5.1，下载依赖由导入脚本复制到被忽略的 Binaries；SPM C 桥接独立于 Capacitor 自动生成包，包含厂商隐私资源。未使用 NotificationServiceExtension。
- 设备 tags = cmr_ + SHA256("cmr-push-v1" + NUL + sessionId + NUL + PairToken) 的前 16 字节十六进制，36 字符，不上传原始 Token。它仅授权接收无正文提醒，不能认证 Remote；配对轮换改变 tag。App 关闭通知调用 turnOffPush，删除配对覆盖 tags，AppKey 未配置时不启动 SDK。
- PC 在独立有界订阅上向极光 REST v3/push 发送 iOS notification，手机退后台/退出不依赖 WebSocket 常驻；Host 仍运行且联网。PC 出站访问极光可先于 Relay 工作，点击通知恢复公网会话仍依赖 Relay/VPN。Master Secret 仅 PC 配置，通知不含正文、路径、工具输出或 PairToken。推送队列满时合并提醒，无持久重试队列，不保证每条消息送达。
- 公共 remote.Config 显式 EnablePush 时才读取本机配置；WhaleTerm/真实 CLI 启用，Fake Host 不读取配置。配置缺失保持关闭，错误配置明确报错。当前不修改 Harmony/Android 原生能力。
- 最新 Turn snapshot 正文有 1 MiB 上限，超限有显式提示；不建设跨轮次离线历史。

官方依据：[iOS 集成指南](https://docs.jiguang.cn/jpush/client/iOS/ios_guide_new)、[创建推送 API](https://docs.jiguang.cn/jpush/server/push/rest_api_v3_push)。
