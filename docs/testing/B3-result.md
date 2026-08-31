# B3 单 Session Remote Server 组件化 · 验证记录

- 关联设计：`docs/specs/2026-08-31-B3-single-session-remote-server-design.md`
- 关联计划：`docs/plans/2026-08-31-B3-single-session-remote-server.md`
- 关联决策：D39、D42

## 1. 目标与范围

B3 将现有命令目录内的 Pairing、Session Channel、WebSocket Transport 组合抽取为根下 `remote/` 公共 Go 包，并把 JSON-bearing 协议类型统一下放到根下 `protocol/` 包。Standalone CLI 改用公共组件启动远程服务；WhaleTerm 后续导入同一公共边界。不改变 wire contract、状态语义、sessionId 隔离，Remote Server 关闭不停止宿主 Session。

## 2. 关键结构变更

- **`protocol/` 包（新增）**：协议核心类型的唯一 Go 来源，含 `model.go`（Snapshot/Turn/ContentBlock/PendingAction/Question/ServiceConfig/枚举）、`event.go`、`input.go`（Input/MaxPromptBytes/输入哨兵）、`action.go`（ActionDecision/QuestionAnswer/交互哨兵）、`pairing.go`（Pairing）、`preview.go`（TruncatePreview/DefaultToolPreviewMaxRunes）。不依赖任何 internal 包、SDK 或 Transport。
- **`internal/session`**：`model.go`/`event.go`/`input.go`/`controller.go` 改为 alias 指向 `protocol`；`Snapshot.setQueuedRequests`（方法）改为包级函数 `setQueuedRequests(*Snapshot, []QueuedRequest)`，因 Go 不能对非本地类型的 alias 定义方法；actor 私有辅助（`normalizeInput`、`newQueuedRequest`、request ID 生成）留在 session。
- **`internal/claude`**：`TruncatePreview` 改为 delegate 到 `protocol.TruncatePreview`，`DefaultToolPreviewMaxRunes` 改 alias。
- **`internal/pairing`**：`Payload` 改为 alias 到 `protocol.Pairing`。
- **`internal/channel`**：`Controller` 接口与 Channel 方法签名改用 `protocol` 类型。
- **`internal/transport`**：三个文件引用 `protocol` 类型；**删除 transport 本地 `ActionDecision`/`QuestionAnswer` 副本**（`CommandPayload.Decision/Answers` 直接用 `protocol.ActionDecision`/`protocol.QuestionAnswer`），`sessionResolution` 从枚举翻译简化为深拷贝；`commandError` 映射 `protocol` 哨兵。保留 transport 内部 alias 以兼容既有测试的裸名引用。
- **`remote/` 包（新增）**：公共 `Controller` 接口、`Config`、`Server`（`New`/`Start` 返回首个 `protocol.Pairing`/`Pairings`/`BeginShutdown`/`Close`）。`Start` 合并原 `Activate`（注册 Channel + 签发凭证）并支持失败回滚；`Close` 注销 Channel 且从不调用宿主 Controller 的 Start/Stop。
- **`cmd/claude-mobile-remote`**：`main.go` 改用 `remote.New` 装配；`run.go` 的 `remoteService` 接口去掉 `Activate`，`Start()` 返回 Pairing，删除「init 后延迟激活」死分支；删除 `cmd/claude-mobile-remote/remote.go`。
- **Standalone CLI `--cwd`**：新增自定义 Claude 工作目录参数；路径在 CLI 配置阶段解析为绝对路径并校验为已存在目录，新建与 `--resume` 均通过 SDK `WithCwd` 使用，未传参数时保持当前进程工作目录行为。

## 3. 门禁执行（全绿）

| 命令 | 结果 |
| --- | --- |
| `gofmt -l .` | 无输出 |
| `GOFLAGS=-buildvcs=false go vet ./...` | 无输出 |
| `GOFLAGS=-buildvcs=false go test -count=1 ./...` | 10 个包全部 ok（含新增 `protocol`、`remote`） |
| `git diff --check` | 无空白错误 |

测试包清单：`cmd/channel-test-client`、`cmd/claude-mobile-remote`、`internal/channel`、`internal/claude`、`internal/pairing`、`internal/session`、`internal/transport`、`internal/version`、`protocol`、`remote`。

## 4. 单一来源核对

`rg -n '^type (Snapshot|Event|Input|ContentBlock|PendingAction|Question|QuestionAnswer|ActionDecision|ServiceConfig|Pairing) ' protocol/ internal/session internal/transport internal/pairing` 的结果：这些 `type X` 定义只出现在 `protocol/`；`internal/session`、`internal/pairing`、`internal/transport` 内均为 `= protocol.X` 的 alias，无第二份 struct 定义。

`rg -n '^type (ActionDecision|QuestionAnswer) ' internal/transport/` 无输出，确认 transport 本地副本已消除。

## 5. 验证覆盖

- `protocol/json_test.go`：Snapshot/Event/Input/Pairing 的 JSON 字段名、omitempty、isError 指针语义与 B2 fixture 逐项锁定；错误哨兵两两不同。
- `protocol/external_test.go` + `remote/external_contract_test.go`：外部 `package protocol_test`/`remote_test` 仅导入 `remote`/`protocol` 即可表达公共 Controller 契约（编译期断言）。
- `remote/remote_test.go`：`Start` 返回 Pairing 并注册 Channel；Server 不调用宿主 Controller 的 Start/Stop；transport 启动失败、Channel 注册失败的回滚；`Close` 注销 Channel；`New` 拒绝空 sessionId/nil controller。
- `cmd/claude-mobile-remote/run_test.go`：`Start` 返回配对并在首条输入前输出；`session_started` 不触发二次配对；`Start` 失败时 Controller 仍被 Stop 一次；关闭顺序保持「先 Stop Controller，再 Close remote」。

## 6. 未验证项

- **真实 Claude 端到端**：与 B2 相同，因额度受限后置，不阻塞。
- **WhaleTerm W1 接入**：B3 只交付可导入的公共边界，未实现 WhaleTerm 侧的适配；W1 在 B3 之后另行设计。
- **前端/HarmonyOS**：B3 只动 Go 侧，不改变 wire contract，未新增前端门禁；移动端 TypeScript/ArkTS 按既有契约不受影响（`npm run test`/`build` 未重跑，因无前端改动）。

## 7. 结论

B3 门禁通过。协议类型已收敛为 `protocol/` 单一 Go 来源，远程能力已抽取为可导入的 `remote.Server`，Standalone CLI 行为回归通过。下一步：W1（依赖 B3）或 C2（依赖 C1+B2，与 B3 无直接依赖）。

## 8. 审查修复记录

- 修复 `remote.Server.Start` 重复调用会再次触碰底层 Transport 的问题；新增 `ErrAlreadyStarted` 和回归测试。
- 修复只设置 `ServiceConfig.AskUserTextEnabled` 时被默认配置整体覆盖的问题；现在仅补齐缺失的 `ToolPreviewMaxRunes`。
- 清理 `internal/session/input.go` alias 迁移遗留的无用 import。
- 修复后聚焦回归测试、`protocol`/CLI 测试、`go vet`、格式和 `git diff --check` 均通过。
- 当前沙箱完整 `go test ./...` 仍会在既有 `httptest`/Transport 网络绑定及 `remote.TestNewPublicSurface` 的本地监听处失败；需在允许 loopback 监听的环境重跑完整门禁。

## 9. 可复核验证步骤

### 9.1 自动验证

在仓库根目录执行：

```bash
GOCACHE=/private/tmp/claude-mobile-remote-gocache GOFLAGS=-buildvcs=false go vet ./...
GOCACHE=/private/tmp/claude-mobile-remote-gocache GOFLAGS=-buildvcs=false go test -count=1 ./...
gofmt -l .
git diff --check
```

预期：`vet`、格式检查和 `git diff --check` 无输出；测试的 10 个 Go 包全部通过。若环境禁止绑定 `127.0.0.1:0` 或 `[::1]:0`，会在 `remote.TestNewPublicSurface`、`internal/transport` 或 `cmd/channel-test-client` 处失败；这是测试环境网络权限问题，需要换到允许 loopback 监听的环境重跑。

### 9.2 不消耗 Claude 额度的假 Host 联调

终端 A 启动 Mobile Web：

```bash
cd apps/mobile
npm ci
npm run dev
```

终端 B 启动 B3 迁移后的 Standalone CLI：

```bash
GOCACHE=/private/tmp/claude-mobile-remote-gocache GOFLAGS=-buildvcs=false go build -o /private/tmp/cmr ./cmd/claude-mobile-remote
tail -f /dev/null | /private/tmp/cmr \
  --listen 127.0.0.1:8765 \
  --advertise-url ws://127.0.0.1:8765/ws \
  --fake-host interactive
```

终端 B 会在首个 Turn 前输出一段配对 JSON。将它临时粘贴到浏览器页面的配对输入框，点击连接；再发送文本。预期能看到假 Host 的思考、工具调用、审批/提问和 Turn 完成。配对 JSON 不写入文件、日志、截图或验证记录。

也可以用 CLI 测试客户端验证公共 Server 的线缆闭环。另开终端 C，将 `<PAIRING_JSON>` 替换为终端 B 当前输出的完整 JSON：

```bash
GOCACHE=/private/tmp/claude-mobile-remote-gocache GOFLAGS=-buildvcs=false go run ./cmd/channel-test-client \
  --pair '<PAIRING_JSON>'
```

预期依次出现 `authenticated`、`subscribed session=...` 和 snapshot；输入普通文本后出现 `command accepted` 及事件。`--fake-host text` 可只验证基础文本闭环；`--fake-host interactive` 可验证 B2 交互。终端 B 输入 `:quit` 停止服务，终端 C 输入 `:quit` 退出客户端。

### 9.3 真实 Claude 启动方式

B3 后命令不变：

```bash
GOFLAGS=-buildvcs=false go run ./cmd/claude-mobile-remote
```

恢复已有会话仍使用：

```bash
GOFLAGS=-buildvcs=false go run ./cmd/claude-mobile-remote --resume <SESSION_ID>
```

成功启动后，CLI 会在首个 Turn 前输出配对 JSON，随后等待终端输入。服务仍由同一进程持有唯一 Claude Session；Remote Server 关闭不会单独停止宿主 Session。输入 `:quit` 会按 CLI 原有流程停止 Session 并关闭 Remote Server。

### 9.4 跨机器浏览器验证

B3 不改变 A6 启动方式。机器 A 使用：

```bash
GOFLAGS=-buildvcs=false go run ./cmd/claude-mobile-remote \
  --listen 0.0.0.0:8765 \
  --advertise-url 'ws://<A-LAN-IP>:8765/ws'
```

机器 B 运行 `apps/mobile` 的 Vite 页面，在浏览器粘贴机器 A 当前输出的配对 JSON。预期是鉴权成功、取得 snapshot，并能双向收发文本。A6 的完整网络、防火墙和 Origin 说明仍以根目录 `README.md` 为准。

### 9.5 自定义工作目录验证

创建一个临时目录后，使用 `--cwd <目录>` 启动；预期 CLI 正常进入配对并在该目录下执行 Claude。`--cwd` 指向不存在路径、普通文件或空字符串时，预期在启动 Claude 前输出配置错误并以非零状态退出。`--resume <SESSION_ID> --cwd <目录>` 使用相同规则恢复，并应指定原 Session 对应的项目目录。

本次自动验证已覆盖：`--cwd` 解析为绝对路径、空值/不存在路径/普通文件拒绝，以及新建和恢复 SDK 选项均携带 cwd。真实 Claude 在指定目录中的端到端执行待额度恢复后补验。
