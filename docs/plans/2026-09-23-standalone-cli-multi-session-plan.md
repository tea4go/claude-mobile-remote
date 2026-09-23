# Standalone CLI 多会话管理实施计划

> 对应设计：`docs/specs/2026-09-23-standalone-cli-multi-session-design.md`。用户已检查并要求实施；具体产品语义由 D70/D71 确认。代码阶段正在验证，尚未完成真机门禁。

## M1：协议与运行时边界

- 确认创建授权、默认容量和 PC 终端管理语义；记录最终三端共用契约决策。
- 在现有 v2 wire contract 上增加按可选 Pairing 能力字段启用的管理消息，给 Go 公共协议及 React/Capacitor、HarmonyOS NEXT 增加严格解码，保留既有 Session 内命令与事件语义；验证不带能力字段的旧配对记录仍可使用。
- 先写契约测试：未认证/未订阅拒绝、跨 Session 拒绝、创建结果只回复请求连接、Token 不进入广播或错误、同 `commandId` 幂等。

## M2：CLI 多 Session Runtime Manager

- 固定启动目录；抽出每 Session 的 Host、Controller、Pairing Manager 与生命周期对象。
- 使用一个 Listener、认证分发和 Channel Registry 服务多个 Session；原公共 `remote.Server` 保持单 Session API。
- 完成原子创建、容量/限流、停止单个、进程全量关闭、失败回滚及独立凭证文件；增加并发、race 与故障注入测试。

## M3：移动端与 PC 终端

- React/Capacitor 在现有 App Store/Runtime 上接入“新建会话”和“停止此会话”；收到完整新 Pairing 后复用原安全存储与多连接逻辑。
- HarmonyOS NEXT 接入等价消息、结果和操作入口；各端不引入专有配对字段。
- 人工体验反馈后调整详情：保留基于当前配对入口的新建能力，但将按钮移至页面最底部；终止操作使用清晰的危险操作文案与说明，明确关闭 Runtime、原 Session 不可继续输入、配对入口仍保留。列表与终态分区保持原状；验证终态详情仍可新建。
- 后续入口优化：CLI 配对详情将新建入口移到输入区左下角 `+`；React/Capacitor 附件入口改为 `document.svg`，WhaleTerm 配对仍只有附件而无新建入口；两端详情暂隐藏终止入口。验证能力门控、终态可新建、创建中禁用及附件入口不回归，不改协议和 PC 管理命令。
- CLI 终端增加会话列表、目标切换、新建/停止及配对信息重显；普通文本保持仅进入选中 Session 的队列。

## M4：门禁

- Go 新逻辑测试、`go test -race` 定向、`gofmt`、`GOFLAGS=-buildvcs=false go build ./...`（按项目既有 Go module 基准）；移动端 Vitest、lint、build 与 Harmony 工程可执行验证。
- 双 Session 真机/浏览器联调：A 运行中创建 B、同目录、双端交错输入、独立权限/取消/停止、容量、幂等、凭证隔离与进程退出。
- 将实际结果写入 `docs/testing/` 与 `HANDOFF.md`，记录未覆盖项；Git 写操作仍须用户另行明确授权。
