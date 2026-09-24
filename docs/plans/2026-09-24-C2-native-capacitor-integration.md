# C2-C Capacitor 原生能力接线计划

依据：`docs/specs/2026-09-24-C2-native-capacitor-integration-design.md`。

1. 固定 Capacitor 8 兼容插件版本；配置 Android 最低 SDK、iOS 相机用途说明和 Preferences 隐私声明，并禁用会输出 Pair Token 的 Capacitor 桥接日志。
2. 实现系统安全存储、普通元数据、扫码及前台事件的原生端口；设置原生平台门控，禁止浏览器实例化持久化端口。
3. 将原生恢复与多 Session Store/Runtime 连接；补齐无凭证占位、扫码入口、保存/重命名/删除/最近连接持久化，并避免原生 URL Fragment 留存 token。
4. 使用假插件端口覆盖关键流程及浏览器路径回归；执行 Vitest、lint、Web 构建、Capacitor sync 和环境可用的原生构建。
5. 更新 C2 验证记录与 HANDOFF，准确列出模拟器和真机/VPN 未验证项。设计文档交用户检查，Git 写操作另等明确授权。

### iOS 顶部安全区修复（真机反馈）

1. 为 WebView viewport 启用整屏布局，建立顶部导航共用安全区样式；让导航背景铺至屏幕顶端，标题和按钮按顶部及横向安全区定位。
2. 将详情、列表、配对导航和启动恢复提示接入该样式，保留详情 `100dvh` 高度与底部安全区。
3. 跑移动端测试、lint、构建和差异检查；请用户在 iOS 真机纵横屏核对导航遮挡和底部输入区，人工结果再写入交接记录。

通过条件：有效原生凭证跨 App 启动可恢复且按 Session 隔离；失效占位不连接；浏览器不持久化 token；本机可执行门禁通过。
