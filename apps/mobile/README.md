# Claude Mobile Remote — Mobile Web

这是 Claude Mobile Remote 的 React/TypeScript 共用移动界面。A6 中它作为 Windows 机器 B 的本地浏览器客户端，只连接机器 A 配对负载中的 WebSocket 地址，不在 B 上运行 Go Service。

## 本地启动

前提：Node.js 20 或更高版本。进入本目录后执行：

```powershell
npm ci
npm run dev
```

打开 Vite 输出的 `http://localhost:5173`，粘贴机器 A 当前 Service 输出的配对 JSON。页面会把有效凭证保留在 URL Fragment 和当前内存中，不写入 `localStorage`。

A6 标准流程不使用 `vite --host`，因为浏览器只需从 B 本机访问页面；跨机器链路是浏览器连接机器 A 的 `wsUrl`。默认允许的浏览器 Origin 已包含 `http://localhost:5173`。

配对 JSON 含控制凭证，不要写入文件、日志、截图、问题单或 Git。页面提示配对失效时，应从机器 A 获取当前 Service 最新输出的配对负载。

## 检查命令

```powershell
npm test
npm run lint
npm run build
```

机器 A 的完整启动命令和局域网故障定位见仓库根目录 `README.md`。

## 原生壳（C1 起）

本目录同时承载 Capacitor iOS / Android 工程。Web UI 是唯一业务实现，原生壳只加载构建产物，不含扫码、安全存储等原生能力（归 C2）。

```bash
npm run build        # 生成 dist（tsc -b && vite build）
npm run sync:native  # 等价 npm run build && cap sync，把 dist 同步进 ios/android
npx cap open ios     # 用 Xcode 打开 iOS 工程
npx cap open android # 用 Android Studio 打开 Android 工程
npx cap run android  # 构建并安装到已连接的 Android 模拟器/真机
```

### 开发态 live reload

改 UI 时无需每次 `build → cap sync → 重装`：设置 `CAP_SERVER_URL` 后，原生壳加载 Vite dev server，改 `src/` 保存即热更新。

```bash
# 终端 1：起 Vite dev server（--host 让模拟器能访问）
npm run dev -- --host 0.0.0.0

# 终端 2：指向 dev server 跑 App（模拟器访问宿主机用 10.0.2.2）
CAP_SERVER_URL=http://10.0.2.2:5173 npx cap run android
```

仅用于开发；未设置时（默认）加载本地 `dist`，发布构建始终用 `dist`。live reload 下 Server 端仍需带 `--allowed-origin http://localhost`。

### 壳内连接 PC 的两条前提（C1 实测结论）

1. **Android WebView 用 `http` scheme**：`capacitor.config.ts` 已设 `server.androidScheme: 'http'`。Capacitor 默认 `https://localhost` 会触发 WebView 混合内容策略，拦截到局域网 `ws://` 的明文连接，导致配对后页面崩溃。
2. **显式放行原生壳 Origin**：Android 壳的页面 Origin 是 `http://localhost`（无端口），不在 Service 默认 Origin 白名单内。联调时启动 Service 需加 `--allowed-origin http://localhost`。模拟器访问宿主机用 `10.0.2.2` 而非 LAN IP，因此 advertise-url 应指向 `ws://10.0.2.2:<port>/ws`。真机/VPN 走真实 LAN 地址，在 C2 验证。

配对 JSON 含控制凭证，不要写入文件、日志、截图、问题单或 Git。

### Android 模拟器端到端测试步骤（C1 实测通过）

模拟器网络经 NAT，访问宿主机用 `10.0.2.2` 而非 LAN IP；本流程用 `--fake-host` 桩，不耗真实 Claude 额度。

```bash
# 1. 启动模拟器（首次冷启动约 1-2 分钟）
emulator -avd pixel7

# 2. 启动 Remote Server（--advertise-url 用 10.0.2.2 + --allowed-origin 放行壳 Origin）
cd <仓库根目录>
GOFLAGS=-buildvcs=false go build -o /tmp/claude-mobile-remote ./cmd/claude-mobile-remote
tail -f /dev/null | /tmp/claude-mobile-remote \
  --listen 0.0.0.0:8765 \
  --advertise-url ws://10.0.2.2:8765/ws \
  --fake-host interactive \
  --allowed-origin http://localhost
# ↑ 记下输出的 pairing JSON；`tail -f /dev/null` 保持 stdin 打开，避免 CLI 读到 EOF 退出

# 3. 构建并安装 App（改过 Web 代码需先 sync）
cd apps/mobile
npm run sync:native
cd android && ./gradlew installDebug

# 4. 在模拟器里打开 App，粘贴 pairing JSON（把 wsUrl 的 IP 换成 10.0.2.2），点「连接」，发送消息，
#    应看到 fake-host 回显：思考 → 工具调用 → 权限请求卡片
```

停止模拟器：`adb emu kill`。

## 移动端调试

### 看 JS console / 页面状态（命令行，推荐）

`chrome://inspect` 点 inspect 常因弹窗被拦截或 DevTools 前端资源从 Google 服务器加载失败而无反应。本目录提供命令行兜底脚本，自动 `adb forward` 并实时打印 console：

```bash
node scripts/cdp-console.mjs               # 持续监听 console.log/error、JS 异常、网络错误
node scripts/cdp-console.mjs --state --once # 单次抓当前页面文本 + 按钮
node scripts/cdp-console.mjs --pid <pid>    # 指定 App 进程（默认自动从 adb 获取）
```

前置：模拟器已启动、App 已打开、`adb` 在 PATH 中。

### 看原生日志（logcat）

WebView 的 `console.error` 与 JS 异常会自动打到 logcat 的 `Capacitor/Console` 标签：

```bash
adb logcat | grep -i "Capacitor/Console"              # JS console 错误
adb logcat | grep -iE "chromium|AndroidRuntime|FATAL" # WebView 加载 / 原生崩溃
```

### 图形化 DevTools（可选）

`chrome://inspect` → Remote Target 里选 `com.tea4go.claudemobileremote` 的 WebView → inspect，可看 Elements / Network / 断点。iOS 壳用 Safari「开发」菜单接管 WKWebView。
