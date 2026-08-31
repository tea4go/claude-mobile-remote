# CLI 产品化配置设计

## 1. 目标

普通用户启动 Claude Mobile Remote 时只关心新建或恢复会话、工作目录和 Claude 参数，不必重复填写 Remote Server 的底层网络参数。开发文档区分正常开发使用与底层调试使用。

## 2. 用户界面

常用参数保留在顶层 CLI：

```text
--resume <UUID>
--cwd <directory>
--permission-mode <mode>
--model <model>
--max-turns <number>
```

远程环境通过 `--profile` 选择，默认 `local`：

| Profile | listen | advertise URL | 用途 |
| --- | --- | --- | --- |
| `local` | `127.0.0.1:8765` | `ws://127.0.0.1:8765/ws` | 本机浏览器，安全默认 |
| `emulator` | `0.0.0.0:8765` | `ws://10.0.2.2:8765/ws` | Android 模拟器 |
| `device` | `0.0.0.0:8765` | 本机可达 IPv4 | LAN 或 VPN 真机 |
| `debug` | 当前显式底层参数 | 当前显式底层参数 | 手动排障与特殊环境 |

`device` 不区分 LAN 和 VPN；两者在协议层都是设备直连 PC。默认选择本机第一个可用的非 loopback IPv4，并允许用高级参数 `--advertise-host` 覆盖多网卡或 VPN 选择。`debug` 保留 `--listen`、`--advertise-url` 和 `--allowed-origin`，用于覆盖 profile 生成的值。

## 3. 约束

- 默认 `local` 不暴露局域网；只有显式选择 `emulator`、`device` 或 `debug` 的非 loopback 配置才开放远程访问。
- profile 只影响 Remote Server 网络配置，不创建额外 Session、不改变 Claude 生命周期和协议 JSON。
- `permission-mode`、`model`、`max-turns` 映射到 SDK Option；非法值在启动前报错。
- 不支持的 Claude 参数本轮不静默透传，避免 CLI 参数和 SDK 能力产生不透明分叉。

## 4. 文档范围

README 提供开发版最短命令和调试版完整命令；说明 `10.0.2.2` 只属于 Android 模拟器，`device` 的地址必须是手机实际可达的 LAN/VPN 地址。
