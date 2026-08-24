# A4 Channel 通信设计

- 日期：2026-08-24
- 子阶段：A4 Channel 通信（里程碑 A）
- 状态：已确认，待实施
- 依据：`REQUIREMENTS.md` §5.1、§9.1，总体设计 §4–§9，D10、D12、D16、D19、D24、D25、D27、D28

## 1. 目标

A4 在 A3 `SessionController` 之上建立可由移动客户端复用的 Session Channel 和 WebSocket 传输边界。客户端使用短期 Pair Token 完成认证，显式订阅唯一 `sessionId`，先取得原子 snapshot，再持续接收增量事件并发送文本命令。

A4 完成后，测试客户端可以通过真实 WebSocket 完成“认证 → 订阅 → snapshot → 文本命令 → command result → Turn 事件 → 断线重连后最新 snapshot”的闭环。A4 不实现产品移动页面；A5 的 React Web、后续 Capacitor 和 HarmonyOS NEXT 均消费这里确定的同一协议。

## 2. 范围边界

### 2.1 A4 实现

- `internal/pairing` 的短期 token 签发、校验、自然过期和主动撤销；
- 可供二维码或链接编码的纯 JSON 配对负载，不生成二维码图片；
- `internal/channel` 的 session 校验、每连接独立原子订阅和手机文本命令路由；
- `internal/transport` 的版本化 JSON 协议、WebSocket Server、鉴权和连接生命周期；
- 每连接有界提前事件缓存和网络发送队列；
- `command_result` 与同一 request ID 的 Turn 事件顺序门控；
- 多个已认证连接订阅同一单 Session，慢连接相互隔离；
- 默认 loopback 监听、显式 VPN 监听地址和对外地址配置；
- 自动化 WebSocket 集成测试和最小 Go CLI 协议测试客户端。

### 2.2 A4 不实现

- React 移动 Web 页面、二维码图片、摄像头扫码或真机 VPN 联调；
- 审批、回答、取消和权限交互；
- 完整 transcript、事件历史、断线增量重放或跨连接命令幂等；
- 心跳定时器、自动重试、公网 relay、TLS 终止、账号体系或长期客户端凭证；
- 多 Session、多机器、网络接口枚举或 VPN 地址自动探测。

## 3. 方案选择

采用“每个已订阅连接持有一个 Controller 原子订阅”的方案：

- Channel 为每个连接调用一次 A3 `Subscribe`，原子取得 snapshot 和后续事件流；
- Channel 的事件泵立即消费 Controller 事件并写入该连接的有界队列；
- 网络 writer 只消费连接队列，慢客户端不会反压 Controller 或其他连接；
- 连接关闭或溢出时立即取消自己的 Controller 订阅；
- 每个连接独立拥有 D25 所需的命令结果顺序门控。

未采用的方案：

- 单 Session 上游订阅后广播：连接数较多时更节省订阅，但 A4 必须额外维护 snapshot reducer，并处理 snapshot/订阅竞态，重复 A3 的状态真相；
- 有界事件环与断线 replay：恢复能力更强，但超出 A4“重新订阅最新 snapshot”的范围，留给 B4；
- 网络 socket 直接消费 Controller：慢网络会把背压传入可信进程内订阅，违反 A3 的消费约束。

A4 首期连接数很小，每连接一个进程内订阅的成本可接受，换取了最简单且无缺口的状态边界。

## 4. 组件与依赖

```text
internal/pairing/
├── manager.go          # token 签发、校验、轮换、撤销和连接通知
├── payload.go          # 三端共用的纯数据配对负载
└── *_test.go

internal/channel/
├── channel.go          # session 校验、Controller 订阅和文本提交
├── subscription.go     # 连接级事件泵与取消
└── *_test.go

internal/transport/
├── protocol.go         # envelope、payload、稳定错误码和字段校验
├── server.go           # HTTP/WebSocket listener 与 Origin 策略
├── connection.go       # reader、writer、状态机和有界队列
└── *_test.go

cmd/claude-mobile-remote/
└── run.go              # 组合 Controller、Pairing、Channel 和 Transport

cmd/channel-test-client/
└── main.go             # A4 人工验收用的最小协议客户端
```

Transport 使用 `github.com/gorilla/websocket` 处理 WebSocket upgrade、帧读写和测试客户端。协议结构和 Channel 接口不暴露该依赖，未来更换传输或增加 relay 时不修改 Session Core。

职责边界：

- Pairing 不读取 snapshot，也不处理 WebSocket；
- Channel 只接收强类型操作、复用 Controller，不解析 JSON；
- Transport 管理 JSON、鉴权和连接资源，不调用 Claude SDK；
- `cmd` 只负责生命周期编排和用户可见的配对信息；
- A2 Adapter 和 A3 Controller 不依赖网络、token 或 WebSocket。

## 5. 协议 envelope

协议版本 1 使用固定顶层字段：

```json
{
  "version": 1,
  "type": "command",
  "sessionId": "claude-session-id",
  "commandId": "client-generated-id",
  "requestId": "",
  "seq": 0,
  "payload": {}
}
```

规则：

- 每条消息都必须包含受支持的 `version`、非空 `type` 和非空 `sessionId`；
- `commandId` 只用于 `command` / `command_result`，由客户端生成；
- `requestId` 由 PC Controller 为成功接受的工作生成，并出现在成功的 `command_result` 和对应 Turn 事件；
- `seq` 只在 `snapshot` 和 `event` 中有业务意义；snapshot 携带基准 seq，后续事件必须大于该值；
- 不适用的 ID 使用空字符串，非 snapshot/event 消息的 `seq` 使用 0；
- `payload` 始终是 JSON object，不用平台专属的二进制或对象语义；
- 文本沿用 A3 的 1 MiB 上限；Transport 的完整 JSON 消息上限必须为 envelope 留出固定余量，超限连接按协议错误关闭。

基础消息：

| 方向 | 类型 | 作用 |
| --- | --- | --- |
| Client → Service | `hello` | 提交 `pairToken`，开始认证 |
| Service → Client | `hello_ack` | 确认认证成功，不回显 token |
| Client → Service | `subscribe` | 显式订阅已授权 `sessionId` |
| Service → Client | `subscribed` | 确认订阅建立 |
| Service → Client | `snapshot` | 返回原子订阅取得的当前状态 |
| Service → Client | `event` | 推送 A3 增量领域事件 |
| Client → Service | `unsubscribe` | 取消当前订阅 |
| Service → Client | `unsubscribed` | 确认取消，可再次订阅同一 Session |
| Client → Service | `command` | 发送文本操作 |
| Service → Client | `command_result` | 返回接受或业务拒绝结果 |
| Service → Client | `error` | 返回协议或连接级错误 |

`ping` / `pong` 类型名为后续心跳保留，但 A4 不实现应用级 heartbeat timer。

连接的正常消息顺序固定为：

```text
WebSocket connected
  → hello
  → hello_ack
  → subscribe
  → subscribed
  → snapshot(seq = baseline)
  → event(seq > baseline) / command / command_result
```

订阅时必须先把 `subscribed` 和 snapshot 入队，再开放事件泵输出，确保客户端无需猜测边界。

## 6. 命令关联与可观察顺序

A4 只定义一种命令 payload：

```json
{
  "kind": "send_text",
  "text": "请继续当前任务"
}
```

客户端的 `source` 字段不可信且不属于 payload。Channel 构造 A3 `Input` 时始终强制使用 `InputMobile`。

同一连接的命令串行处理。收到命令后：

1. 校验 `commandId`、session 和文本，但不提前生成 request ID；
2. 进入该连接的顺序门控，暂存提交期间到达的 A3 事件；
3. 调用 `Controller.Submit`；
4. 接受成功后，把同时含 `commandId` 和 `requestId` 的 `command_result` 放入有序输出队列；
5. 接受失败时，返回 `requestId` 为空的失败 `command_result`；
6. result 已入队后，按原 seq 顺序释放暂存事件。

该门控保证同一 request ID 的 `command_result` 先于 `turn_started` 对该连接可见，落实 D25；客户端不需要乱序缓存或平台线程调度约定。

同一连接重复使用 `commandId` 时返回稳定业务错误且不再次提交。断线后不保留 command ID 记录，不提供跨连接 exactly-once 语义；结果不确定时客户端重新订阅 snapshot，不自动重发旧命令。

## 7. 有界缓冲与多客户端

同一个有效 Pair Token 可以认证多个连接，用于页面刷新重叠和多个客户端；每个连接最多订阅该 token 绑定的一个 Session。

每连接限制：

- 提交期间的提前事件缓存：最多 32 个 envelope 或 256 KiB；
- 普通有序输出队列：最多 64 个 envelope 或 1 MiB；
- 数量和序列化后字节数任一达到上限即视为溢出；
- 一个串行 reader 负责接收，一个串行 writer 负责发送，业务 goroutine 不直接写 socket。

溢出只关闭当前连接并取消其 Controller 订阅；不得阻塞 Session Controller、关闭其他客户端或丢弃后假装连接仍同步。客户端重连后重新认证并取得最新 snapshot，不 replay 完整事件历史。

## 8. Pair Token 与配对负载

Pair Token 使用 `crypto/rand` 生成 256-bit 随机值，以无填充 Base64URL 编码。Manager 将 token 与精确 `sessionId` 和当前签发代绑定，并使用恒定时间比较进行校验。

默认有效期为 10 分钟，可通过 `--pair-token-ttl` 调整：

- 有效期内允许多次认证；
- 自然过期只拒绝新认证，不强制关闭已经认证的连接；
- token 自然过期后，Service 轮换并输出新的配对信息；轮换不影响旧 token 已建立的连接；
- 用户主动撤销或 Service 停止时，token 立即失效，并关闭由相关 token 认证的现有连接；
- token 永不出现在普通日志、错误、文档内容、测试 snapshot 或服务端响应中。

纯 JSON 配对负载：

```json
{
  "version": 1,
  "wsUrl": "ws://100.64.0.10:8765/ws",
  "sessionId": "claude-session-id",
  "pairToken": "<temporary-secret>",
  "expiresAt": "2026-08-24T15:30:00+08:00"
}
```

A4 可把该负载输出到交互终端供用户验收，但不生成二维码图片。A5 如生成 Web 配对链接，应把编码后的负载放入 URL Fragment（如 `#pair=...`），使浏览器不会把 token 发送给 HTTP Server、访问日志或 Referer。WebSocket 鉴权在首条 `hello` 消息中提交 token，不把 token 放进 WebSocket URL 查询参数。

该负载只包含 JSON 字符串和整数版本，React/TypeScript、Capacitor 与 HarmonyOS ArkTS 均可直接解析。

## 9. 监听、Origin 与鉴权

默认配置：

```text
--listen 127.0.0.1:8765
--advertise-url ws://127.0.0.1:8765/ws
--pair-token-ttl 10m
```

VPN 真机联调由用户在 A6 显式配置，例如：

```text
--listen 0.0.0.0:8765
--advertise-url ws://100.64.0.10:8765/ws
```

`listen` 是 Service bind 地址，`advertise-url` 是写入配对负载、必须可由客户端实际访问的地址，两者分别校验。A4 不枚举网卡、不自动判断 VPN 地址。

Origin 策略：

- 无 `Origin` 的 CLI 测试客户端和后续原生客户端允许发起连接，但仍须 Pair Token 鉴权；
- 浏览器带 `Origin` 时必须精确匹配允许列表；
- 默认允许本机 A5 开发来源 `http://127.0.0.1:5173` 和 `http://localhost:5173`；
- 额外来源使用可重复的 `--allowed-origin` 显式配置，不允许任意 Origin 通配；
- Origin 只降低浏览器跨站滥用风险，不替代应用层鉴权。

建立 WebSocket 后，客户端必须在 5 秒内发送 `hello`。认证完成前不返回任何 session 数据，也不接受订阅或命令。无效、过期或 session 不匹配的 token 只得到通用 `unauthorized`，不泄露 session 是否存在或 token 失败原因，随后关闭连接。连接认证成功后不能切换 token 或 session。

协议和请求日志使用明确的字段白名单或脱敏视图，不直接格式化完整 hello、配对负载或原始 JSON。

## 10. 连接、Session 与停机生命周期

连接状态：

```text
connected → authenticated → subscribed
                 ↑              │
                 └─ unsubscribe ┘
任意状态 → closing → closed
```

- `connected` 只接受 `hello`；
- `authenticated` 允许订阅 token 绑定的 Session；
- `subscribed` 接收事件和文本命令；
- `unsubscribe` 取消 Controller 订阅并回到 `authenticated`；
- `closing` 停止接受新命令，清理订阅并让 writer 有界退出。

Transport 可以先开始监听，但真实 session ID 产生前 Pairing Manager 没有活动凭证，认证统一失败。A3 发布 `session_started` 后，CLI 编排层注册 Channel、签发 token 并输出配对负载。

A3 事件映射规则：

- 只有非空且匹配注册 Channel 的 `sessionId` 才能进入网络；
- snapshot 直接映射 A3 纯数据 snapshot，并可组合 Transport 持有的连接状态；
- Session/Turn 状态和错误码不由 Transport 重新解释；
- `session_failed` / `session_stopped` 先作为最终事件入队，Session 终态后命令返回业务拒绝；
- Session 终态连接可保留供客户端展示最终状态，直至客户端或 Service 关闭。

Service 正常停止顺序：

1. 停止接受新连接并撤销 token，禁止现有连接提交新命令；
2. 调用 Controller 停止其托管的 Claude Session；
3. 让现有事件泵把最终 `session_stopped` 或 `session_failed` 放入连接队列；
4. 在较短且可配置的有界时间内尝试写出最终事件；
5. 取消全部 Controller 订阅并关闭 WebSocket；
6. 所有等待超时后强制收敛，网络客户端不得阻塞 Controller 或 Service 停机。

用户只撤销配对时不停止 Claude Session，但立即关闭由被撤销 token 认证的连接。

## 11. 错误模型

协议/连接级错误包括非法 JSON、不支持版本、未知类型、错误状态顺序、鉴权失败和消息超限。服务端尽可能发送不含敏感信息的稳定 `error` code，然后关闭连接；若连接已不可安全写入则直接关闭。

业务级拒绝包括空或过长文本、Session 不可接受输入、Controller 队列已满和重复 `commandId`。服务端返回失败 `command_result`，`requestId` 为空，通常保持连接。

对客户端只暴露稳定 code 和安全 message，不暴露 Go 类型、堆栈、token 校验细节、Claude 配置或原始 SDK 错误。

## 12. 测试策略

### 12.1 Pairing 单元测试

- 随机源、编码格式、session 绑定和恒定时间校验边界；
- 有效期内复用、注入时钟驱动的过期和轮换；
- 自然过期不关闭现有连接，撤销通知关闭相关连接；
- 并发签发、认证、撤销无数据竞争；
- 错误、日志捕获和测试 snapshot 不包含真实 token。

### 12.2 协议测试

- 所有消息的 JSON 编解码、必填字段和 ID/seq 适用范围；
- 非法 JSON、未知字段策略、未知类型、版本和大小限制；
- 稳定错误码和敏感字段过滤；
- 协议 decoder fuzz 测试，任意输入不得 panic。

### 12.3 Channel 测试

- 原子 snapshot 与后续事件无缺口、无重复；
- 每连接独立订阅、多连接同 Session 和错误 session 隔离；
- 手机输入强制为 `InputMobile`；
- `command_result` 先于同一 request ID 的 `turn_started`；
- 重复 command ID、Controller 业务拒绝和提前缓存溢出；
- 连接关闭释放订阅，Session 最终状态可见。

### 12.4 WebSocket 集成测试

使用内存 HTTP Server 和真实 WebSocket 客户端覆盖：

- `hello → subscribe → snapshot → event`；
- 鉴权前无 Session 数据，无效、过期和跨 Session token 被拒绝；
- 握手 deadline、Origin allowlist 和无 Origin 客户端；
- 文本命令到 Controller、result 和 Turn 事件的闭环；
- 多客户端、刷新式重连和最新 snapshot；
- 慢客户端溢出只关闭自身；
- token 撤销和 Service 有界停机。

测试注入时钟和缩短的 deadline，不通过真实 sleep 等待 5 秒或 10 分钟。

## 13. A4 可执行验收与门禁

A4 使用两层闭环验证，不制作临时移动 Web Demo：

1. Go 自动化集成测试启动内存 Service 和真实 WebSocket 客户端，确定性验证协议、安全、并发和顺序；
2. `cmd/channel-test-client` 读取配对负载，连接实际 Service 并在终端展示认证、订阅、snapshot、命令结果和事件。

人工测试客户端预期流程：

```text
authenticated
subscribed session=<session-id>
snapshot seq=12 state=ready
> 请继续当前任务
command accepted commandId=<id> requestId=<id>
event seq=13 type=turn_started requestId=<same-id>
```

断开再启动客户端后，应重新认证并取得最新 snapshot，不自动重发上一条命令。

代码门禁：

```text
gofmt
go test ./...
go test -race ./...
GOFLAGS=-buildvcs=false go vet ./...
GOFLAGS=-buildvcs=false go build ./...
```

若本地需要从 `go.mod.base` 恢复 `go.mod`，沿用项目既有流程。`go.mod`、`go.sum`、真实 token 和构建产物均不得纳入提交；新增正式依赖必须同步更新 `go.mod.base`。

## 14. 三端共用契约核对

| 契约 | A4 结论 | 三端可实现性 |
| --- | --- | --- |
| 通信协议 | WebSocket 上的版本化 JSON envelope，固定字符串 ID、整数 seq 和对象 payload | React/TypeScript、Capacitor 网络栈、HarmonyOS ArkTS 均原生支持；无 DOM、localStorage 或 Go 类型依赖 |
| 状态模型 | A3 snapshot/event 纯数据映射；连接状态由协议顺序表达 | 三端使用同一枚举和归并规则，无平台线程或 UI 生命周期语义 |
| 配对负载 | JSON 包含 version、wsUrl、sessionId、pairToken、expiresAt | 三端均可扫码或读取字符串并解析；URL Fragment 只是 Web 呈现方式，不属于鉴权协议依赖 |
| 验收用例 | 认证、订阅、snapshot、文本命令、结果先于 Turn 事件、重连最新 snapshot | Go 客户端在 A4 固化用例，A5/C/D 复用相同行为断言 |

协议不要求客户端解决 D25 的乱序、不依赖浏览器 Origin 存在、不使用浏览器持久化保存长期身份，也不引入端特有二进制格式。三端可实现性核对通过。

## 15. 实施前检查点

本文经用户检查确认后，下一步只编写 A4 实施计划，按 TDD 拆分 Pairing、Protocol、Channel、Transport、CLI 编排和闭环门禁。实施计划获确认前不修改 A4 代码或依赖。
