# HANDOFF.md — Claude Mobile Remote 交接文档

> 新会话先读 `AGENTS.md`，再读本文件、`DECISIONS.md`、`REQUIREMENTS.md` 和总体设计。完成阅读后先复述当前状态和下一步，再按“当前执行”继续；涉及尚未确认的设计、计划或产品决策时先等用户确认。
>
> 本文件只记录**当前状态、当前执行、未闭环项和阻塞**。已完成阶段的过程明细见 `docs/HANDOFF-archive.md`（新会话不需要读），阶段门禁证据见 `docs/testing/`，已确认决策见 `DECISIONS.md`。

## 0. 一句话现状

A1–A6 已提交；B1 已完成并通过门禁；**B2 已完成并通过用户验收**（2026-08-28，含三批移动端 UI 打磨；真实 Claude 端到端验证后置，不阻塞）；C1 已完成并通过门禁（Capacitor iOS/Android 工程 + Web 接入 + 局域网明文放行 + Android 模拟器壳内连通）；服务重启后的配对恢复已闭环；**B3 已完成并通过门禁**（协议类型下放到 `protocol/`、远程能力抽取为 `remote.Server`）。下一步：W1（依赖 B3）与 C2（依赖 C1+B2，与 B3 无直接依赖）。

## 当前执行（AI 自动维护）

- **子阶段**：仓库配置维护。
- **本轮任务**：将 `claude-mobile-remote` CLI 二进制产物加入 `.gitignore`。
- **通过条件**：`.gitignore` 明确忽略该二进制名称，不影响现有 Go 源码和测试规则。
- **实际完成项**：已在 Go 二进制忽略区新增 `claude-mobile-remote`。
- **实际验证**：`git diff --check` 通过；`rg` 确认 `.gitignore` 第 5 行包含精确规则 `claude-mobile-remote`。

- **子阶段**：B3 已收口。本轮完成协议类型下放与单 Session Remote Server 组件化，门禁全绿，证据见 `docs/testing/B3-result.md`。
- **实际完成项**：
  1. **`protocol/` 包（新增）**：JSON-bearing 协议类型（Snapshot/Turn/Event/Input/ContentBlock/PendingAction/Question/QuestionAnswer/ActionDecision/ServiceConfig/Pairing/TruncatePreview）的唯一 Go 来源，不依赖任何 internal 包、SDK 或 Transport。
  2. **`internal/session`/`claude`/`pairing` 改 alias**：`session` 的 `model.go`/`event.go`/`input.go`/`controller.go` 类型与哨兵改 alias 指向 `protocol`；`Snapshot.setQueuedRequests` 方法改包级函数（Go 不能对非本地 alias 定义方法）；`claude.TruncatePreview` 改 delegate，`pairing.Payload` 改 alias。
  3. **`internal/channel`/`transport` 指向 `protocol`**：Channel 的 `Controller` 接口、Transport 三文件的 payload/snapshot/event 引用改用 `protocol` 类型；**删除 transport 本地 `ActionDecision`/`QuestionAnswer` 副本**，`sessionResolution` 简化为深拷贝。
  4. **`remote/` 包（新增）**：公共 `Controller`/`Config`/`Server`，`Start` 合并原 `Activate`（注册 Channel + 签发凭证，失败回滚），`Close` 注销 Channel 且从不调用宿主 Controller 的 Start/Stop。
  5. **`cmd/claude-mobile-remote` 迁移**：`main.go` 改用 `remote.New` 装配；`run.go` 的 `remoteService` 去掉 `Activate`、`Start()` 返回 Pairing、删除「init 后延迟激活」死分支；删除旧 `cmd/claude-mobile-remote/remote.go`。
- **实际执行的验证及结论**：`gofmt -l .` 无输出；`GOFLAGS=-buildvcs=false go vet ./...` 无输出；`GOFLAGS=-buildvcs=false go test -count=1 ./...` → **10 个包全部 ok**（新增 `protocol`、`remote`）；`git diff --check` 无空白错误。`rg` 核对确认协议 `type X` 定义只出现在 `protocol/`，`internal/session`/`pairing`/`transport` 内均为 alias，transport 本地 `ActionDecision`/`QuestionAnswer` 副本已消除。
- **未验证项**：真实 Claude 端到端（额度受限，不阻塞）；WhaleTerm W1 接入（B3 只交付公共边界，未实现 WhaleTerm 侧适配）；前端/HarmonyOS（B3 只动 Go 侧，wire contract 不变，未重跑 `npm`）。
- **未执行**：任何 Git 写操作（仅用只读 `git diff --check`）。

- **子阶段**：B2 已收口。用户于 2026-08-28 反馈「B2 验证已通过」，本轮工作是提交前重跑完整门禁、做敏感信息与产物核对，并把验收结论写入 `docs/testing/B2-result.md`。
- **提交前门禁（本轮实际执行，全绿）**：`gofmt -l .` 无输出；`GOFLAGS=-buildvcs=false go vet ./...` 无输出；`GOFLAGS=-buildvcs=false go test -count=1 ./...` → 8 个包全部 ok（非缓存）；`cd apps/mobile && npm run test` → 16 文件 **400 passed**；`npm run lint` **0 error**（1 个既有 warning 在 `scripts/cdp-console.mjs`）；`npm run build` 通过（`index-*.js` 490.71 kB / gzip 155.25 kB）。`git diff --check` 无空白错误；`go.mod`/`go.sum`/`apps/mobile/dist` 均未入库；凭证特征扫描仅命中「描述扫描动作的文档文字」与 `fakehost_test.go` 的守卫用例，无真实凭证。
- **提交时刻意排除**：`docs/specs/images/qoder/{sessionDetail,sessionTools,toolDetail}.png` **未入库**。它们是用户提供的 Qoder App 界面参考图，画面内含真实 Claude 会话正文与本机绝对路径，与 AGENTS.md 开发规范「不提交会话内容」冲突；仅作为本地参考保留，`docs/HANDOFF-archive.md` §6 已标注未入库。
- **上一批（第三批两项）实际完成项**（仅移动端样式，未动协议、状态模型与交互语义，Go 侧零改动）：
  1. **表格与底色分离**：页面底色收归到 `ui.ts` 的 `PAGE_BACKGROUND`（`#fafbfd`）与 `SESSION_BACKGROUND`（偏白渐变，原顶部 `#eef2f7` 偏灰蓝），`App.tsx` 两处 `#f5f5f5` 与 `SessionDetail` 硬编码渐变改为引用常量；`MessageMarkdown` 表格改为白底卡片（容器 `#fff` + 8px 圆角 + 轻阴影，网格线 `#e3e6ec`，th `#f5f7fa`）——根因是正文区无气泡背景且单元格透明，页面底色直接透上来；代码块同理补一条 `#e3e6ec` 边框。
  2. **抽屉透亮化**：根因是 `backdropFilter` 会把身后的遮罩一起采样进模糊结果，antd-mobile 默认 `rgba(0,0,0,0.55)` 会把整个抽屉染成灰面。新增 `DRAWER_MASK_STYLE`（`rgba(15,23,42,0.16)`）并传给两个抽屉的 `Popup maskStyle`（遮罩仍在，点击拦截与 `onMaskClick` 语义不变）；`DRAWER_SHEET_STYLE` 不再复用 `GLASS_SURFACE_STYLE`，改为白底 0.6 + `blur(30px) saturate(200%) brightness(1.14)` + 顶部白高光边 + 外投影；`GLASS_PANEL_STYLE` 改用白色高光边（原深色描边在亮玻璃上显脏），preview 底色从 `rgba(0,0,0,0.04)` 改为半透明白。
- **实际执行的验证及结论**：
  - `npx tsc -p tsconfig.app.json --noEmit` 无输出（也回证 antd-mobile `Popup` 确实接受 `maskStyle`）；`npm test` → 16 文件 **400 passed**（+1）；`npm run lint` 0 error（1 个既有 warning 在 `scripts/cdp-console.mjs`）；`npm run build` 通过；
  - 新增护栏用例「抽屉遮罩显著淡于默认档，玻璃靠 brightness 提亮」：在 jsdom 里对**真实渲染的** `.adm-mask` / `.adm-popup-body` 断言遮罩 alpha < 0.3 且玻璃带 `brightness`，锁住设计约束而不锁具体色值（否则改回默认遮罩就会退化成灰面）。
- **视觉观感的证据边界**：三批 UI 打磨的实际观感由**用户人工确认通过**，仓库内没有截图证据（AI 侧浏览器子代理环境不可用：viewport 20px、`visibilityState=hidden`、截图固定报 `NATIVE_BROWSER_VIEWPORT_UNAVAILABLE`）。不得对外表述为「AI 已视觉验证」。
- **仍未验证**：真实 Claude 端到端（工具审批放行/拒绝后 Turn 继续）因额度受限后置；真机、VPN、原生扫码与安全存储属 C2。
- **已执行的 Git 写操作**：按用户本次明确授权完成一次本地 `git commit`（未 push、未改分支）。授权仅限本次提交，后续 Git 写操作需重新授权。
- **上一批（第二批四项）实际完成项**:
  1. **审批与提问改抽屉（D53）**：`PendingActionCard` 从固定卡片改为底部抽屉（`Popup position="bottom" destroyOnClose closeOnMaskClick={false}`），`pendingAction` 非空即自动打开；只有标题栏 ✕ 可关闭，关闭即复用既有 `cancel_turn`（`SessionDetail` 新传 `cancelEnabled`/`onCancel`）；`cancelEnabled=false` 时 ✕ 禁用，取消命令被拒后抽屉自动重开，事项不会无人处理。抽屉视觉常量上移到 `src/components/ui.ts`（`DRAWER_SHEET_STYLE`/`DRAWER_HEADER_STYLE`/`ICON_BUTTON_STYLE`）与 `ActivityGroup` 共用。
  2. **状态并入抽屉标题**：`ActivityGroup` 详情删除独立「状态」区块，状态标签渲染在 `DrawerHeader` 标题行（新增 `status` 入参）。
  3. **答案可见（D52）**：`action_resolved` 新增可选 `answers`（每项 `{questionId, header, question, values}`），数据源是服务端校验后交给 SDK 的同一份答案，**不涉及 Claude 工具结果正文（D46 不变）**、不进 snapshot、重连不重放；Go 侧 `ResolvedAnswer` + `resolvedAnswers`/`cloneResolvedAnswers` + `validateResolvedAnswers`（仅 `action_resolved` 可携带，其它 kind 出现即协议错误）；移动端解码同规则校验，reducer 新增 `AnswerMessage`，`MessageFeed` 渲染「你的回答」（问题 + 选中值 Tag）。
  4. **文档精简**：`HANDOFF.md` 291 行 → 本文件规模，历史执行明细迁入 `docs/HANDOFF-archive.md`。
- **实际执行的验证及结论**：
  - `gofmt -l internal cmd` 无输出；`GOFLAGS=-buildvcs=false go vet ./...` 无输出；`go test ./...` 8 个包全部 ok（本次沙箱下未出现 httptest 绑定失败）；
  - `cd apps/mobile && npm test` → 16 文件 **399 passed**（本轮 +15：ActivityGroup 状态标题、PendingActionCard 抽屉 5 例、decode 答案 8 例、reducer 2 例、MessageFeed 回显 1 例）；`npm run lint` 0 error（1 个既有 warning 在 `scripts/cdp-console.mjs`）；`npm run build` 通过；
  - 浏览器实测（`--fake-host interactive` + Vite dev 5174，390×844）四点**全部通过**：审批抽屉从底部弹出且点遮罩不关闭 → 「本次允许」；提问抽屉单选+多选作答后消息流出现「你的回答」，两个多选值均可见；再次发送后点 ✕ → Claude 回复出现「已取消」且输入框恢复可用；工具详情标题右侧带状态标签且正文无独立「状态」区块。
- **未验证项**：本轮实测由浏览器子代理执行，因其 viewport 受限**未能产出截图**（结论基于 DOM 与计算样式断言）；console 无 error/warn 这一条本轮未单独取证；真实 Claude 端到端仍未验（额度受限，不阻塞）。
- **未执行**：任何 Git 写操作（按硬约束 9 仅可能用到只读 `git diff`）。

## 1. 当前目标

B2 已整体收口（结论见 `docs/testing/B2-result.md` §5）。B3 已完成并通过门禁：协议类型下放到 `protocol/` 单一 Go 来源、远程能力抽取为可导入的 `remote.Server`，Standalone CLI 行为回归通过（结论见 `docs/testing/B3-result.md`）。下一步：W1（WhaleTerm 接入，依赖 B3）与 C2（扫码/安全存储/多配对列表/真机/VPN/生命周期，依赖 C1+B2，与 B3 无直接依赖）。

```text
机器 A：PC 输入 ──> Session Controller ──> Claude
                         └──LAN WebSocket──> 机器 B 浏览器
```

## 2. 已确认事项

- 项目仓库：`/Users/zhangym/code/work/utils/claude-mobile-remote`；独立于 WhaleTerm，旧方案、阶段和记忆不继承；W1 只复用本项目新的单 Session Remote Server；
- 使用 `github.com/tea4go/claude-agent-sdk-go` 托管 Claude；一个 Standalone CLI 进程只管理一个 Session，低频并行 Session 用多进程 + 不同端口；
- CLI 保留简单行输入输出作为 PC 输入和调试入口，不产品化 Claude Code 终端；
- WhaleTerm 后续只为其已持有的一个 Session 启动进程内 Remote Server，不创建第二个 SDK Client；
- 扫码绑定真实 `sessionId`；手机认证后显式订阅对应 Session Channel，Channel 只负责消息路由；
- 移动端后续安全保存多个已扫描 Session，一次只连接当前进入的 Session；
- Claude 的完整上下文、会话状态和所有操作执行都留在 PC，手机只展示数据并回传操作；手机不展示扫码前完整历史；
- Claude 一轮输出完成只结束 Turn，不结束 Session；
- Pair Token 默认有效期 30 天（D51）；原生 App 使用系统安全存储，浏览器不写 `localStorage`；服务重启时恢复本地 pairing 文件中的未过期 token（D50 之前的配对决策），过期或删除该文件后重新扫码；服务端不做滑动续期；
- A6 为 LAN 跨机器桌面浏览器门禁；手机默认通过 VPN 直连 PC，C2 做真机验收；
- iOS / Android 使用 Capacitor（C1 已建工程），HarmonyOS NEXT 在 D1 独立开发；
- C2 依赖 C1/B2，W1 依赖 B3；B3 是单 Session Remote Server 组件化，不再建设 B4 持久恢复阶段；
- 后期公网访问使用可信的每 CLI 单出站隧道 Relay，不建设 PC 聚合 daemon、云端 Session 中心或永久 route；心跳、事件 replay、系统推送、单进程多 Session 和云端多 PC 管理后置；
- AI 可主动使用只读 `git diff`；Git 写操作与其他未明确允许的 Git 命令仍需用户授权（D23）。

## 3. 文档状态

- 常驻文档均已用户确认：`REQUIREMENTS.md`、`AGENTS.md`、`CLAUDE.md`、`DECISIONS.md`、总体设计、A5.2 后简化路线设计。
- A1–A6、B1、B2、C1、B3 的分阶段设计与实施计划均已确认并执行；门禁记录见 `docs/testing/`（A1/A2/A3/A4/A5.1/A6/B1/C1/B3 已通过，A5.2 真实 Claude 门禁因额度受限仅部分通过，`A5-result.md` 待补写）。
- 完整逐项状态表（含每个文件路径与当时结论）见 `docs/HANDOFF-archive.md` §8。
- 业务代码：B1 及之前、B2、C1 均已**本地提交未推送**；B3 及后续改动的提交状态以 `git status` 为准。

## 4. 下一步（仅未闭环项）

1. **进入 W1 设计**（WhaleTerm 为已持有的单个 Claude Session 启动进程内 Remote Server，依赖 B3 公共边界）与 **C2 设计**（扫码、安全存储、多配对列表、两端真机、VPN 与生命周期）；
2. 移动端会话列表持久化（`store.ts` 仅内存，刷新即清空）留待 C2；届时需连同硬约束 7、D41 的浏览器存储约束和 `store.test.ts` 的「不写 Storage」护栏一并正面修改，并重新评估是否下发/续期 `expiresAt`（D51 已记原因）；
3. C1 遗留待办：Origin 默认值是否加入 `http://localhost`（C1-result §7 发现 2），用户决定留待 C2；
4. 额度恢复后重跑真实 Claude 门禁（`TestLiveReservedSessionIDPreTurn`），补验首轮完成、跨 Turn 上下文和 Resume 同 ID，并一并补验 B2 的真实 Claude 工具审批端到端；
5. 按需补做 A5.2 重连/resume 人工步骤并补写 `docs/testing/A5-result.md`；
6. 行内代码当前为 `wordBreak: 'break-all'`，会在任意字符处切断标识符（B1-result §8.3），待确认是否改为 `break-word`；
7. SDK `WithSessionID` 发布正式版本后，把本地 `replace` 换回正式版本并同步 `go.mod.base`。

## 5. 首期关键验证

已验证：SDK 生命周期/流式消息/取消、真实 `sessionId` 与二维码时机、Turn 完成后 Session 可继续、终端与手机输入在单一 Controller 串行、snapshot 最小字段、显式 listen/advertise/Origin 支持跨机器访问且保持默认 loopback 边界。
待验证：手机 VPN、原生扫码、安全存储和前后台恢复（C2 真机阶段）。

## 6. 环境与已知事项

- 本机具备 Go、Node.js、Xcode；Android SDK 已就绪（platform-tools 37.0.1 / platforms 36 / build-tools 36.0.0 / emulator 37.1.11 / android-36 arm64-v8a，AVD `pixel7`），`ANDROID_HOME=~/Library/Android/sdk` 已写入 `~/.zshrc`，JDK 21 由用户 JVMS 管理；
- 模拟器只用于界面适配与构建验证；其网络经 NAT（宿主机 `10.0.2.2`），不能代替 C2 真机与 VPN 验证；
- Android 壳需 `server.androidScheme: 'http'` 才能连 `ws://` 明文（D49）；dist 加载路径下壳内 Origin 为 `http://localhost`（无端口），live reload 下为 dev server 地址（如 `http://10.0.2.2:5173`），联调需对应的 `--allowed-origin`；
- 桩服务必须以管道方式启动（CLI 读 stdin，`nohup ... &` 会因 EOF 立即退出）：`tail -f /dev/null | /tmp/cmr ... --allowed-origin http://localhost:5174`；
- antd-mobile 5.42.3 部分传递依赖与 React 19 peer 范围不一致，用户已评估可忽略；
- 本地沙箱禁止绑定 `[::1]:0`，`internal/transport` 与 `cmd/channel-test-client` 的部分 httptest 用例可能因此失败，属环境限制而非回归。

## 7. 参考提醒

WhaleTerm 的旧远程控制方案、阶段和记忆不读取也不继承。当前只参考 SDK 已验证用法；进入 W1 后再按已确认的单 Session 边界检查 WhaleTerm 当前 Claude manager/client 接口。
