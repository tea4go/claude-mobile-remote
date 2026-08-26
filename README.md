# claude-mobile-remote

通过独立 Standalone CLI 托管一个本地 Claude Session，让用户离开电脑后可在手机上继续查看和控制。移动端后续可保存多个已扫描 Session；WhaleTerm 可复用同一单 Session Remote Server，但每个活跃 Session 始终只有一个 Host 和一个 SDK Client。

## 如何开始

无论首次加入还是中途接手，都从项目根目录按以下顺序开始：

1. 启动开发工具，并按“文档地图”完整阅读项目文档；
2. 以 `HANDOFF.md` 作为当前进度的唯一来源，重点核对“当前执行”、未验证项和下一步；
3. 若上一任务已通过子阶段门禁或完成重要联调，再查看 `docs/testing/` 中的验证记录；
4. AI 先向工作人员复述当前状态、待处理问题和下一步，确认后再修改文件；
5. 按当前子阶段的实施计划执行；尚无实施计划时，先编写计划并确认，不直接开始编码；
6. 阶段设计文档由 AI 起草后交工作人员检查；检查完成后，只有工作人员明确要求时 AI 才执行 Git 提交，否则由工作人员自行提交；
7. 任务结束前，由 AI 按 `AGENTS.md` 更新交接、决策和必要的验证记录。

如果文档之间不一致、已有结果无法复核或任务边界不清楚，应先停止并报告，不自行推断后继续。工作人员负责范围确认、决策、体验验收和 Git 写操作授权；AI 负责开发、验证和文档维护，可主动使用只读 `git diff` 检查改动，但暂存、提交、推送、恢复、分支、合并等操作仍需工作人员明确授权。

## 文档地图

新会话按以下顺序阅读：

1. `AGENTS.md`：长期背景、技术约束和协作规则；
2. `HANDOFF.md`：当前进度、下一步和已知问题；
3. `DECISIONS.md`：已确认的产品与技术决策；
4. `REQUIREMENTS.md`：完整产品需求与阶段范围；
5. `docs/specs/2026-08-21-claude-mobile-remote-design.md`：总体技术设计；
6. `docs/specs/2026-08-25-simplified-roadmap-and-session-remote-design.md`：A5.2 后的单 Session、移动端多配对、阶段依赖与可信 Relay 路线。

`CLAUDE.md` 用于引导 Claude Code 读取同一套项目记忆，避免维护重复规则。

## 快速开始

前提：Go ≥ 1.25 与 Node ≥ 20。

`go.mod`/`go.sum` 是本机状态（可能含指向本地 SDK 的 `replace`），不入库；仓库提交的是基准 `go.mod.base`。克隆后先恢复：

```bash
cp go.mod.base go.mod && go mod tidy
```

```bash
# Go CLI
go run ./cmd/claude-mobile-remote -version

# 移动 Web
cd apps/mobile && npm install && npm run dev
```

构建与测试：`go test ./...`、`gofmt -l .`（无输出即通过）、`cd apps/mobile && npm run test && npm run build`。注意 `go vet` / `go build` 需加 `GOFLAGS=-buildvcs=false`（go.mod 不入库的副作用，见 D18）。依赖变更时：本地改 `go.mod`，确认后同步更新 `go.mod.base` 提交。

## 仓库

https://github.com/tea4go/claude-mobile-remote
