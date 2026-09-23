# Standalone CLI 多会话自动化联调记录

日期：2026-09-23。状态：自动化联调通过；用户反馈实际测试整体通过，并确认本阶段完成。用户未逐项报告所有场景细节，Harmony 工程构建仍未验证。

## 覆盖与结果

- Go Runtime Manager：创建独立 A/B、输入与事件隔离、同 `commandId` 幂等、默认容量 4、停止释放容量、未知来源拒绝、创建限流、终端目标切换及 EOF 等待 Turn；CLI/Claude/协议包定向 `go test -race` 通过。
- loopback WebSocket：A 订阅后创建 B，B 停止后仍可用 B 凭证订阅终态并再新建；B 凭证不能鉴权 A。测试使用 fake Host，不代表真实 Claude 或手机人工验收。
- `GOFLAGS=-buildvcs=false go build ./...` 通过。全量 `go test ./... -count=1` 仍有既有 `internal/transport` 两项生命周期失败：慢客户端隔离会关闭快速连接、Server Close 未刷出最终事件。
- React/Capacitor：Vitest 429/429，lint、build 通过。lint 仅有既有 `scripts/cdp-console.mjs` warning，build 仅有既有大 chunk 提示。
- HarmonyOS NEXT：公共协议、配对、Client、Store 的非 UI `.ets` 文件完成语法解析及内建 self-check 用例扩展。当前机器缺 DevEco/hvigor/ohpm，UI 与工程构建尚未验证。

## 人工反馈与剩余限制

用户反馈多会话测试“整体通过”，并在试用中确认终止后原 Session 无法继续运行、WebSocket 仍保留连接；这是已确认的终态与配对入口语义。用户最终选择保留详情页新建入口、移至页面最底部，并优化终止操作的文案/样式；曾提出的列表新建与已结束分区未采用。UI 收尾后 React/Capacitor Vitest 431/431、lint、build 与 `git diff --check` 通过；lint 仅有既有脚本 warning，build 仅有既有大 chunk 提示。未收到逐项验收清单，不能推定每个场景均单独通过；Harmony 仍需在 DevEco 环境构建。

## 后续入口优化验证

用户随后要求隐藏移动端终止入口，并将 CLI 配对详情的新建入口改为输入区左下角 `+`；React/Capacitor 原附件入口改用现有 `document.svg`，无 CLI 创建能力的 WhaleTerm 配对不显示新建入口。React/Capacitor Vitest **433/433**、build、lint 和 `git diff --check` 通过；测试覆盖 CLI/WhaleTerm 能力门控、终态可新建、创建中禁用和附件图标。lint 仍只有既有脚本 warning，build 仍只有既有大 chunk 提示。Harmony UI 同步调整，但当前机器无 DevEco/hvigor/ohpm，工程构建及真机体验仍未验证。本轮入口变更已获用户确认测试通过；原生 Harmony 工程仍需在 DevEco 环境构建。
