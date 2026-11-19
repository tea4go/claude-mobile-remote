# iOS 11月19日测试问题记录

### 问题记录

1. iOS/android/harmony的首页和设置页面，都需要记录 版本号和构建时间，版本号为3位 (10进位），例如：1.0.9 下个版本号为1.1.0，构建时间：2026-10-20 14:30。
   首页可以放在标题位置；设置界面内可以当成 版本更新说明放在最下方

2. 配对界面需要调整：
   2.1. PC List标题请按照设计稿调整，设计稿：docs/testing/images/11-19/connect-paring.png，加分割线、字体调整
   2.2. 进入配对界面后，PC List状态没有自动刷新，需要手动刷新后，才能看到哪些已连接，哪些未连接
   2.3. PC Item样式需要调整：docs/testing/images/11-19/pc-item.png，上下预留的空间需要小一点，并且需要一样
   2.4. 配对界面/编辑界面内的 tip提示语/操作结果，文字尽量不要换行，文字调小处理：docs/testing/images/11-19/connect-paring-tip.png

3. 去除首页内session List 目录右侧的 plus；首页上方的PC连接状态需要体现在 它的图标里

4. 首次配对的session会话，需要同步最近一次的 用户 发送/claude回复消息，如果PC上没有，那就没有。此需求之前提了的，目前看是没有效果的，首次配对进入是空的

### iOS 修复与验证

- 按补充设计稿在首页和设置页固定显示版本号和构建时间，首页信息位于「新会话」按钮下方。首页 Windows 路径只显示末级目录，完整路径用于分组。
- 配对页进页自动刷新 PC，批量加载仅在标题刷新按钮展示。调整标题分割线、列表留白和配对／编辑提示字号。首页移除目录加号，PC 图标按状态着色。
- 更新 WhaleTerm 和 Remote 后，iOS 首次配对可展示目标会话最近一轮的用户提问和 Claude 正文，空会话保持空白。自动化覆盖首次快照、去重、旧协议隔离、Windows 目录和底栏位置。
- 681 项移动端测试、定向 Go 回归和 iOS 1.0.9 模拟器构建、安装启动通过。WhaleTerm Windows amd64 调试程序已构建，真机双端联调待验收。
- 真机验收检查首页与设置固定栏、进配对页自动刷新、Windows 末级目录、已有正文及空会话首次配对。iOS 验收通过后同步 Android 和 HarmonyOS。

### 问题记录2

1. docs/testing/images/11-19/connect-paring2.png，配对界面里，PC List的标题距离下方列表的空隙太大了，调小
2. 去除设置界面底部的版本说明吧，只需要保留首页的即可
3. 首页继续优化：
   3.1. session item的右侧操作，请改成和配对界面的PC Item操作一致，左滑：重命名和删除，用图标表示 docs/testing/images/11-19/session-item-action.png
   3.2. session item的重命名操作有问题：docs/testing/images/11-19/session-item-rename.png，点击重命名后，弹出对话框修改即可，需要限制下长度

### 问题记录2 验证

- iOS 已缩小 PC 列表标题间距，设置页移除版本栏，首页会话改为左滑图标操作。重命名弹窗限 40 字符且拒绝空名，删除须确认。
- 682 项移动端测试、iOS 1.0.9 模拟器构建及安装启动通过。真机验收检查列表间距、左滑与弹窗，验收后同步 Android 和 HarmonyOS。

### 问题记录3

1. 首页和配对界面的左滑操作，展示的action和左侧的item衔接不流畅，action左侧没有圆角，item右侧有圆角，导致看起来不是一体的，不好看：docs/testing/images/11-19/session-item-action-new.png,docs/testing/images/11-19/pc-item-action.png
2. 首页的session-item rename dialog有一些问题：docs/testing/images/11-19/session-item-rename-dialog.png
   2.1. dialog弹出框请和配对界面的 验证码弹出框 样式保持一致：docs/testing/images/11-19/connect-verify-dialog.png
   2.2. dialog出现时，默认全选当前的输入框内的内容，方便删除
   2.3. 当点击 取消/保存时，dialog会先移动到界面中间的区域，然后再消失，我怀疑是 键盘消失导致的，若暂时无法优化，可以先放着不处理
3. 配对界面的PC List标题 上下间隔需要保持一致：docs/testing/images/11-19/pc-list-title.png,目前是距离上方的分割线太小了

### 问题记录3 验证

- iOS 两处左滑卡片只在外框保留圆角，配对页 PC 标题与分割线、首张卡片等距。重命名沿用验证码弹窗样式，打开全选名称，关闭时直接卸载弹窗。
- 682 项测试、iOS 1.0.9 模拟器构建及覆盖安装启动通过。圆角接缝、实际键盘弹出和关闭过程待真机验收。

### 问题记录4

1. 配对界面内的PC Item左侧的图标，保持和右侧内容顶部对齐即可

### 问题记录4 验证

- iOS 配对页 PC Item 左侧图标已与右侧内容顶部对齐。682 项测试、iOS 1.0.9 模拟器构建及覆盖安装启动通过。

### Android 和 HarmonyOS 同步验证

- 用户确认 iOS 四轮反馈全部验收通过，随后同步 Android 和 HarmonyOS。两端版本信息固定在首页底部，设置页无版本栏；同步 PC 状态色、末级目录、配对页自动探测与单项／全量加载反馈、卡片圆角与左滑编辑，以及限 40 字符的重命名弹窗。Harmony 由 Hvigor 注入构建时间。
- Android 共用测试 683 项、原生单测 9 项、Web 构建、Capacitor 同步、APK 资源和签名核验通过。1.0.5（6）Debug APK 位于 `apps/mobile/android/app/build/outputs/apk/debug/ClaudeMobileRemote.apk`，使用默认调试配置（华为渠道推送关闭）。
- HarmonyOS 98 项测试、22 个逻辑文件、API 24 Debug HAP 构建、签名与 ZIP 核验通过。1.0.8（1000008）开发签名 HAP 位于 `.local-tools/mobile-1119/ccBuddy-harmony-parity-debug-signed.hap`。
- 两端均启用 D131 v11。自动化验证最近一轮正文及 Harmony 的重复快照去重、v10 和旧版回退；真机待复验扫码与六位码配对、Windows 目录、首次配对正文与空会话、批量刷新、左滑及软键盘。

### harmony问题记录

测试harmonyOS发现和iOS有一些差异，请参考docs/testing/images/11-19/ios 相同的图片名截图 进行修改

1. 首页截图：docs/testing/images/11-19/harmony/index.png，问题如下：
   1.1. 右上角 "设置" 图标缺失
   1.2. PC展示区域左侧的图标 样式和iOS有差别
   1.3. 选中的session Item 缺少左侧边的 蓝色条，右侧缺少标记"刚刚查看"
   1.4. windows PC的目录下，没有展示出完整的目录，mac PC没问题.（iOS/android/harmony均有此问题）
   1.5. 首页的PC状态，需要调整，参考设计稿：docs/testing/images/11-19/session_index.png。ip和连接状态靠右展示；连接状态用tag区分；（iOS/android/harmony均有此问题）

2. 配对界面截图：docs/testing/images/11-19/harmony/connect-paring.png，问题如下：
   2.1. 配对界面的标题 "连接Claude会话"距离顶部应当和 首页保持一致，首页的空隙太大了，以配对界面的为准.（iOS/android/harmony均有此问题）
   2.2. 右上角 "设置" 图标缺失,样式应当与首页保持一致
   2.3. PC List上方的分割线 上下空隙保持一致，现在距离上方太挤了；PC List标题 上下空隙保持一致，目前距离PC List太远了；
   2.4. PC Item左侧图标太小；选中PC Item不会跳转到首页；选中PC Item后，左侧的图标不应当有背景

3. 首页 重命名会话截图：docs/testing/images/11-19/harmony/rename-session.png，问题如下：
   3.1. 限制提示没有，请和iOS保持一致
   3.2. 取消和保存 样式应当区分

4. session detail截图：docs/testing/images/11-19/harmony/session_detail.png，问题如下：
   4.1. 用户发送的消息，文字颜色不对，harmony端需要注意 亮色/暗色模式的兼容

5. 设置截图：docs/testing/images/11-19/harmony/settings.png，问题如下：
   5.1. 图标请和iOS保持一致，没有背景色
   5.2. 调试和本地保存 模块 应当有小标题，有分割线；全局消息提醒，由于系统有差异，可以不用和iOS保持一致

### Harmony 真机反馈修复验证

- 三端首页保留末级目录标题并恢复完整路径副标题，完整路径继续作为分组依据。PC 卡片右侧显示 IP 和状态标签，首页标题上方间距以配对页为准。
- Harmony 首页和配对页使用同一个设置入口。最近查看的会话增加蓝色侧条和文字。配对页调整分割间距及 PC 图标，选中电脑后返回对应的会话列表。重命名显示 40 字符提示，取消和保存分别使用描边和实心样式。用户气泡使用白色正文，保留 Markdown 和文本选择。设置页补齐图标、分组标题和分隔线。
- 共用 683 项、Harmony 98 项测试、Web 构建、Android Debug、iOS 模拟器和 Harmony API 24 构建通过。Harmony 1.0.8 调试包已通过 USB 覆盖安装并启动，具体交互及亮暗模式视觉待真机验收。

### harmony问题记录2

1. 首页截图：docs/testing/images/11-19/harmony/active-session.png，被选中的session，高度变得非常高;PC展示项的上下padding太大了，调小一点；PC展示项内的token，"..."空隙为何这么大？
2. 设置截图：docs/testing/images/11-19/harmony/settings_new.png，item项高度太高了；图标太小了；
3. 配对界面截图：docs/testing/images/11-19/harmony/connect-paring-new.png，IP输入框上下的间距太小了，调大一点；分割线的上下间隙还是不一致，应当距离上方的tip大一点；
4. 配对界面的底部 也需要 构建号和构建时间，和首页一致（iOS/android/harmony均有此问题）
5. 设置界面，添加一项 "关于" 把当前版本和构建时间加上去（iOS/android/harmony均有此问题）

#### 第二轮修复验证

- 选中会话使用固定 26 vp 的蓝色标记；PC 卡片减少上下留白，Token 以单个省略号和原有末四位展示。设置收紧卡片间距与内边距，设置图标去静态留白并放大。配对 IP 框增加上下留白，分割线与上方提示的距离大于下方。
- 三端首页与配对页固定底栏及设置「关于」均展示版本、构建号和构建时间。iOS/Android 自动化验证三个位置的内容一致。
- 共用 683 项、Harmony 98 项及 22 文件逻辑检查、Web 构建、Android Debug 构建与单测、iOS 模拟器构建、Harmony API 24 构建和签名包 ZIP 检查均通过。Harmony 1.0.8 调试包经 USB 覆盖安装并启动。真机布局和交互待验。

### harmony问题记录3

1. 设置截图：docs/testing/images/11-19/harmony/settings3.png，设置内的图标有问题，如果不能和iOS保持一致的话，可以用回原来的图标
2. iOS内的设置界面，最下方的关于 通知edo服务的提示 请去除。只需要展示 本机远程通知是否就绪即可。
3. debug图标，默认显示在右下角，并且可拖动（iOS/android/harmony均有此问题）
4. harmony内的debug入口有问题：docs/testing/images/11-19/harmony/index-debug.png，应当是圆形背景
5. harmony debug面板有问题：docs/testing/images/11-19/harmony/debug.png，请和iOS端保持一致

#### 第三轮修复验证

- Harmony 设置图标使用明确的蓝色描边；调试入口使用圆形虫形图标，调试弹窗居中，标题显示条数、下方是清空／复制／导出工具栏及可滚动日志。
- 三端调试入口默认位于右下角并限制在可视区域内拖动。iOS 移除 edo 登记说明，保留本机远程通知状态与授权操作。
- 共用 684 项、Harmony 98 项及 22 文件逻辑检查、Android Debug 构建与单测、iOS 模拟器构建、Harmony API 24 构建及签名包 ZIP 检查通过。Harmony 1.0.8 经 USB 覆盖安装并启动。拖动与视觉待真机复验。

### harmony问题记录4

1. debug默认展示在 按钮组的上方，界面右下角的位置，现在过于靠下了；debug改为 浅色背景，深色图标（iOS/android/harmony均有此问题）
2. harmony打开debug面板，面板里无法响应 点击和滚动，不知为何

#### 第四轮修复验证

- 三端默认将浅色调试按钮放在页面右下方操作区上方，保留手动拖动。Harmony 面板切换命中模式，按钮和滚动区可接收触摸。
- 共用 684 项、Harmony 98 项及 22 文件逻辑检查、Web、Android Debug 构建与单测、iOS 模拟器构建、Harmony API 24 构建、签名包 ZIP 检查均通过。
- Harmony 1.0.8 调试包经 USB 覆盖安装并启动。设备 UI 检查确认首页入口在「新会话」按钮上方，滑动后可见日志发生变化，点击「关闭」后面板消失。iOS 和 Android 真机布局待验。

### harmony问题记录5

1. debug拖动后，需要缓存位置，目前是每次进入应用都重置了（iOS/android/harmony均有此问题）

#### 第五轮修复验证

- 三端拖动完成后保存调试入口坐标。启动时恢复有效坐标并限制在可视区域。无有效记录时使用默认位置。
- 共用 686 项（含 iOS 和 Android 拖动保存及重挂载恢复）、Harmony 98 项及 22 文件逻辑检查、Web、Android Debug 构建与单测、iOS 模拟器及 Harmony API 24 构建、签名包 ZIP 检查均通过。
- Harmony 1.0.8 经 USB 覆盖安装。真机拖动后强制结束应用进程并重启，调试入口恢复到相同坐标。iOS 和 Android 真机待验。
- 用户确认本轮验收通过，未单独说明 iOS 与 Android 的真机复验结果。
