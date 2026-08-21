# A1 项目骨架实施计划

- 日期：2026-08-21
- 子阶段：A1 项目骨架（里程碑 A）
- 依据：`REQUIREMENTS.md` §9.1、D14（A1 只创建 Go CLI、React/Vite 与基础工程配置）、总体设计 §10
- 状态：待用户确认

**目标**：创建可独立启动、构建和测试的 Go CLI 骨架与 React/Vite 移动 Web 骨架，补齐工程级 `.gitignore` 与 README 快速开始，作为后续 A2–A6 的地基。

**边界（A1 不做）**：不引入 `claude-agent-sdk-go` 依赖（A2）；不创建 Session/Channel/Transport/Pairing 任何代码（A3–A4）；不创建 Capacitor / HarmonyOS 工程（C1 / D1）；不做二维码、WebSocket、CLI 交互逻辑。

**约定**：本计划编写时按原 D11 由用户自行提交；该规则现已被 D20 替代——AI 默认不主动执行 Git，用户明确要求时可在该次授权范围内执行。本次 A1 收口已获得用户明确提交授权。

---

## 前置条件

| 项 | 已验证 | 说明 |
| --- | --- | --- |
| Go ≥ 1.25 | ✅ go1.25.3 darwin/arm64 | |
| Node ≥ 20 | ✅ v25.4.0 | |
| npm | ✅ 11.12.1 | |
| Xcode | ✅ | A1 不使用，C 阶段才需要 |

## 文件结构（A1 交付后新增）

```text
claude-mobile-remote/
├── cmd/claude-mobile-remote/main.go      # CLI 入口：启动时打印启动横幅后退出（A1 占位）
├── internal/version/version.go           # 版本号（用于 -version 标志与后续日志）
├── internal/version/version_test.go
├── go.mod                                # module github.com/tea4go/claude-mobile-remote
├── apps/mobile/                          # React + TS + Vite + antd-mobile
│   ├── package.json
│   ├── vite.config.ts
│   ├── tsconfig.json / tsconfig.node.json
│   ├── index.html
│   └── src/
│       ├── main.tsx                      # antd-mobile 挂载
│       ├── App.tsx                       # 骨架页：标题 + 状态徽标占位
│       ├── App.test.tsx                  # vitest 渲染测试
│       └──（其余目录 B 阶段再建）
├── .gitignore                            # 在现有 Go 规则上追加前端规则
└── README.md                             # 追加"快速开始"一节
```

设计要点：

- `internal/version` 是 A1 唯一带逻辑的 Go 代码。选它是因为每个后续阶段都需要版本化日志/协议握手，A1 顺手建立 `-version` 这个最小可测行为，避免空壳 main 无测试可用；
- 前端骨架保持 Vite react-ts 模板默认结构，只加 antd-mobile 依赖与一个页面级渲染测试，不加状态管理、路由、协议客户端（YAGNI，A4/A5 再加）。

---

## Task 1：Go module 与版本包（TDD）

**Files:**
- Create: `go.mod`
- Create: `internal/version/version.go`
- Create: `internal/version/version_test.go`
- Create: `cmd/claude-mobile-remote/main.go`

- [ ] **Step 1.1：初始化 module**

Run: `go mod init github.com/tea4go/claude-mobile-remote`
Expected: `go: creating new go.mod: module github.com/tea4go/claude-mobile-remote`（go 指令行显示 go 1.25.3）

- [ ] **Step 1.2：写失败测试**

`internal/version/version_test.go`：

```go
package version

import "testing"

func TestVersion(t *testing.T) {
	if Version == "" {
		t.Fatal("Version must not be empty")
	}
}
```

- [ ] **Step 1.3：运行确认失败**

Run: `go test ./internal/version/`
Expected: FAIL，报 `Version` 未定义（`undefined: Version`）

- [ ] **Step 1.4：最小实现**

`internal/version/version.go`：

```go
// Package version 提供构建版本信息，供 CLI 标志、日志和后续协议握手使用。
package version

// Version 是当前开发版本。发布流程确定后改为 ldflags 注入，当前先固定。
const Version = "0.1.0-dev"
```

- [ ] **Step 1.5：运行确认通过**

Run: `go test ./internal/version/`
Expected: `ok github.com/tea4go/claude-mobile-remote/internal/version`

## Task 2：CLI 入口

**Files:**
- Create: `cmd/claude-mobile-remote/main.go`

- [ ] **Step 2.1：实现 main**

```go
// claude-mobile-remote 在 PC 上托管本地 Claude 会话，并通过 WebSocket
// 向连接 VPN 的手机提供会话接力（A1 仅提供骨架入口）。
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/tea4go/claude-mobile-remote/internal/version"
)

func main() {
	showVersion := flag.Bool("version", false, "打印版本并退出")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.Version)
		return
	}

	fmt.Printf("claude-mobile-remote %s — A1 skeleton\n", version.Version)
	fmt.Println("会话托管与手机接力将在后续子阶段实现（见 docs/plans/）。")
	os.Exit(0)
}
```

- [ ] **Step 2.2：构建并运行验证**

Run: `go build ./... && go run ./cmd/claude-mobile-remote -version`
Expected: 第一条命令无输出；第二条输出 `0.1.0-dev`

Run: `go run ./cmd/claude-mobile-remote`
Expected: 输出两行横幅后退出（退出码 0）

- [ ] **Step 2.3：gofmt 检查**

Run: `gofmt -l .`
Expected: 无输出（所有文件已格式化）

**检查点：Task 1–2 完成后暂停，待用户检查并提交 Go 骨架。**

## Task 3：React/Vite 移动 Web 骨架

**Files:**
- Create: `apps/mobile/`（Vite 脚手架生成后裁剪）
- Modify: `apps/mobile/package.json`（脚本与依赖）
- Create: `apps/mobile/src/App.test.tsx`

- [ ] **Step 3.1：脚手架生成（在仓库根目录执行）**

Run: `npm create vite@latest apps/mobile -- --template react-ts`
Expected: 生成 `apps/mobile/` 目录，包含 package.json、vite.config.ts、src/ 等；命令行提示成功

- [ ] **Step 3.2：安装依赖并加 antd-mobile**

Run: `cd apps/mobile && npm install && npm install antd-mobile`
Expected: 安装成功无 error；`package.json` dependencies 出现 `antd-mobile`（当前 5.42.3）

- [ ] **Step 3.3：安装测试依赖（vitest + testing-library）**

Run: `cd apps/mobile && npm install -D vitest @testing-library/react @testing-library/jest-dom jsdom`
Expected: 安装成功；devDependencies 出现 vitest

- [ ] **Step 3.4：配置 vitest**

`apps/mobile/vite.config.ts` 整体替换为：

```typescript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  // vitest 配置：jsdom 环境 + react 插件复用
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: './src/setupTests.ts',
  },
})
```

`apps/mobile/src/setupTests.ts` 新建：

```typescript
import '@testing-library/jest-dom'
```

`apps/mobile/package.json` 的 scripts 增加（保留其他脚本）：

```json
"test": "vitest run"
```

- [ ] **Step 3.5：写失败测试**

`apps/mobile/src/App.test.tsx`：

```tsx
import { render, screen } from '@testing-library/react'
import App from './App'

test('renders skeleton title', () => {
  render(<App />)
  expect(screen.getByText('Claude Mobile Remote')).toBeInTheDocument()
})
```

- [ ] **Step 3.6：运行确认失败**

Run: `cd apps/mobile && npm run test`
Expected: FAIL，`Unable to find an element with the text: Claude Mobile Remote`

- [ ] **Step 3.7：最小实现（替换模板 App）**

`apps/mobile/src/App.tsx` 整体替换为：

```tsx
import { NavBar, Tag } from 'antd-mobile'

function App() {
  return (
    <div style={{ minHeight: '100vh', background: '#f5f5f5' }}>
      <NavBar back={null}>Claude Mobile Remote</NavBar>
      <div style={{ padding: 16 }}>
        <Tag color="default">A1 骨架 · 会话接力将在 A5 实现</Tag>
      </div>
    </div>
  )
}

export default App
```

同时删除模板附带的演示文件：`apps/mobile/src/App.css`、`apps/mobile/src/index.css`（如 App.tsx 不再引用）、`apps/mobile/src/assets/react.svg`。保留 `main.tsx`，但移除其中 `index.css` 引用（若有）。

- [ ] **Step 3.8：运行确认通过**

Run: `cd apps/mobile && npm run test`
Expected: PASS，1 个测试通过

- [ ] **Step 3.9：构建验证**

Run: `cd apps/mobile && npm run build`
Expected: `vite build` 成功，输出 `dist/` 产物大小信息，无 TypeScript 错误

**检查点：Task 3 完成后暂停，待用户检查并提交前端骨架。**

## Task 4：工程配置与 README

**Files:**
- Modify: `.gitignore`（追加前端规则）
- Modify: `README.md`（追加快速开始）

- [ ] **Step 4.1：追加 `.gitignore` 规则**

在现有文件末尾追加：

```gitignore

# Node / 前端（apps/mobile 及未来原生工程）
node_modules/
dist/
*.local
npm-debug.log*
.DS_Store

# 前端测试与缓存
coverage/

# 环境与密钥
*.pem
*.key
```

- [ ] **Step 4.2：README 追加"快速开始"**

在「文档地图」小节之前插入：

```markdown
## 快速开始

前提：Go ≥ 1.25 与 Node ≥ 20。

```bash
# Go CLI（A1 骨架）
go run ./cmd/claude-mobile-remote -version

# 移动 Web（A1 骨架）
cd apps/mobile && npm install && npm run dev
```

构建与测试：`go test ./...`、`gofmt -l .`（无输出即通过）、`cd apps/mobile && npm run test && npm run build`。
```

- [ ] **Step 4.3：验证 gitignore 生效**

Run: `go build ./... && cd apps/mobile && npm run build`
Expected: 两条构建成功。随后 `git status`（**由用户执行**）应显示 `node_modules/`、`dist/` 不在未跟踪列表中

**检查点：Task 4 完成后暂停，待用户检查并提交。**

## Task 5：A1 门禁验证与记录

- [ ] **Step 5.1：跑全部门禁命令**

```bash
go test ./...          # Expected: ok github.com/tea4go/claude-mobile-remote/internal/version
gofmt -l .             # Expected: 无输出
go vet ./...           # Expected: 无输出
go build ./...         # Expected: 无输出
cd apps/mobile && npm run test && npm run build
# Expected: 测试 PASS；构建成功
```

- [ ] **Step 5.2：写门禁记录**

新建 `docs/testing/A1-result.md`，记录：执行日期、各命令实际输出摘要、环境（Go 1.25.3 / Node 25.4.0）、结论（通过/不通过）、遗留问题（如无则写"无"）。

- [ ] **Step 5.3：更新 HANDOFF**

按 D17 收尾：完成项、验证结论、未验证项（如"二维码、WebSocket 未实现，属后续子阶段"）、下一步（编写 A2 计划）。

**检查点：A1 门禁通过后，待用户检查 `docs/testing/A1-result.md` 与 HANDOFF 更新并提交。**

---

## 自查记录

- **Spec 覆盖**：REQUIREMENTS §9.1 A1 行（Go CLI ✅ Task 1–2；React/Vite ✅ Task 3；基础工程配置 ✅ Task 4）；D14 边界（无 SDK 依赖 ✅；无 Capacitor/HarmonyOS ✅）；HANDOFF §6 gitignore 缺口 ✅ Task 4.1；README 快速开始（前轮商议的 A1 交付项）✅ Task 4.2。
- **占位符扫描**：所有代码步骤均给出完整代码；无 TBD/TODO。
- **类型一致性**：`version.Version` 在 Task 1 定义、Task 2 使用，一致；前端测试选择器与 App.tsx 文本一致。
- **门禁对照**：REQUIREMENTS A1 通过条件"Go 与移动 Web 骨架可独立启动、构建和测试"——Task 2.2（启动）、Task 1.5/2.2/3.9（构建）、Task 1.5/3.8（测试）、Task 5 汇总，全覆盖。
