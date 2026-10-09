# C2-C Capacitor 原生能力接线设计

- 日期：2026-09-24
- 范围：接续已确认的 C2-C 纯逻辑层；本轮实施 iOS/Android 原生适配和移动端接线，不以真机/VPN 为本机门禁

## 实现路径

### 详情固定状态与边缘返回（2026-10-09 真机反馈）

详情页把连接、会话和 Turn 状态压缩到固定标题区右侧，并在断线或协议错误时将重连入口置于同一区域。详细错误和队列说明仍留在消息滚动区。长消息滚动时状态和重连保持可见；窄屏优先保留返回及状态操作，超长标题截断。

从详情页屏幕左边缘开始横向右滑，达到阈值后返回首页列表。纵向消息滚动、输入框编辑和非左边缘滑动不得触发。沿用现有返回转场、列表滚动恢复和减少动态效果设置。仅改变 React/Capacitor 本地导航，不改变共用协议；iOS 手势冲突与手感待真机验收。

### PC 编码与设置页（D76）

Standalone CLI 首次运行在用户配置目录生成随机 128 bit 十六进制 `pcId`，用权限受限的文件保存，重启后复用；假宿主测试可以使用进程内编码，避免测试污染用户配置。Pairing v1 增加可选 `pcId`，由 Go 公共 Pairing Manager 附加到签发及轮换负载，CLI 新建的 Session 共享同一编码。公共 Remote Server 提供可选配置供嵌入宿主传入；旧服务无需提供。React/Capacitor 与 Harmony 的严格解码均接受字段并继续接受旧负载。编码是展示标识，不参与鉴权。

移动端首页增加设置按钮，原实现的设置页按 `pcId` 分组展示设备与其本地配对会话；其 UI 层级与单会话操作位置现由 D77 修订。没有编码的旧记录用规范化连接地址分组，并明确标为旧版地址分组。原生端先移除安全存储记录再更新内存，浏览器维持当前内存边界。设备删除不发送服务端停止命令。已失效配对的普通元数据若没有 `pcId`，在重新扫码前只能显示旧版/未知设备；不得凭会话 ID 猜测 PC 身份。

### C2-C 设备与列表体验修订（D77）

首页设置先展示通用设置列表，点击「PC 管理」进入下一层；PC 管理只列设备，保留整机移除，单会话移除继续由首页列表菜单负责。「新增 PC 配对」只在首页保留。首页会话按 PC 身份分组，每台 PC 内再按工作目录分组；相同目录在不同 PC 不能合并。PC 分组的稳定键仍为 `pcId`，旧负载按连接地址兼容。设备连接状态仅由本机已配对会话的实时连接状态推导：任一会话已连接显示「已连接」，全部需重新配对显示「需重新配对」，其余按正在建立连接与否显示「连接中」或「未连接」；不宣称设备级探测结果。

Pairing v1 新增可选 `pcName`，Standalone CLI 读取本机 hostname，嵌入宿主可通过 `remote.Config.PCName` 显式传入。机器名只作展示，不承担身份或鉴权，限制为非空、最长 255 个 Unicode 字符且不含控制字符。Go 生产、React/Capacitor 解析与安全存储元数据、Harmony 解码均接受此字段；旧配对与旧服务仍合法。PC 管理优先显示机器名，旧负载只能显示连接地址或编码。换名后以最近已连接会话携带的名字为准。

详情活动和待处理事项抽屉共用较短且确定的 CSS 进退场时长，面板高度上限提高到可视高度约 86%，较短内容给出适度最小高度，并保留键盘时可视区域限制、遮罩和显式关闭语义。真机验收动画节奏和面板位置。

React Web 与 Capacitor 共用现有多 Session UI 和 Runtime。仅 `Capacitor.isNativePlatform()` 为真时构造原生适配器；浏览器保留 Fragment/内存，不能调用任何插件的 Web 存储回退。原生端完整 Pairing 由 `@aparajita/capacitor-secure-storage` 存储：iOS Keychain，Android 由 Keystore 密钥 AES-GCM 加密后放入应用私有 Preferences。普通元数据由 `@capacitor/preferences` 保存，仅允许 `sessionId`、名称、自定义名称标记、最近连接时间、`pcId` 和 `pcName`。使用独立 key 前缀和单记录 key，避免把 token 混入元数据索引。Capacitor 桥接日志在 Debug 构建也会打印插件请求和返回，故设置 `loggingBehavior: 'none'`，禁止凭证进入桥接日志。

`@capacitor/barcode-scanner` 只扫 QR，返回文本交给既有 `parsePairingInput`；取消不写入。`@capacitor/app` 的 `appStateChange` 在前台恢复时刷新全部有效连接状态，不自动重连。Android 扫码所需最低 SDK 26；iOS 增加相机用途说明及 Preferences 隐私声明。

## App 接线

原生启动先读取普通元数据及对应安全记录，校验有效期和 Session 绑定，再为有效项调用现有 Store 与 Runtime；缺失、损坏、错绑或过期项只恢复无凭证列表占位，显示“需要重新配对”，点击进入输入/扫码页，绝不建立 WebSocket。恢复期间不显示空白配对页闪烁。输入、扫码及服务端新建 Session 返回的 Pairing 都先安全写入，再进入列表和连接；存储失败显示固定错误且不将未持久化凭证连接为已保存项。

原生端重命名、删除和最近连接时间同步普通元数据。删除先移除安全记录，成功后再移除元数据与内存连接。原生端不把 token 放入 URL Fragment；已有 Fragment 若由外部打开，仅作为一次性导入输入并在处理后清除。Web 端的原有 hash 行为保持。恢复仅恢复配对和连接，不恢复消息正文。

## 验证与限制

### iOS 顶部安全区修复

真机详情截图显示状态栏时间与返回按钮重叠。React/Capacitor 共用页面采用设备安全区布局：viewport 允许 WebView 覆盖整屏，顶部导航容器的背景延伸至屏幕顶端，其内部导航控件按 `safe-area-inset-top` 下移；横屏时左右控件也遵循对应安全区。列表、配对、详情共用该容器，恢复提示同样避开顶部。详情高度仍由 `100dvh` 控制，底部输入区继续使用既有 `safe-area-inset-bottom`。不引入固定机型高度、不修改通信协议或 Harmony 页面。

验证需覆盖 Web 构建、现有移动端测试，以及 iOS 真机纵横屏的顶部导航和底部输入区视觉；自动化构建不能替代真机遮挡验收。

### 页面转场体验改进（iOS 真机反馈，优先于 C2-D）

React App 当前按 `view/adding` 直接替换列表、详情和配对 JSX，`antd-mobile` 的 `NavBar` 只提供栏内控件。增加本地导航呈现层：进入详情或配对向左推进，返回列表向右退回，扫码成功进入详情按前进方向处理。转场期间短暂保留旧页面用于退出动画，旧页面不能再接收触控或键盘输入；页面完成后立即卸载。快速连续导航应只保留最近一次旧页面，不堆叠动画层。

页面容器填满可视区域，列表和配对页在自身容器内滚动；详情页仍由消息流单独滚动。返回列表恢复此前滚动位置，顶部与底部安全区继续生效。使用 `transform` 进行短转场，并尊重系统 `prefers-reduced-motion`；首次启动和原生配对恢复不播放无意义的切页动画。网络连接和 Session Store 生命周期仍在 App/Runtime 中，与退出动画解耦。

这项改进处理切页跳变。iOS 真机还需分别观察长消息列表滚动、键盘弹出和返回手势；是否接入原生导航/手势栈取决于这些实测结果，不把 Web 转场等同于原生渲染性能。通信协议、状态与配对负载均不变，HarmonyOS NEXT 的 ArkUI 导航独立实现。

### 转场节奏与软键盘遮挡（后续真机反馈）

用户实测发现 280ms 切页过快，而 antd-mobile `Popup` 使用内部弹簧动画，不能通过普通 CSS `transition-duration` 精确调整。先将切页进退动画与旧页清理计时统一为 360ms，作为两者之间的节奏；保持系统减少动态效果分支。抽屉保留组件自身动画，避免覆盖其 transform 与弹簧状态。

键盘弹出时 `100dvh` 在 iOS WebView 中可能未与实际可见区域同步。应用根层监听 `visualViewport` 的 resize/scroll，并在窗口 resize 时更新可见高度与底部被遮挡距离；页面栈和详情页以该可见高度布局，底部固定的 Popup 按遮挡距离上移，抽屉最大高度随可见区域收缩。聚焦的文本框在视口变化后滚入自身滚动容器；键盘收起时恢复原高，避免持久空白。无 `visualViewport` 的环境回退到原有 `100dvh`。该逻辑仅影响 React/Capacitor UI，不改变协议或 Harmony 实现。

#### iOS 键盘返工（真机验收不通过）

`ios_test1.png`/`ios_test2.png` 表明上述多重补偿在 WKWebView 中造成静态底部留白和聚焦后整页上卷。撤销 `visualViewport` 高度/底部偏移写入及聚焦 `scrollIntoView`，不再独立上移 Popup。iOS 显式启用 Capacitor Keyboard 的 `native` resize：系统在键盘出现时缩放 WebView，页面栈使用缩放后的 `100dvh`；详情输入栏作为页面内 flex 底栏，Popup 继续固定在缩放后 WebView 底部。禁用原生 WebView 根滚动，仅保留应用内的列表、配对页和消息流滚动，避免系统把整页卷到状态栏下。`body`/`#root` 不产生独立滚动高度；键盘出现时去除详情底部 home indicator 安全区内边距，收起后恢复。Android 和浏览器保留 CSS 动态视口路径，不使用 iOS 键盘事件来移动内容。

验证需覆盖键盘未打开时底栏贴近底部、键盘打开时输入栏贴近键盘、收起后无残留留白、配对和抽屉输入可见，以及列表/消息流仍能滚动。自动化只能检查配置与样式接线，iOS 真机结果仍由用户验收。

#### iOS 键盘二次返工（聚焦延迟与黑色背景）

真机再次验收发现键盘先遮住输入栏，随后页面跳到键盘上方。核对当前依赖的 Capacitor Keyboard 8.0.5 实现：`native` 模式收到 `UIKeyboardWillShowNotification` 后，将 WebView frame 更新延迟至系统动画时长再加 0.2 秒，直接导致布局晚于键盘。改用插件 `resize: none` 保留键盘事件，iOS `SceneDelegate` 监听系统键盘 frame 变化通知，按键盘末端 frame 与当前 window 的实际重叠高度设置 WebView frame，并采用通知中的时长和曲线同步动画。收起、交互式改变和横竖屏时均以 window/键盘相交区域重新计算，避免旧高度残留。应用仍仅有一套原生 WebView 缩放，不做 JS 键盘高度补偿。

用户明确黑色区域是页面上移后露出的窗口背景，并非键盘皮肤。给 Capacitor WebView、原生 window 和 DOM body 设置与页面一致的浅色背景，使键盘动画中的背板不再露黑；输入法外观保持系统设置。此项只涉及 iOS 呈现，不改变协议或 Harmony 实现。

#### iOS 键盘跟随与详情滚动结构（后续真机反馈）

真机确认遮挡和黑底消失，但按通知参数对根 WebView 的 frame 做 `UIView.animate`，仍需 WebKit 另行布局 CSS 的 `100dvh` 与 flex 底栏，因此输入栏观感不同步。Capacitor 负责原生桥接，并不禁止原生键盘跟随；当前插件 `native` resize 的延迟实现和手写 frame 动画均不足以保证输入栏逐帧贴着键盘。将 `CAPBridgeViewController` 的 WebView 放入全屏容器，底边约束到系统 `keyboardLayoutGuide.topAnchor`，顶边/左右贴容器边缘；iOS 17+ 关闭 guide 的底部安全区替代，未唤起键盘时 WebView 仍覆盖整屏。iOS 15/16 保留通知驱动的约束回退，避免 guide 默认安全区造成静态空白。由 UIKit 跟随键盘及交互式收起更新约束，不再在 iOS 17+ 用通知时间估算 frame。仍需真机确认 WKWebView 的布局刷新能否达到目标手感，不能以原生约束推断 Web 内容逐帧一致。

底部输入栏直接采用 WebView 的 `safe-area-inset-bottom`，不再通过异步插件事件切换 0px 底部补偿，避免动画中再发生一次离散跳动。详情视图调整为导航栏与输入栏固定，中间一个贴满页面宽度的内容滚动区。状态、提示、消息都在该滚动区，横向留白放在内部内容层，滚动容器自身不留右侧 padding。消息流滚到底的判断使用同一滚动容器，避免内外双滚动；列表和配对页滚动边界不变。无需修改共享契约或 Harmony 页面。

#### 输入栏聚焦与禁用反馈（后续真机反馈）

输入框已聚焦时，底排附件和权限按钮及弹出菜单的触点应保留输入焦点，使系统键盘保持打开；仅这些控件在按下阶段阻止浏览器将焦点转给按钮，菜单的 click 行为继续执行。新建会话、语音和发送不纳入焦点保持范围；点击输入区外的普通内容继续由系统收起键盘。输入不可用时保留真实 textarea 的 `disabled` 语义，在文本框上方设置可触控的禁用反馈层；触碰后短时显示明显但不改变布局的边框反馈及原因提示，不尝试聚焦或发送。原因仅从当前详情状态推导，不改变服务端协议或状态契约。

注入插件形状的假端口测试原生恢复、扫描、保存、重命名、删除、过期占位及前台事件；Web 回归测试确保无持久化。运行 Vitest、lint、Vite 构建、Capacitor sync，以及本机可用的 Android/iOS 构建。模拟器可验证原生插件的调用与应用重启，但没有真机时不能宣称相机、系统杀进程、VPN 和安全存储在物理设备上的结论。通信协议、配对负载和三端共用状态契约不变；Harmony 后续用独立平台 API 实现同等行为。

插件依据：[@aparajita/capacitor-secure-storage](https://github.com/aparajita/capacitor-secure-storage/blob/main/README.md)、[@capacitor/preferences](https://github.com/ionic-team/capacitor-plugins/blob/main/preferences/README.md)、[@capacitor/barcode-scanner](https://github.com/ionic-team/capacitor-barcode-scanner)、[@capacitor/app](https://github.com/ionic-team/capacitor-plugins/blob/main/app/README.md)。
