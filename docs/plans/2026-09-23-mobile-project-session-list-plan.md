# 移动端项目分组与会话可识别性实施计划

> 对应设计：`docs/specs/2026-09-23-mobile-project-session-list-design.md`。用户已确认设计，按 L1–L4 实施。

## L1：共用契约与兼容性门禁

- 固化 v2/v3 连接协商、v3 Snapshot/Event 可选字段、RFC3339 时间、标题长度上限和订阅授权；先写旧客户端↔新服务、新客户端↔旧服务与新客户端↔新服务的兼容测试。
- 在 Go 公共协议、Transport、CLI Runtime 与 WhaleTerm Remote Host 提供只读展示信息。v2 wire 输出保持原样，v3 只向该已鉴权订阅的 Session 连接输出元数据；覆盖跨 Session、过期凭证和终态。
- 更新 React/Capacitor 和 Harmony 严格解码与 `unsupported_version` 回退；配对二维码与 Pair Token 规则不变。任何双版本互操作失败都先修订设计，不推进 UI。

## L2：宿主展示信息

- CLI 从固定 `cwd` 和 Session Controller 维护创建时间、首次输入标题与最近有意义活动时间；停止后保留终态展示信息。
- WhaleTerm 从 Session 元数据的 `ProjectPath` / `CreatedAt` 和所属 Remote Host 事件取得等价信息；历史首条输入不可得时按回退规则处理，不跨 `chatKey` 串线。
- 首行提取、空白规范化、Unicode 截断、超长路径与摘要、手动标题覆盖均配聚焦测试；`--resume` 缺原始创建时间时不得伪造创建时间。

## L3：列表状态与 UI

- React/Capacitor App Store 按 `sessionId` 保存内存展示信息和最近查看 ID；按设计 §3 计算稳定的项目组与排序。`SessionList` 改为 antd-mobile `List`，展示标题、路径、预览、时间和状态，并保持重命名/删除与配对入口。
- Harmony App Store、分组算法和 ArkUI 列表实现相同语义；UI 使用 ArkUI 等价组件，不依赖 antd-mobile。
- 返回列表恢复刚查看位置；前台列表打开期间冻结行序，但内容状态继续更新。手机不把配对凭证、标题或消息正文存到浏览器持久存储。

## L4：验证与交接

- Go 定向与 race 测试、共享协议测试、React Vitest/lint/build、Harmony 工程构建；本机缺工具链时如实记录并在 DevEco 环境补门禁。
- 人工验收 CLI、WhaleTerm 不同目录/同目录、多会话运行及重连、自动标题/手动名称、列表返回定位、旧配对兼容。
- 更新 `docs/testing/` 和 `HANDOFF.md` 实际结果。Git 写操作仍需用户明确授权。

## L3 列表紧凑化补充（用户确认）

- 仅改 React/Capacitor `ProjectSessionList`：antd-mobile `List.Item` 保留双行核心内容，右侧仅放关键状态、活动时间与未读；重命名/删除改为“更多”菜单，整行保持可进入。
- 列表专属 CSS 独立维护，删去组件内布局/颜色内联样式；更新旧列表操作及分组用例，验证排序、预览、菜单操作、凭证不展示。运行 Vitest、lint、build，人工验收紧凑度。
