# HarmonyOS 后台推送联调记录

## 2026-11-13 后续验证

- 华为后台“工作事项提醒”权益已审核通过；服务端 Harmony 载荷由 MARKETING 改为 WORK，其他平台和协议未变。`GOFLAGS=-buildvcs=false go test -race ./push ./cmd/notify-server` 通过。
- 新版已部署 edo，服务 active，`/healthz` 返回 204；用户确认 Harmony 推送验证通过。用户未逐项说明审批及任务完成通知各自的验证结果。
- iOS TestFlight 1.0（12）桌面使用新版图标，通知曾显示旧图标；用户重启 iPhone 后确认恢复。本次未修改 iOS 代码或图标资源。

2026-11-10，用户要求恢复 HarmonyOS 系统推送。沿用 WhaleTerm → edo HTTPS → 极光 → 华为 Push Kit，通知仅含固定事件类型和不透明路由；点击只定位本地有效配对。新增 Harmony 平台登记与独立路由，iOS 旧登记兼容，Android 使用原凭据。Harmony SDK 仅在用户开启后台推送并允许系统通知后初始化。

- 华为应用包名和应用级 Client ID 已核对，推送服务开启。用户授权项目级服务账号及向极光上传密钥；新文件所属项目与应用一致，极光 Harmony 配置回显包名、标题、Client ID 和文件名，厂商通道已开启。用户表示误建于其他同名项目的凭证已删除。
- 服务类自分类权益未申请。当前发送使用 MARKETING 分类；华为页面提示未获权益时默认每设备每天两条，提醒方式亦受平台限制。不得宣称服务类提醒或不限量送达。
- edo 已部署 SHA256 `692059e102e8fdeb95c9ef24afe87b7c1f5fd125283205dbe6cfa225577133be`，旧程序及环境有备份。服务 active、健康检查通过；虚构设备登记 204、事件 502（极光 HTTP 400 / 1003）、撤销 204。请求到达极光，非真实送达。
- Go push race 和 Harmony 登记、超时重试、撤销定向测试通过；Harmony 6.1.1 / API 24 签名 Release APP 编译及官方验签通过，产物 `.local-tools/harmony-apptest/signed/WhaleRemoteControl-1.0.5-harmony-push-signed.app`，SHA256 `3f9881356af8b62e0000a834208afbcc6512678b5e29d8bfd178a5b036c72128`。版本仍为 1.0.5，未上传发布。
- 华为“添加SHA256公钥指纹”弹窗显示 `ClaudeRemoteControl-Mac-Release-1028` 发布证书已登记；项目此前核实该证书匹配本机 team-1028 私钥及发布 Profile，本次 APP 签名和 Profile 验证通过。后台公钥指纹与本机证书摘要是不同对象，无需按数值不同判断证书不匹配；本次未修改已有指纹。
- 未进行真实设备的注册、厂商后台通知、冷启动点击、关闭及撤销测试。发布 APP 仍为 1.0.5，实际分发是否接受同版本须由平台决定。
