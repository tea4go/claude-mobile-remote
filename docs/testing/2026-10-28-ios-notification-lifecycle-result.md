# iOS 通知生命周期最小修复（2026-10-28）

用户确认精简修复；不构建 App/WhaleTerm，不部署 edo，不执行 Git 写操作。

## 修复与边界

- Runtime 原先相同配对已有 entry 即返回，断开也不重新连接；现先检查存活，断开时重连，连接中和已连接继续复用。通知点击与详情共用入口。
- iOS 原先仅远程前台抑制，本地事件仍可能显示提醒；现前台两类均禁止系统提醒。PC/edo仍可能发送，iOS决定是否展示，不增加在线状态同步。
- 删除原有订阅同步与尽力 DELETE 保持。通知异步查询改用提前捕获的 topic 集合，JS 等待原生清理入口返回后再移除配对。已投递通知/离线撤销不能保证即时撤回；旧通知不能恢复已删除会话。

## 验证

- `npm test -- src/App.test.tsx src/App.native.test.tsx src/native/experience.test.ts src/protocol/client.test.ts src/debug/connection.test.ts`：5文件101项通过。
- 新增断开后通知点击重连、重复点击不重复建连及审批展示；删除后空订阅与旧点击不打开/重连回归。
- `npx tsc -b --noEmit` 通过。
- `xcrun swiftc -frontend -parse apps/mobile/ios/App/App/SessionExperiencePlugin.swift apps/mobile/ios/App/App/SessionPushManager.swift` 通过；仅语法检查，不代表链接和签名构建通过。
- `git diff --check` 通过。
- 初轮扩大回归因等待空通知端口改变浏览器删除时序出现2项失败；限定有通知端口时等待后，以上101项全部通过。

## 人工验收

1. 后台及退出后点击新通知，进入正确详情并自动连接，展示审批/AskUserQuestion。
2. 前台停留列表或其他会话不显示系统通知；后台仍正常提醒。
3. 联网删除会话后触发新事件，检查不再通知；旧通知点击不能恢复已删除会话。已进入APNs的消息和断网撤销不保证立即生效。

2026-10-28：用户反馈“验证通过”，并授权连同 Harmony 相关修改提交；未逐项提供设备与操作结果，以上清单不补记为逐项实测证据。本次提交不重新构建、不部署 edo、不推送。
