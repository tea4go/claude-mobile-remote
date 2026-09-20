# W1 Task 2 结果：WhaleTerm → remote/protocol 跨仓库公共依赖

- 日期：2026-09-09
- 关联：W1 实施计划 Task 2（`docs/plans/2026-08-31-W1-whaleterm-in-process-remote-plan.md`）
- 结论：**通过**。WhaleTerm 可编译调用 `remote.New`/`Server.Start`/`Server.Close`，协议类型来自 `protocol` 单一来源，未复制 DTO，未 import 任何 `internal/` 包。

## 1. 实施内容

在 WhaleTerm（`/Users/zhangym/code/work/whale-term`）开发环境接入本项目公共包：

1. `go.mod` 取消注释两行（此前被 W1 前期准备注释）：
   - `require` 段：`github.com/tea4go/claude-mobile-remote v0.0.0-00010101000000-000000000000`
   - `replace` 段：`replace github.com/tea4go/claude-mobile-remote => /Users/zhangym/code/work/utils/claude-mobile-remote`（本地联调阶段精确目录 replace）
2. `go mod tidy` 成功（exit=0），tidy 后 go.mod 形成新 require 块：
   ```go
   require (
       github.com/fsnotify/fsnotify v1.9.0
       github.com/tea4go/claude-mobile-remote v0.0.0-00010101000000-000000000000
   )
   ```
   并把 `github.com/skip2/go-qrcode`（本项目 `remote` 的传递依赖）合并进 indirect 段。
3. 新增最小外部编译测试 `whale-term/backend/remote/w1_external_contract_test.go`（`package remote_test`）：
   - `remote.New(remote.Config{...})` → `Server.Start()` 返回完整 Pairing（WSURL/SessionID/PairToken 均非空）→ `Server.Close(ctx)` 正常返回；
   - `var _ remote.Controller = (*fakeController)(nil)` 断言适配器可实现 `remote.Controller`；
   - 空 SessionID / nil Controller 被 `remote.New` 拒绝；
   - `remote.Controller` 六方法全部按 `protocol` 类型实现（Submit/Subscribe/Snapshot/ResolveAction/CancelTurn/CancelQueued）。

## 2. 验证证据

- `go mod tidy`：两次均 exit=0；第二次（测试文件就绪后）输出 `go: found github.com/tea4go/claude-mobile-remote/protocol` 与 `go: found .../remote`。
- `cd whale-term && go test -count=1 ./backend/remote/` → **ok 0.689s**（3 个用例全部 PASS，非缓存）。
- `cd whale-term && go vet ./backend/remote/` → 无输出（VET-OK）。
- `gofmt -l whale-term/backend/remote/` → 无输出（格式干净）。
- `go mod graph` 确认存在 whale-term → claude-mobile-remote 依赖边及其 4 个传递依赖。

## 3. 提交基准未污染

- WhaleTerm `go.mod` / `go.sum` 均被 `.gitignore` 忽略（`.gitignore` 第 150 / 105 行），本地 require+replace 只是开发态依赖，不会进入提交。
- `go.mod.base`（仓库提交基准）经核对**无任何 mobile-remote 引用**，未被本次开发改动污染。

## 4. 正式版本依赖的切换条件（待办记录）

当前阶段用本地 `replace` 指向本项目目录联调。切换条件（稳定后执行）：

1. `github.com/tea4go/claude-mobile-remote` 发布正式版本 tag；
2. 在 WhaleTerm `go.mod` 把 require 改为正式版本号，删除本地 `replace`；
3. 运行 `go mod tidy` 确认解析到正式版本；
4. 同步更新 `go.mod.base` 提交基准（把正式版本行并入对应 require 段，并补充 `go mod tidy` 收敛出的完整依赖清单）；
5. 重新运行本 Task 的编译测试与 `go vet`。

切换之前，本地 `replace` 属开发态状态，**不得**写入 `go.mod.base`。

## 5. 通过条件复核

- WhaleTerm 能编译调用 `remote.New`/`Server.Start`/`Server.Close`：**满足**（外部编译测试通过）；
- 协议类型来自 `protocol` 单一来源：**满足**（测试 import 仅 `.../remote` 与 `.../protocol`）；
- 没有复制 DTO：**满足**（测试只引用协议类型，无本地副本）；
- 不 import `internal/`：**满足**（测试与后续适配器均只 import 公共包）。
