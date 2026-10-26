import type { CapacitorConfig } from '@capacitor/cli';

// 开发态 live reload：设置 CAP_SERVER_URL（如 http://<MAC-LAN-IP>:5173）后
// 原生壳加载 Vite dev server 而非本地 dist；未设置时不输出 server 字段，
// 发布构建始终加载本地 dist。仅用于开发，不得进入发布构建。
const serverUrl = process.env.CAP_SERVER_URL;

const config: CapacitorConfig = {
  appId: 'com.tea4go.claudemobileremote',
  appName: 'Claude Mobile Remote',
  webDir: 'dist',
  backgroundColor: '#f2f2f7',
  // Capacitor 的 Debug 桥接日志会打印插件请求/结果，包含安全存储中的 Pair Token。
  loggingBehavior: 'none',
  android: {
    webContentsDebuggingEnabled: true,
  },
  ios: {
    // 用户要求 Debug 与 Release 都可通过 Safari 检查 WebView。
    webContentsDebuggingEnabled: true,
    // 页面内部滚动，避免 iOS 将整个 WebView 卷到状态栏下。
    scrollEnabled: false,
  },
  plugins: {
    Keyboard: {
      // 插件 native 模式会延迟缩放；iOS 根控制器改由 keyboardLayoutGuide 约束 WebView。
      resize: 'none',
      autoBackdropColor: 'auto',
    },
  },
  server: {
    // Android 壳内页面走 http://localhost 而非默认 https://localhost：
    // 默认 https 会触发 WebView 混合内容策略，拦截到 ws:// 局域网的明文连接（见 D48）。
    androidScheme: 'http',
    ...(serverUrl ? { url: serverUrl } : {}),
  },
};

export default config;
