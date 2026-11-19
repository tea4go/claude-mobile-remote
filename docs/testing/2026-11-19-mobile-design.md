# 移动端 UI及功能变更

移动端界面需求调整，集中在两个界面，：

1. sessionList：docs/testing/images/11-19-1/sessionList.png
2. 配对界面：docs/testing/images/11-19-1/connect-paring.png

调整点为15个，我会逐一列出：docs/testing/images/11-19-1/ui-change.png

以下为整体的公约：

1. 以配对界面为首页，默认进入应用后，打开的是配对界面(除了从系统推送消息 精准进入对应的session页面外),以下会简称为"首页"
2. 首页的右上角 展示 设置图标，用于进入设置界面；其余页面右上角 展示 home图标，用于进入首页
3. 所有的图标按钮 交互状态保持一致：未点击时，只展示图标；点击状态，展示圆角背景色
4. 所有的token，密码相关的展示规则：最前面展示5位，最后展示4位，中间显示"***",比如：txH6t***65gh
5. 所有的完整路径展示规则调整：若一行能展示，则全部展示；若一行无法展示，则保留 首尾路径，中间显示"...";兼容windows，mac，linux路径，比如：/Users/.../test-connect
6. 只有首页底部需要显示 构建版本及时间，其余界面底部均不用展示
7. iOS优先，测试完毕后，同步到android，harmony

### 调整列表

1. 公约"2" "3"，除了首页以外的所有界面右上角，展示home图标，home图标请使用：apps/mobile/src/assets/icons/svgs/home.svg
2. sessionList内，目录的展开/折叠图标 请使用：apps/mobile/src/assets/icons/svgs/expand.svg，apps/mobile/src/assets/icons/svgs/collapse.svg
3. sessionList内，目录全路径的展示规则需要遵循 公约"5"
4. sessionList内，sessionItem调整：
   4.1. 由原来的3行改为2行，每行两端对齐
   第一行：连接状态用 圆点 展示，放在标题左侧；标题右侧展示时间；
   第二行：右侧展示当前session会话的状态，此功能可能需要同步whale-term内的frontend/src/layout/components/content/components/leftPane/components/chat/components/LocalClaudeList.vue session status，涉及到了修改 whale-term以及remote cli服务。
5. 就是 4.2，状态图标请使用：apps/mobile/src/assets/icons/svgs/status，不再重复赘述
6. 首页标题栏左侧 展示应用logo图标
7. 首页和sessionList页面 的标题 ccBuddy 加粗加大显示，参考设计稿
8. 公约"2" "3",首页右上角展示 设置图标
9. 首页 PC List标题调整，加粗加大显示，参考设计稿
10. 首页 PC Item左侧 图标调大，保持和右侧内容区域上下高度一致,图标请使用apps/mobile/src/assets/icons/svgs/session/pc-status.svg；右侧内容控制在2行显示，第2行的 目录 tags请一行显示，不要折行，显示不下 展示"..."
11. debug日志入口默认悬浮在右下角，可拖动(已实现，此条作废)
12. 公约"6"
13. 公约"4"，sessionList PC展示内的token 展示规则调整
14. sessionList 目录右侧的 plus功能，需要放开。(之前有被删除了)。点击plus的交互请遵循 公约"3"
15. 首页 PC Item内的目录名tag内，不要展示"/"(我看了目前就是只有目录名，所以此条作废)

### 测试结果

1. 由于首页已变成了 配对界面，所以原本的 从配对界面点击PC Item进入sessionList 以及 从sessionList进入配对界面 切换动画，返回动画，右滑动画，需要调整，请审查并修改
2. 从sessionList界面 点击下方的按钮，点击目录右侧的plus 进入 首页配对界面后，左上角应当变为：返回，并且需要响应右滑返回；其他情况下 首页左上角展示应用图标
3. 首页PC Item左侧的状态图标还是换成原来的吧，新的不好看
4. 首页PC Item的选中active状态，不明显，看不出来
5. 首页PC Item右侧的测试图标，不用遵循公约"3"的交互，它默认就应该有背景
6. 首页PC List 标题，点击后应当收缩/展开 PC List，而不是跳转到session List
7. 首页PC Item中间第2行的目录tag 展示不下截断规则应该是：tag内直接截断，不用展示"...";在最后一条tag外，追加"..."
8. sessionList内 点击顶部的"会话列表" 收缩/展开 下方的所有目录
9. sessionList内 目录为一个单位的区块，有个背景色，此背景色不需要
10. 首页的右上角 "设置"图标后面需要追加文本 "设置"
11. 从session List的目录 plus进入 首页配对界面，会在 底部出现提示，目前提示里只有 目标目录，缺少目标PC

### 测试结果2

1. 首页的自动刷新PC List状态功能，只需要在初始化应用进入首页时执行一遍即可，后续都手动刷新。现在是每次切换到首页都会触发刷新
2. 首页的 "查看全部会话"删除，因为现在点击了它，sessionList界面上方的PC展示不对
3. 首页底部的版本信息展示有问题：docs/testing/images/11-19-1/index_app_tip.png,构建号后面为何多了一个"1"

### 测试结果3

1. 首页的 刷新 放大，现在太小了
2. 首页的 PC Item内的目录tag排列应当从近到远，顺序和sessList内的目录顺序保持一致
3. 首页PC Item内的目录tag截断改完后很奇怪：docs/testing/images/11-19-1/index-paring.png，最后的"..."换行了；tag内容截断了，但是内部左右padding需要保持一致

### harmonyOS测试结果

1. 公约"3"没有遵守，首页PC List的刷新，sessionList的目录右侧的plus，所有界面右上角的 图标， 手指按住的背景应当有圆角
2. 首页PC Item右侧的 测试图标多了一层背景
3. 首页PC Item的选中状态 应当和iOS保持一致
4. 首页PC Item左侧的图标 高度应当和中间区域的内容一致
5. 首页PC List的标题 样式不对
6. 所有界面右上角的 home图标不对
7. session List界面内的item不对：连接状态应当在第一行标题左侧；第一行右侧的 时间丢失了；请和iOS保持一致

#### 修复与复验记录

- HarmonyOS 已对齐圆角按压反馈、PC 标题和选中样式、左侧图标高度、测试按钮单层底色及 Home 图标。会话首行左侧展示连接圆点、右侧展示最近活动时间，次行右侧展示轮次状态图标。
- Harmony 100 项自动化测试及 22 个逻辑文件检查通过。API 24 Debug HAP 编译、调试签名与包完整性检查通过。通过 USB 覆盖安装 1.0.8 并启动，界面树确认配对首页、扫码按钮、PC 列表标题和底部版本栏。按压视觉、PC 行样式及会话卡片布局待真机体验复验。

### harmonyOS测试结果2

1. docs/testing/images/11-19/harmony/harmony-test1.png,首页配对界面有问题：
   1.1. 全部电脑 手指按住它的背景应当有圆角，请遵循公约"3"
   1.2. PC Item左侧的图标还是不对，高度应当和中间的整块内容保持一致；右侧的 测试图标 太大了
   1.3. 下方 "扫描二维码"前面应当有图标

2. docs/testing/images/11-19/harmony/harmony-test2.png，sessionList界面有问题：
   2.1. 目录的展开/收缩 图标有变更，请参考iOS
   2.2. 目录右侧的plus图标 稍微放大一点

3. 首页和sessionList顶部的标题文字 应当加粗加大 和iOS保持一致
4. 右上角的home图标太大了，线条太寸了，请和 设置的图标保持一致

#### 第二轮修复与复验记录

- 首页「全部电脑」补齐圆角按压底色，PC 图标裁掉画布留白并按中间内容高度显示，缩小测试图标，扫码按钮补上二维码图标。会话列表换用 iOS 目录开合图标并放大加号。两页 ccBuddy 标题改为 24 fp 加粗，Home 图标缩到与设置图标相同的 15 vp，并调细线条。
- 100 项 Harmony 回归、22 个逻辑文件检查、API 24 Debug 构建和签名包完整性检查通过。1.0.8 通过 USB 覆盖安装并启动。真机界面树确认配对首页、已保存电脑区域、扫码按钮和版本栏，按压及各图标视觉待体验复验。

### harmonyOS测试结果3

1. docs/testing/images/11-19/harmony/harmony-test3.png，首页PC Item左侧的图标太大了，明显高度已经超出了中间内容高度了
2. 初始化进入首页，自动刷新PC List状态，在还未得到结果前，PC Item左侧的图标应当是黄色警告状态 交互和iOS保持一致
3. 当用户手动刷新 sessionList界面时，在还未得到结果前，上方PC右侧的状态 应当是：连接中 交互和iOS保持一致

#### 第三轮修复与复验记录

- 首页缩小 PC 图标可见轮廓，两行卡片图标高度由 44 vp 改为 36 vp。自动刷新时，各待检测 PC 显示黄色并逐台更新。会话列表手动刷新时，PC 徽标显示「连接中」，等到已有探活收到响应或重连成功、失败后恢复实际状态。
- 101 项 Harmony 回归及 22 个逻辑文件检查、API 24 Debug 构建和签名包完整性检查通过。已通过 USB 覆盖安装 1.0.8 并启动，真机界面树确认配对首页正常显示。图标比例与两页实际刷新状态待体验复验。

#### 首页 PC 图标尺寸复验

按用户确认，首页 PC 图标统一为 24 vp 高，在单行和双行卡片中垂直居中，双行卡片的中间内容高于图标。101 项回归、API 24 构建、签名包完整性检查及 USB 覆盖安装启动通过。真机界面树确认首页和电脑列表，用户已确认图标尺寸复验通过。

### 整体测试

1. 我发现 iOS/android/harmony sessionList item都缺少了一个功能：右下方应当要展示当前会话的状态，和whale-term里是一致的。此功能是不兼容旧的已经匹配过的session会话吗？

#### 整体测试修复记录

三端会话卡片右下角根据 PC 快照及事件显示待处理、进行中、已完成或已中断。已同步的空闲会话显示完成，运行状态不依赖回复正文。旧缓存不迁移状态，配对仍有效时重连并收到快照即可更新。

移动端 698 项、HarmonyOS 102 项及 22 文件逻辑检查通过。iOS Simulator Debug、Android Debug、HarmonyOS API 24 Debug 构建通过。用户确认本轮整体真机验收通过。

HarmonyOS 会话状态修复版已重新编译、签名并通过 USB 覆盖安装 1.0.8。真机启动和首页检查通过，用户确认实时会话状态验证通过。
