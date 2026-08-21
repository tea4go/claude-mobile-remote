# A1 门禁验证记录 — 项目骨架

- 日期：2026-08-21
- 子阶段：A1 项目骨架（里程碑 A）
- 对应计划：`docs/plans/2026-08-21-A1-project-skeleton.md`
- 环境：macOS（darwin/arm64），Go 1.25.3，Node v25.4.0，npm 11.12.1
- 结论：**通过**

## 交付物

| 项 | 说明 |
| --- | --- |
| `go.mod` | module `github.com/tea4go/claude-mobile-remote`，Go 1.25 |
| `internal/version/` | 版本包 + 单元测试（TDD 流程） |
| `cmd/claude-mobile-remote/main.go` | CLI 入口：`-version` 标志 + 启动横幅 |
| `apps/mobile/` | Vite react-ts 脚手架 + antd-mobile 5.42.3 + vitest/jsdom |
| `.gitignore` | 追加前端忽略规则（node_modules、dist、密钥等） |
| `README.md` | 追加「快速开始」一节 |

## 门禁命令与实际结果

| 命令 | 预期 | 实际 |
| --- | --- | --- |
| `go test ./...` | version 包 ok | ✅ ok（cmd 无测试文件属预期） |
| `gofmt -l .` | 无输出 | ✅ 无输出 |
| `go vet ./...` | 无输出 | ✅ 无输出 |
| `go build ./...` | 无输出 | ✅ 无输出 |
| `go run ./cmd/claude-mobile-remote -version` | 输出 `0.1.0-dev` | ✅ `0.1.0-dev` |
| `go run ./cmd/claude-mobile-remote` | 横幅两行，退出码 0 | ✅ 符合，exit=0 |
| `apps/mobile: npm run test` | 1 个测试通过 | ✅ 1 passed |
| `apps/mobile: npm run build` | tsc + vite 构建成功 | ✅ 成功（JS 197.40 kB / gzip 63.58 kB） |

gitignore 生效：`node_modules/`、`dist/` 均被忽略（`git check-ignore` 验证；脚手架自带 `apps/mobile/.gitignore` 与根 `.gitignore` 双重覆盖）。

## TDD 过程记录

- Go version 包：先写 `TestVersion` 确认 FAIL（`undefined: Version`），再实现 `Version = "0.1.0-dev"` 确认 PASS；
- 前端 App：先写渲染测试确认 FAIL（找不到标题），再替换模板 App 确认 PASS。

## 计划外调整（实施中发现并处理）

1. **tsconfig 需补 vitest 类型**：模板的 `tsconfig.app.json` `types` 只有 `vite/client`，`tsc -b` 报 `Cannot find name 'test'/'expect'`。已加 `"vitest/globals"`；
2. **vite.config.ts 需类型引用**：`UserConfig` 无 `test` 字段报 TS2769。已按 vitest 官方方式加 `/// <reference types="vitest/config" />`；
3. **模板裁剪**：删除了模板演示文件（App.css、index.css、assets、icons.svg），`main.tsx` 移除 index.css 引用。

以上均为使计划门禁命令通过的最小修复，不改变计划范围。

## 已知事项（不阻塞门禁）

- antd-mobile 5.42.3 的部分传递依赖（@react-spring 系）peer 声明仅到 React 18，与模板 React 19 存在 npm ERESOLVE 警告（npm 已 override 安装）。当前测试与构建均正常；若后续组件运行异常，可评估降 React 18 或等 antd-mobile 更新。属供应链层面已知状态，留观；
- Android SDK / DevEco Studio 未安装，C1 / D1 前准备（原计划已知）。

## 遗留问题

无。A1 门禁通过，可进入 A2（SDK 托管）计划编写。

## 提交前复核

- 日期：2026-08-21
- 原 A1 门禁命令已全部重新执行并通过；额外执行 `npm run lint`，结果通过；
- 用户已评估并确认走查发现可忽略；
- Git 操作规则按用户确认新增 D20：AI 默认不执行 Git，用户明确要求时仅在授权范围内执行。本次用户已明确授权全部 Git 操作；实际按任务目标只提交 A1，不主动执行无必要的推送、合并、恢复或分支操作。
