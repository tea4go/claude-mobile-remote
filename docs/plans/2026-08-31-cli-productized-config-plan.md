# CLI 产品化配置实施计划

## 任务 1：profile 配置解析

- 增加 `local`、`emulator`、`device`、`debug` profile；默认 `local`。
- `device` 解析可达 IPv4，并支持高级 `--advertise-host` 覆盖。
- profile 生成的网络配置允许 debug 参数显式覆盖。
- 先补配置测试，覆盖默认值、四种 profile、非法 profile 和覆盖优先级。

## 任务 2：常用 Claude 参数

- 增加 `--permission-mode`、`--model`、`--max-turns`。
- 在 `internal/claude` 通过现有 Option 边界映射到 SDK。
- 补 Host/CLI 测试，确认新建与 resume 均使用同一组参数。

## 任务 3：文档与验证

- README 分为开发版和调试版，更新模拟器、真机 LAN/VPN 示例。
- 更新 HANDOFF 和 `docs/testing/` 验收记录。
- 执行 Go 格式、测试、构建、vet；确认不改变 Pairing/WebSocket 契约。
