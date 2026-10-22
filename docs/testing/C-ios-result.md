### ios移动端验证结果

> AI 处理记录（2026-10-21）：第 1–4 项已完成代码修复和本机回归，待真机复验；第 5 项已接入用户指定的极光 SDK 与 PC 发送链路，尚待 AppKey/APNs/PC 凭证配置及真实送达验证。原始反馈保留如下，详见 [修复与验收记录](2026-10-21-ios-feedback-result.md)。

#### 问题列表

1. 连接成功后，移动端发送图片，PC端接收到了并且返回了正确的结果，此过程中存在以下几个问题：
   1.1 两端的消息历史里 图片都没有展示出来，PC端展示的是临时路径，移动端展示的是一个名称：
   PC：docs/testing/images/iosMobile/pc_accept_image.png
   ios：docs/testing/images/iosMobile/claude_send_image.png

   1.2 移动端在发送完图片后，连接自动断开了，导致没有接收到后续的消息，即使我点击了右上角重新连接，这部分消息永远丢失了，不知为何

2. 移动端的session会话详情里，有些claude工具的result结果无法展示。请参考`happy UI`, `codex mobile`等等，丰富完善移动端会话详情的展示。比如：优化permission抽屉，高度限制，内容超出在内部滚动；删除底部input输入区域里的tips无用提示；会话进行中时，暂停按钮直接替换发送按钮，input输入区域上方用loading动画展示；等等，目前的UI体验需要优化。
   移动端工具结果：docs/testing/images/iosMobile/claude_tool_result.png

3. 移动端sessionList首页内，三级分级不明显，建议参考iphone的布局，将二级以下用背景色包裹区分，优化布局；另外，PC只展示了ip地址，而不是机器编码：`zhanglingsi的Mac mini` 这种信息获取不到吗？
   移动端session列表：docs/testing/images/iosMobile/claude_session_list.png

4. 移动端"设置"内的"PC管理"也是同样的问题，机器编码没有获取到，只展示了ip：docs/testing/images/iosMobile/pc_manage.png

5. "消息通知"功能验证不通过，我需要的是 系统级别的消息通知：应用在后台运行/退出应用，都能在手机的系统消息通知里推送消息。此功能应该依赖于公网Relay，并且请使用我文档里标注的第三方的服务商。可以先做功能，等待公网Relay建设完毕后，再回头进行此功能测试。docs/testing/images/iosMobile/setting_config.png

#### 问题列表10-22

> 处理记录：按用户指定 1→2→3→5→4 实施；第 5 项补充放大会话行和操作触点。代码修复、开源缓存预研及本机验证见 [10-22 修复记录](2026-10-22-ios-feedback-result.md)，真机复验待用户反馈。

1. whale-term内启动远程管理后，运行了一段时间，此时点击关闭，第一次点击无响应,第二次点击才生效
2. 在whale-term——claude会话列表内，新增一个claude会话，在会话开始之前 启动远程管理，可以正常在手机上连接交互，但是手机右上角的状态标志不对，
   详见：`docs/testing/images/10-22/claude-detail.png`

3. 当两个session对话交叉使用，PC上会出现 已结束的会话仍然在loading的现象。(会话都是手机端发起的)
4. 在手机上进行了若干次会话后，形成了一些历史详情记录，当应用杀死重新进入应用，这些记录全没了，会话列表的工作目录，状态等等也没了。这些可以换存下来吗？
   `docs/testing/images/10-22/claude-storage.png`
   `docs/testing/images/10-22/claude_detail_storage.png`

5. 手机端的 会话列表样式需要优化，现在太拥挤了，分层过于明显，还是平铺展示吧；PC的机器编码展示有问题，空格渲染成了特殊字符；PC详情item内的 "机器编码"不用展示，属于无用信息；
   `docs/testing/images/10-22/claude-session-list.png`
   `docs/testing/images/10-22/pc-detail.png`
