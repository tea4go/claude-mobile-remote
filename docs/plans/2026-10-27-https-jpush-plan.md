# E3 iOS 极光实施计划

1. 登记当前执行、专项设计和用户确认决策，保留其他平台同期记录。
2. 实现HTTPS sender、edo handler/服务、私有配置与部署模板；宿主默认出口改为edo。
3. 恢复iOS SDK/APNs、tag同步、冷启动点击映射和设置状态；不改其他原生端行为。
4. Go/race、移动测试和Web/iOS构建、Linux双架构打包，记录实际验证及平台缺口。
5. 交付部署和极光/APNs清单；配置与签名齐备后真机验收，不自动发布或提交。

## D102 故障修复增量

1. 保留现有 PC 事件 HTTPS POST 和固定通知文字；补充安全的状态码诊断。
2. 在同一精确 HTTPS 路由增加手机 RID 登记/撤销，edo 私有持久化受限 topic→RID 映射，极光按 RID 发送；不增加公网端口。
3. iOS 改为登记 RID，保留 topic 本地点击定位；启用状态只在登记成功后显示就绪。
4. 只运行相关源代码测试与静态检查；按用户要求不编译 WhaleTerm 或 App，由用户自行构建和真机验证。

## 通知点击修复（2026-10-27）

用户已验证系统通知送达。Capacitor 初始化会覆盖 AppDelegate 中提前安装的通知代理，因此在自定义插件 load（Bridge 初始化之后）重新安装 SessionNotificationRouter；Scene 冷启动 connectionOptions.notificationResponse 也交给同一个点击入口。入口继续只解析本地有效 topic 映射，事件通过 retainUntilConsumed 等待 Web 配对恢复后导航，不改变协议、配对或 edo。验证后台点击、冷启动点击及不存在的本地 Session 不串线；本轮不编译 App。

## 2026-10-28 最小修复

- 已有连接对象若已断开，打开详情时重新连接；正在连接或已连接不重复建连。
- iOS 前台同时禁止本地与远程系统提醒，后台沿用 APNs；不增加服务端前后台状态同步。
- 清理通知先固定当前 topic 映射，避免删除配对后的异步读取漏清理；沿用 DELETE 重试，已投递/离线撤销无法即时保证。
- 定向回归、类型及 Swift 语法检查；不构建 App/WhaleTerm，不部署 edo，不执行 Git 写操作。
