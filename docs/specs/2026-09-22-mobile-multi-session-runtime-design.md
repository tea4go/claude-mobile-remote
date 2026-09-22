# 移动端 App 级多 Session 同步设计

- 日期：2026-09-22
- 状态：架构方向与 Zustand 选型已确认，待用户检查修订稿
- 关联决策：D41、D63、D64、D66、D67

## 1. 目标

用户配对多个 Claude Session 后，移动端在 App 前台生命周期内持续接收并保存每个 Session 的新增事件。列表与详情只是同一份 App 级状态的不同视图；返回列表或切换详情不得停止其他 Session 的同步。

本阶段解决：

- 列表页期间继续接收所有有效配对 Session 的事件；
- 再次进入详情时保留 App 生命周期内已收到的消息；
- Turn 在其他页面结束后，详情与列表及时清除运行/停止状态；
- 多 Session 的消息、权限、待处理事项、命令结果和连接状态严格隔离。

本阶段不实现数据库、完整历史、事件 replay、系统后台常驻、系统推送、机器身份或机器分组。

## 2. 参考实现结论

Happy App 的应用级 `Sync` 单例不依赖详情页接收事件，状态和同步游标均按 Session 分桶；详情页只消费 Store 并触发补拉。其断线补偿还依赖服务器持久化消息和按 `seq` 增量查询，本项目当前没有该能力，因此只借鉴“同步生命周期属于 App、状态按 Session 隔离”的部分，不复制云端历史架构。

## 3. 状态与连接所有权

```text
SessionRuntimeManager（网络与副作用）
├── clients: sessionId → ProtocolClient
├── generations: sessionId → number
└── connect / reconnect / close / closeAll
                         │ dispatch
                         ▼
Zustand Session Store（可观察内存状态）
├── pairings: sessionId → SessionRecord
├── sessions: sessionId → MobileState
├── unread: sessionId → boolean
├── currentSessionId
└── view
        ├── SessionList selectors
        └── SessionDetail selector(currentSessionId)
```

- Zustand Store 是 UI 状态的唯一来源；不再由 `App.tsx` 同时维护一套 `useState`/`useRef` 状态副本。
- `SessionRuntimeManager` 是连接副作用的唯一所有者；`ProtocolClient` 不作为组件订阅的响应式对象。
- 一个配对记录最多对应一个活动 `ProtocolClient`；重复渲染或进入详情不得创建重复连接。
- Client 创建时绑定不可变的目标 `sessionId`；其 Snapshot、事件、关闭原因和命令结果只能更新该 Session 的状态。
- `currentSessionId` 只表示当前展示目标，不参与连接存续判断。
- 连接和 Zustand Store 位于 App 级，不由 `SessionDetail` mount/unmount 创建或销毁。
- 删除记录、配对失效或 App 卸载时先让回调失效，再关闭 Client，避免旧连接回调污染新状态。
- 单 Session 的状态变化继续调用现有纯 `reducer(MobileState, Action)`；Runtime 和组件不得复制 Snapshot/Event/Command 的归并规则。
- Store 使用 selector 订阅单个 Session 或列表摘要；Session A 的流式增量不得要求 Session B 详情组件重新渲染。

## 4. 生命周期

### 4.1 配对与导航

1. 新配对校验通过后 upsert `SessionRecord`；
2. Runtime 确保该 `sessionId` 只有一个 Client，并连接 `wsUrl`；
3. 返回列表只切换视图；
4. 进入详情只设置 `currentSessionId`；
5. 重新配对同一 Session 时，若连接参数变化则以新的 generation 替换旧 Client，旧回调失效；
6. 删除配对时关闭对应 Client 并删除对应内存状态。

### 4.2 Snapshot 与终态校正

Snapshot 继续只表达当前或最近 Turn，不承担历史回放。收到同一 Session 的 Snapshot 时保留已经接收的消息，并校正 Session 状态、权限、队列、待处理事项和对应 Turn：

- Snapshot 表明 Turn 已完成、失败或取消时，必须把缓存中同 request ID 的流式消息更新为终态；
- Snapshot 没有活动 Turn 时，不得继续保留由旧连接推导出的可停止状态；
- Snapshot 不包含的已结束回复正文不得伪造或清空已有正文。

### 4.3 前后台边界

App 前台且 JavaScript 正常运行时维护所有连接。操作系统挂起 WebView、杀死进程或网络断开后，WebSocket 可能停止；当前协议没有 replay，恢复时只能通过最新 Snapshot 收敛可表达状态。系统推送和完整断线补偿另行设计，不能作为本阶段已解决能力。

## 5. 列表状态

列表至少从每个 `MobileState` 派生：连接状态、Session/Turn 状态、等待决策、错误和未读。未读只表示该 Session 在非当前详情时收到新的可见事件；进入详情后清除。列表状态不得通过额外服务器聚合接口取得。

## 6. 多机器兼容边界

当前 Pairing 只有 `wsUrl`、`sessionId`、`pairToken` 和有效期，没有稳定机器 ID。不同机器的 Session 已可通过各自 `wsUrl` 独立连接，但本阶段不按机器分组，也不使用 IP、端口或 hostname 伪造机器身份。未来增加稳定 `machineId` 和可选 `machineName` 时单独更新配对契约，并保证旧二维码进入未分组集合、新客户端兼容缺失字段。

## 7. 三端可实现性

- React Web/Capacitor：Zustand 保存可观察状态，独立 Runtime Manager 持有 Clients 与 generation；不启用 persist middleware；
- HarmonyOS NEXT：ArkTS 使用等价 Map 和状态容器；
- 两端都只依赖现有 Pairing、WebSocket、Snapshot/Event/Command 契约；
- 不新增浏览器 API、数据库或平台专属 wire 字段。

## 8. 存储与安全

- Zustand 只保存本次 App/页面生命周期内的内存状态；
- 禁止使用 Zustand `persist` middleware；
- 浏览器不得把 Pair Token、消息正文或 Runtime 状态写入 Web Storage/IndexedDB；
- 原生端未来跨重启保存 Pair Token 仍通过系统安全存储桥接，不由 Zustand 决定持久化介质；
- Store 调试与错误日志不得输出完整 Pairing 或 Pair Token。

## 9. 验收标准

1. 配对 A/B 后，两者同时连接；在 A 详情、B 详情和列表三种页面下，两者事件均归入正确状态；
2. 返回列表不调用任一有效 Client 的 `close`；切换详情不创建重复 Client；
3. B 在用户查看 A 时完成 Turn，列表更新 B 状态，进入 B 后回复仍在且停止入口消失；
4. A/B 同时流式、权限变化、待处理事项和取消互不交叉；
5. 删除 A 只关闭并清理 A，不影响 B；重新配对 A 不接受旧 generation 的延迟回调；
6. App 卸载关闭全部连接；刷新/销毁后不恢复消息正文；
7. 不修改 Go Remote Server、Channel、wire protocol 或 Pairing 字段。
8. selector 测试证明一个 Session 更新不会通知只订阅另一个 Session 状态的消费者；依赖中不存在 Zustand persist 配置。
