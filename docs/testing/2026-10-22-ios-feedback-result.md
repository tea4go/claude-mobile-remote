# iOS 10-22 反馈修复记录

按用户确认 1→2→3→5→4 实施。对应原始反馈 `C-ios-result.md` 的“问题列表10-22”。iOS 优先，用户最终真机验收；本轮无 Git 写操作。

## 实现与根因

1. **首次关闭无响应**：WhaleTerm Service.Stop 原先等待 transport subscription pump 退出后才 Close Remote Host；Host 订阅持续存活导致 5 秒超时，但 Registry 已删除服务，第二次才显示成功。现先停止接入、关闭仅 Remote 的适配器/事件流，再等待 websocket 退出；不停止 Manager 持有的 Claude。前端关闭期间使用已有 loading 并阻止重复点击，作废未完成的配对刷新/恢复。
2. **启动中 / done**：首轮前配对 snapshot 为 starting，后续 Turn 事件没有推送 ready；现首个 turn_started 前发布既有 session_started/ready。移动 App 把消息内 status=done 映射为协议展示状态 completed/已完成。
3. **交叉 Session loading**：Remote 空闲时 listenKey 必须保留以接收下一轮；视图切回却按 listenKey 是否非空恢复 loading，导致已完成会话重新转圈。现根据所属会话真实 running 状态恢复；手机发起的 Turn 同时固定所属会话的 model，避免读取另一个当前选中会话的全局 model。
5. **列表与名称**：取消目录背景嵌套，PC 下会话平铺，每行携带目录文字；行高至少 100px，更多操作触点至少 44×44px。PC 管理隐藏机器编码但仍用其分组。Base64 解码原先直接把二进制字符串当 JSON，导致中文 UTF-8 字节变为 Latin-1 字符；现严格 UTF-8 解码，显示层兼容恢复已存的可逆乱码名称。
4. **缓存**：完成 Happy/HAPI/Claude Code UI/Claw 源码预研；实际 SDK 已有 GetSessionMessages/Info 和 limit/offset/directory 只读接口。选择本轮先做手机已见内容的文件缓存，后续 PC 历史分页需独立授权边界及协议设计。iOS Application Support 中每 Session 一个 SHA256 文件名的 JSON，原子写/系统文件保护/排除备份/串行 I/O，无新数据库和 npm 依赖。最多 200 个展示项、单会话 2 MiB、总计 20 MiB、30 天保留。缓存文本/工具结果/已回答结果和目录/标题/时间，不存附件原件、PairToken、pendingAction/Command、连接/运行态。每秒合并写，Turn 结束及退后台立即请求写入；强杀可能丢失最近未落盘增量。重启先恢复后连接，详情显示缓存时间与 Turn 待同步；snapshot 按 requestId 校正当前 Turn。失效配对可只读查看缓存，删除配对/PC 同时清理文件，设置可清除离线缓存。

预研依据与后续历史方案：[设计](../specs/2026-10-22-ios-cache-design.md)，执行范围：[计划](../plans/2026-10-22-ios-feedback-plan.md)。

## 本机验证（2026-10-22）

- 移动全套 28 文件 **501/501** 通过；最后样式清理及缓存状态断言后，App.native + sessionCache **12/12** 定向回归通过。TypeScript/Vite build 通过。lint 仅已有 cdp-console.mjs no-unused-expressions warning。
- 缓存覆盖隔离、损坏/过期/未来时间/版本拒绝、UTF-8 字节和数量上限、去除附件/凭证/执行权限、工具与答案恢复、snapshot 去重校正、流式合并、删除/清除排序及写入失败隔离。App.native 实际页面流程覆盖启动恢复目录/消息、失效配对只读、删除清理。
- WhaleTerm `GOFLAGS=-buildvcs=false go test -vet=off ./backend/remote ./backend/chat -run 'TestService|TestClaudeRemote' -count=1` 通过；两项新增关闭/首轮测试 race 通过。保留既有仓库 vet 问题的 -vet=off 处理。
- 真实 websocket 回归：保持手机等价订阅开启，第一次 Stop 在 1 秒测试 deadline 内成功、Remote 不再可查询。首轮配对回归验证 starting→session_started/ready→turn_started→completed；保留多 Session 路由隔离用例。
- WhaleTerm Remote 前端 **23/23** 通过，新增测试执行实际 Vue 可见性处理器，在 A 已完成、B 运行期间 A→B→A 切换保持各自 Footer 状态。
- 公共 Go remote/hostinfo/internal transport 测试和 vet 通过。
- WhaleTerm 完整 `wails build -skipbindings` 成功；产物 `/Users/zhangym/code/work/whale-term/build/bin/WhaleTerm.app`。只有既有链接警告。
- 最终 Web 资源已 cap sync ios；iPhone arm64 `generic/platform=iOS` Debug 无签名编译成功，实际编译新增 Swift 文件缓存桥接。没有新增或恢复 Push capability，极光保持暂停。
- 两仓库 `git diff --check` 通过。未提交/推送。

## 用户验收反馈

2026-10-22 用户明确反馈“ios移动端验证完毕”，同意保存当前 iOS 实现并同步 Android。此前未执行的 AI 真机操作说明保留为历史事实，不替代本次用户验收反馈。极光离线推送仍暂停。

## 用户真机验收

先运行新 WhaleTerm.app，再用 `scripts/macos/3_ios_device.sh` 在 Xcode 安装更新 iOS App。

1. 开启远程管理并保持手机连接，交互数轮后单击关闭：应一次成功、手机断开，PC Claude 仍可继续。
2. 新建 Claude Session，在首轮前扫码，完成一轮后手机显示就绪/已完成。
3. 手机分别向 A/B 交叉发送、在 PC 来回切换：已完成的会话不再显示 loading，运行中的会话仍可停止。
4. 检查平铺列表、行高与更多按钮触点；中文机器名正确，PC 管理不显示机器编码。
5. 收到消息后等一秒或退后台，再强杀重开；离线时列表仍有目录/标题/摘要，进入可读缓存，Turn 为待同步且无法操作旧审批；联网后当前状态校正。设置清除缓存或删除配对后再重启复验。

## 未验证及边界

本轮未操作真实 iPhone、不标记人工验收通过；Android/Harmony 原生缓存和设备门禁后续同步。缓存不补回 App 被关闭时未收到的事件，不提供扫码前全量历史，也不提供离线发送；受容量或时间限制的旧缓存会被清理。推送相关 Apple 资料与退出后送达仍暂停。
