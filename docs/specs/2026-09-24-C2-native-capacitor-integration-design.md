# C2-C Capacitor 原生能力接线设计

- 日期：2026-09-24
- 范围：接续已确认的 C2-C 纯逻辑层；本轮实施 iOS/Android 原生适配和移动端接线，不以真机/VPN 为本机门禁

## 实现路径

React Web 与 Capacitor 共用现有多 Session UI 和 Runtime。仅 `Capacitor.isNativePlatform()` 为真时构造原生适配器；浏览器保留 Fragment/内存，不能调用任何插件的 Web 存储回退。原生端完整 Pairing 由 `@aparajita/capacitor-secure-storage` 存储：iOS Keychain，Android 由 Keystore 密钥 AES-GCM 加密后放入应用私有 Preferences。普通元数据由 `@capacitor/preferences` 保存，仅允许 `sessionId`、名称、自定义名称标记和最近连接时间。使用独立 key 前缀和单记录 key，避免把 token 混入元数据索引。Capacitor 桥接日志在 Debug 构建也会打印插件请求和返回，故设置 `loggingBehavior: 'none'`，禁止凭证进入桥接日志。

`@capacitor/barcode-scanner` 只扫 QR，返回文本交给既有 `parsePairingInput`；取消不写入。`@capacitor/app` 的 `appStateChange` 在前台恢复时刷新全部有效连接状态，不自动重连。Android 扫码所需最低 SDK 26；iOS 增加相机用途说明及 Preferences 隐私声明。

## App 接线

原生启动先读取普通元数据及对应安全记录，校验有效期和 Session 绑定，再为有效项调用现有 Store 与 Runtime；缺失、损坏、错绑或过期项只恢复无凭证列表占位，显示“需要重新配对”，点击进入输入/扫码页，绝不建立 WebSocket。恢复期间不显示空白配对页闪烁。输入、扫码及服务端新建 Session 返回的 Pairing 都先安全写入，再进入列表和连接；存储失败显示固定错误且不将未持久化凭证连接为已保存项。

原生端重命名、删除和最近连接时间同步普通元数据。删除先移除安全记录，成功后再移除元数据与内存连接。原生端不把 token 放入 URL Fragment；已有 Fragment 若由外部打开，仅作为一次性导入输入并在处理后清除。Web 端的原有 hash 行为保持。恢复仅恢复配对和连接，不恢复消息正文。

## 验证与限制

### iOS 顶部安全区修复

真机详情截图显示状态栏时间与返回按钮重叠。React/Capacitor 共用页面采用设备安全区布局：viewport 允许 WebView 覆盖整屏，顶部导航容器的背景延伸至屏幕顶端，其内部导航控件按 `safe-area-inset-top` 下移；横屏时左右控件也遵循对应安全区。列表、配对、详情共用该容器，恢复提示同样避开顶部。详情高度仍由 `100dvh` 控制，底部输入区继续使用既有 `safe-area-inset-bottom`。不引入固定机型高度、不修改通信协议或 Harmony 页面。

验证需覆盖 Web 构建、现有移动端测试，以及 iOS 真机纵横屏的顶部导航和底部输入区视觉；自动化构建不能替代真机遮挡验收。

注入插件形状的假端口测试原生恢复、扫描、保存、重命名、删除、过期占位及前台事件；Web 回归测试确保无持久化。运行 Vitest、lint、Vite 构建、Capacitor sync，以及本机可用的 Android/iOS 构建。模拟器可验证原生插件的调用与应用重启，但没有真机时不能宣称相机、系统杀进程、VPN 和安全存储在物理设备上的结论。通信协议、配对负载和三端共用状态契约不变；Harmony 后续用独立平台 API 实现同等行为。

插件依据：[@aparajita/capacitor-secure-storage](https://github.com/aparajita/capacitor-secure-storage/blob/main/README.md)、[@capacitor/preferences](https://github.com/ionic-team/capacitor-plugins/blob/main/preferences/README.md)、[@capacitor/barcode-scanner](https://github.com/ionic-team/capacitor-barcode-scanner)、[@capacitor/app](https://github.com/ionic-team/capacitor-plugins/blob/main/app/README.md)。
