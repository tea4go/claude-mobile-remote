# 移动端 UI及功能变更

### 背景

在 docs/testing/2026-11-19-mobile-design.md以及docs/testing/2026-11-19-mobile-test-result.md的基础上，UI有变更

三端都需要更改，iOS优先，测试通过后同步到android和harmony

### 调整列表

1. 首页配对界面调整，新的设计稿：docs/testing/images/11-20/index.png
   1.1. PC List和按钮组更换位置
   1.2. PC List收起后 贴在界面底部
   1.3. 去除 "全部电脑"右侧的 展开/收起图标
   1.4. PC List Item请严格按照设计稿修改，左侧状态图标更换为：apps/mobile/src/assets/icons/svgs/session/pc-status.svg；Item样式 边框 圆角 左滑的操作按钮组样式 边框 圆角 等等调整；
   1.5. 界面的tip 操作结果面板等 固定在界面最下方，不占PC List的空间(可以重叠),在面板上添加 关闭按钮，可以手动关闭，就不影响操作后面的PC List

2. docs/testing/images/11-20/expand.png,docs/testing/images/11-20/expand1.png 审查下，去除 展开/收起 的图标

### iOS测试结果

1. docs/testing/images/11-20/ios_index.png,首页问题如下：
   1.1. 首页下方的PC List，在高度撑不满的情况下，向下移，底部不要留白
   1.2. 没有PC List时，不要展示此区域，并将按钮组固定到界面下方
   1.3. PC Item左右需要预留padding，或者宽度顶到底，你思索一下，因为目前的选中样式不好看；选中时，左侧不要出现蓝色的线，只需改变背景色即可；
   1.4. PC Item目前垂直方向有点拥挤了，请参照设计稿调整
2. docs/testing/images/11-20/ios_index_error_tip.png,docs/testing/images/11-20/ios_index_test_result.png 首页底部的tip/result结果面板内的关闭按钮 放到左侧内容里，现在分离的状态不好看

### iOS测试结果2

1. docs/testing/images/11-20/pc_list_action.png,docs/testing/images/11-20/index_shrink.png，首页问题如下：
   1.1. PC List Title点击 除了刷新之外 都应该响应 展开/收缩 事件，现在只能点击title文字才触发，请放大响应区域，并遵循docs/testing/2026-11-19-mobile-design.md内的公约"3"
   1.2. 首页现在可以缩放，需要禁止
   1.3. PC Item的上下padding太大了，不要预留上下margin，现在左滑后的action按钮组 上下会有明显的空隙，不要有空隙；按钮组item不需要圆角和左右间距；
   1.4. PC List收缩后，隐藏刷新，给标题行添加一个背景吧，现在展示得不好看

### iOS测试结果3

1. PC Item内中间区域的文本和目录tag上下间隔调大一点点
2. 首页PC List还是按照设计稿展示吧！当最大高度没有撑满时，不要贴底，还是正常追加在按钮组下方即可
3. PC Item的上下padding再调大一点点

### Android 与 HarmonyOS 同步

iOS 真机验收通过。Android 启用与 iOS 共用的原生页面布局，HarmonyOS 用 ArkUI 对齐。两端同步最终版 PC 行上 6、下 8 的内边距和同一台电脑下相邻目录组分割线。验证顺序为平台回归、调试构建及真机体验。

- Android：706 项移动端回归、Web 构建、Capacitor 同步及调试 APK 构建通过。
- HarmonyOS：103 项回归、22 文件逻辑检查及 API 24 未签名调试 HAP 构建通过。
- 两端待真机验收：展开和收起电脑列表、左滑操作、关闭底部提示、会话目录分割线。Android 配对页同时检查双指缩放与正常滚动。

HarmonyOS 最新调试包已通过 USB 覆盖安装并启动，布局和手势待真机验收。

### HarmonyOS测试结果

1. sessionDetail界面问题如下：
   1.1. docs/testing/images/11-20/harmony/askuserquestion.png askuserquestion和permission抽屉面板的 取消按钮 样式不对
   1.2. docs/testing/images/11-20/harmony/bottom_top.png 界面右下角的 滚到顶部/底部 svg不对，背景添加一些box-shadow和iOS保持一致；用户发送的消息面板，当没有占满宽度时，应当以文本的实际宽度为准，而不是现在这样 蓝色的面板都固定宽度，请和iOS保持一致

2. docs/testing/images/11-20/harmony/queue.png 排队引导功能有问题，PC上表现没有问题，手机有问题（iOS/android/harmony均有此问题）
   2.1. 在会话进行中时，手机上再次发送消息会进入queue队列，但是手机上此时不应当渲染成 用户发送的消息
   2.2. 当在PC上点击引导一条会话时，当此会话真正插队成功后，应当在手机上同步渲染此会话到界面内，目前的情况是：此条消息不渲染，但是claude回复正常渲染
   2.3. 插队的过程同步到手机可以弄简单一点，首先会话在queue排队队列内，不要渲染成发送的消息；当消息引导并真正干预成功后，手机上同步渲染此条消息到界面；将排队消息引导出去，但还未干预成功的阶段不用处理

实施顺序：先对齐 HarmonyOS 详情样式并核对现有排队和引导的事件边界，再按宿主确认的执行时机归并消息。回归覆盖普通启动、排队、引导成功和重复事件。

本轮完成 HarmonyOS 详情样式调整及三端排队展示修复。引导只在 WhaleTerm 确认实际处理后通过 v12 下发正文，回复仍归属正在运行的轮次。已覆盖安装 HarmonyOS 调试包并验证启动。真机联调用本轮 Windows WhaleTerm 测试程序，依次检查手机入队、PC 点击引导、引导生效后出现一次用户气泡及回复继续输出。

### HarmonyOS测试结果2

1. docs/testing/images/11-20/harmony/bottom_top2.png sessionDetail内右下角的图标不对，应当是圆形背景，图标的箭头看着像是被截断了

已改为 44 vp 原生圆形按钮，保留阴影，并将箭头和箭杆绘制为同一条 SVG 路径。HarmonyOS 回归、API 24 调试构建、签名、USB 覆盖安装及启动通过。待打开长会话确认两个箭头显示完整，且可分别滚到顶部和底部。

2. 首页 PC 行右侧测试按钮采用 8 vp 圆角，保持 34 × 34 vp。回归、API 24 构建及 USB 覆盖安装和启动通过，真机截图确认图标与圆角背景可见。用户确认本轮真机验收全部通过。
