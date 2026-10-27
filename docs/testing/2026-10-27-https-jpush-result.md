# E3 iOS 极光 HTTPS 通知验证

2026-10-27；D100/D101，用户确认实施完整范围。代码与部署包已交付，真实送达尚未验收。

## 2026-10-27 D102 故障定位与 RID 修复

- 用户真机反馈：Xcode Debug 构建已获 iOS 通知授权，App 内显示“本机远程通知已就绪”，WhaleTerm 实际触发审批，但后台无系统通知。检查 Debug `aps-environment=development`、edo `CMR_JPUSH_PRODUCTION=false` 匹配。
- 现有私有配对计算的 topic 经 PC 配置公网接口发送最小 `approval`，edo 返回 502。直接在 edo 用现有私有极光凭据向不存在的 tag 发送，得到 HTTP 403 / 极光 2009；极光官方 Push API 文档指出非 VIP 标签推送受限。用其文档中的无效 RID 发送，得到 400 / 1011（目标不存在），说明基础 RID 目标接口可调用。未输出密钥、Pair Token 或 RID 实值。
- 用户明确仅需固定文字提醒与点击进入对应 Session。按 D102 改为 iOS 向同一精确 HTTPS 路径登记 topic/RID/到期时间，edo 私有持久化后按 RID 调极光；通知正文与 APNs extras 中的 topic 保持最小化。保留 PC `POST` bearer 与现有点击本地映射。新增数字错误码诊断，不输出平台响应正文。
- `GOCACHE=/tmp/cmr-go-cache GOFLAGS=-buildvcs=false go test -race ./push ./cmd/notify-server` 通过；`go vet ./push ./cmd/notify-server` 通过；`go test ./remote ./push ./cmd/notify-server` 在允许本机 loopback bind 后通过。首次受限沙箱 remote 测试因 bind 被拒，非业务失败。`git diff --check` 通过。按用户要求未编译 WhaleTerm 或 iOS App。
- Linux amd64 服务二进制 SHA256 `8732bc350bcbdbebb2f362fad52c9fa7090e29f45a893a69975d41491205803d`，已备份旧二进制/systemd 单元至 `/etc/cmr-notify/backups/20261027T0930Z-rid/` 并部署；systemd active、loopback health204，私有状态目录0700、文件0600。原 Nginx 两条精确路由原样复用，公网 GET405、PUT204、DELETE204。测试用匿名 topic + 极光文档中无效 RID 的 POST 返回502，上游日志仅 `HTTP 400, code 1011`；DELETE 后设备文件为 `{}`，无真实设备消息。
- 未验证：新 iOS 构建在真机向 edo 登记真实 RID 后，极光/APNs 的系统送达与点击 Session 详情。用户将自行编译安装并反馈；如再失败，优先查 edo 数字错误码和 App“就绪”状态。WhaleTerm 现有 PC POST 载荷未变，无需为了功能重编，但本次更清晰的 PC HTTP 状态诊断需使用新组件版本。

## 实施

- 公共 push HTTPS sender + edo gateway/cmd/notify-server。最小JSON仅eventId/topic/kind/expiresAt；拒绝正文/Session ID/Pair Token，专用bearer、5分钟TTL、有限队列/重试、幂等/冲突/并发/限流；Master Secret只读于edo。默认宿主不再读旧jpush.json。
- iOS SDK/APNs恢复、极光tag同步/过期清理、冷启动有效本地配对映射、远程通知状态。旧版只启用本地提醒的用户需显式启用后台通知；不因升级自动启动SDK。前台走本地提醒，后台/退出走APNs。
- Linux amd64/arm64包、systemd/env/Nginx精确路径模板和部署指引。无服务器/平台配置写入，无Git写操作。

## 已执行

- `GOFLAGS=-buildvcs=false go test -race ./push ./remote ./cmd/notify-server ./cmd/claude-mobile-remote`：通过（包含旧WebSocket临时端口测试）。覆盖鉴权、未知/过期/超大载荷、并发重复/碰撞、错误重试与隐私、端到端HTTPS sender→gateway→模拟JPush、凭证轮换后丢弃排队旧路由、外部发送阻塞不阻断事件消费。
- 相同包 `go vet` 通过；最终服务启动/打包调整后 `go test ./cmd/notify-server ./push` 与服务vet通过。首次沙箱测试被禁止bind，放行本机测试权限后通过，并非业务失败。
- 只用 `/tmp/cmr-e3-whaleterm.mod/.sum` 本地replace运行 `go test ./backend/remote`：通过，日志 `/tmp/cmr-e3-whaleterm-test.log`；WhaleTerm源码/go.mod/go.sum未改。现有运行二进制仍引用旧模块，不能据此认为已切换出口。
- 移动端37文件545测试通过，日志 `/tmp/cmr-e3-mobile-tests.log`；Web构建、cap sync ios通过，lint仅既有cdp-console警告，Vite仍有既有大包提示。
- Xcode Release generic iOS `CODE_SIGNING_ALLOWED=NO`：BUILD SUCCEEDED，日志 `/tmp/cmr-e3-ios-build.log`。首次沙箱失败为SwiftPM/Clang缓存权限，放行开发工具权限后完成。Swift接口差异修正后最终无新增代码警告，仅AppIntents无依赖提示。
- 编译App的Web assets与最新dist逐字节一致，JPush/JCore隐私bundle存在，Info.plist环境为production，Bundle ID正确。未把该结果等同于签名/APNs送达。
- 有签名Debug generic iOS构建：失败，日志 `/tmp/cmr-e3-ios-signed.log`；现有Profile明确缺少Push Notifications和aps-environment。未自动更新Apple后台，不是源码编译错误。
- Linux双架构静态ELF构建、包内7文件清单及SHA256通过，禁用macOS tar附加元数据。shell语法、gofmt、git diff --check及新增文本无行尾空白检查通过。

## 交付

`.local-tools/notify-20261027-final/`（忽略入库，无真实凭证）：

- amd64 tar.gz SHA256 `48fbe7577292af29f0597e8256b092247c5e2406af49f4e71729bf382c036016`
- arm64 tar.gz SHA256 `dc6fa1cd27202990856f72b8df1710c66aeb45455b70eff52d4cfb12f40e3d9a`

部署步骤见 `deploy/notify/README.md`。当前edo可能是Docker Nginx，loopback代理模板不能直接当作容器内宿主地址；部署时复用既有受控桥接并核实精确路径。没有部署服务或修改现有下载/WAF路由。

## 未验证与人工清单

待补：Apple App ID能力/Profile、极光AppKey和APNs证书/密钥配置、edo私有token/极光凭证/部署、重新构建安装iOS及引用新组件的WhaleTerm。当前TestFlight build3不包含本次改动，未上传新构建。

用户后续反馈通过或问题即可：

1. 真机启用后台通知，本机注册就绪；App后台/退出时真实审批、问答、完成/失败提醒显示固定文案。
2. 点击冷启动进入对应有效本地配对会话，手机与PC网络互通；第二个Session/第二台手机不串线。
3. 前台正在查看的会话不提醒，其他会话不重复；关闭通知或移除配对后新事件/旧点击按设计处理（平台已入队通知的传播延迟另记）。
4. edo不可用时PC输入、审批/取消正常；恢复后只收后续通知，凭证轮换/过期正常。

APNs非必达，通知短TTL和合并、内存去重在重启后丢失、设备离线不能即时撤销tag等边界见专项设计。本轮未承诺退出后端到端送达，也未同步Android/Harmony远程推送。

## 通知点击修复验证

用户反馈真实系统通知已成功送达，重新扫码后点击仅到列表。源码确认 CapacitorBridge.init 设置默认通知 delegate，覆盖 AppDelegate 的提前设置。修复在自定义插件 load 时恢复 SessionNotificationRouter，并将 Scene connectionOptions.notificationResponse 转交共同入口；仅默认打开动作触发导航，已移除/过期 topic 不匹配本地 Session。

执行 `npm --prefix apps/mobile test -- src/App.native.test.tsx`：8/8通过，覆盖原生恢复后的点击、缺失 Session 不跳转及冷启动 retained 事件在注册监听时立即回放。`git diff --check`通过。该测试验证 Web 桥接后的导航，不证明系统原生回调；未编译 iOS/WhaleTerm、未修改 edo。用户需重新构建安装后分别验收后台点击、退出后点击及多 Session 目标准确性。

## 用户复验反馈与提交（2026-10-27）

用户在点击修复后反馈“大部分功能验证通过”，并授权提交。此前系统推送真实送达已明确确认；本次未逐项说明后台点击、冷启动、多 Session/设备及关闭撤销的验证结果，保留这些细项待确认，不将概括反馈视为全部门禁通过。本次不编译 App、不重新部署 edo。
