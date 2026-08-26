# C1 Capacitor 工程与 Web UI 接入设计

- 日期：2026-08-26
- 状态：用户已确认（含 D44 阶段范围与 D45 应用标识）
- 前置阶段：A6 局域网跨机器桌面浏览器联调已通过；B1 设计与实施计划已确认
- 关联决策：D7、D14、D19、D40、D41、D44、D45

## 1. 目标

C1 完成三个交付项（D44）：

1. 在 `apps/mobile/` 下创建 Capacitor iOS 与 Android 工程；
2. 将现有 React + TypeScript + Vite Web UI 接入两个原生壳，并完成 Web 构建、Capacitor 同步和工程级验证；
3. 放行局域网明文网络，并提供仅开发用的 `server.url` live reload，使壳内页面能连接 PC 的 `ws://<LAN-IP>:port` 并显示真实数据。

第 3 项是第 2 项的前提：不放行明文，WebView 连不上 Remote Server，壳子只能显示空布局，无法判断 Web 界面在两端 WebView 的适配程度。

C1 只建立可继续开发的工程边界，不提前实现扫码、安全存储、NativeBridge、生命周期或真机业务验收（均属 C2）。

## 2. 采用的方案

使用 Capacitor 官方 CLI 和标准 Web 同步流程：

```text
apps/mobile/src
        │
        ▼
   npm run build
        │
        ▼
apps/mobile/dist  ── npx cap sync ──>  apps/mobile/ios
                                  └──>  apps/mobile/android
```

不手工复制 Web 文件，不创建第二套 React 页面，不引入自定义原生插件。Web UI、协议客户端和状态模型继续由 `apps/mobile/src/` 维护；C1 的 iOS/Android 目录只承载 Capacitor 生成的原生工程和平台配置。

## 3. 工程结构

```text
apps/mobile/
├── src/                    # 现有 React Web UI 与协议客户端
├── public/
├── dist/                   # npm run build 生成；不作为手工源码维护
├── capacitor.config.ts     # C1 新增，webDir 指向 dist
├── ios/                    # C1 由 Capacitor CLI 创建
│   └── App/App/Info.plist  # C1 增加局域网明文放行与本地网络权限文案
└── android/                # C1 由 Capacitor CLI 创建
    └── app/src/main/       # C1 确认或增加 cleartext 放行
```

原生应用标识按 D45 固定：

- App name：`Claude Mobile Remote`；
- App ID：`com.tea4go.claudemobileremote`；
- Web directory：`dist`；
- Web asset scheme：沿用 Capacitor 默认配置，不在 C1 自定义路由或协议。

App ID 属于工程与分发标识，不进入 WebSocket envelope、配对负载或三端共用协议；变更 App ID 视为新应用。

## 4. 依赖和命令边界

C1 只引入 Capacitor 核心工程依赖：

- `@capacitor/core`；
- `@capacitor/cli`；
- `@capacitor/ios`；
- `@capacitor/android`。

不引入以下依赖：

- 扫码插件；
- Keychain/Keystore 或其他安全存储插件；
- 自定义 NativeBridge；
- 推送、语音、生命周期或网络诊断插件；
- 与 antd-mobile 并列的第二套 UI 组件库。

计划中的标准流程为：

```bash
cd apps/mobile
npm ci
npm run build
npx cap sync
```

工程初始化只执行一次；后续 Web 修改统一通过 `npm run build` 和 `npx cap sync` 更新原生资源。为避免两步遗漏，`apps/mobile/package.json` 增加一个组合脚本：

```json
"sync:native": "npm run build && cap sync"
```

不增加其他原生脚本；`cap open` 、`cap run` 等直接用 `npx` 调用。

### 4.1 开发态 live reload

为尽快比较两端 WebView 的界面适配，`capacitor.config.ts` 支持把 `server.url` 指向 Mac 上的 Vite dev server（如 `http://<MAC-LAN-IP>:5173`），壳子直接加载开发服务器而不是 `dist`，改一行 UI 即时刷新，不需每次 `build → cap sync`。约束：

- 只用于开发，通过环境变量或本机未入库的覆盖开启，默认关闭；
- 不得进入发布构建；发布构建必须加载本地 `dist`；
- 依赖 §6.1 的局域网明文放行，不为它额外放宽网络策略。

调试工具：iOS WKWebView 用 Safari “开发” 菜单接管，Android WebView 用 Chrome `chrome://inspect`。

`dist`、Xcode DerivedData、Android 构建输出、`local.properties`、签名文件和用户本机配置不得提交到 Git，对应忽略规则在 C1 写入根 `.gitignore`。

## 5. Web UI 接入边界

- 现有 `App`、协议客户端、reducer、配对 Fragment 和 B1 页面继续作为唯一 Web 业务实现；
- C1 不复制 `apps/mobile/src` 到 iOS/Android 原生代码；
- C1 不修改版本 1 JSON envelope、Session/Turn 状态、配对负载或 `sessionId` 语义；
- Capacitor WebView 仅加载构建后的 `dist`，不改变 `wsUrl`、`hello → subscribe → snapshot` 或消息归并；
- 浏览器的 Fragment/内存 Pair Token 边界在 C1 不扩展为原生持久化；系统安全存储留到后续阶段；
- C1 验证页面可被原生壳加载、构建通过并能连接局域网 Remote Server；不宣称原生扫码、安全存储、VPN 或后台恢复已可用。

## 6. 平台和 HarmonyOS 边界

### 6.1 iOS / Android

C1 创建标准平台工程、验证 Web 资源同步，并只做与“壳内页面能连上 PC”直接相关的平台网络配置，范围收窄到局域网明文（D44）：

- **iOS**：`Info.plist` 的 `NSAppTransportSecurity` 只设 `NSAllowsLocalNetworking`（放行局域网明文，公网仍强制 TLS），不使用 `NSAllowsArbitraryLoads`；同时提供 `NSLocalNetworkUsageDescription`，满足 iOS 14+ 本地网络权限弹窗要求。ATS 与本地网络权限是两层独立机制，两者都必需；
- **Android**：Capacitor 生成的模板是否已带 `usesCleartextTraffic` 属于待实测事实，必须在 `cap add android` 后查看生成的 `AndroidManifest.xml` 确认：已带则只记录不重复添加，未带则优先用 `network_security_config.xml` 限定放行，不全局开启；
- 两端配置只服务局域网调试与 VPN 直连，不放行公网明文；后续 Relay（D43）走 WSS，不依赖本节例外。

扫码权限、系统安全存储、NativeBridge、后台生命周期和推送全部归 C2；不存在 C1.1 子阶段。

### 6.2 HarmonyOS NEXT

HarmonyOS NEXT 不加入 Capacitor 工程，也不要求运行 React/Capacitor 原生目录。C1 只确保以下内容可被 D1/D2 独立复用：

- 版本 1 JSON envelope；
- `sessionId`、request ID、`seq` 和 Session/Turn 状态语义；
- 配对负载字段和 `hello → subscribe → snapshot` 顺序；
- Web UI 的页面职责和错误边界，不复用具体 React 组件。

C1 不引入任何会迫使 HarmonyOS NEXT 复刻 iOS/Android 原生插件的协议字段或业务依赖。

## 7. 验证范围

C1 实施后至少验证：

- `npm ci` 成功；
- `npm run build` 成功并生成 `dist`；
- `npx cap sync` 成功，iOS 和 Android 工程均识别 `dist` Web 资源；
- Capacitor 配置的 app name、App ID 和 `webDir` 与 D45 一致；
- iOS/Android 工程目录存在且能被对应 IDE/CLI 识别；
- Android 原生构建通过（已安装 JDK 与 Android SDK，见 §7.1）；
- iOS `Info.plist` 含 `NSAllowsLocalNetworking` 与 `NSLocalNetworkUsageDescription`，且不含 `NSAllowsArbitraryLoads`；
- Android cleartext 放行已生效，并记录它来自 Capacitor 模板默认还是本阶段新增；
- 至少一端原生壳（模拟器即可）能加载页面并成功连接 PC 的 `ws://<LAN-IP>:port`，看到真实状态与消息，而不是空布局；
- `server.url` 在仓库默认配置中未开启；
- 根 `.gitignore` 已覆盖 `android/local.properties`、两个 `public/` 同步目录、Pods、DerivedData、Gradle 缓存与签名文件，且未误伤 `apps/mobile/public/`；
- 现有 `npm test`、`npm run lint` 回归通过；
- 没有新增协议字段、原生插件、凭证存储或真实设备结论；
- Git 差异不包含 `dist` 构建产物、签名文件、DerivedData、Gradle 缓存或本机 IDE 状态。

### 7.1 环境前置

C1 开始前在 Mac 上安装 JDK 与 Android SDK（cmdline-tools 或 Android Studio），使 Android 原生构建纳入门禁；Apple Silicon 需选 arm64-v8a 系统镜像。模拟器只用于界面适配与构建验证；它的网络经 NAT（宿主机为 `10.0.2.2`），不能代替 C2 的真机与 VPN 验证。

## 8. 后续关系

- B1 与 C1 可并行实施；B1 继续负责 Web UI，C1 只负责原生壳和资源同步；
- 后续 C2 再设计扫码、安全存储、NativeBridge 和生命周期；
- C2 依赖 C1 和 B2，进行真机、多配对持久化、VPN、前后台恢复和失效重连验收；
- D1 单独创建 HarmonyOS NEXT 工程，复用共用契约但不复制 Capacitor 原生工程。
