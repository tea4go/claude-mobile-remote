# WhaleTerm 双向队列实施与验证记录

日期：2026-11-09。依据：D110、D120 及队列设计的 2026-11-09 补充。状态：源码实现及 Go 定向验证完成；用户反馈 queue 功能验收通过。

## 实施内容

- WhaleTerm Manager FIFO 为唯一执行队列；PC 原先延迟提交的 Vue 待发项改为及时提交，Vue 只保留展示及完整草稿元数据。宿主等待项上限 32，截断预览 200 字符；不建立第二个 SDK Client 或队列执行器。
- Manager 在变更锁内捕获完整视图及 revision，锁外发布 Wails 队列事件；Remote Host 转为现有 v8 queue_changed。拒绝迟到旧视图，生命周期快照抢先刷新时同样广播最新队列，订阅和重连取当前状态。
- PC 入队、自然出队、取消、引导成功、编辑取回和排序均影响宿主；移动端取消/引导调用同一接口。取消与引导经活动 Turn 锁串行并重新检查归属，避免 SDK 写入期间释放附件或重复执行；排序按目标 ID 移动，不用旧全列表覆盖并发新增项。
- PC 保留完整图片、附件和其他草稿字段；明确拒收时保留可编辑的失败草稿，未知结果不自动重发。编辑先准备图片和验证原草稿，再请求宿主移除，成功后回填；若期间页面变化导致回填失败，保留可取回的本地失败草稿，不自动重新入队。手机来源项未持有完整 PC 草稿，不提供 PC 编辑按钮。
- React 输入卡 permission 右侧仅在宿主支持且队列非空时显示图标与数量；打开限高抽屉查看顺序、来源和预览，取消或引导按连接和活动状态门控。断线保留上次视图、禁用操作，重连快照校正；异常 Toast，队列清空/新审批出现时收起。
- 新增 PC 专用队列快照/取消/排序服务及手写窄桥接，不手改生成的 Wails 文件。共用协议继续 v8，旧版本过滤保持；Standalone 不开放此入口，Harmony 已有解析保持兼容，独立 UI 尚未同步。iOS/Android 共享 React 源码，未进行设备验收。

## 实际验证

1. 公共 Remote：`GOFLAGS=-buildvcs=false go test -race ./internal/channel ./remote ./internal/transport` 通过，涵盖可选能力门控、队列快照/事件及传输版本兼容。
2. WhaleTerm：使用临时 modfile 指向本地 Remote 源码，`go test -vet=off -race ./backend/chat ./backend/remote ./backend/services -run 'TestClaudeQueueSync|TestClaudeRemote|TestClaudeCode.*(Queue|Steer|Cancel|Turn)|TestController' -count=1 -timeout=120s` 通过（GOFLAGS 含 `-buildvcs=false`）；services 没有匹配测试，完成包和测试编译。
3. 以 WhaleTerm 正式 go.mod/go.sum 的临时副本、不加本地 Remote replace，在 `GOFLAGS='-buildvcs=false -mod=readonly'` 下执行相同回归，通过。当前 v0.0.9 已含所需接口，未升级依赖或发布 CLI。
4. 新增 Go 用例覆盖 PC/手机来源、多订阅广播、取消、排序、出队、重连、刷新先于迟到广播、引导与取消竞争只写一次，以及满队列拒绝保留原队列。同步更新 PC 队列及移动入口/状态回归用例，前端用例均未执行。
5. 默认 WhaleTerm go test 被无关 DeepWiki/Gemini 历史 vet 错误拦截（动态格式串及 Sprintf 的 `%w`）；不修改这些历史文件，后续定向显式关闭 vet。构建存在既有 macOS 语音弃用和链接警告，不写成无警告通过。
6. 两仓 `git diff --check` 无诊断；新增文件 no-index 空白检查无诊断（存在差异时返回 1），本轮全部 Go 文件 `gofmt -l` 无输出。

按用户要求未运行任何 npm lint/build/test，未构建/安装两端应用或复制旧 Web 资源；没有真实 SDK 端到端、iPhone、Android 或 Harmony 设备验收。没有 Git 暂存、提交或推送。

## 人工验收清单

请同时更新 WhaleTerm 和移动端后验证：

1. Claude 运行中，PC 连续排队，再由手机排队；两端顺序、数量和来源一致，每项仅执行一次。空队列隐藏入口，非空入口位于 permission 右侧。
2. PC 排序、取消、取回编辑并重新发送，手机及时更新；图片/附件草稿取回完整，提交失败可恢复文字与附件。
3. 手机取消 PC 项、PC 取消手机项；两端分别引导。目标已开始或被其他端移除时 Toast，不误取消当前回复，不重复发送。
4. 自动出队直至清空；停止当前回复后保留既有 FIFO，下一项正常继续；新审批到来不被队列抽屉遮住。
5. 手机断线期间 PC 改队列，重连以最新快照收敛；断线时禁用操作，结果未知不自动重发；切换 Session 不串线。
6. 长文本、多项、横竖屏及键盘安全区：抽屉限高可滚动，工具栏不被挤压；Standalone/旧宿主不误显示入口。Android 与 Harmony 按各自当前范围另验。

验收结果（2026-11-09）：用户反馈 queue 功能验收通过，未逐项报告设备及操作场景；不据此推断 Android、Harmony 或清单中每项均已验证。移动端旧会话缓存问题按用户要求暂缓。
