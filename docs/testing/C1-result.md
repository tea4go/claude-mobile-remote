# C1 Capacitor 工程与 Web UI 接入验证记录

- 日期：2026-08-27
- 子阶段：C1（Capacitor iOS/Android 工程 + Web 接入 + 局域网明文放行）
- 关联决策：D44（范围）、D45（应用标识）
- 结论：自动门禁与 Android 模拟器壳内连通均通过

## 1. 环境前置（D44 §7.1）

| 组件 | 版本 | 状态 |
| --- | --- | --- |
| JDK | 21（用户已自行升级） | ✅ |
| Android cmdline-tools | latest | ✅ |
| platform-tools | 37.0.1（adb） | ✅ |
| platforms | android-36 | ✅ |
| build-tools | 36.0.0 | ✅ |
| emulator | 37.1.11 | ✅ |
| system-images | android-36 / google_apis / arm64-v8a | ✅ |
| AVD | pixel7（arm64-v8a） | ✅ |

Capacitor 8.5.0 Android 侧要求 `compileSdk/targetSdk = 36`、`sourceCompatibility = Java 21`、AGP 8.13.0（读包内 `build.gradle` 确认），与 JDK 21 + build-tools 36.0.0 + platform 36 配套。

## 2. 依赖与构建

- Capacitor 四个包统一 `@^8`（实际 8.5.0）：`@capacitor/core`、`@capacitor/cli`、`@capacitor/ios`、`@capacitor/android`；
- `package.json` 新增 `"sync:native": "npm run build && cap sync"`；
- 未安装扫码/安全存储/推送/语音/NativeBridge 等越界依赖（脚本扫描确认）；
- Web 回归：`npm test` 16 files / 376 tests 全绿、`npm run lint` 0/0、`npm run build` 通过。

## 3. 工程创建与同步

- `npx cap init "Claude Mobile Remote" com.tea4go.claudemobileremote --web-dir dist`（D45 标识一致）；
- `npx cap add ios` / `npx cap add android` 成功；Android 首次 Gradle 同步下载依赖约 463s（国内网络正常耗时）；
- iOS 标识：`PRODUCT_BUNDLE_IDENTIFIER = com.tea4go.claudemobileremote`；Android：`namespace`/`applicationId` 一致、`app_name = Claude Mobile Remote`；
- `npm run sync:native` 可重现（删除两个 `public/` 同步目录后重建内容一致）；
- `.gitignore` 覆盖：`local.properties`、`.gradle/`、`build/`、两个 `public/` 同步目录、`Pods/` 均命中忽略规则；`apps/mobile/public/` 未被误伤。

## 4. 平台构建门禁

- iOS：`xcodebuild -list` 识别成功（Targets: App；Schemes: App / CapApp-SPM；Capacitor 8.5.0 经 SPM 解析）。
- Android：`./gradlew assembleDebug` → BUILD SUCCESSFUL（首跑 1m37s，93 tasks；产物 `app-debug.apk` 4.1M）。按 D44 属 C1 硬门禁，已通过。

## 5. 网络明文放行（D44）

- **iOS**：`Info.plist` 增 `NSAppTransportSecurity` → `NSAllowsLocalNetworking = true` 与 `NSLocalNetworkUsageDescription` 文案；未使用 `NSAllowsArbitraryLoads`。
- **Android**：实测 Capacitor 8 模板 `AndroidManifest.xml` **不带** `usesCleartextTraffic` 且无 `network_security_config.xml`。已新建 `res/xml/network_security_config.xml`（`base-config cleartextTrafficPermitted="true"`）并在 Manifest 的 `<application>` 加 `android:networkSecurityConfig` 引用；未使用全局 `usesCleartextTraffic` 属性。Android 的 Network Security Config 无法按 IP 网段精确限定 cleartext，且产品无公网明文入口（后期 Relay 走 WSS），故用 base-config 表达。

## 6. 壳内连通验证（Android 模拟器，核心门禁）

流程：`emulator -avd pixel7` → Go CLI（`--fake-host interactive --listen 0.0.0.0:8765 --advertise-url ws://10.0.2.2:8765/ws --allowed-origin http://localhost`）→ `adb install` → 配对 → 发送消息。

结果（经 Chrome DevTools Protocol 读取页面真实 DOM，非截图判断）：

- 页面正常渲染配对页（`Claude Mobile Remote / 连接 Claude 会话 / 粘贴配对 JSON / 连接`），非空布局；
- 配对成功进入会话详情：`已连接 / 会话：就绪 / Turn：等待中`；
- 发送「你好，测试消息」→ 显示「手机端 · 已接收」；
- 收到 fake-host 结构化回显：思考过程 → 工具调用（1）→ 权限请求卡片（`Claude 请求使用工具：Bash` + `echo fake-preview` + 三个决议按钮）→ 取消当前 Turn 按钮；
- 证明协议 v2 的审批流、结构化内容、工具白名单 preview 在 Android WebView 内正常渲染与交互。

## 7. 两个超出 D44 的实测发现（已处理/待决策）

### 发现 1：Android 默认 https scheme 拦截 ws://（已解决，需记入 DECISIONS）

Capacitor 8 Android WebView 默认 `server.androidScheme = https`（`CapConfig.java` 源码确认），页面走 `https://localhost` 时，浏览器混合内容策略会拦截一切 `ws://` 明文连接，报错：

```
Mixed Content: The page at 'https://localhost/...' was loaded over HTTPS,
but attempted to connect to the insecure WebSocket endpoint 'ws://10.0.2.2:8765/ws'.
Uncaught SecurityError: Failed to construct 'WebSocket': An insecure WebSocket
connection may not be initiated from a page loaded over HTTPS.
```

现象：配对页能渲染，点「连接」后 React 组件树崩溃（`#root` 空）。

解决：`capacitor.config.ts` 设 `server.androidScheme: 'http'`（用户确认方案 A）。这是 Capacitor 官方允许的合法 scheme，与「局域网明文直连」定位一致；`http://localhost` 加载的是本地 dist 资源（无中间人风险），唯一明文是到 PC 的 `ws://`，本就属 D44 要放行的局域网明文。iOS 默认 `capacitor://` scheme 不触发此拦截，无需改动。

### 发现 2：原生壳 Origin 无端口，不在默认白名单（暂不决策，留待 C2）

Capacitor 原生壳的页面 Origin 是 `http://localhost`（**无端口**），而 Service 默认白名单只有 `http://127.0.0.1:5173` 与 `http://localhost:5173`（带 Vite 端口）。CheckOrigin 对带 Origin 的请求做精确匹配 → 拒绝 → 握手 403（客户端侧表现 1006）。

实测确认：

| 握手 | 结果 |
| --- | --- |
| 不带 Origin | 101 成功 |
| `Origin: http://localhost` | 403 拒绝 |
| `Origin: http://localhost:5173` | 101 成功 |

本轮联调用 `--allowed-origin http://localhost` 显式 flag 解决（未改代码）。是否把 `http://localhost` 加入 `defaultAllowedOrigins` 默认值，用户决定暂不决策、留待 C2 真机联调时再定。

## 8. 未验证项（不宣称通过）

- iOS 壳内连通（本轮只验证到 `xcodebuild -list` 工程识别，未在 iOS Simulator 跑通配对连通；Android 已覆盖「至少一端」门禁）；
- 真机、VPN 直连、后台恢复、扫码、安全存储、多配对持久化——均属 C2；
- 模拟器网络经 NAT（宿主机 `10.0.2.2`），不代表真机/VPN 网络行为。

## 9. 边界与安全核对

- 未修改协议 envelope、状态模型、配对负载、`sessionId`/request ID/`seq` 语义；平台明文策略属端内配置，HarmonyOS NEXT 可自行实现等价策略；
- `git diff --check` 无空白错误；无真实 Pair Token、真实 sessionId、LAN IP、签名文件、构建缓存或本机绝对路径入库；
- 未执行任何 Git 写操作。
