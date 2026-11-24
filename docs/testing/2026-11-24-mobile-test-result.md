# 11月24日移动端测试结果

### 背景

经过两天的三端移动端的测试，发现了一些问题和优化点，需要按照 iOS优先，其他两端同步的顺序进行

文中引用的实机截图保留在本地验收目录。

### 问题及优化列表

1. 目前移动端sessionDetail内 抽屉展示工具列表需要优化：docs/testing/images/11-24/claude_tools_list.png
   1.1. 工具item之间的间隔太大，需要紧凑一点
   1.2. subagent工具展示得太简单了，请参考whale-term端的subagent展示进行优化：docs/testing/images/11-24/subagent.png

2. 移动端sessionDetail内的工具详情 需要优化：
   2.1. 由于工具详情内容 可能会很大，比如 "Read" "Edit"等等，参考了 `Happy UI`等应用的处理方式，建议详情应当从PC端获取，而不是存储在手机缓存内，否则很容易造成内存溢出
   2.2. 详情展示需要优化：docs/testing/images/11-24/claude_tool_detail.png，改成 新建一个界面专门用于展示工具的详情；且工具详情很多都是代码块，代码展示的markdown目前过于简单了；且需要横向滚动，尽量保持文本的阅读连贯性(和sessionDetail内一样)；
   2.3. 在加载工具详情时，添加整屏的loading效果，加载失败需要提示

3. sessionDetail内的 用户发送消息和claude回复消息需要优化：
   3.1. 目前的User消息下方的tip总是：电脑端 已接收，手机端 已接收，不太直观：docs/testing/images/11-24/user_send_message_tip.png，建议参考：docs/testing/images/11-24/claude_response_message.png 当claude还未处理消息时，展示：等待处理；claude开始回复后，则取消User消息下方的tip；请你将这个过程仔细审查思索下，是否有改造的必要
   3.2. 目前移动端是无法切换模型的，也无法看到当前用的是哪个模型，建议参考：docs/testing/images/11-24/claude_response_message.png 在claude回复的标题之后追加 当前的模型

## iOS 首轮处理结果

- 调整步骤抽屉行距，子代理行展示任务描述、执行状态和下属工具，子工具可展开。
- 工具详情独立全屏，从已配对的 PC 按需读取，提供加载、失败和重试。长行可以横向滚动，退出页面后释放详情，iOS 缓存写入和恢复均去除工具结果正文。
- 本机发送且 PC 确认接收后显示「等待处理」，同一请求开始产生思考、工具、回复、审批或完成后收起。断线未确认仍显示结果未知。Claude 标题显示 SDK 返回的实际回复模型。

## 自动化与构建

- 移动端 51 个文件、709 项测试通过。覆盖 iOS v13、等待提示、子代理嵌套、按需读取、失败重试、原始换行和旧缓存迁移。生产构建、Capacitor iOS 资源复制、无签名 iOS 模拟器构建及资源比对通过。Lint 通过并保留既有警告。
- 公共 Go 全量 race 回归通过。实际回环 WebSocket 覆盖 v12 与 v13 的事件和快照、同序号兼容、详情请求关联以及轮次和会话隔离。另覆盖 PC 详情 UTF-8 截断和存储上限。
- WhaleTerm Remote 回归和宿主定向用例通过。宿主用例使用临时模块文件引用本地 Remote，并因既有 DeepWiki、Gemini 格式检查错误使用 `-vet=off`。
- 使用同一临时模块文件构建 Windows x64 WhaleTerm，确认产物为 PE32+ GUI 程序，可用于真机联调，路径为 `.local-tools/mobile-1124/WhaleTerm.exe`。正式构建需升级到包含 v13 的 Remote 依赖。

## iPhone 待验

1. 发送一轮包含子代理的任务。检查工具行间距、不同子代理的描述和状态，展开工具，并检查返回步骤后仍保持展开。
2. 打开较长的 Read 或 Edit，在全屏详情查看原始换行和横向滚动。断开 PC 后尝试读取，重新连接并重试。
3. 观察发送后的「等待处理」何时收起，确认实际模型出现在回复标题后。旧 PC 配对应仍可收发，工具详情沿用旧版本能提供的内容，回复标题在缺少模型数据时显示「Claude」。

iPhone 和实际 WhaleTerm 联调待验收，通过后同步 Android 和 HarmonyOS。

### iOS测试问题

1. 工具详情界面需要优化：
   1.1. 缺少切屏动画，且不支持向右滑屏返回
   1.2. 标题右侧的 执行状态，建议用tag展示更加清晰
   1.3. claude tool的标题应当参考whale-term，有专门的字段供展示的，sessionDetail内的抽屉内的claude tool标题也需要同步更改

## iOS 补充问题处理结果

工具详情已接入与会话页一致的进退动画和左缘右滑返回，返回后保留步骤抽屉及子工具展开状态。详情标题右侧使用状态标签。抽屉和详情共用标题，例如 Read 展示文件路径，Bash 优先展示命令描述，Agent 展示任务描述。新旧 v13 客户端按连接能力接收可选摘要，旧 PC 使用已有预览或工具名。

移动端 709 项、公共 Go 全量 race、WhaleTerm quickpair/remote 回归及 chat 宿主定向测试通过。chat 定向测试使用本地 Remote 临时 modfile 和 `-vet=off`。Web 构建、iOS 资源同步、模拟器构建及包内资源比对通过。首轮 Windows 联调包漏用了 Wails production 构建标签，本轮已用 `desktop,production` 重建同路径程序，并核验 Windows x64 GUI 格式和构建标签。Windows 启动与 iPhone 手势体验待真机复验。

### iPhone 复验清单

1. 更新两端后，打开一个含 Read、Bash 或子代理的步骤，核对抽屉和详情标题一致，在工具完成前后观察右侧状态标签。
2. 展开子工具并滚动抽屉，进入详情。分别通过返回按钮和屏幕左缘右滑退出，检查进退动画、原滚动位置和展开状态。
3. 打开有长行的工具结果，在代码区横向滑动查看行尾，再从左缘空白处右滑返回。开启系统「减弱动态效果」后再次返回。

### iOS测试问题2

1. whale-term在昨天支持了：检测并实时展示 异步subagents的功能，此功能还涉及到了排队消息queue得插入，目前移动端测试 异步subagent表现有问题，需要同步此功能。
   可参考whale-term commit：01ed00e25c39694b44e46d6e65845ab7df089e78

2. claude tool detail需要优化：
   2.1. 思考过程 也需要用markdown渲染，和sessionDetail内的claude回复一样
   2.2. 部分 Claude Read/Edit/Write 工具未展示实际结果。实测仅出现文件已更新、无需重新读取的提示。
   2.3. 标题的内容过多，展示有问题，折行了
   2.4. 在会话进行过程中，进入工具详情界面，停留一会儿会自动返回sessionDetail，不知为何
   2.5. 有一些claude tool detail会提示：暂无可显示的结果正文，请在电脑端查看。为何会有这种提示？这个应该不是 断网的提示，是因为内容太多了吗？
   2.6. AskUserQuestion claude tool detail详情没有展示出来，提示：去电脑端查看
   2.7. bash指令相关的claude tool detail，建议渲染成 代码的样式，支持快速拷贝

3. sessionDetail界面问题如下：
   3.1. step抽屉内的左侧垂直方向的线条没了

4. sessionList 目录之间的分割线去除
5. 首页配对界面内的PC Item不需要active选中的样式，把它改为：手指按压上去显示此样式即可

## iOS 测试问题 2 处理结果

- 接入 WhaleTerm 异步子代理的 v14 状态。步骤抽屉中的子代理默认折叠，面板头显示描述、状态和子工具计数，点击后原位展开。主回复结束和下一条排队消息启动后，后台子工具仍归属启动它的轮次。重连快照恢复宿主仍保留的任务和子工具。
- 工具详情保持在打开状态，标题单行省略，状态标签保留。思考过程使用会话回复的 Markdown 渲染。Read、Edit、Write 展示 SDK 已提供的内容或 diff 以及执行提示。AskUserQuestion 显示问题、选项和可用的已提交答案。Bash 的命令和输出可横向滚动、分别复制。已区分无文本结果、内容截断、详情过期和连接失败。
- 恢复步骤左侧时间线。iOS 会话列表去掉目录之间的分隔线，配对首页 PC 行在按压时短暂高亮。

移动端 51 个文件共 712 项测试通过，覆盖长回复重排时详情页不退出、后台任务跨轮更新、队列状态、折叠及返回、思考 Markdown 和问答详情。Go 全量 race 验证通过，覆盖 v13/v14 回环 WebSocket、快照和序号隔离。WhaleTerm quickpair/remote 与 chat 定向回归通过，后者因既有格式检查错误使用 `-vet=off`。Lint、Web 生产构建、Capacitor 资源同步和无签名 iOS 模拟器构建通过，包内 12 个 Web 文件与构建产物逐一比对一致。Windows x64 生产联调包为 `.local-tools/mobile-1124/WhaleTerm-ios-1124-2.exe`，已核验 PE32+ GUI 格式、Wails 生产标签和本地 Remote 依赖。正式 WhaleTerm 构建需要含 v14 的 Remote 版本。

### iPhone 和 Windows 复验

1. 运行新版 Windows WhaleTerm 并配对 iPhone。发起异步子代理任务，在主回复完成前从手机再发一条消息。观察队列、新轮次和原子代理面板是否各自更新，再断线重连核对状态。
2. 展开子代理并进入一个子工具详情，在 Claude 继续输出时停留，确认页面保持打开。返回后核对展开和滚动位置，检查状态标签、长标题省略和左侧时间线。
3. 分别运行 Read、Edit、Write、AskUserQuestion 和 Bash。检查文件内容或 diff、问答及命令与输出的复制，并核对思考 Markdown 和空结果提示。
4. 查看多个目录的会话列表和配对首页，确认目录无分割线、PC 行高亮随抬手消失。

### iOS 测试问题 3

1. 异步并发subagents验证不通过，PC上的会话还在继续，但手机上已经停止了：docs/testing/images/11-24/pc-subagents.png

2. sessionDetail内subagent展示优化，docs/testing/images/11-24/ios-subagents.png：
   2.1. 取消抽屉内subagent的二级目录，从折叠面板改回原来的list item；然后将子任务列表全部放到claude tool detail内展示，可以参照PC上的展示：docs/testing/images/11-24/pc-subagent-detail.png，折叠列表放在claude tool detail内，抽屉里只保留简单的入口即可
   2.2. subagent正在进行中的标记 应当是loading，可参考session List Item的loading状态

## iOS 测试问题 3 处理结果

- iOS 在主回复完成后显示实时后台运行状态。断开连接或最后一个任务结束后收起后台运行提示。前台轮次的取消和队列按钮按原目标操作。
- 子代理整行进入详情，运行中使用会话列表的加载图标。子工具默认折叠，展开后在详情原位加载输入与结果，流式状态更新保留已展开的条目。
- WhaleTerm 队列在前台轮次结束后可唤醒，同一宿主的任务继续运行。空闲读取器把已登记任务完成通知后的续答下发到原轮次。较新的轮次及排队消息保持独立。v13 对跨轮结果继续收到同序号的兼容通知。

移动端 51 文件共 713 项通过。Go 全量 race 和 WhaleTerm 后台读取交接、队列、远程宿主定向 race 通过。WhaleTerm 定向回归使用本地 Remote 临时 modfile，并因既有格式检查错误使用 `-vet=off`。Lint、Web 构建和 Capacitor 资源同步通过，无签名 iOS 模拟器包内 12 个文件逐一核对一致，已在 iPhone 17 QA 模拟器安装启动。已构建并核验 Windows x64 GUI 联调包 `.local-tools/mobile-1124/WhaleTerm-ios-1124-3.exe`，使用 `desktop,production` 标签及本地 Remote。

### iPhone 和 WhaleTerm 实机复验

1. 并行启动 5 个异步子代理。主回复结束后观察手机继续显示「正在执行子任务」，进入其中一个子代理确认子工具折叠、展开与实时刷新。
2. 后台运行期间分别从 PC 和手机提交排队输入。检查先入先出、前台取消目标与原轮状态，并关注任务完成通知和新输入相邻时，续答是否仍对应原轮次。
3. 在至少一个后台任务运行时断开并重连。检查任务与子工具恢复、剩余任务继续更新，最后一个任务结束后提示收起。

## Android 与 HarmonyOS 同步结果

- Android 启用共享 v14 会话详情、后台任务、模型和等待提示。系统返回键从工具详情返回步骤，下一次可关闭抽屉。已运行 51 个文件 714 项测试，覆盖 Android v14 详情、原有 v12 宿主和系统返回。Lint、Web 构建、Gradle Debug APK 通过。APK 内 12 个 Web 文件与产物逐一比对一致，Pixel 7 模拟器覆盖安装及冷启动通过。
- HarmonyOS 原生 ArkUI 已同步。106 项测试与 22 文件逻辑检查通过，覆盖 v14/v13/旧宿主协商、五任务跨轮更新及快照、旧轮次续答和真实模型、队列与详情请求隔离，以及缓存去除工具结果正文。DevEco 6.1.1 / API 24 原生编译和未签名 Debug HAP 完整性检查通过。
- 两端待和新版 WhaleTerm 做真机联调。沿用上一节的五任务、队列、续答和重连清单，补验 Android 系统返回和 HarmonyOS 子工具展开、Bash 横向滚动及复制。HarmonyOS HAP 需用目标设备对应的调试签名后安装。
