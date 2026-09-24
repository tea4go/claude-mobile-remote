# C2-C 原生配对逻辑准备实施计划

依据：`docs/specs/2026-09-24-C2-native-pairing-logic-design.md`。本轮只实施跨平台纯逻辑和测试。

1. 在 `apps/mobile/src/native/` 定义安全 Pairing、普通元数据、原生扫码与前台事件的注入端口、协调器和扫码输入入口。只使用既有配对解析器；所有存取由调用方注入，不提供 Web Storage 默认实现。
2. 完成恢复、扫描保存、重命名、删除以及过期/错绑/损坏/存储失败的行为。安全存储先删、元数据后删；元数据先存、安全记录后存。
3. 增加纯函数前台恢复计划：有效记录刷新连接状态，过期记录需重新配对，不自动创建 WebSocket。
4. 使用内存测试端口验证上述行为与跨 Session 隔离；运行 `npm test`、`npm run lint`、`npm run build`、`git diff --check`。
5. 更新 `HANDOFF.md` 和本轮验证记录。原生 Keychain/Keystore、元数据适配器、扫码插件、App 状态监听、真机/VPN 属于后续接线与门禁。

通过条件：浏览器运行路径不调用新端口；普通元数据不含 Pair Token；无设备自动化通过；明确标记未实现的原生接线。
