# iOS 手机端了连接（2026-10-26）

## 配置背景

- ios手机和PC处于同一局域网络下，但是ip不一致(局域网络下有主网和副网)
- ios可以正确安装claude-remote-control应用，并打开

## 实际验证

- 扫码后，进入了session详情，但是右上角详情提示一直是：连接中。过了很长时间后，提示：未连接
- 网络配对方面，没有任何可用的调试和提示；扫码后，PC端也没有任何提示，是否配对成功；

## 验收结论

- 需解决上述描述中的网络问题
- 移动端需要开放可调式的面板
- 网络连接，配对等的超时时间目前整体都过长，需要优化
- 配对，配对后的连接状态，交互等需要优化：扫码后需要反馈,PC端(whale-term内)同样需要记录当前会话连接了几台设备；第一次扫码后的移动端需要验证后，才可进入详情
- ios先行，验证通过后，同步到android和harmony

## 修复与复验状态（2026-10-26）

用户补充：PC连接路由器A，手机连接路由器B时失败；手机切换A后成功。表明故障与跨路由访问相关，具体NAT、路由、防火墙或隔离规则尚未核实；共用宽带不等于设备互通。未将原跨路由网络门禁标记为通过。

已实施：验证snapshot后保存并进入详情；建连8秒、认证/订阅各4秒、总预算15秒；脱敏诊断、PC在线订阅连接数、网卡地址选择及心跳清理。扫码后配对页直接显示目标PC地址（scheme、IP/主机名、端口），失败及取消后保留，不显示配对凭证或手机本地IP。

验证：移动端全套514/514；本次地址展示后的App/原生配对回归47/47、Web生产构建及仅iOS资源同步通过。Go transport/remote race及vet通过。WhaleTerm前端28/28、Remote race、services定向测试、生产前端及完整Wails构建通过；services默认vet存在既有无关格式化错误，定向测试使用-vet=off。WhaleTerm通过临时modfile引用当前本地Remote代码，未修改实际依赖版本。iOS先前版本arm64编译、Release archive及IPA导出通过，但该IPA不包含随后新增的配对页地址展示；此展示需重新打包后在真机验收。未执行Git写操作或Android/Harmony原生同步。

验收清单：

1. 在A网络扫码，核对显示地址与PC一致，验证成功后进入详情，PC在线数增加。
2. 切换B网络重新扫码，记录页面目标地址、失败阶段和耗时，并核对两端IP、掩码及网关；避免复制含凭证的二维码原文。
3. 检查拒绝本地网络权限、关闭PC、取消连接时反馈；失败不新增已保存配对。

### 调试方式修订

用户要求采用平台调试工具，已移除移动端自建网络诊断面板及两处入口；保留目标PC地址、阶段和错误提示。配对定向47/47、生产构建、仅iOS资源同步通过。Capacitor开发构建使用Safari Web Inspector/Xcode（Android使用Chrome DevTools/Android Studio），当前Release IPA未开启WebView远程检查，且未重打包含本次变更的IPA。保留loggingBehavior:none，避免安全存储插件桥接日志泄露凭证。实际设备调试尚未执行。

### Debug与Release远程检查交付

用户明确要求两种构建均开启平台检查器，替代此前Release不启用安排。Web生产构建、仅iOS同步、Release archive/export通过；实际IPA配置确认iOS/Android远程检查为true、桥接日志none、无开发server.url，全部内置Web资源与dist一致，深度严格验签通过。IPA：apps/mobile/ios/DerivedData/inspect-20261026/export/App.ipa，SHA256 eebf933e997617dcfd4eb75d621f0d89a90a1153d77b12432de1a1201b897fb1。实际Safari连接未验收；Android配置已改但原生未同步，Harmony不适用Capacitor；未安装设备或上传、无Git写操作。

复验：安装新IPA，开启iPhone Safari Web检查器，连接并信任Mac；Mac Safari开发菜单选择设备/App页面，分别在路由器A/B下观察WebSocket握手和网络结果。

## 应用内悬浮调试日志（2026-10-26，用户后续确认）

- 用户已确认Safari入口正常可用；实际使用后确认增加App内面板，见D92。
- 已完成设置开关、配对前设置入口、全局悬浮按钮、最近200条内存记录、清空/复制/导出；网络阶段复用原记录，动作包括扫码/配对/保存、命令发送与PC受理结果，异常仅固定类别。没有记录原始异常、凭证、消息正文、Session ID或完整帧。
- iOS复制使用本机剪贴板，5分钟过期；导出使用原生分享文件，取消/完成回收临时目录。浏览器保留复制/下载。
- 定向回归91/91通过：App、App.native、ProtocolClient、日志隐私/容量、面板交互、首次配对入口和iOS分享接口mock；生产构建及cap sync ios通过，Release archive成功。lint仅既有scripts/cdp-console.mjs警告，diff检查通过。
- 真机悬浮位置/滚动、系统剪贴板与分享实际落盘、A/B路由扫码对比未验证；不据此判断跨路由问题已解决。Android/Harmony原生工程未同步。
- 验收：安装本轮IPA → 设置开启日志 → 关闭面板后扫码复现 → 打开悬浮日志核对目标/阶段/耗时 → 复制或导出 → 清空/关闭开关；分别使用路由器A和B，反馈通过或问题。

交付包：`apps/mobile/ios/DerivedData/debug-panel-20261026/export/App.ipa`；SHA256 `768a817cb77318bde0a59ba3d1214a7b6735c5a2de93abda567b4eedfcefb965`。Release archive/export成功，实际IPA的11个Web资源与dist逐文件一致，远程检查true、桥接日志none、无开发server.url，深度严格签名验证通过。未另编Debug包，面板代码无构建模式限制。未安装或上传。

## 调试面板真机反馈优化（mobile_log.png）

已查看用户截图：旧面板正序且行距/按钮偏大，连接诊断缺少事件关联与close信息。新增固定事件码、INFO/WARN/ERROR、毫秒时间、C连接编号/尝试次数、阶段及阶段耗时、总耗时、协议版本、阈值、readyState、close code和wasClean；关闭码附固定解释，不读取原始reason。补主动关闭、手动重连、协议降级、构造失败及error后的close观测，延迟close归属旧尝试，不重复触发业务回调。记录系统online/offline和页面前后台变化；系统状态不等同于PC可达，后台冻结期间无法保证实时回调。现有行为无自动重试，日志明确等待手动重连。

列表最新在上实时更新，紧凑工具栏、字体/间距、错误级别边框；入口左下，基于输入区域和visualViewport避让键盘/输入框。95项定向测试通过（连接错误/延迟close/手动重连/超时/脱敏/倒序/位置计算及App回归），Web生产构建与仅iOS同步通过；lint仅既有cdp-console警告，diff检查通过。实际真机样式、键盘避让、断网/重连和分享仍待复验。

优化版交付：`apps/mobile/ios/DerivedData/debug-panel-v2-20261026/export/App.ipa`，SHA256 `6b75e993750ca0d16ae1655c4f6e7221dace45ccd594ced794900ec0db8987a6`。Release archive/export、11个内置Web资源与dist逐文件比较、远程检查/桥接配置核验和深度严格验签通过。未安装/上传。真机验收：打开日志后断网/恢复并手动重连，检查事件码/close/尝试次数；最新日志应在首行；输入区及键盘展开时悬浮入口应位于左侧上方空隙。


## Xcode窗口构建失败恢复（2026-10-26）

用户报告Build Failed。最新Xcode Build日志（33EE5564、72242A38等）均在编译前报5个Missing package product：AparajitaCapacitorSecureStorage、CapacitorApp、CapacitorBarcodeScanner、CapacitorKeyboard、CapacitorPreferences。本地插件Package.swift与CapApp-SPM引用均存在。

重新执行xcodebuild -resolvePackageDependencies后全部依赖解析成功；默认DerivedData下generic iOS Debug签名构建得到BUILD SUCCEEDED（日志/tmp/cmr-xcode-repair-resolve.log、/tmp/cmr-xcode-repair-debug.log）。打开的GUI首次Cmd+B仍失败，随后File → Packages → Resolve Package Versions，再Cmd+B，GUI明确显示Build Succeeded（14:13）。证据支持打开的Xcode依赖图未刷新；具体触发动作未确定。未修改应用代码、依赖版本或签名，未安装/启动手机App，真机运行由用户继续验收。
