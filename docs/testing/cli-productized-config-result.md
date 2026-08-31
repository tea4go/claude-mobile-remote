# CLI 产品化配置 · 验证记录

- 日期：2026-08-31
- 关联设计：`docs/specs/2026-08-31-cli-productized-config-design.md`
- 关联计划：`docs/plans/2026-08-31-cli-productized-config-plan.md`
- 关联决策：D55

## 已验证

- `local`、`emulator`、`device`、`debug` profile 的默认值和底层参数覆盖规则；
- `device` 的 `--advertise-host` IPv4 覆盖和自定义监听端口派生；
- `--permission-mode`、`--model`、`--max-turns` 解析并映射到 SDK Option；
- 新建和 resume Host 保留测试注入 Client，并在创建真实 SDK Client 前应用 Claude 参数；
- README 已区分开发版日常启动和调试版底层网络排障。

## 本轮命令

```text
GOCACHE=/private/tmp/claude-mobile-remote-gocache GOFLAGS=-buildvcs=false go test ./cmd/claude-mobile-remote ./internal/claude -count=1
```

通过。

```text
GOCACHE=/private/tmp/claude-mobile-remote-gocache GOFLAGS=-buildvcs=false go vet ./...
GOCACHE=/private/tmp/claude-mobile-remote-gocache GOFLAGS=-buildvcs=false go build ./cmd/claude-mobile-remote
```

均通过。

## 未验证项

- 用户已完成开发版日常启动、Android 模拟器 profile 和真机 `device` profile 验收；LAN/VPN 两种网络路径均按 `device` 语义验证通过。
- `device` 自动选择多网卡时仍建议通过 `--advertise-host` 明确指定；本轮不引入网卡探测优先级配置。

真实 Claude 的额度相关 live 测试仍未执行；本轮不影响 CLI 参数和网络 profile 验收。
