# Claude Mobile Remote — 移动界面

React + TypeScript 移动界面，是三端唯一的业务实现。原生壳（Capacitor iOS / Android）只加载它的构建产物。

前提：Node.js 20+。所有命令都在本目录（`apps/mobile`）执行。

## 常用命令

```bash
npm ci               # 装依赖
npm run dev          # 浏览器开发，打开 http://localhost:5173
npm test             # 单测
npm run lint         # oxlint
npm run build        # 生成 dist（tsc -b && vite build）
```

页面启动后粘贴 Service 输出的配对 JSON 即可连接。凭证只保留在 URL Fragment 和内存中，不写 `localStorage`。

> 配对 JSON 含控制凭证：不要写入文件、日志、截图、问题单或 Git。页面提示配对失效时，重新从 Service 取最新配对负载。

## 场景一：浏览器调 UI

```bash
npm run dev
```

Service 默认已放行 `http://localhost:5173`，无需额外参数。跨机器联调（A6）时浏览器仍在本机打开，只是连接对端 Service 的 `wsUrl`；机器 A 的启动命令见仓库根 `README.md`。

## 场景二：Android 模拟器 + live reload（改代码即刷新）

改 UI 时不必反复 `build → sync → 重装`。四个终端：

```bash
# 终端 0：先启动模拟器并等它开机完成（不启动就装不上，见文末排错）
npm run emu:start

# 终端 1：dev server 对模拟器开放
npm run dev:emu

# 终端 2：仓库根目录起 Service（用 --fake-host 桩，不耗 Claude 额度）
GOFLAGS=-buildvcs=false go build -o /tmp/cmr ./cmd/claude-mobile-remote
tail -f /dev/null | /tmp/cmr \
  --listen 0.0.0.0:8765 \
  --advertise-url ws://10.0.2.2:8765/ws \
  --fake-host interactive \
  --allowed-origin http://10.0.2.2:5173

# 记下输出的配对 JSON

# 终端 3：把壳指向 dev server 并安装
npm run sync:live
npm run android:install
```

打开 App，粘贴配对 JSON，点「连接」，发消息应看到桩回显：思考 → 工具调用 → 权限请求卡片。之后改 `src/` 保存即热更新。

关键点（都有实测依据，弄错就连不上）：

- **模拟器访问宿主机是 `10.0.2.2`**，不是 LAN IP。`--advertise-url` 和 `CAP_SERVER_URL` 都用它。
- **live reload 下壳内页面的 Origin 是 dev server 地址 `http://10.0.2.2:5173`**，所以 Service 必须 `--allowed-origin http://10.0.2.2:5173`。写成 `http://localhost` 会在握手时被拒（403），页面报 `Unexpected response code: 403`。
- `CAP_SERVER_URL` 是在 `cap sync` 时写进 `android/app/src/main/assets/capacitor.config.json` 的。**要关掉 live reload，必须重新跑不带该变量的 `npm run sync:native`**，否则壳会一直指向 dev server。
- 换配对 JSON 前先重开 App（`adb shell am force-stop com.tea4go.claudemobileremote`），否则新 JSON 会追加在旧文本后面。

用完关模拟器：`npm run emu:kill`。

## 场景三：Android 模拟器 + dist（贴近发布形态）

```bash
npm run emu:start        # 同样要先有模拟器
npm run sync:native      # build + cap sync，壳加载本地 dist
npm run android:install
```

这条路径下壳内页面 Origin 是 **`http://localhost`（无端口）**，Service 要 `--allowed-origin http://localhost`，`--advertise-url` 仍用 `ws://10.0.2.2:8765/ws`。

另外 `capacitor.config.ts` 已设 `server.androidScheme: 'http'`：Capacitor 默认的 `https://localhost` 会触发 WebView 混合内容策略，拦掉到 `ws://` 的明文连接（D48）。

真机 / VPN 走真实 LAN 地址，在 C2 验证。

## 看日志和页面状态

`chrome://inspect` 的 inspect 经常因弹窗被拦或 DevTools 前端资源加载不出来而无反应，优先用命令行：

```bash
npm run android:console   # 实时打印 console.log/error、JS 异常、网络错误
npm run android:state     # 单次抓当前页面文本 + 按钮，用于确认 UI 状态
```

前置：模拟器已启动、App 已打开、**`adb` 在 PATH 中**（`export PATH="$PATH:$ANDROID_HOME/platform-tools"`）。脚本调用裸 `adb`，PATH 里没有时会报「找不到 App 进程」。

原生侧日志：

```bash
adb logcat | grep -i "Capacitor/Console"              # JS console 错误
adb logcat | grep -iE "chromium|AndroidRuntime|FATAL" # WebView 加载 / 原生崩溃
```

图形化可选：`chrome://inspect` → 选 `com.tea4go.claudemobileremote` 的 WebView → inspect。iOS 壳用 Safari「开发」菜单接管 WKWebView。

## 排错

**`npm run android:install` 报 `DeviceException: No connected devices!`** —— 模拟器没起来（或还没开机完成）。`npm run emu:start` 会启动并阻塞到开机完成，返回后再装。手工确认：

```bash
adb devices               # 应列出 emulator-5554 device
npm run emu:list          # 看有哪些 AVD
AVD=<其他名字> npm run emu:start   # 默认 AVD 是 pixel7
```

`emu:start` 可重复执行：已在运行时立即返回，未启动时阻塞到开机完成（冷启动可能 1-2 分钟）。它依赖 `ANDROID_HOME`（通常 `~/Library/Android/sdk`）；未设置时会找不到 `emulator`。

**连不上 / 页面报 403** —— 见场景二的关键点，多半是 `--allowed-origin` 与实际 Origin 不匹配。

## 其他

```bash
npx cap open ios       # Xcode 打开 iOS 工程
npx cap open android   # Android Studio 打开 Android 工程
```

Windows PowerShell 下 `emu:*` / `sync:live` / `android:install` 的内联环境变量与 `./gradlew` 不适用，改用 `emulator -avd pixel7`、`$env:CAP_SERVER_URL="http://10.0.2.2:5173"; npx cap sync android` 和 `cd android; .\gradlew.bat installDebug`。

原生壳当前不含扫码、安全存储等原生能力，归 C2。
