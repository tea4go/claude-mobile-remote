# D2 HarmonyOS NEXT 与 Android 功能拉齐设计

- 日期：2026-09-01
- 状态：执行中
- 前置阶段：D1 已在主工作区通过 `devecocli build --modules entry@default`
- 关联决策：D7、D19、D46、D47、D48、D52、D53、D54

## 1. 目标

将 `apps/harmony/` 从 D1 的“独立工程 + 协议接入最小壳”推进到与当前 Android 壳所承载的 `apps/mobile` 业务能力对齐。

本次“功能拉齐”按以下口径执行：

1. 对齐当前 `apps/mobile` 在 Android 壳中的业务能力与交互语义；
2. 不要求复用 React/Capacitor 运行时，也不要求组件树或像素样式一致；
3. 不把 Android 平台专属壳配置（Capacitor、Manifest、cleartext 策略）搬到 Harmony；
4. 仍不提前纳入扫码、安全存储、VPN 真机、前后台恢复和推送。

## 2. 需要拉齐的功能面

### 2.1 会话入口

- 已配对 Session 的页面内存列表；
- 新增配对；
- 单条记录进入详情；
- 本地重命名；
- 删除记录；
- 同一时刻只保持一个活动连接。

### 2.2 会话详情

- 连接状态、Session 状态、Turn 状态、排队深度、最后通知；
- 当前 Turn 取消；
- 排队请求列表与逐项取消；
- 配对失效、协议异常、普通断开三种错误态区分；
- 手动重连。

### 2.3 消息流

- 手机输入消息的 `accepted/unknown` 状态；
- Claude 文本消息；
- `thinking`、`tool_use`、`tool_result` 的结构化展示；
- `action_resolved.answers` 的答案回显；
- `turn_completed(cancelled)` 与 `turn_failed` 的终态呈现；
- 不支持通知的只读提示。

### 2.4 待处理事项

- `tool_approval` 的三决议；
- `ask_user_question` 的单选、多选、文本输入；
- 事项提交后本地禁用，等待 `command_result`；
- 显式关闭等价于取消当前 Turn；
- `waiting/pendingAction` 期间禁止发送新文本。

## 3. 不在本次范围

- 扫码；
- 安全存储与跨重启 Session 列表持久化；
- 真机/VPN；
- 推送；
- 发布签名；
- 复用 Android 的 React 组件、Capacitor 代码或第三方 npm Markdown 组件。

## 4. 实现策略

### 4.1 状态模型

Harmony 端不再只保留 D1 的 `snapshot + timeline` 调试视图，而是补齐与 Android 等价的本地状态归并语义：

- 连接 store 维护当前活动 Session 的协议状态；
- AppShell 维护页面内存 Session 列表与当前视图；
- 消息流保留“输入 / Claude / block / answer”四类消息；
- 所有命令继续共用单在途门控。

### 4.2 页面结构

新增三层页面结构：

1. Session List
2. Pairing
3. Session Detail

行为与 Android 一致：

- 无记录时直达 Pairing；
- 有记录时先看列表；
- 详情返回列表时关闭当前连接但保留记录。

### 4.3 消息展示

Harmony 不复用 React Markdown 实现，但展示语义对齐：

- Claude 文本保留换行；
- 代码块与普通文本分开展示；
- 工具活动以摘要块展示；
- 不展示工具原始 `input` 或工具结果正文；
- 答案回显仅展示问题和最终取值。

这意味着 D2 关注“同一信息是否可见、同一交互是否可用”，而不是强行复制 Android 的 DOM/样式实现。

### 4.4 待处理事项

Harmony 保持 D53 的业务语义：

- 待处理事项出现后强提示；
- 关闭 = 取消当前 Turn；
- `tool_approval` / `ask_user_question` 共用同一提交门控；
- 支持文本题，但只按服务端已确认的 `inputType='text'` 字段渲染。

## 5. 通过条件

本次“已与 Android 拉齐”按以下条件判定：

1. `apps/harmony` 已具备 Android 当前的会话列表、配对进入、详情页、三类待处理事项交互、取消入口、答案回显和结构化消息展示；
2. 主工作区真实 `devecocli build --modules entry@default` 通过；
3. 不新增 Harmony 专属协议字段，不改变 Go 端既有契约；
4. 未验证项若仍存在，只能是设备/真机运行态，不得是代码层功能缺口。

## 6. 风险

- ArkTS 对 union、对象字面量和局部函数限制严格，必须继续用显式接口和 class；
- Harmony 无法直接复用 React Markdown 组件，消息展示只能做语义等价实现；
- 若运行态 smoke 受设备环境阻塞，不得误报为“已完成真机一致性”。
