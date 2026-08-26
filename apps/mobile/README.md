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
