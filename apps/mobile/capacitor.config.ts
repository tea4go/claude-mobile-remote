import type { CapacitorConfig } from '@capacitor/cli';

// 开发态 live reload：设置 CAP_SERVER_URL（如 http://<MAC-LAN-IP>:5173）后
// 原生壳加载 Vite dev server 而非本地 dist；未设置时不输出 server 字段，
// 发布构建始终加载本地 dist。仅用于开发，不得进入发布构建。
const serverUrl = process.env.CAP_SERVER_URL;

const config: CapacitorConfig = {
  appId: 'com.tea4go.claudemobileremote',
  appName: 'Claude Mobile Remote',
  webDir: 'dist',
  server: {
    // Android 壳内页面走 http://localhost 而非默认 https://localhost：
    // 默认 https 会触发 WebView 混合内容策略，拦截到 ws:// 局域网的明文连接（见 D48）。
    androidScheme: 'http',
    ...(serverUrl ? { url: serverUrl } : {}),
  },
};

export default config;
