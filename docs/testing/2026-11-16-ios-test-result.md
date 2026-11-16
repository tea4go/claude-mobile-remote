# 11-06 iOS端测试问题记录

### 问题列表

1. 移动端用户消息 没有换行，需要渲染成markdown,问题截图：docs/testing/images/11-16/claude_user_msg.png
2. 移动端Claude回复里，文本和工具调用是分开渲染的，没有按照它们实际返回的顺序进行渲染，导致文本和对应的工具 对不上，问题截图：docs/testing/images/11-16/claude_response.png，而PC上则正常：docs/testing/images/11-16/pc_claude_response.png
3. 在PC端发送一条 queue排队消息进入队列，然后在移动端将它引导，PC端正常显示此发送的消息，但移动端不显示发送的消息，只有回复的消息
4. 移动端发送图片后，后续的对话 下方一直会出现："附件已传到电脑，Claude将按现有权限读取",详见：docs/testing/images/11-16/claude_file_response.png
5. askuserQuestion和权限 抽屉面板的右上方按钮，建议改为："收起"按钮，用户需要根据上下文做决策，目前点击右上角则直接取消了，影响使用体验
6. 如果已经配对后，执行claude任务中途断开几次，重新连接后，发现手机上同步的内容有缺失（建议配对成功后，同步最近一次的用户消息和claude回复消息）
7. 执行复杂claude长任务，初始化加载历史会有卡顿现象，需要像PC端那样将执行过程默认收起
8. sessionDetail内需要添加 到底部/到顶部的导航菜单，建议和进行中的status loading放在一行，始终在右侧展示
