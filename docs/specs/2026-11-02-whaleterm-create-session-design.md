# WhaleTerm 同目录远程新建会话设计

- 日期：2026-11-02
- 需求与归属：用户已确认新会话由 WhaleTerm 管理，并显示在 PC 会话列表；优先于当前发布/推送任务。
- 对应决策：D108。

## 行为

WhaleTerm Claude 列表的目录操作新增“启动远程 CLI”入口。使用该目录及当前宿主默认配置，先预留新 UUID 和配对服务、展示二维码；手机完成认证、订阅并收到快照后，才把这条新会话加入 PC 的 Claude 列表。放弃扫码不会在列表留下空会话。首条 PC/手机消息由 WhaleTerm 唯一 Manager 启动 SDK Client。原有会话继续运行。

既有与新增 WhaleTerm 配对均可声明 create_session。手机 + 复用现有请求/结果和各端 UI，在发起会话已固化的 cwd 创建新会话；新会话返回独立配对并可继续新建；手机 + 的成功响应即触发 PC 列表新增。旧配对不含能力时重新扫码升级。手机不能指定路径，不提供服务器端移动 Session 列表。

## 实现

公共 remote.Server 仍只注册和认证一个 Session。新增可选 SessionCreator 回调（只创建，不附带停止权）；Transport 鉴权/订阅成功后转交宿主。成功结果只发给请求连接，外层仍为源 sessionId，新 Pair Token 不广播。无回调保持旧行为；首张、恢复和轮换 Pairing 均保留能力。

WhaleTerm TClaudeRemoteService 保存各 Session 创建种子（只驻内存，含完整配置），创建走唯一 Manager，并为每个新 Session 建独立 Remote Server/端口。agentId/chatKey 使用 PC 本地会话既有 claude-local:<sessionId>，PC 打开时不重绑定到其他 Host。失败清理新 Adapter/Server 和未启动草稿，不影响源会话。关闭某个 Remote 不关闭 Claude 或其他 Session；WhaleTerm 退出仍走 Remote→Manager 顺序。

端口完全复用现有 `backend/remote/Registry.Start`：从基准端口开始，遇 EADDRINUSE 递增重试，并同步修改配对地址。不另建端口分配器。关闭目录二维码窗口只隐藏窗口，待配对草稿仍不进入列表；同目录再次打开复用待配对服务，避免关窗与刚完成配对竞争。退出应用时按既有生命周期关闭服务。

### 既有能力复用审查

| 能力 | 复用位置 | 本轮补充 |
| --- | --- | --- |
| 自定义规范 sessionId、唯一 Client、FIFO | WhaleTerm Manager / resolveSessionForRequest / CreateClaudeCodeChat | 只预生成 UUID，交现有 Host 预留；不重写 SDK 生命周期或队列 |
| 多 Session 独立端口、地址同步和停服隔离 | WhaleTerm remote.Registry / Service | 原样复用；已删除重复的临时端口分配实现 |
| 配对凭证、二维码、续期及鉴权订阅 | pairing / remote.Server / transport / ClaudeRemotePairingContent | 只加订阅成功的宿主通知 |
| 手机 +、创建请求/结果、三端保存和连接 | 既有 transport 管理消息、React 与 ArkTS 实现 | 不新增手机协议、页面或存储实现 |
| PC 列表、会话标识、详情和实时消息 | ListClaudeLocalSessions / GetClaudeLocalSessionDetail / localClaude store / W1 监听 | 仅合并首轮前无 JSONL 的已配对草稿及触发刷新 |
| 新建事务归属 | Standalone managedCLI 只管理自身 Controller，不能创建 WhaleTerm Manager 的 Session | 公共可选 SessionCreator 转交 WhaleTerm；宿主内保留自身幂等、限流及失败回滚，不复用 CLI 的生命周期管理器 |

创建串行化，按源 Session/commandId 有界缓存最近 128 个结果，断线重试不重复创建；源会话每秒最多一次。沿用 CLI 默认容量：每个 cwd 最多四个由本功能新增且仍由 Manager 持有的会话（已有源会话不占新增名额），总保留 128 个；Manager 已释放项不计名额。关闭 Remote 本身不释放仍由 Manager 持有的会话名额。固定错误码表示容量/限流/不可用，不向手机发送宿主原始错误。

新会话在 手机成功完成订阅后 PC 本地列表才可见，包括首轮前没有 JSONL 的草稿。只向本机 Wails 发不含凭证/配置/正文的变更事件，前端刷新现有列表；后端把存活草稿与磁盘列表按 ID 合并去重，首轮前详情返回空展示模型。该展示补充不用于读取/接管活跃 JSONL；执行始终走 Manager。普通历史加载规则不变。

知识库作用域会话需独立的绑定创建事务，本轮不擅自复制或降级其安全范围；此类新建明确拒绝，原远程接力保留。

## 三端与验收

沿用既有 v2–v6 JSON 管理请求与 Pairing v1 的 create_session 字符串能力；React Web、iOS/Android 和 ArkTS 已具备解析/新建路径，无平台字段及存储变更。不声明 stop_session，不扩大手机停止权。

门禁覆盖：首张/轮换能力、无回调兼容、未认证/未订阅拒绝、同目录与配置复制、UUID/队列隔离、同 commandId 幂等、容量/限流、失败回滚、PC 列表及首轮前详情、关闭原 Remote 后子会话独立。Go race/vet/build 和 WhaleTerm 定向 Go/前端测试及构建；真实 PC 与三端扫码、并行 Turn、审批、关闭/重开由用户验收。

2026-11-02 用户验收修订：目录扫码及手机 + 新建已由用户复测通过；新建完成后 PC 需复用既有 selectClaudeLocalSession 打开、touchClaudeLocalSession 更新最近活动，继续使用原目录排序与激活后展开/定位。不再创建前端草稿或分配新 ID；本机事件增加首次创建标记，普通 Turn 活动仍只刷新，防止抢焦点。不改变三端共享契约。
