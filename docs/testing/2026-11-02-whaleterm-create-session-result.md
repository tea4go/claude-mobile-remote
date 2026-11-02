# WhaleTerm 同目录远程新建：实现与复用审查

- 日期：2026-11-02。
- 范围：公共组件与 `/Users/zhangym/code/work/whale-term` 本机源码；无 Git 写操作、依赖发布、桌面安装或移动端发版。
- 设计：`../specs/2026-11-02-whaleterm-create-session-design.md`；需求归属见 D108。

## 实现结果

目录菜单启动配对服务，二维码预先包含有效 UUID sessionId 与凭证；仅认证不会把空会话加入 PC 列表，订阅成功后才加入。手机既有 `+` 通过同一 WhaleTerm Manager 在固定 cwd 创建新 Session，返回独立配对，并立即刷新 PC 列表。新 Session 首轮前无 JSONL 时可通过既有详情入口打开空会话；实际 SDK 首轮与后续输入仍由现有 Manager/FIFO 处理。

已配对会话独立持有 Remote Server；创建结果按 source/commandId 幂等，保留有界缓存及限流/容量。关目录二维码只隐藏，重开复用未配对草稿；关闭源 Remote 不停止子会话或 Claude。

## 复用审查

用户指出原有端口机制已经支持独立会话不同端口，要求审查并避免重复实现。核对确认 `backend/remote/Registry.Start` 已在 EADDRINUSE 时递增端口，并通过 `startConfigWithPortOffset` 同步配对地址；本轮临时新增的分配器已完整删除，Registry 本身未改。原先把固定基准端口误判为多会话冲突缺口的结论撤回。

同时核对并复用：

- `resolveSessionForRequest` 的自定义 ID 与 `CreateClaudeCodeChat` 的 Client/FIFO，无第二个运行时管理器。
- 原有 Pairing、鉴权订阅、续期、WebSocket 管理消息及二维码内容组件。
- React/Capacitor 与 Harmony ArkTS 已有的 `+`、创建响应解析、保存及连接逻辑，本轮未修改移动端实现。
- WhaleTerm 本地会话的稳定 chatKey、列表刷新、详情加载、Remote 状态恢复及 W1 实时监听。

仅新增公共可选 SessionCreator / OnSubscribed 宿主桥接、WhaleTerm 创建事务、首轮前空会话投影与目录入口。Standalone 的 `managedCLI` 位于 cmd/main 且管理其自身 Controller，不能用于持有 WhaleTerm 的 Session；未复制其运行时管理器。未使用的 Host 工厂注入也已移除。Manager 身份判断调用已有 `currentClaudeSessionID`。

## 实际验证

公共组件：

```sh
GOCACHE=/private/tmp/cmr-go-cache GOFLAGS=-buildvcs=false go test -race ./...
GOCACHE=/private/tmp/cmr-go-cache GOFLAGS=-buildvcs=false go vet ./...
GOCACHE=/private/tmp/cmr-go-cache GOFLAGS=-buildvcs=false go build ./...
```

全部通过。初次默认缓存访问及回环监听受沙箱阻止；改用临时缓存，回环测试获准后重跑通过。包含未认证/未订阅管理消息、公共可选创建入口、跨 Session 拒绝及订阅成功通知等既有/新增回归。

WhaleTerm：使用临时 `/private/tmp/cmr-whale.mod`，在原 go.mod 基础上仅为验证添加本机公共组件 replace；未改 WhaleTerm 的正式依赖文件。

```sh
GOCACHE=/private/tmp/cmr-whale-go-cache GOFLAGS=-buildvcs=false go test \
  -modfile=/private/tmp/cmr-whale.mod -vet=off -race \
  ./backend/remote ./backend/services ./backend/chat \
  -run 'TestRemote|TestRegistry|TestStartConfigWithPortOffset|TestServiceStart|TestClaudeRemoteSessionHost|TestOnSubscribed' -count=1
```

通过。真实回环 WebSocket 覆盖 hello → subscribe/snapshot → create_session；验证认证前/仅认证不显示草稿、成功订阅后 PC 可见、关闭重开复用、规范身份/同 cwd、两个真实监听端口不同、手机创建可见、同 commandId 不重复、限流/容量、关闭源 Remote 后子 Remote 存活。另测草稿回滚不误删不同 ID 或已启动会话；沿用既有 Registry 端口重试及 Host 隔离测试。WhaleTerm 定向测试关闭 vet，因为仓库其他既有 deepwiki/gemini vet 错误；不宣称 WhaleTerm 全量 vet 通过。macOS speech/链接器既有告警保留。

前端：`node --test tests/claudeRemoteManagement.test.mjs` 28 项通过；`npm run build -- --outDir /private/tmp/cmr-whale-dist` 成功。既有 CSS/deep/chunk 告警保留。ESLint 因现有缺失 `.eslintrc-auto-import.json` 未完成，未修改配置绕过。两仓 `git diff --check` 通过；既有无关工作区修改保留。

## 限制与下一步

WhaleTerm 当前正式依赖为公共组件 v0.0.4，不包含本轮新增 API。现阶段是本机源码联调通过；正式依赖构建须先发布包含新增 API 的公共组件版本，再更新 WhaleTerm 依赖。本轮未获 Git 发布授权，未提交/tag/push，也未用机器专属 replace 污染正式依赖文件。临时 modfile 仅用于本机验证，不是可移植发布结果。

未执行真实 SDK Turn、WhaleTerm 桌面运行体验或 iOS/Android/Harmony 真机扫码验收；Go 元数据/网络测试不能代替真实 UI 与 SDK 闭环。首轮前草稿只驻内存，应用重启前尚未产生 JSONL 的草稿不承诺恢复。知识库受控会话创建本轮明确拒绝。

人工验收清单（依赖接入后反馈通过或问题即可）：

1. Claude 列表目录启动远程 CLI：二维码可扫，未配对时列表无新增；关闭重开复用同一 ID。
2. 手机扫码完成后：PC 同目录出现新会话；手机与 PC 交替发送，身份/正文/审批/取消同步，原会话继续运行。
3. 手机 `+`：同目录两端均新增另一会话；分别发送，消息和权限请求不串线。
4. 关闭源 Remote、手机断线重连及 PC 切换会话后，子会话仍可独立使用。


## 用户复验与列表激活优化

用户复测确认目录扫码及手机 + 创建通过，之前扫码超时不再记为可复现缺陷；未提供逐平台/SDK Turn 验收明细。本轮反馈为新会话未置首、未自动打开。

原因：本机事件只刷新列表，未调用已有会话选择及更新时间能力；目录会话的时间还停留在打开二维码的预留时刻。修复复用 selectClaudeLocalSession、touchClaudeLocalSession 及原分组排序、激活后的目录展开/滚动定位，不调用 createClaudeLocalDraftSession 另分配 UUID。本机事件仅两条首次创建路径携带 activate=true，活动刷新为 false；目录正式创建时间为配对完成时。公共协议、手机及 SDK 生命周期不变。

本轮验证：node --test tests/claudeRemoteManagement.test.mjs 全部 31 项通过；新增两种创建按原排序置首/打开、普通活动不抢焦点、打开失败不伪造草稿的执行回归。使用既有临时本地 modfile，TestRemoteDirectoryPairingAndMobileCreationStayInWhaleTerm 的 Go race 回归通过（-vet=off，保留既有 macOS 告警）。npm run build -- --outDir /private/tmp/cmr-whale-dist 生产构建通过（保留既有 CSS/deep/chunk 告警），两仓 diff --check 通过；用户随后反馈本轮优化验证通过；未提供逐平台或完整 SDK Turn 验收明细。

验收：目录扫码成功后，以及手机 + 后，PC 同目录新会话均出现在第一条并打开；切到其他会话后，旧会话运行/完成事件不会抢走当前页面。

用户验收结果（2026-11-02）：用户反馈上述优化“验证通过”。
