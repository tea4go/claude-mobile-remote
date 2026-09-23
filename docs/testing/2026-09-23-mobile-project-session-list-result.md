# 移动端项目分组与可识别会话列表验证记录

- 日期：2026-09-23
- 范围：Standalone CLI、WhaleTerm Remote Host、Go v2/v3 wire、React/Capacitor、HarmonyOS NEXT 代码
- 状态：自动化验证完成；用户已反馈本阶段验证通过，未逐项报告具体场景；Harmony DevEco 工程构建/真机仍待补

## 人工验收反馈

- 用户于本轮反馈“此阶段验证通过”，并明确要求提交。本记录据此标记本阶段人工验收通过，不推断未单独报告的设备、系统版本或逐项操作结果。
- HarmonyOS NEXT 的 DevEco 工程构建与真机验证未在当前环境执行，仍保留为独立缺口。

## 已执行

- Go：`GOFLAGS=-buildvcs=false go build ./...` 通过；新增协议、CLI 双会话展示元数据、EventStream 标题/活动时间、v2/v3 Snapshot/Event 隔离的 `go test -race` 定向通过。
- Go 全量：`go test ./... -count=1` 除 `internal/transport` 的既有 `TestWebSocketSlowClientIsolation` 与 `TestWebSocketCloseFlushesFinalSessionEvent` 外均通过。两失败分别为快连接异常关闭和最终事件未刷出，与本轮代码无交集；未宣称全量通过。
- WhaleTerm：`go test -vet=off ./backend/chat -run 'ClaudeRemote|PermissionMode' -count=1` 通过；新增目录与创建时间快照定向测试通过。WhaleTerm 当前 `go.mod` 使用本项目本地 replace，故消费本轮公共协议和 EventStream 改动。
- React/Capacitor：Vitest **438/438**、`npm run build`、`npm run lint` 通过；lint 仅有既有 `scripts/cdp-console.mjs` warning，构建仅有大 chunk 提示。新增旧服务 v2 回退、v3 元数据解码、工作目录分组/活动排序及列表冻结/新会话并组用例。
- Harmony：7 个变更的非 UI `.ets` 文件经 TypeScript 语法解析；通过内存转译执行 `runSelfChecks()`，8 项检查通过，含新增 v3 目录/标题解码。当前环境未安装 DevEco/hvigor/ohpm，ArkUI 页面类型检查、工程构建与真机运行未验证。
- 本项目 `git diff --check` 通过；WhaleTerm 两个改动 Go 文件的 `gofmt -d` 无输出。未执行任何 Git 写操作。

## L3 React/Capacitor 列表紧凑化补充

- 用户确认只精简 React/Capacitor 列表 UI，不改项目分组、排序、协议或 Harmony 逻辑。`List.Item` 改为标题、单行预览、关键状态/活动时间/未读；整行进入，重命名和删除从“更多”菜单操作；ID、地址、来源、重复的“进入”按钮和正常“已连接”标签不再常驻显示。布局与状态样式放在独立 CSS。
- React/Capacitor Vitest **438/438**、`npm run lint`、`npm run build` 通过；测试覆盖菜单删除不误进详情、重命名、单 Session 删除隔离、未读、列表冻结及新组并入。lint 和构建仅有此前已记录的 warning。
- 紧凑度、窄屏触控与真实手机视觉效果尚未人工验收；Harmony UI 未在本次同步压缩，既有 DevEco 工程构建缺口仍在。

## 人工验收建议

1. 分别配对 CLI 两个会话和 WhaleTerm 两个会话：同 `cwd` 归入同组，不同 `cwd` 分组；旧 v2 宿主连接显示“工作目录未知”但仍能收发。
2. 首轮输入后标题取首个非空首行；第二轮、重连、重新配对都不改自动标题；手动重命名优先，清空名称恢复自动标题。
3. 运行、待处理、断线、停止/失败和未读都能辨认；列表停留时流式输出或重连不改变行序，再次进入列表后按活动时间排序，返回时“刚刚查看”定位。
4. CLI 会话仍有新建入口，WhaleTerm 会话仍无手机新建入口；目录、标题只在已授权的正确 `sessionId` 连接出现。
5. 在 DevEco 环境补 Harmony 工程构建与上述同源验收。
6. React/Capacitor 窄屏确认标题与预览截断、右侧状态/时间不拥挤；整行进入，“更多”菜单可重命名/删除且不会误进详情。
