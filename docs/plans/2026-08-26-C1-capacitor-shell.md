# C1 Capacitor 工程与 Web UI 接入实施计划

本计划使用项目现有 Codex/Claude/Qoder 等智能体按任务顺序执行；步骤使用复选框（`- [ ]`）跟踪。计划不依赖额外工作流技能。

**Goal:** 创建 Capacitor iOS/Android 工程，把现有 React/Vite Web UI 通过标准 `dist → npx cap sync` 流程接入两个原生壳，并放行局域网明文使壳内页面能连上 PC 显示真实数据（D44）。

**Architecture:** Capacitor 只作为现有 Mobile Web 的原生容器，不复制 React 业务代码，不增加自定义原生插件。`apps/mobile/src` 继续是唯一 Web 实现，`apps/mobile/ios` 和 `apps/mobile/android` 由 Capacitor CLI 生成并通过配置指向 `dist`。平台配置只改网络明文策略，不引入原生业务能力。

**Tech Stack:** React 19、Vite、TypeScript、Capacitor Core/CLI/iOS/Android、Xcode、JDK + Android SDK（按 D44 已确认在 C1 开始前安装，Android 原生构建纳入 C1 门禁）。

---

## 文件与边界

- Modify: `apps/mobile/package.json`、`apps/mobile/package-lock.json`，增加四个 Capacitor 工程依赖和一个 `sync:native` 组合脚本（`npm run build && cap sync`）。
- Create: `apps/mobile/capacitor.config.ts`，固定 app name、App ID 和 `webDir: 'dist'`（D45），并预留默认关闭的开发态 `server.url`。
- Create: `apps/mobile/ios/`，仅包含 Capacitor CLI 生成的 iOS 工程。
- Create: `apps/mobile/android/`，仅包含 Capacitor CLI 生成的 Android 工程。
- Modify: `apps/mobile/ios/App/App/Info.plist`，增加 `NSAllowsLocalNetworking` 与 `NSLocalNetworkUsageDescription`。
- Modify or Create: Android 明文放行配置（`android/app/src/main/AndroidManifest.xml` 或 `android/app/src/main/res/xml/network_security_config.xml`），具体取哪一种取决于 Task 4 Step 2 的实测结果。
- Modify: `.gitignore`（根），确认原生产物忽略规则已生效；缺失则补齐。
- Modify: `apps/mobile/README.md`，补充 Web build、sync、工程打开与 live reload 说明，明确 C1 未包含扫码/安全存储。
- Create: `docs/testing/C1-result.md`，记录依赖、构建、sync、网络放行、连通验证和未验证项。
- Modify: `HANDOFF.md`，记录实际结果和 C1/C2 下一步。

不修改 `apps/mobile/src` 的协议、Session、配对或业务逻辑；不创建扫码、安全存储、NativeBridge、生命周期、VPN 或其他原生插件代码；平台配置只放行局域网明文，不使用 `NSAllowsArbitraryLoads`，不放行公网明文；不提交 `dist`、签名、DerivedData、Gradle 缓存、`local.properties` 和本机 IDE 状态。

### Task 1: 建立工具链和依赖基线

**Files:**
- Verify: `apps/mobile/package.json`, `apps/mobile/package-lock.json`
- Verify: `apps/mobile/README.md`

- [ ] **Step 1: 确认环境前置**：执行 `node --version`、`npm --version`、`xcodebuild -version`、`java -version`，并检查 `ANDROID_HOME`/`ANDROID_SDK_ROOT` 与 `adb version`。JDK 与 Android SDK 是 C1 前置（D44）；若缺失，先完成安装（Apple Silicon 选 arm64-v8a 系统镜像）再继续，不跳过 Android 门禁。不修改用户全局环境以外的配置。
- [ ] **Step 2: 运行 Web 基线**：在 `apps/mobile` 执行 `npm ci`、`npm test`、`npm run lint`、`npm run build`；确认现有 Web 在引入 Capacitor 前通过。
- [ ] **Step 3: 锁定同一主版本的 Capacitor 依赖**：先执行 `npm view @capacitor/core version` 得到当前 latest 主版本 N，再以 `@^N` 显式安装：`npm install @capacitor/core@^N @capacitor/ios@^N @capacitor/android@^N` 和 `npm install --save-dev @capacitor/cli@^N`。四个包必须落在同一主版本，避免 `cap add` 时出现版本校验告警；不手工编辑版本号。
- [ ] **Step 4: 增加组合脚本**：在 `apps/mobile/package.json` 的 `scripts` 增加 `"sync:native": "npm run build && cap sync"`；不增加其他原生脚本，`cap open`/`cap run` 直接用 `npx` 调用。
- [ ] **Step 5: 检查依赖边界**：确认没有安装扫码、安全存储、推送、语音、生命周期或自定义 NativeBridge 插件；确认 Web 业务依赖和 antd-mobile 未被替换。
- [ ] **Step 6: 验证 CLI**：执行 `npx cap --version` 和 `npx cap --help`，确认 CLI 可用；若网络或 npm registry 阻断，只记录阻塞，不用未锁定的临时包替代。

### Task 2: 初始化 Capacitor 配置和平台工程

**Files:**
- Verify: `.gitignore`
- Create: `apps/mobile/capacitor.config.ts`
- Create: `apps/mobile/ios/`
- Create: `apps/mobile/android/`

- [ ] **Step 1: 先确认原生产物忽略规则生效**：在创建平台工程前执行 `git check-ignore -v apps/mobile/android/local.properties apps/mobile/ios/App/App/public/x.js apps/mobile/android/app/src/main/assets/public/x.js apps/mobile/ios/App/Pods/x`，四条均需命中根 `.gitignore`；同时执行 `git check-ignore -v apps/mobile/public/favicon.svg` 确认已跟踪的 `apps/mobile/public/` **未**被误伤。任何一条不符合预期就先修正规则，再进入下一步。
- [ ] **Step 2: 写入初始化配置**：在 `apps/mobile` 执行 `npx cap init "Claude Mobile Remote" com.tea4go.claudemobileremote --web-dir dist`；确认生成的配置等价于：

```ts
import type { CapacitorConfig } from '@capacitor/cli'

const config: CapacitorConfig = {
  appId: 'com.tea4go.claudemobileremote',
  appName: 'Claude Mobile Remote',
  webDir: 'dist',
}

export default config
```

- [ ] **Step 3: 先构建 Web 资源**：执行 `npm run build`，确认 `apps/mobile/dist/index.html` 存在；不把 `dist` 手工复制进平台目录。
- [ ] **Step 4: 添加 iOS 平台**：执行 `npx cap add ios`；确认 `ios/App/`、Xcode 工程和 Capacitor 依赖由 CLI 生成，未出现业务代码副本。
- [ ] **Step 5: 添加 Android 平台**：执行 `npx cap add android`；确认 Android Gradle 工程和 Capacitor 依赖由 CLI 生成，未出现业务代码副本。
- [ ] **Step 6: 确认本机产物未进入待提交范围**：执行 `git status --short`，确认 `local.properties`、`Pods/`、`public/` 同步目录和构建缓存未出现在未跟踪列表；若出现则补齐 `.gitignore` 而不是逐文件排除。
- [ ] **Step 7: 检查平台标识**：核对两个平台的 app name 和 bundle/application ID 为 `com.tea4go.claudemobileremote`（D45）；若 CLI 生成值不一致，只修改平台工程标识配置，不修改 Web 协议或业务代码。

### Task 3: 接入 Web 资源并验证同步

**Files:**
- Verify: `apps/mobile/dist/`
- Verify: `apps/mobile/ios/`, `apps/mobile/android/`
- Modify only if needed: `apps/mobile/capacitor.config.ts`

- [ ] **Step 1: 执行标准同步**：在 `apps/mobile` 执行 `npm run sync:native`（等价于 `npm run build && cap sync`）；预期 iOS 和 Android 均报告 Web 资源同步成功。
- [ ] **Step 2: 检查同步目标**：确认原生工程中的 Web 资源位于 `ios/App/App/public/` 与 `android/app/src/main/assets/public/`，内容来自 `dist`，没有额外的 `src` 副本、手工复制目录或未跟踪用户会话内容。
- [ ] **Step 3: 检查 Web 路由边界**：确认现有配对 Fragment 和内存状态未改为 History API 路由；C1 不添加路由库、不改变 `#pair` 行为。
- [ ] **Step 4: 验证同步可重现**：删除 `apps/mobile/ios/App/App/public/` 与 `apps/mobile/android/app/src/main/assets/public/` 两个同步目录后重新执行 `npm run sync:native`，确认两个目录被重新生成且内容与 `dist` 一致；配置不得写入绝对本机路径。
- [ ] **Step 5: 验证原生工程识别**：执行 `npx cap ls`，确认 iOS/Android 均被识别；检查 `capacitor.config.ts` 的 `webDir`、app name 和 App ID 与设计一致。

### Task 4: 放行局域网明文并验证壳内连通

**Files:**
- Modify: `apps/mobile/ios/App/App/Info.plist`
- Modify or Create: `apps/mobile/android/app/src/main/AndroidManifest.xml`、`apps/mobile/android/app/src/main/res/xml/network_security_config.xml`
- Modify: `apps/mobile/capacitor.config.ts`

- [ ] **Step 1: 配置 iOS 局域网明文与本地网络权限**：在 `Info.plist` 的 `NSAppTransportSecurity` 下只加 `NSAllowsLocalNetworking = true`，并添加 `NSLocalNetworkUsageDescription` 文案（说明用于连接局域网内的 PC Remote Server）。不得使用 `NSAllowsArbitraryLoads`；文案不包含真实 IP、`sessionId` 或凭证。
- [ ] **Step 2: 实测并配置 Android 明文放行**：先查看 `cap add android` 生成的 `AndroidManifest.xml` 是否已带 `android:usesCleartextTraffic`，并把实测结果记入 `docs/testing/C1-result.md`：已带则不重复添加；未带则优先新建 `network_security_config.xml` 只对局域网网段放行并在 Manifest 引用，不全局开启。
- [ ] **Step 3: 加入默认关闭的开发态 live reload**：在 `capacitor.config.ts` 以环境变量（如 `CAP_SERVER_URL`）条件开启 `server.url`，未设变量时不输出 `server` 字段，发布构建始终加载本地 `dist`；不把任何具体 LAN IP 硬编码或提交入库。
- [ ] **Step 4: 验证壳内页面能连上 PC**：启动 Go CLI（`--listen 0.0.0.0:<port>`、`--advertise-url ws://<MAC-LAN-IP>:<port>/ws`），在至少一端原生壳（iOS Simulator 或 Android 模拟器均可）加载页面并完成配对，确认能看到真实状态与消息而不是空布局；可用 Safari “开发” 菜单或 Chrome `chrome://inspect` 看 console 定位拦截。验证用的 Pair Token、真实 `sessionId` 和 IP 不得写入任何文档或截图。
- [ ] **Step 5: 确认网络策略边界**：确认两端均未放行公网明文，仓库默认配置中 `server.url` 未开启，且未引入扫码、安全存储或生命周期相关原生代码。

### Task 5: 运行平台级构建门禁

**Files:**
- Verify: `apps/mobile/ios/`
- Verify: `apps/mobile/android/`

- [ ] **Step 1: 运行 Web 回归门禁**：执行 `cd apps/mobile && npm test && npm run lint && npm run build`，预期全部通过。
- [ ] **Step 2: 运行 iOS 工程门禁**：执行 `cd apps/mobile && npx cap sync ios`，再使用 `xcodebuild -list` 或 Xcode 检查工程可识别；不要求在无签名设备上发布或安装。
- [ ] **Step 3: 运行 Android 工程门禁**：执行 `cd apps/mobile && npx cap sync android`，再执行 `cd android && ./gradlew tasks`（或 `./gradlew assembleDebug` 等无安装构建检查）；按 D44，Android 原生构建属于 C1 门禁，必须通过，不要求真机安装。
- [ ] **Step 4: 处理构建失败**：若 Android 构建失败，先区分环境问题（SDK/JDK/Gradle 版本、镜像缺失）与工程问题，修复环境或平台配置后重试；不得修改业务代码绕过，也不得把未通过直接记为环境豁免。
- [ ] **Step 5: 检查产物边界**：执行 `git status --short` 与 `git check-ignore -v` 确认 `dist`、Xcode DerivedData、`Pods/`、Android `.gradle`/`build` 输出、`local.properties`、两个 `public/` 同步目录、签名文件和本机 IDE 状态均已被忽略。

### Task 6: 记录 C1 结果并交接

**Files:**
- Create: `docs/testing/C1-result.md`
- Modify: `apps/mobile/README.md`
- Modify: `HANDOFF.md`

- [ ] **Step 1: 更新 Mobile README**：写明 `npm run build`、`npm run sync:native`、iOS/Android 工程目录、开发态 `server.url` 用法和当前不包含扫码/安全存储；不记录本机绝对路径、真实 LAN IP、证书或用户配置。
- [ ] **Step 2: 创建验证记录**：记录 Capacitor 版本、Web test/lint/build、sync、`npx cap ls`、iOS `xcodebuild -list`、Android `./gradlew` 结果、Android 模板是否自带 cleartext 的实测结论、壳内连通验证结果（哪一端、是否看到真实数据），以及未验证项（真机、VPN、后台恢复均属 C2）。
- [ ] **Step 3: 复核三端契约**：确认 C1 未修改 envelope、状态、配对负载、`sessionId`、request ID、`seq` 或验收语义；平台明文策略属于端内配置，HarmonyOS NEXT 可自行实现等价策略，不加入 Capacitor 工程。
- [ ] **Step 4: 安全和产物扫描**：确认没有 Pair Token、真实 `sessionId`、会话内容、LAN IP、签名文件、构建缓存或绝对本机路径；执行 `git diff --check`。
- [ ] **Step 5: 更新 HANDOFF**：记录实际完成项、平台未验证项和 C2 下一步；不执行 `git add`、`git commit` 或 `git push`，除非用户另行明确授权。

## C1 门禁结论

- C1 必须通过：Web build、Capacitor 配置、iOS/Android 工程创建、`npm run sync:native`、`.gitignore` 覆盖检查、iOS 工程识别、Android `./gradlew` 构建，以及至少一端壳内页面成功连接局域网 Remote Server。
- Android 原生构建按 D44 纳入门禁，不再作为环境豁免项；环境未就绪时先安装再推进。
- 模拟器结论只覆盖界面适配与构建，不代表真机网络行为（模拟器网络经 NAT）。
- C1 不宣称扫码、安全存储、公网明文/Relay、VPN、真机、生命周期或后台恢复通过。
- C2 负责真实设备、多配对持久化、扫码、安全存储、VPN 和生命周期验收。
