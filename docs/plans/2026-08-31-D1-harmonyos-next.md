# D1 HarmonyOS NEXT 工程与协议接入实施计划

> **面向 AI 代理的工作者：** 必需子技能：使用 `subagent-driven-development`（推荐）或 `executing-plans` 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法跟踪进度。Git 写操作仍需用户单独授权。

**目标：** 在 `apps/harmony/` 建立独立的 HarmonyOS NEXT Stage 工程，使用 ArkTS/ArkUI 直接消费当前版本 2 协议和配对负载，完成最小配对页、单 Session 详情页、文本发送、手动重连和可审计的 smoke 验证，不复用 React/Capacitor 运行时。

**架构：** `devecocli create` 生成基础 Stage 工程，`entry/src/main/ets/protocol/` 承载三端共用契约的 ArkTS 表达与严格解码，`session/` 负责单连接状态归并，`pages/` 和 `components/` 提供最小 ArkUI 调试壳。验证不依赖未确认的 Harmony 单测框架，而是使用应用内自检信号、`devecocli build/run` 与 HDC `uitest` smoke 证明协议、连接和页面最小闭环。

**技术栈：** HarmonyOS NEXT Stage、ArkTS、ArkUI、`devecocli`、`@kit.NetworkKit` 的 `webSocket.createWebSocket()`、HDC/`uitest`、现有 Go Remote Server（优先 `--fake-host interactive`）。

---

## 文件与边界

- Modify: [\.gitignore](/abs/path/C:/MyWork/GitCode/claude-mobile-remote/.gitignore)，补 Harmony 构建产物、缓存和签名材料忽略规则。
- Modify: [README.md](/abs/path/C:/MyWork/GitCode/claude-mobile-remote/README.md)，补 Harmony 前置、目录和最小运行说明。
- Modify: [apps/mobile/README.md](/abs/path/C:/MyWork/GitCode/claude-mobile-remote/apps/mobile/README.md)，把“是三端唯一的业务实现”改为仅适用于 Web/Capacitor iOS/Android，避免与 D1 事实冲突。
- Create: `apps/harmony/` Stage 工程及其基础文件：`AppScope/app.json5`、`build-profile.json5`、`hvigorfile.ts`、`entry/src/main/module.json5`、`entry/src/main/resources/base/profile/main_pages.json` 等。
- Create: `apps/harmony/entry/src/main/ets/protocol/types.ets`、`pairing.ets`、`decode.ets`、`client.ets`。
- Create: `apps/harmony/entry/src/main/ets/session/model.ets`、`reducer.ets`、`store.ets`。
- Create: `apps/harmony/entry/src/main/ets/debug/fixtures.ets`、`selfCheck.ets`，提供配对/解码/首轮前 source/命令门控的应用内自检。
- Create: `apps/harmony/entry/src/main/ets/pages/Index.ets`、`PairingPage.ets`、`SessionPage.ets`。
- Create: `apps/harmony/entry/src/main/ets/components/StatusBar.ets`、`MessageList.ets`、`PendingSummary.ets`、`QueuedSummary.ets`、`SelfCheckPanel.ets`。
- Create: [apps/harmony/README.md](/abs/path/C:/MyWork/GitCode/claude-mobile-remote/apps/harmony/README.md)，记录 `devecocli`、模拟器/设备 smoke 和当前不做项。
- Create: [docs/testing/D1-result.md](/abs/path/C:/MyWork/GitCode/claude-mobile-remote/docs/testing/D1-result.md)，记录 DevEco 前置、构建、smoke、自检信号和未验证项。
- Modify: [HANDOFF.md](/abs/path/C:/MyWork/GitCode/claude-mobile-remote/HANDOFF.md)，记录 D1 实际完成项、验证结果、未验证项和 D2/D3 下一步。

边界固定：

- 不创建 WebView 套壳，不复制 `apps/mobile/src` 到 Harmony 工程。
- 不改 Go 协议、Session 状态模型、Pairing JSON 字段、`sessionId`、`requestId` 或 `seq` 语义。
- 不实现扫码、安全存储、真机 VPN、后台恢复、推送和多 Session 持久化。
- 不引入 Harmony 侧自定义服务器、第二套配对字段或浏览器持久化。
- 不把真实 Pair Token、真实 `sessionId`、用户会话内容、本机绝对路径、签名文件或构建缓存写入仓库或验证记录。

### Task 1: 固化 DevEco 前置和仓库边界

**Files:**
- Modify: [\.gitignore](/abs/path/C:/MyWork/GitCode/claude-mobile-remote/.gitignore)
- Modify: [README.md](/abs/path/C:/MyWork/GitCode/claude-mobile-remote/README.md)
- Modify: [HANDOFF.md](/abs/path/C:/MyWork/GitCode/claude-mobile-remote/HANDOFF.md)

- [ ] **Step 1: 用 `devecocli` 先做工具可用性探测**

运行：

```powershell
devecocli --help
devecocli create --help
devecocli build --help
devecocli docs search webSocket --limit 1
```

预期：

- 前三个命令输出帮助；
- 第四个命令若 DevEco 已安装，应返回本地文档命中；
- 若输出 `DevEco Studio installation not found in registry or default locations.`，把它记录为**环境阻塞**，先补 DevEco Studio 或命令行工具，再继续代码实现；不要把该错误误判为 D1 代码失败。

- [ ] **Step 2: 先补 Harmony 构建忽略规则**

在 [\.gitignore](/abs/path/C:/MyWork/GitCode/claude-mobile-remote/.gitignore) 追加：

```gitignore
# HarmonyOS NEXT 工程（D1）
apps/harmony/.hvigor/
apps/harmony/oh_modules/
apps/harmony/build/
apps/harmony/entry/build/
apps/harmony/.preview/
apps/harmony/**/*.hap
apps/harmony/**/*.app
apps/harmony/**/*.p7b
apps/harmony/**/*.csr
```

要求：

- 只补 Harmony 独有的产物和签名材料；仓库根已全局忽略的 `*.p12`、`*.cer` 不重复追加；
- 不误伤将要入库的 `AppScope/`、`entry/src/`、`oh-package.json5`、`build-profile.json5`。

- [ ] **Step 3: 验证忽略规则和文档入口**

运行：

```powershell
rg -n "apps/harmony/" .gitignore
rg -n "Harmony|harmony|DevEco|devecocli" README.md HANDOFF.md
```

预期：

- `.gitignore` 中已出现 Harmony 规则；
- `README.md` 和 `HANDOFF.md` 已能承接 D1 文档入口。

- [ ] **Step 4: 在根 README 预留 Harmony 段落**

把 [README.md](/abs/path/C:/MyWork/GitCode/claude-mobile-remote/README.md) 的平台说明补成最小可执行描述，至少包含：

```md
## HarmonyOS NEXT（D1）

前提：本机已安装 DevEco Studio 或可被 `devecocli` 识别的命令行工具。

```bash
cd apps/harmony
devecocli build
```

当前 D1 只覆盖独立 ArkTS/ArkUI 工程、协议接入和最小 smoke；
扫码、安全存储、真机/VPN、推送和发布验证留待 D2/D3。
```

- [ ] **Step 5: 修正文案冲突**

把 [apps/mobile/README.md](/abs/path/C:/MyWork/GitCode/claude-mobile-remote/apps/mobile/README.md) 开头的“三端唯一业务实现”改为只描述当前 Web/Capacitor iOS/Android，示例：

```md
React + TypeScript 移动界面，是当前 Web 与 Capacitor iOS/Android 的业务实现。
HarmonyOS NEXT 在 `apps/harmony/` 使用 ArkTS + ArkUI 独立实现，同样复用三端共用契约。
```

### Task 2: 创建独立 Stage 工程并稳定基线

**Files:**
- Create: `apps/harmony/AppScope/app.json5`
- Create: `apps/harmony/build-profile.json5`
- Create: `apps/harmony/oh-package.json5`
- Create: `apps/harmony/package.json`
- Create: `apps/harmony/hvigorfile.ts`
- Create: `apps/harmony/entry/oh-package.json5`
- Create: `apps/harmony/entry/build-profile.json5`
- Create: `apps/harmony/entry/src/main/module.json5`
- Create: `apps/harmony/entry/src/main/resources/base/profile/main_pages.json`
- Create: `apps/harmony/entry/src/main/ets/pages/Index.ets`
- Create: [apps/harmony/README.md](/abs/path/C:/MyWork/GitCode/claude-mobile-remote/apps/harmony/README.md)

说明：DevEco 模板当前实际生成 `main_pages.json` 于 `entry/src/main/resources/base/profile/main_pages.json`。实现时以真实模板结构为准，不手工伪造第二份 `resources/profile/` 副本。

- [ ] **Step 1: 用 `devecocli create` 生成基础工程**

运行：

```powershell
devecocli create `
  --app-name ClaudeMobileRemoteHarmony `
  --project-path apps/harmony `
  --bundle-name com.tea4go.claudemobileremote
```

预期：

- `apps/harmony/` 生成标准 Stage 工程；
- 若目标目录非空，先人工清理未跟踪草稿目录后重试，不要让 `devecocli create` 覆盖已有业务文件。

- [ ] **Step 2: 固定应用标识和入口资源**

在 `AppScope/app.json5`、应用字符串和模块配置中保留统一标识。目标内容至少等价于：

```json5
{
  "app": {
    "bundleName": "com.tea4go.claudemobileremote",
    "vendor": "tea4go",
    "versionCode": 1000000,
    "versionName": "1.0.0",
    "icon": "$media:app_icon",
    "label": "$string:app_name"
  }
}
```

以及应用名称资源：

```json5
{
  "string": [
    {
      "name": "app_name",
      "value": "Claude Mobile Remote"
    }
  ]
}
```

- [ ] **Step 3: 在 `module.json5` 声明网络权限**

WebSocket 需要 `ohos.permission.INTERNET`。在 `apps/harmony/entry/src/main/module.json5` 的 `module` 下追加：

```json5
"requestPermissions": [
  {
    "name": "ohos.permission.INTERNET"
  }
]
```

同时保留：

- `mainElement: "EntryAbility"`
- `pages: "$profile:main_pages"`
- `icon` / `startWindowIcon`

不要提前加相机、剪贴板、推送或安全存储权限。

- [ ] **Step 4: 用 `Index.ets` 挂业务根组件**

先把默认 smoke 页面替换成可承载配对页和会话页的根组件，最小目标：

```ets
import { AppShell } from '../pages/AppShell';

@Entry
@Component
struct Index {
  build() {
    AppShell()
  }
}
```

如果不新增 `AppShell.ets`，则 `Index.ets` 至少要明确挂 `PairingPage()`，不要继续停留在模板 `SmokeCounter()`。

- [ ] **Step 5: 先跑一次基线构建**

运行：

```powershell
cd apps/harmony
devecocli build
```

预期：基础工程 build 通过。若失败，先修复模板生成与权限声明层的问题，再进入协议与页面实现。

### Task 3: 实现协议类型、配对解析和严格解码

**Files:**
- Create: `apps/harmony/entry/src/main/ets/protocol/types.ets`
- Create: `apps/harmony/entry/src/main/ets/protocol/pairing.ets`
- Create: `apps/harmony/entry/src/main/ets/protocol/decode.ets`
- Create: `apps/harmony/entry/src/main/ets/debug/fixtures.ets`
- Create: `apps/harmony/entry/src/main/ets/debug/selfCheck.ets`

- [ ] **Step 1: 先把契约和固定样例写成 ArkTS 常量**

在 `types.ets` 里使用字符串枚举和接口，不照搬 React 联合类型。示例：

```ets
export enum SessionStatus {
  STARTING = 'starting',
  READY = 'ready',
  STOPPED = 'stopped',
  FAILED = 'failed'
}

export enum TurnStatus {
  IDLE = 'idle',
  RUNNING = 'running',
  WAITING = 'waiting',
  COMPLETED = 'completed',
  CANCELLED = 'cancelled',
  FAILED = 'failed'
}

export interface PairingPayload {
  version: number
  wsUrl: string
  sessionId: string
  pairToken: string
  expiresAt: string
}
```

在 `fixtures.ets` 放入至少四组固定样例：

- 正常配对 payload；
- 已过期 payload；
- 首轮前 `turn.source == ''` 的 snapshot；
- 带 `content_block` / `pendingAction` / `queuedRequests` 的事件流。

- [ ] **Step 2: 先让自检代码引用尚未实现的解析器，确认 RED**

在 `selfCheck.ets` 先写固定断言，直接调用待实现函数：

```ets
import { decodePairingPayload } from '../protocol/pairing';
import { decodeServerEnvelope } from '../protocol/decode';
import { FIXTURES } from './fixtures';

export function runSelfChecks(): Array<string> {
  const results: Array<string> = [];
  decodePairingPayload(FIXTURES.validPairing);
  decodeServerEnvelope(FIXTURES.preTurnSnapshot);
  results.push('pairing-ok');
  results.push('preturn-source-ok');
  return results;
}
```

运行：

```powershell
cd apps/harmony
devecocli build --modules entry@default
```

预期：因解析器尚未实现而编译失败，确认 D1 的协议开发入口是 RED，不是把未实现逻辑混进 UI 里。

- [ ] **Step 3: 实现配对解析器**

`pairing.ets` 至少覆盖：

- `JSON.parse` 后逐字段校验；
- `ws://`/`wss://` 限制；
- 非空 `sessionId` / `pairToken` / `expiresAt`；
- 过期时间比较；
- 返回结构化错误而不是抛裸异常。

建议接口：

```ets
export interface PairingDecodeResult {
  ok: boolean
  payload?: PairingPayload
  error?: string
}

export function decodePairingPayload(raw: string): PairingDecodeResult {
  // 严格校验字段和过期时间
}
```

- [ ] **Step 4: 实现版本 2 envelope 与事件解码**

`decode.ets` 至少提供：

```ets
export interface DecodeResult<T> {
  ok: boolean
  value?: T
  error?: string
}

export function decodeServerEnvelope(raw: string): DecodeResult<ServerMessage> {
  // 校验 version/type/sessionId/commandId/requestId/seq/payload
}
```

严格规则必须覆盖：

- 只接受版本 2；
- `unsupported` 是独立消息，不是 `content_block.kind`；
- 首轮前 `turn.source` 允许空串；
- `error.code = unauthorized` 可被上层识别为重新配对；
- `queuedRequests` 与 `queueDepth` 同时存在时必须一致。

- [ ] **Step 5: 跑 GREEN 并产出自检信号**

运行：

```powershell
cd apps/harmony
devecocli build --modules entry@default
```

预期：协议模块编译通过；`runSelfChecks()` 至少能生成以下稳定信号：

- `pairing-ok`
- `pairing-expired-ok`
- `preturn-source-ok`
- `unsupported-ok`
- `queued-requests-ok`

### Task 4: 实现 `ProtocolClient` 和单连接状态归并

**Files:**
- Create: `apps/harmony/entry/src/main/ets/protocol/client.ets`
- Create: `apps/harmony/entry/src/main/ets/session/model.ets`
- Create: `apps/harmony/entry/src/main/ets/session/reducer.ets`
- Create: `apps/harmony/entry/src/main/ets/session/store.ets`

- [ ] **Step 1: 先让页面状态依赖尚未实现的 store，确认第二个 RED**

先在页面代码中引用将要存在的 store 接口，例如：

```ets
import { createSessionStore } from '../session/store';

@State store = createSessionStore();
```

运行：

```powershell
cd apps/harmony
devecocli build --modules entry@default
```

预期：因 `session/store.ets` 不存在或接口不完整而失败。

- [ ] **Step 2: 实现最小状态模型**

`model.ets` 至少定义：

```ets
export enum ConnectionStage {
  CONNECTING = 'connecting',
  AUTHENTICATING = 'authenticating',
  SUBSCRIBING = 'subscribing',
  CONNECTED = 'connected',
  DISCONNECTED = 'disconnected',
  PAIRING_INVALID = 'pairingInvalid',
  PROTOCOL_ERROR = 'protocolError'
}

export interface HarmonySessionState {
  pairing: PairingPayload | null
  connection: ConnectionStage
  snapshot: SessionSnapshot | null
  timeline: Array<DomainEvent>
  pendingCommandId: string
  lastError: string
}
```

- [ ] **Step 3: 用 `@kit.NetworkKit` 接 WebSocket 客户端**

`client.ets` 直接按离线文档的能力实现，导入：

```ets
import { webSocket } from '@kit.NetworkKit';
import { BusinessError } from '@kit.BasicServicesKit';
```

连接骨架至少等价于：

```ets
const ws: webSocket.WebSocket = webSocket.createWebSocket();
ws.on('open', (_err: BusinessError, _value: Object) => {
  // 先发 hello，再发 subscribe
});
ws.on('message', (_err: BusinessError, value: Object) => {
  // 解码服务端 envelope
});
ws.on('close', (_err: BusinessError, value: webSocket.CloseResult) => {
  // 进入断线态
});
ws.on('error', (err: BusinessError) => {
  // 记录协议/网络错误
});
```

必须遵守：

- `send()` 只在收到 `open` 事件后调用；
- 同一时刻最多一个在途 `commandId`；
- `reconnect()` 先 `close()` 再重建实例；
- `error.code == unauthorized` 进入 `PAIRING_INVALID`，普通 close 进入 `DISCONNECTED`。

- [ ] **Step 4: 实现 reducer 和 store**

`reducer.ets` 至少支持以下动作：

- `startConnect`
- `helloAck`
- `snapshotReceived`
- `eventReceived`
- `commandAccepted`
- `socketClosed`
- `pairingInvalid`
- `protocolError`

示例：

```ets
export function reduce(state: HarmonySessionState, action: SessionAction): HarmonySessionState {
  switch (action.type) {
    case 'snapshotReceived':
      return { ...state, connection: ConnectionStage.CONNECTED, snapshot: action.snapshot };
    case 'eventReceived':
      return { ...state, timeline: [...state.timeline, action.event] };
    default:
      return state;
  }
}
```

- [ ] **Step 5: 跑 GREEN**

运行：

```powershell
cd apps/harmony
devecocli build --modules entry@default
```

预期：协议客户端和状态归并编译通过，未出现把 React/浏览器 API 带入 ArkTS 的错误。

### Task 5: 组装最小 ArkUI 页面壳

**Files:**
- Create: `apps/harmony/entry/src/main/ets/pages/AppShell.ets`
- Create: `apps/harmony/entry/src/main/ets/pages/PairingPage.ets`
- Create: `apps/harmony/entry/src/main/ets/pages/SessionPage.ets`
- Create: `apps/harmony/entry/src/main/ets/components/StatusBar.ets`
- Create: `apps/harmony/entry/src/main/ets/components/MessageList.ets`
- Create: `apps/harmony/entry/src/main/ets/components/PendingSummary.ets`
- Create: `apps/harmony/entry/src/main/ets/components/QueuedSummary.ets`
- Create: `apps/harmony/entry/src/main/ets/components/SelfCheckPanel.ets`
- Modify: `apps/harmony/entry/src/main/ets/pages/Index.ets`

- [ ] **Step 1: 先把页面节点 ID 和自检信号定死**

为后续 `uitest dumpLayout` smoke，所有关键节点都要给稳定 ID。至少包括：

```text
pairing-input
connect-button
connection-state
session-state
turn-state
message-list
send-input
send-button
reconnect-button
self-check-summary
pending-summary
queue-summary
```

- [ ] **Step 2: 实现配对页**

`PairingPage.ets` 至少包含：

```ets
TextArea({ text: this.rawPairing })
  .id('pairing-input')

Button('连接当前 Session')
  .id('connect-button')
  .onClick(() => {
    this.store.connectFromRawPairing(this.rawPairing);
  })
```

同时展示：

- 解析成功/失败；
- 已过期；
- URL 非 `ws/wss`；
- 需要重新配对。

- [ ] **Step 3: 实现会话页**

`SessionPage.ets` 至少包含：

- 连接状态、Session 状态、Turn 状态；
- `sessionId` 摘要；
- 文本消息流；
- `pendingAction` 摘要；
- `queuedRequests` 摘要；
- 文本输入与发送按钮；
- 手动断开/重连。

发送按钮骨架：

```ets
Button('发送')
  .id('send-button')
  .enabled(this.store.canSend())
  .onClick(() => {
    this.store.sendText(this.inputText);
  })
```

- [ ] **Step 4: 加入自检面板**

`SelfCheckPanel.ets` 在 D1 默认可见，渲染 Task 3 的结果。至少输出一行：

```ets
Text(this.results.join(', '))
  .id('self-check-summary')
```

这样即使没有单测框架，也能通过布局树稳定证明：

- 配对解析已跑过；
- 首轮前 `source == ''` 已兼容；
- 版本 2/`unsupported`/队列字段已被识别。

- [ ] **Step 5: 跑 GREEN**

运行：

```powershell
cd apps/harmony
devecocli build
```

预期：页面、状态和客户端全部通过构建。

### Task 6: 用模拟器或设备完成最小 smoke

**Files:**
- Modify: [apps/harmony/README.md](/abs/path/C:/MyWork/GitCode/claude-mobile-remote/apps/harmony/README.md)
- Create: [docs/testing/D1-result.md](/abs/path/C:/MyWork/GitCode/claude-mobile-remote/docs/testing/D1-result.md)

- [ ] **Step 1: 启动本地 Go 服务**

从仓库根目录运行：

```powershell
GOFLAGS=-buildvcs=false go run ./cmd/claude-mobile-remote `
  --listen 0.0.0.0:8765 `
  --advertise-url ws://<HOST-IP>:8765/ws `
  --fake-host interactive
```

记录当前输出的配对负载，但**不要**把真实 Pair Token、`sessionId` 或 IP 写进仓库文档。

- [ ] **Step 2: 选择可用目标**

先运行：

```powershell
devecocli device list
devecocli emulator list
```

分支处理：

- 有已连接设备/模拟器：直接继续；
- 没有目标但已有 HVD 名称：`devecocli emulator start "<name>"`；
- 若返回 license/agreement 阻塞，停止自动化，要求用户在交互终端执行 `devecocli emulator license accept` 后再继续；
- 若当前机器没有 DevEco/模拟器，则 D1 只能停在构建通过，不能宣称 smoke 通过。

- [ ] **Step 3: 安装并启动应用**

运行：

```powershell
cd apps/harmony
devecocli build
devecocli run --module entry --device <name-or-serial>
```

预期：应用成功安装并启动到 `Index` 页。

- [ ] **Step 4: 用 HDC + `uitest` 取证**

参照最小模板 smoke，执行：

```powershell
hdc -t <target> shell uitest dumpLayout -p /data/local/tmp/d1_before.json -a
hdc -t <target> file recv /data/local/tmp/d1_before.json ./d1_before.json
hdc -t <target> shell uitest screenCap -p /data/local/tmp/d1_before.png
hdc -t <target> file recv /data/local/tmp/d1_before.png ./d1_before.png
```

断言布局树中至少出现：

- `pairing-input`
- `connect-button`
- `self-check-summary`

之后手工粘贴当前配对负载并再次抓取布局，断言出现：

- `connection-state`
- `message-list`
- `send-button`

- [ ] **Step 5: 验证一条真实文本命令**

在应用内发送一条文本，随后重新抓取布局或日志，确认：

- `command_result` 已被接受；
- 页面出现新文本消息或状态变化；
- `send-button` 在发送完成后恢复可用。

若有需要，使用：

```powershell
devecocli log --device <name-or-serial> --level I --from 5m --tail 200
```

只记录“看到连接/发送/消息事件”的结论，不记录真实会话正文。

### Task 7: 收口文档、验证边界和交接

**Files:**
- Modify: [apps/harmony/README.md](/abs/path/C:/MyWork/GitCode/claude-mobile-remote/apps/harmony/README.md)
- Modify: [README.md](/abs/path/C:/MyWork/GitCode/claude-mobile-remote/README.md)
- Create: [docs/testing/D1-result.md](/abs/path/C:/MyWork/GitCode/claude-mobile-remote/docs/testing/D1-result.md)
- Modify: [HANDOFF.md](/abs/path/C:/MyWork/GitCode/claude-mobile-remote/HANDOFF.md)

- [ ] **Step 1: 写 `apps/harmony/README.md`**

至少包含：

```md
## 常用命令

```bash
devecocli build
devecocli run --module entry --device <target>
devecocli device list
devecocli emulator list
```

## 当前范围

- D1：独立工程、协议接入、最小页面壳、文本发送、手动重连
- 不包含：扫码、安全存储、真机/VPN、推送、多 Session 持久化
```

- [ ] **Step 2: 创建 `docs/testing/D1-result.md`**

必须记录：

- 执行日期；
- `devecocli` / DevEco 可用性结论；
- `devecocli build` 结果；
- `devecocli run`、设备/模拟器 smoke 结果；
- 自检信号列表；
- 未验证项：扫码、安全存储、真机 VPN、后台恢复、推送、发布；
- 若被环境阻塞，记录阻塞点和原始错误摘要。

- [ ] **Step 3: 复核三端契约不变**

运行：

```powershell
rg -n "version|sessionId|requestId|seq|pairToken|expiresAt" apps/harmony/entry/src/main/ets/protocol
```

人工确认：

- 没有新增 Harmony 专属配对字段；
- 版本仍是 2；
- `turn.source` 首轮前空串已处理；
- 未把 React、Capacitor 或 DOM 概念带进 ArkTS 协议层。

- [ ] **Step 4: 做最终格式与差异检查**

运行：

```powershell
git diff --check
```

预期：无空白错误。该步骤不包含任何 Git 写操作。

- [ ] **Step 5: 更新 HANDOFF**

写入：

- D1 实际完成项；
- 自动构建和 smoke 结果；
- 环境阻塞与未验证项；
- 下一步进入 D2 还是先补 DevEco/模拟器条件。

## D1 门禁结论

- 必须通过：DevEco/devecocli 可用性探测、独立 Stage 工程创建、`ohos.permission.INTERNET` 权限声明、`devecocli build` 成功、ArkTS 协议/状态模块编译通过、最小页面壳编译通过。
- 至少一项运行态通过：设备或模拟器上成功启动应用，并能看到 `pairing-input`、`self-check-summary` 与连接后的 `message-list`。
- 必须明确不宣称：扫码、安全存储、真机 VPN、后台恢复、推送、多 Session 持久化、完整 B2 交互、发布签名和上架。
- 若 DevEco 缺失、模拟器许可未接受或没有可用设备，D1 只能声明“计划与构建路径已准备，运行态 smoke 被环境阻塞”，不得冒称功能已验证。
