# B1 消息与移动 UI 验证记录

- **子阶段：** B1 消息与移动 UI
- **日期：** 2026-08-26
- **范围：** `apps/mobile/`（页面内存 Session 列表、列表/详情单连接导航、受限 Markdown、Thinking/Tool 只读占位、会话错误展示）
- **未触碰：** Go 端（`internal/`、`cmd/`）、`apps/mobile/src/protocol/types.ts`、协议 envelope、snapshot/event 字段、Pairing payload、`DECISIONS.md`

## 1. 自动化验证

在 `apps/mobile` 执行：

| 命令 | 结果 |
| --- | --- |
| `npm ci` | 通过，锁文件可复现安装 |
| `npm test` | **14 files / 240 tests 全部通过**（B1 前基线 9 files / 167 tests） |
| `npm run lint` | `oxlint` 0 warnings 0 errors（32 files / 116 rules） |
| `npm run build` | `tsc -b` + `vite build` 通过，1106 modules |
| `git diff --check` | 通过，无空白错误 |

新增测试分布（+69）：

| 文件 | 数量 | 覆盖 |
| --- | --- | --- |
| `src/session/store.test.ts` | 17 | 新增/同 ID 更新/不同 ID 并存、删除、重命名（含空名回退）、状态与 `lastConnectedAt`、排序稳定性、不写浏览器存储 |
| `src/components/SessionList.test.tsx` | 11 | 空态、新增入口、名称/ID 摘要/地址摘要/状态/最近连接、就地重命名、进入、删除、不渲染 Pair Token |
| `src/components/SessionDetail.test.tsx` | 9 | 名称与状态栏、消息流、返回、发送、重连、配对失效、协议错误、命令错误、不渲染 Pair Token |
| `src/components/MessageMarkdown.test.tsx` | 18 | 受限语法 7 项 + 安全边界 7 项 + 折行策略 4 项（代码块/表格不折行、列表与顶层段落保持自然折行） |
| `src/components/StructuredBlockPlaceholder.test.tsx` | 6 | 只读占位文案、无按钮/输入、不出现审批与取消字样、不伪造工具字段 |
| `src/components/MessageFeed.test.tsx` | +4 | Claude 文本走 Markdown、恶意 HTML 不入 DOM、输入消息保持纯文本、空文本不渲染气泡 |
| `src/components/SessionStatus.test.tsx` | +2 | `sessionError` 展示与缺省不展示 |
| `src/App.test.tsx` | +6 | 返回列表（关闭连接、清 Fragment）、单连接生命周期（进入/返回/切换只保留一个活动连接）、刷新边界 |

回归：A5 / A5.1 / A5.2 既有 167 个测试全部保持通过，未修改其断言语义，只把「更换配对」入口改名为「返回」列表。

## 2. 第三方依赖结论

新增运行时依赖（精确版本锁定）：

| 包 | 版本 | 许可证 | 用途 |
| --- | --- | --- | --- |
| `react-markdown` | 10.1.0 | MIT | 受限 Markdown 渲染（虚拟 DOM，不用 `dangerouslySetInnerHTML`） |
| `remark-gfm` | 4.0.1 | MIT | GFM 表格与删除线 |

- **未引入 `rehype-sanitize`。** 依赖门禁实测结论：不启用 `rehype-raw` 时 `react-markdown` 不解析 raw HTML，`javascript:` 等协议由内建 `html-url-attributes` 过滤，再加 `disallowedElements: ['img']` 后，全部安全断言通过。按设计 §7.4 不装冗余依赖；若 B2 需要放行受信 HTML，必须先补该插件并重跑安全测试。
- **未引入第二套 UI 组件库。** 列表、卡片、标签、输入、导航栏全部用现有 antd-mobile 组件，缺口为零。
- **包体影响（诚实记录）：** 传递依赖 +98 个包（unified/remark/rehype 生态）；产物 `index-*.js` 由 233.69 kB（gzip 74.57 kB）增至 390.06 kB（gzip 121.11 kB），即 +156 kB / gzip +46.5 kB。这是本阶段唯一显著的体积代价，Capacitor 内嵌 Web 资源同样承担；如后续需要压缩，可在 C 阶段考虑按需拆包，但不在 B1 处理。
- **无浏览器全局依赖：** 两个包为 ESM，不要求 `window`、`localStorage`、Node 运行时或外部脚本；jsdom 下测试全绿即为佐证。

## 3. 安全与边界核查

- **Markdown 安全（自动化断言）：** raw `<script>`、`<iframe>`、`onerror`/`onclick` 属性均不进入 DOM 且不执行；`javascript:` 链接的 `href` 不含 `javascript`；`![]()` 图片语法被 `disallowedElements` 拦下，不发起远程请求；代码块内 HTML 保持纯文本；外链统一带 `rel="noreferrer noopener"`。
- **不使用 `dangerouslySetInnerHTML`：** 全仓无该调用；`MessageMarkdown.tsx` 中只有一条说明性注释提到它（静态扫描器对该注释误报 MEDIUM，非实际用法）。
- **浏览器存储边界（硬约束 7、D41）：** `grep localStorage|sessionStorage|indexedDB` 仅命中 `store.ts` 的约束注释与 `store.test.ts` 的 spy 注释，无任何写入调用；`store.test.ts` 用 `vi.spyOn(Storage.prototype, 'setItem')` 断言 Store 操作全程零次调用。页面刷新即清空列表。
- **Pair Token 不外泄：** 列表只显示 `host:port` 和 `sessionId` 前 8 位；`SessionList` 与 `SessionDetail` 各有一条 `container.innerHTML` 断言确认 token 不出现在 DOM。
- **无 B2 控件：** 占位组件不渲染任何 `button`/`input`/`textarea`，且断言文案不含「批准/允许/拒绝/取消」，避免被误读为可用审批能力。
- **本记录不含真实 `sessionId`、Pair Token、用户会话内容或构建产物。**

## 4. 单连接不变量

`App.tsx` 用独立的 `activePairing` 作为连接 effect 依赖（不用 `SessionRecord`），因为重命名与状态更新会产生新记录对象，若以记录为依赖会触发误重连。测试断言：进入详情创建 1 个 client；返回列表调用 `close()` 并清空 Fragment；切换到第二个 Session 后未关闭的 socket 恒为 1 个。

实施中踩到并修掉的一个真实缺陷：`now` 的默认参数是内联箭头函数，每次渲染引用都不同，早期版本把它放进了连接 effect 依赖数组，导致无限重连、测试进程挂死。现改为 `nowRef` 读取，依赖只保留 `[activePairing, socketFactory]`。

## 5. 跨端结论

- React Web 与 C1 Capacitor 直接复用本实现（同一份 Web 资源）。
- **HarmonyOS NEXT 不复用 npm 组件**，只复用语义契约：同一受限 Markdown 语法子集（标题、粗体、列表、行内代码、代码块、GFM 表格与删除线）、raw HTML 一律纯文本、不加载远程图片、外链安全过滤、Thinking/Tool 只读占位不伪造数据。ArkUI 侧用原生富文本或自绘实现同一子集；若某语法无等价能力，降级为纯文本展示，不改协议。
- C1 原生插件仍需另行提供 iOS / Android / HarmonyOS 能力矩阵；本阶段未引入任何原生插件。

## 6. 人工浏览器验收

**状态：✅ 用户已执行并反馈通过（2026-08-26）。** 用户在真实浏览器逐项测试以下清单后反馈「基本的功能没有问题」，并提出 §8 的排版优化建议（已处理）；优化后再测确认通过。

1. 新增多个配对 JSON → 列表出现多条记录，同一 `sessionId` 再次新增只更新不重复；
2. 就地重命名生效且不影响 `sessionId`、地址与其他记录；空名称回退默认摘要；
3. 进入 → 返回 → 切换另一个 Session，PC 端观察任意时刻只有一个活动 WebSocket；
4. Claude 回复中的标题/列表/代码块/表格正常展示，代码块与表格可横向滚动且不擅自折行，列表与段落保持自然折行（见 §8）；
5. 状态区正确显示连接、会话、Turn、队列提示；会话失败时显示「会话错误」原因；消息带电脑端/手机端来源标签；
6. 刷新页面 → 列表清空（回到配对页）；带 `#pair=` 的详情链接刷新仍能重连当前 Session，且不恢复旧消息；
7. 页面无任何审批/拒绝/取消控件，Thinking/工具调用区域明确标注「只读占位 · 后续支持」。

## 7. 未实现与后置

- Thinking/Tool **结构化数据**未接入（协议尚无对应事件），B1 只有只读占位；结构化事件、审批、AskUserQuestion 与取消在 B2 设计。
- 服务器端 Session 列表、跨重启持久化、原生安全存储不在 B1（分别属产品边界外与 C2）。
- 代码块无语法高亮（按计划只用 CSS/`pre`，不引入高亮器或编辑器）。

## 8. 优化（用户反馈）

### 8.1 已采纳：表格与代码块不擅自折行

用户实测反馈【尽量不折行、展示不下就用横向滚动，便于阅读】。实现：`<pre>` 加 `whiteSpace: 'pre'`；`<th>`/`<td>` 加 `whiteSpace: 'nowrap'`；表格外层包裹加 `overflowX: 'auto'`；移除最外层 `wordBreak: 'break-word'` 对表格/代码块的干扰。适用理由：列对齐与缩进本身携带信息，折行会破坏结构。

### 8.2 已回退：列表项 nowrap + 横向滚动

曾按用户要求把同一原则扩到列表（`<li>` 加 `nowrap`、`<ul>`/`<ol>` 外层加 `overflowX: 'auto'`，并用 `InListItemContext` 处理松散列表内 `<p>` 盖掉 `nowrap` 的问题）。**用户实测后判定不可用并要求回退**：列表属于正文，没有列对齐结构，而移动端几乎每条都超宽，逐条向右拉到尾再拉回最左，阅读负担反而变大。

已删除 `InListItemContext` 与 `MarkdownParagraph`，`ul`/`ol` 恢复为无包裹元素，`li` 不再自定义。保留一条回归守卫测试锁定该结论（`li.style.whiteSpace === ''` 且无 `overflowX` 包裹）。

**结论**：横向滚动只适用于有内在对齐结构的内容（表格、代码块）；正文（段落、列表）保持随宽度自然折行。后续三端实现均按此边界。

### 8.3 待定：行内代码 `wordBreak: 'break-all'`

行内代码目前为 `break-all`，会在任意字符处切断标识符（如路径被断成两段），属于同类的「擅自折行」问题，尚未修改。
