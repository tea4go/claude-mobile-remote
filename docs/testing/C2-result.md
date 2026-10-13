# C2-A/C2-B 配对二维码与 Web 生命周期 · 验证记录

- 日期：2026-08-31
- 关联设计：`docs/specs/2026-08-31-C2-pairing-lifecycle-design.md`
- 关联计划：`docs/plans/2026-08-31-C2-pairing-web-lifecycle-plan.md`
- 关联决策：D54

## 1. 本轮范围

本轮只实施 C2-A 后端终端二维码与现有 pairing 持久化回归、C2-B Mobile Web 生命周期。二维码使用 `#pair=<Base64URL(JSON)>` Fragment，不新增 WebSocket 字段或 HTTP API；CLI 保留原有配对 JSON 输出并追加终端二维码。

未实施：原生扫码、安全存储、原生多 Session 跨重启持久化、PNG、HTTP 二维码接口、真机、VPN、WhaleTerm W1 和外部 Claude 进程接管。

## 2. 自动验证

### Go

```text
gofmt -l protocol internal/pairing cmd/claude-mobile-remote
```

通过：无输出。

```text
GOCACHE=/private/tmp/claude-mobile-remote-gocache GOFLAGS=-buildvcs=false go test ./protocol ./internal/pairing ./cmd/claude-mobile-remote -count=1
```

通过：`protocol`、`internal/pairing`、`cmd/claude-mobile-remote` 均通过。

```text
GOFLAGS=-buildvcs=false go build ./cmd/claude-mobile-remote
```

通过：CLI 构建成功。

新增测试覆盖：Go Pairing Fragment 编码与解码字段一致、Base64URL 字符集、终端二维码非空多行输出、二维码不额外显示人类可读 Token、CLI 保留 JSON 并输出 `pairing QR:`；终端二维码使用半块字符并关闭额外外边框以缩小显示尺寸。

```text
GOCACHE=/private/tmp/claude-mobile-remote-gocache GOFLAGS=-buildvcs=false go vet ./...
```

通过：无输出。

```text
GOCACHE=/private/tmp/claude-mobile-remote-gocache GOFLAGS=-buildvcs=false go test -count=1 ./...
```

部分通过：`cmd/claude-mobile-remote`、`internal/channel`、`internal/claude`、`internal/pairing`、`internal/session`、`protocol` 和其它非监听包通过；`cmd/channel-test-client`、`internal/transport`、`remote` 的既有本地监听/httptest 用例在当前沙箱分别因禁止绑定 `[::1]:0` 或 `127.0.0.1:0` 失败。该限制不属于二维码或 Web 生命周期实现回归，需在允许 loopback 监听的环境重跑。

### Mobile Web

```text
npm test -- src/protocol/pairing.test.ts
```

通过：22 个测试通过；新增 Go fixture 可由 Mobile Web `parsePairingFragment` 解析。

```text
npm test -- src/App.test.tsx src/protocol/client.test.ts src/protocol/pairing.test.ts src/session/store.test.ts
```

通过：4 个测试文件、98 个测试通过；覆盖页面 Fragment 恢复、断线不自动重连、显式重连、单连接、`visibilitychange` 状态同步和协议客户端连接阶段。

```text
npm run lint
```

通过：0 error；存在 1 个既有 warning，位置为 `scripts/cdp-console.mjs:76` 的 `no-unused-expressions`。

```text
npm run build
```

通过：TypeScript 编译和 Vite production build 成功。

```text
npm test
```

通过：16 个测试文件、403 个测试通过。

## 3. 行为结论

- CLI 继续输出原有 pairing JSON，并追加终端二维码；Pairing JSON 字段和 WebSocket 协议不变。
- Go 端 Fragment 与 Mobile Web 端 Fragment 解码互操作，`version`、`wsUrl`、`sessionId`、`pairToken`、`expiresAt` 保持一致。
- 现有 pairing 文件恢复、过期替换、损坏降级、`sessionId` 绑定和文件权限逻辑未重写，聚焦 Go 回归通过。
- Web 刷新从 URL Fragment 恢复当前 Session；断线不后台无限重试；只有用户点击“重新连接”才建立新连接。
- 页面恢复可见时只同步当前 Client 连接状态，不自动重连，不修改内存配对列表。

### 用户人工验收

用户已确认以下 4 项通过：

1. 扫码得到的 Pairing Fragment 可用于 Mobile Web 配对完整链路；
2. 页面刷新、断线、手动重连和可见性变化行为符合预期；
3. 服务重启后配对凭证仍可恢复使用；
4. 二维码可被微信识别，且缩小后的终端显示尺寸符合预期。

## 4. 未验证与暂缓

- 终端二维码已通过微信人工识别，并验证缩小后的显示尺寸；本记录不保存二维码截图或真实 Pair Token。
- 原生扫码、Keychain/Android Keystore、安全存储、原生多 Session 列表、移动前后台恢复、真机网络权限和 VPN 未实施/未验证。
- 本轮未运行真实 Claude 端到端，不影响二维码与 Web 生命周期自动测试结论。

## 5. C2-C 原生接线补充（2026-09-24）

本节接续已确认的纯逻辑准备，实施 Capacitor iOS/Android 的安全存储、普通元数据、扫码、前台事件与 App 多 Session UI/Runtime 接线。上文“未实施原生能力”仅描述 2026-08-31 的 C2-A/C2-B 门禁，不代表本节的当前状态。设计与计划见 `docs/specs/2026-09-24-C2-native-capacitor-integration-design.md` 和对应 `docs/plans/` 文档。

- `@aparajita/capacitor-secure-storage@8.0.1` 保存完整 Pairing；`@capacitor/preferences@8.0.1` 只保存白名单元数据。Web 平台门控禁止调用两个插件的浏览器持久化回退。
- `@capacitor/barcode-scanner@3.1.2` 提供 QR 输入，`@capacitor/app@8.1.1` 提供前台事件。过期、缺失或错绑凭证只显示重新配对占位，不启动连接；浏览器继续只使用 Fragment/内存。
- 模拟器验证中发现 Capacitor Debug 桥接日志会显示安全存储插件的请求/返回内容；已设 `loggingBehavior: 'none'`，更新 APK 后复查启动输出不再包含桥接凭证日志。本轮仅使用测试专用假凭证，已通过 App 删除入口清理。

| 验证 | 结果 |
| --- | --- |
| `npm test` | 21 文件、451/451 通过；新增原生 App 与插件端口测试 |
| `npm run lint` | 通过；仅既有 `scripts/cdp-console.mjs:76` warning |
| `npm run sync:native` | TypeScript/Vite 构建与四个 Capacitor 插件的 iOS/Android 同步通过；仅既有大 chunk 提示 |
| Android `./gradlew assembleDebug` | 通过；merged manifest 含最低 SDK 26 与相机权限 |
| Android 模拟器 | 安装/启动通过；测试假凭证保存、WebView 重载、App 进程结束后重启恢复、原生 URL Fragment 为空、删除并清理测试记录通过 |
| iOS 配置 | `Info.plist`、`PrivacyInfo.xcprivacy`、Xcode project `plutil -lint` 通过；四个插件写入 SPM Package.swift |
| `git diff --check` 与新增文件行尾扫描 | 通过 |

iOS Simulator 编译尝试在 Swift Package 解析阶段长时间无进展；单独重试依赖解析时停在扫码插件的 `OSBarcodeLib-iOS` 仓库镜像下载，均已终止，不能宣称 iOS 工程构建通过。模拟器没有验证真实相机扫码、物理设备 Keychain/Keystore 行为、本地网络权限、系统杀进程或 VPN；这些仍属于后续 C2-D 门禁。测试假凭证的目标 WebSocket 端口未启动，故模拟器未验证真实服务连接及 Snapshot。

## 6. C2-C 顶部安全区修复（2026-09-24）

- 移动端入口启用 `viewport-fit=cover`，顶部导航与启动恢复提示统一使用 `env(safe-area-inset-top/left/right)`；详情页底部输入区继续沿用已有底部安全区处理。
- 用户已在 iOS 真机确认详情页顶部工具栏不再被灵动岛/状态栏遮挡，反馈本阶段问题已修复；未报告横屏或底部键盘异常。
- 自动化与资源同步门禁沿用本节上方结果：Vitest **451/451**、lint、TypeScript/Vite build、`npx cap sync` 和 `git diff --check` 通过。

## 7. C2-C 页面转场体验改进（2026-10-08）

- 用户 iOS 真机反馈列表、详情与配对页面切换生硬，决定在 C2-D 前处理。React/Capacitor 增加前进/返回方向转场；退出页短暂保留但禁用交互，快速连续切换仅保留一个退出页。列表改为页面内滚动并恢复位置，系统减少动态效果时直接切页；原生恢复进入首屏不播放动画。Session Runtime 与共用协议未改。
- 自动化：移动端全量 Vitest **454/454**，`npm run lint`、`npm run sync:native`、`git diff --check` 通过。lint 仅有既有 `scripts/cdp-console.mjs` warning，Vite 构建仅有既有大 chunk 提示。同步已把新 Web 资源复制到 iOS/Android 原生工程。
- 未验证：iOS 真机的视觉帧率、边缘返回手势、键盘与长列表触控；iOS Simulator 编译和 C2-D 相机/安全存储/VPN 门禁仍待补。本节不作为真机体验通过结论。

## 8. C2-C 转场节奏与键盘遮挡修复（2026-10-09）

- 用户真机反馈切页 280ms 偏快、antd-mobile 抽屉动画偏慢，并发现聚焦输入时键盘常遮挡输入框。切页 CSS 与旧页清理统一为 360ms；抽屉保留 antd-mobile 内部弹簧。应用监听 `visualViewport`，以可见高度布局页面栈/详情，并让底部 Popup 随键盘上移、限制最大高度；视口变化时将被遮挡的焦点输入滚入视野。
- 自动化：移动端 Vitest **455/455**（23 文件）、`npm run lint`、`npm run sync:native`、`git diff --check` 通过。新增视口变化测试覆盖高度、底部偏移与卸载清理。lint 只有既有 `scripts/cdp-console.mjs:76` warning，构建只有既有大 chunk 提示；同步已复制资源至 iOS/Android。
- 待真机验收：列表/详情往返与抽屉动效节奏；详情输入、配对文本框、待回答抽屉文本框在键盘弹出/收起和连续切页后的可见性；旋转屏幕及长列表。当前不能宣称 iOS 键盘遮挡已通过物理设备验收。

用户后续反馈：页面动画问题已修复；键盘项验收不通过。`ios_test1.png` 显示未聚焦时输入栏下方大片空白，`ios_test2.png` 显示聚焦后内容上卷至状态栏下且输入栏距键盘较远。下节记录返工方案；本节 `visualViewport` 方案已废弃。

## 9. C2-C iOS 键盘布局返工（2026-10-09）

- 移除 `visualViewport` 写入的页面高度、Popup 底部偏移及焦点滚动；iOS 显式接入 Capacitor Keyboard `native` resize，让 WebView 与键盘共用一套高度。页面使用 `100dvh`，根 HTML/body 不滚动，原生 WebView 根滚动关闭，列表、配对和消息流仍使用内部滚动。键盘可见时移除详情底栏 home indicator 内边距，收起后恢复。
- 移动端 Vitest **456/456**（23 文件）、lint、TypeScript/Vite 构建、`npm run sync:native` 和 `git diff --check` 通过。lint 仅既有脚本 warning，构建仅既有大 chunk 提示。同步结果包含 iOS/Android Keyboard 插件，iOS 配置为 `resize: native`、`scrollEnabled: false`。新测试覆盖 iOS 键盘事件切换与监听清理，以及非 iOS 平台不注册监听。
- 未验证：iOS 真机重新安装后的输入栏/键盘间距、收键盘恢复、顶部安全区、内部滚动、配对及待回答抽屉输入。此前 iOS Simulator Swift Package 解析问题仍未解除；本轮不宣称真机键盘体验通过。

用户二次真机验收：仍不通过。聚焦 input 后键盘先遮挡输入框，页面随后才上移，延迟明显。用户明确黑色区域为页面上移后露出的窗口背景，不是输入法皮肤；上一节 `native` resize 方案据此继续返工。

## 10. C2-C iOS 键盘时序与窗口背板返工（2026-10-09）

- 核对本地 Capacitor Keyboard 8.0.5 源码：`native` 模式在键盘将显示后把 WebView frame 更新推迟至系统键盘动画时长再加 0.2 秒，与用户观察到的“先遮挡、后上移”一致。插件改为 `resize: none`；iOS `SceneDelegate` 监听系统键盘 frame 变化并使用同一通知的动画参数缩放 WebView。DOM 仍由缩放后的视口布局，不再测算 JS 键盘高度。
- Capacitor 背景、原生 window 和 DOM body 统一为 `#fafbfd`，插件启用自动背板色；未强制改变系统输入法颜色。
- 移动端 Vitest **456/456**（23 文件）、lint、TypeScript/Vite 构建、`npm run sync:native`、Info.plist 语法与 Swift 文件解析、`git diff --check` 通过。lint 仅既有脚本 warning，构建仅既有大 chunk 提示；同步后的 iOS 配置核对为 `resize: none`、`autoBackdropColor: auto`，无键盘 `style` 覆盖。
- iOS Simulator 构建未通过：本机 CoreSimulatorService 不可用，Swift Package 拉取又因网络代理连接失败，未进入 Swift 类型检查与原生运行。真机仍需验证聚焦开始到结束是否同步、窗口不露黑底、收键盘恢复、配对/抽屉输入和横屏；本节不作为 iOS 真机通过结论。

## 11. C2-C iOS 键盘跟随与详情滚动结构（2026-10-09）

- 用户真机确认上一节的遮挡及黑底已解决，但输入栏上移仍与键盘不同步、观感生硬；详情滚动条距屏幕右缘有明显空隙。本轮将 WebView 放进全屏原生容器，iOS 17+ 底边约束到 `keyboardLayoutGuide`，iOS 15/16 用通知驱动约束回退；移除异步插件事件驱动的底部安全区切换。详情导航和输入栏固定，状态、提示及消息使用一个全宽内容滚动区，内层文字保持 16px 边距。
- 自动化：移动端 Vitest **456/456**（22 文件）、lint、TypeScript/Vite 构建、`npm run sync:native`、Swift 文件语法解析、`git diff --check` 通过。新增测试覆盖详情滚动容器边界与用户向上阅读时不被流式消息拉回底部；lint 仅既有脚本 warning，构建仅既有大 chunk 提示。
- iOS Simulator 构建在 CoreSimulatorService 与沙箱外 Xcode 缓存权限/Swift Package 解析阶段失败，尚未进入原生 Swift 类型检查；没有真机逐帧结果。待用户验证 iOS 17+ 键盘出现/收起及交互式移动、未聚焦底部安全区、长消息与状态一起滚动、输入栏固定、滚动条位置；iOS 15/16 回退也未真机验证。不能据本机测试宣称达到 Happy 的原生动画手感。

## 12. C2-C 输入栏聚焦与禁用触控反馈（2026-10-09）

- 用户要求在输入框已聚焦时，点击底排附件/权限按钮及其菜单保持键盘打开；新建会话、语音、发送维持原操作语义。仅对附件、权限及其 Popover 菜单的 `pointerdown` 阻止默认焦点转移，按钮/menu click 仍执行；页面外部不拦截。禁用输入区增加透明触控层，短时显示禁用边框和按详情状态推导的原因提示，真实 textarea 和发送保持禁用。
- 自动化：移动端 Vitest **459/459**（22 文件）、lint、TypeScript/Vite 构建、`npm run sync:native`、`git diff --check` 通过。测试覆盖保焦触点筛选、菜单操作、禁用反馈及断线原因。lint 仅既有脚本 warning，构建仅既有大 chunk 提示。
- 未验证：iOS WKWebView 真机上触摸附件/权限及菜单后的键盘保持、点击外部收键盘和禁用触控的视觉手感；本轮没有 iOS 原生编译门禁，沿用 §11 的环境阻塞。

用户随后反馈：本节输入栏触控问题已修复。未逐项报告设备操作细节；本结论仅记录用户反馈。

## 13. C2-C 固定状态、边缘返回与 PC 设备管理（2026-10-09）

- 详情连接、会话与 Turn 简要状态，以及断线/协议异常时的“重新连接”，固定到标题栏右侧；错误详情和消息保留在滚动区。左边缘横向右滑返回列表，纵向和非边缘手势不触发，复用页面返回动画。
- Standalone CLI 在本机配置目录持久保存随机 PC 编码；公共 Pairing v1 增加可选 `pcId`，签发、恢复、轮换与 CLI 新建会话沿用同一编码。React/Capacitor 与 Harmony 严格配对解码均接受可选字段，旧负载仍有效。React/Capacitor 首页新增“设置”，按编码管理本地 PC 配对；旧负载按地址标示为兼容分组。整机移除需要二次点击确认，逐条先删除原生安全配对再清理内存；不发送服务端终止操作。
- 验证：Go `go test -race ./...`（`GOCACHE=/tmp/cmr-go-cache GOFLAGS=-buildvcs=false`）全包通过，`go build ./...` 通过；移动端 Vitest **465/465**（23 文件）、lint、TypeScript/Vite build、`npm run sync:native`、`git diff --check` 通过。lint 仅既有脚本 warning，构建仅既有大 chunk 提示。Harmony 改动的四个非 UI `.ets` 文件通过 TypeScript 语法解析；未运行 DevEco 工程构建。
- 自动化覆盖同 PC 跨地址多 Session 合并、不同 PC 分离、旧配对、CLI 编码跨加载及凭证恢复/轮换、原生安全配对与普通编码元数据隔离、整机删除隔离、边缘手势和固定标题区。未做 iOS 真机手势/窄屏视觉、物理设备扫码与跨 PC 联调；WhaleTerm 当前未传稳定 `pcId`，仍按旧版地址分组。

## 14. C2-C 抽屉与 PC 管理层级修订（2026-10-09）

- 活动及待处理事项抽屉改为统一 260ms CSS 动画，面板最大高度提高到约 86dvh，短内容有最小高度；待处理事项仍仅由显式关闭取消 Turn。设置首页改为通用列表，PC 管理作为下一层且只列设备；首页会话按 PC→工作目录→会话展示，单会话操作仍在首页，重复的新增 PC 配对入口已移除。
- Pairing v1 可选 `pcName` 由 Standalone CLI 主机名提供；嵌入宿主可传 `remote.Config.PCName`。Go、React/Capacitor 与 Harmony 解码均保留旧负载兼容；原生普通元数据只增加非秘密 `pcName`。PC 状态由本机已配对会话的实时连接状态归并，不能代表设备级探测。既有配对需要重新扫码才能取得机器名；WhaleTerm 当前未传 `PCID`/`PCName`。
- 验证：Go `GOCACHE=/tmp/cmr-go-cache GOFLAGS=-buildvcs=false go test -race ./...` 全包通过，`GOCACHE=/tmp/cmr-go-cache GOFLAGS=-buildvcs=false go build ./...` 通过；移动端 Vitest **468/468**（23 文件）、lint、`npm run sync:native`、`git diff --check` 通过。lint 仅既有脚本 warning，构建仅既有大 chunk 提示。Harmony 四个改动的非 UI `.ets` 文件通过 TypeScript 语法解析；DevEco 工程构建未运行。
- 自动化覆盖同目录跨 PC 分离、设备整机删除隔离、机器名解码/失效占位恢复、抽屉关闭语义。iOS 真机抽屉速度和位置、键盘唤起后的待回答抽屉、窄屏设备列表、扫码后机器名及连接状态尚待用户验收；本机未验证原生 iOS 工程编译。
- 用户已反馈本轮体验验证完成，并明确授权提交相关改动；未逐项报告设备操作与结果，故不将未报告的具体场景或 iOS 工程编译记为已验证。

## 15. C2-C Android 完整同步与 Harmony 实现边界（2026-10-13）

- 用户反馈 iOS 本轮真机验证暂时结束，要求先同步完整 C2-C 至 Android；HarmonyOS NEXT 暂不同步代码。未逐项提供 iOS 设备和操作结果，本记录不扩展为 C2-D/VPN 门禁结论。
- Android 已复制当前完整生产 Web 资源，包含列表灰底白色圆角模块、纯图标返回、PC 管理与分组、转场与滚动恢复、固定状态、抽屉和输入栏交互，以及既有原生配对、安全存储、扫码和前台生命周期接线。Capacitor 背景更新为 #f2f2f7；同步配置无开发 server.url，loggingBehavior 为 none。iOS 原生键盘约束属于平台实现，Android 沿用其窗口/WebView 视口布局路径，实际键盘手感待验收。
- 验证：Vitest **468/468**（23 文件）、lint、TypeScript/Vite build、`npx cap sync android` 和 `./gradlew assembleDebug` 通过。Gradle 初次因沙箱限制不能访问用户缓存，获准重跑后 BUILD SUCCESSFUL（4m23s，246 tasks）。既有 lint warning、大 chunk 提示、Gradle flatDir 和 SDK XML 版本提示保留。debug APK 位于 `apps/mobile/android/app/build/outputs/apk/debug/app-debug.apk`。
- Harmony 可实现性核对：当前 JSON/WebSocket 契约及状态语义无需 React/DOM/Capacitor；PC 与目录分组、设备删除与确认、实时状态、未读、导航和列表滚动恢复可由 ArkTS 状态与 ArkUI 列表/导航实现；抽屉及动画、固定标题和输入栏、键盘避让、附件/权限保焦和禁用提示由 ArkUI 布局、焦点及手势机制独立实现。安全存储、扫码、跨重启恢复与前台事件需原生平台适配，保持凭证保密、Session 绑定和删除顺序。当前未发现需要改变共用契约的依赖；此结论为实现路径核对，Harmony 代码同步、DevEco 构建与真机验证均未执行。
- 下一步 Android 人工验收：列表/设置/PC 管理与图标返回；详情固定状态、页面和抽屉转场；键盘弹出收起、附件/权限保焦及禁用提示；扫码、重启恢复和设备移除隔离。Android 构建不替代这些设备验证。
- 用户当前没有可用 Android 设备，明确要求本轮跳过设备测试并提交。上述 Android 人工验收暂缓，未记为通过；后续设备可用时补测。
