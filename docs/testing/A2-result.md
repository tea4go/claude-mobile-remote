# A2 SDK 托管门禁记录

- 日期：2026-08-21
- 子阶段：A2 SDK 托管
- 结论：通过
- SDK：`github.com/tea4go/claude-agent-sdk-go v0.1.1`（本地 replace 验证）
- Claude Code：`2.1.179 (Claude Code)`

## 自动验证

| 命令 | 结果 |
| --- | --- |
| `go test ./...` | PASS |
| `go test -race ./internal/claude ./cmd/claude-mobile-remote` | PASS |
| `gofmt -l .` | PASS（无输出） |
| `GOFLAGS=-buildvcs=false go vet ./...` | PASS |
| `GOFLAGS=-buildvcs=false go build ./...` | PASS |
| 真实 Claude 双轮 integration test | PASS |

> 注：`go vet` / `go build` 必须带 `GOFLAGS=-buildvcs=false`。这是 D18（go.mod 不入库）的副作用：`.git` 存在而 `go.mod` 被 ignore 时，Go 的 VCS stamp 读取失败。首次记录漏写该 flag，已在审查后修正（见下方「审查修复记录」）。

## 设计假设核对

- init 在首轮 Result 前提供非空 session ID：通过；
- Result session ID 与 init 一致：通过；
- 同一 Client 顺序执行两个 Turn：通过；
- 第二 Turn 延续第一 Turn 上下文：通过；
- `ReceiveResponse` 统一传递消息与流错误：通过（fake 错误测试与真实流共同验证）。

## 完成前评审修正

- 每个 `ResultMessage` 必须自带非空 session ID，并与 init/已有 ID 一致；首轮和后续 Turn 的空 ID 均有 session scope 失败测试；
- A2 工具权限回调有确定性单元测试，验证 deny、非空理由且不升级为 interrupt；
- CLI 空闲等待输入时可被 context 取消，并验证 Host 随后关闭；
- 错误 Result 同时保留 `Result` 与 `Errors` 诊断，失败 Turn 的部分文本后补换行；
- 修正后代码复核无 Critical 或 Important 问题，完整 unit/race/gofmt/vet/build 与真实双轮门禁重新通过。

## 审查修复记录（2026-08-21，用户走查后）

严格审查发现两个 Important 问题并已修复，修复后全部门禁 fresh 重跑通过：

1. **I-1 AssistantMessage.Error 静默成功**：原 `parseMessage` 忽略 SDK `AssistantMessage.Error` 字段（`server_error` / `authentication_failed` 等），API 出错时 CLI 会输出 `turn_completed` 且退出码 0。修复：`parseMessage` 提取错误类型与文本（`assistantErrorFact`）；`RunTurn` 收到错误 assistant 即终止当前流，发射 `turn_failed`，按类型分级——`server_error` / `rate_limit` 为 turn scope（可重试），其余（认证、计费等）为 session scope。分级依据 WhaleTerm 验证行为，未移植其错误文本关键字匹配（留给 A3）。新增测试：`TestParseMessage/assistant_error_*`、`TestHostAssistantErrorFailsTurn`；
2. **I-2 门禁命令记录失实**：原记录誊写 `go vet ./...` / `go build ./...`，实际必须 `GOFLAGS=-buildvcs=false`（D18 副作用）。修正本记录与 README、AGENTS.md 三处文档。

真实门禁在受限沙箱内曾因外部 API `ConnectionRefused` 失败；在允许网络访问的执行环境中重跑同一测试后通过。完成前评审修正后的最终 fresh 门禁用时约 14.5 秒。

本记录不包含真实 session ID、prompt、回复、认证信息或用户配置。
