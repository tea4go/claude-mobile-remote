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

### 问题列表2

1. session_id=667df68a-19ff-43cc-90ef-21095ba68e54,在移动端发了两张图片，导致直接报错了：接收消息失败: Failed to decode JSON: JSON message exceeded maximum buffer size of 1048576 bytes…。然后在移动端继续发送消息，PC端接收不到；但是再次在PC上继续会话，移动端可以同步
2. sessionDetail内的"到顶部/到底部"需要优化：docs/testing/images/11-16/scroll_top_down.png，已经将左侧的status明细挤得换行了，看看是否有其他解决方案，参考移动端的优秀案例解决

### 问题列表3

1. 移动端发送图片进行会话后，杀死应用重新进入/PC端刷新whale-term应用后，历史的图片都无法渲染，只展示了文件路径：
   User-provided attachments uploaded to this PC. Read these files using the existing tools and permissions:
   name="IMG_6782.jpg" path="/var/folders/bp/3_z_3trx2xz_7jv6_h634zdm0000gn/T/cmr-attachments/upload-716069571/attachment-1.jpg"
   name="IMG_6785.jpg" path="/var/folders/bp/3_z_3trx2xz_7jv6_h634zdm0000gn/T/cmr-attachments/upload-716069571/attachment-2.jpg"

移动端历史截图：docs/testing/images/11-16/mobile_hiostory_image.png

2. askUserQueston/permission 抽屉面板的右上角目前改成了"收起"，那么 "取消"按钮应当放到 下方按钮区域，取消功能必须要有

3. askUserQueston/permission 抽屉面板收缩后，样式需要优化；"到顶部/到底部"还需要继续优化，不要和status状态放在同一行了，固定在面板右下方(input右上方即可)，注意 滚动到底部后(down图标应当隐藏)，滚动到顶部后(up图标应当隐藏)
   问题截图：docs/testing/images/11-16/askuserquestion.png
