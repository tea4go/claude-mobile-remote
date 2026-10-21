# WhaleTerm 配对展示完善验证

- Remote 现有 node 定向检查：23/23 通过（多数为源码约束检查，不等同交互测试）。
- WhaleTerm `npm run build` 通过，保留既有 Vue/CSS/import/chunk 警告。
- 移动端 Vitest 476/476、build、cap sync ios 通过；本轮仅增加设置页保存说明，未重新编译原生。
- 两仓库 `git diff --check` 通过，无 Git 写操作。

## 待人工验收

1. 新建本地 Claude 会话，开启 Remote，输入手机可达的 LAN/VPN 地址，启动后自动弹出二维码并扫码连接。
2. 关闭弹窗，再点查看二维码；切换 PC 会话后不显示前一个会话的配对。
3. 有效期届满时二维码隐藏，刷新跟随服务端轮换；关闭 Remote 后手机断开，PC 会话继续使用。
4. 手机设置查看本地保存说明，重启 App 验证已配对设备与名称恢复；消息历史不承诺跨重启恢复。

## 移除遗留地址输入

启动入口已直接调用 Start；后端自动选择 VPN/LAN IPv4，无可用地址明确失败。Remote 前端检查22/22、TestRemoteAddressSelection 与 TestRemotePairingResponse 通过，diff --check 通过。完整桌面构建结果见 HANDOFF 当前执行；实际扫码及多网卡可达性待用户验收。未执行 Git 写操作。
