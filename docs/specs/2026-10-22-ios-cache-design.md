# iOS 10-22 本地缓存与历史读取预研

依据用户要求：第 4 项最后处理，不额外使用数据库，先预研开源 Remote Control 项目。沿用 D80 自主实施。

## 源码观察（2026-10-22）

- [Happy persistence.ts](https://github.com/slopus/happy/blob/main/packages/happy-app/sources/sync/persistence.ts) 使用 MMKV 保存设置、草稿等；[sync.ts](https://github.com/slopus/happy/blob/main/packages/happy-app/sources/sync/sync.ts) 在消息同步中请求 `/v3/sessions/{id}/messages?before_seq=...&limit=100`，随后按 seq 获取增量及更早消息。值得借鉴的是列表/详情数据分离、分页与增量同步；不是进入详情时启动第二个 Claude Client。
- [HAPI messages.ts](https://github.com/tiann/hapi/blob/main/hub/src/store/messages.ts) 使用 bun:sqlite，按 session_id 和 seq 分页，上限 200。说明查询历史需要有明确的消息来源与排序边界；整套数据库方案不符合本轮约束。
- [Claude Code UI provider](https://github.com/siteboon/claudecodeui/blob/main/server/modules/providers/list/claude/claude-sessions.provider.ts) 通过 DB 中的路径定位本机 JSONL，读取后归一化，再取尾页。它承担全量项目/会话历史管理，范围大于本项目。这里不复制其私有 JSONL 解析，也不扩大已配对 Session 的权限。
- [Claw](https://github.com/raullenchai/claw/blob/main/README.md) 主要读取 tmux pane 并周期刷新，不提供本项目需要的结构化历史缓存语义。
- 实际 go.mod replace 指向 `/Users/zhangym/code/work/utils/claude-agent-sdk-go`，其 session.go 已有 `GetSessionMessages`、`GetSessionInfo` 及 `WithSessionLimit/Offset/Directory`，可只读查历史，无需第二 Client。初次检索的另一个同名目录不是当前依赖，已更正。SDK 支持使后续历史分页可行，但仍需按已配对 sessionId 授权、消息映射/游标和扫码前历史范围设计；本轮先解决跨重启丢失已见内容。

## 本轮选择

采用原生私有文件缓存已收到的展示数据：每 Session 一个 JSON 文件，无新数据库、无新增 npm 依赖。列表所需目录/标题/时间与消息一起保存，配对、PC 名称与本地重命名沿用现有元数据存储。先从文件恢复，再连接最新 snapshot。不会取扫码前全量历史，也不会补造 App 关闭期间未收到的事件。PC 历史查询保留为独立后续方案，需 SDK 支持及显式授权范围/分页设计。

缓存 version 1，包含 sessionId、savedAt、projectCwd/sessionTitle/createdAt/lastActivityAt 与最多 200 个展示消息；单文件不超过 2 MiB，总量 20 MiB，30 天清理。超限先保留最新消息；不保存图片/附件 base64、PairToken、连接状态、pendingAction/Command 或可执行审批。工具块和已回答结果可作为不可操作的历史展示保留。

恢复时清除运行/权限/队列状态，显示本地缓存提示；原 streaming 文本恢复为静态内容，详情 Turn 标记“待同步”，连接后 snapshot 用 requestId 校正当前 Turn。缓存不是 Claude 上下文，不参与 SDK 输入或授权。浏览器继续仅使用内存，Android/Harmony 原生实现待 iOS 验收后同步；等价文件能力均可实现，不改 wire contract。

iOS 通过现有 SessionExperience 原生桥接访问 Application Support 专用目录：文件名为 sessionId 的 SHA256；原子替换、系统文件保护、排除 iCloud/iTunes 备份、串行 I/O。只读取仍在本机配对列表内的 Session 缓存；删除配对同时删除缓存。设置提供清除离线缓存，失败不阻止连接但明确提示。强制杀死可能丢失最近一秒未写入的增量；系统损坏/淘汰后依然可连接获取当前状态。

## 验证

覆盖隔离/损坏/过期/数量和字节上限/去除附件凭证/静态恢复/重连校正/写入节流/删除与清除竞态。Swift 实际编译，移动全套 test/build，最终同步 iOS。真机杀死重开、断网列表/详情、双 Session 与清理由用户验收。
