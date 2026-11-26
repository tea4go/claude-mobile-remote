# HANDOFF.md — Claude Mobile Remote 交接文档

更新于 2026-11-26。新会话先读 [AGENTS.md](AGENTS.md)，再按任务查阅 [DECISIONS.md](DECISIONS.md)、[REQUIREMENTS.md](REQUIREMENTS.md) 和[总体设计](docs/specs/2026-08-21-claude-mobile-remote-design.md)。

## 当前状态

- **会话详情与后台任务**：11 月 24 日 iOS 第三轮体验已获用户确认，Android 和 HarmonyOS 已同步 v14 后台任务、按需读取工具详情、回复模型和发送提示。旧 PC 按协商版本降级。共享端 714 项测试通过，Android 已在 Pixel 7 模拟器安装并冷启动，HarmonyOS 106 项测试和 API 24 构建通过。PC 队列和后台续答已跑定向回归。详见 [D134、D135](DECISIONS.md) 和[联调记录](docs/testing/2026-11-24-mobile-test-result.md)。
- **11 月 25 日移动端修复**：统一配对首页和会话列表的 PC 状态，长任务快照优先保留回复正文，重连按请求 ID 修正消息和后台步骤位置，调整两套详情界面。HarmonyOS 1.0.0（1000016）调试包已通过 USB 安装并启动，iOS 真机由用户构建验收。详见 [11 月 25 日测试记录](docs/testing/2026-11-25-mobile-test-result.md)。
- **11 月 26 日离线恢复复验**：React 与 ArkTS 在读取旧缓存时按 request ID 校正提问和回复。缓存限额内保留已见提问，子工具汇入原轮次的父工具入口，HarmonyOS 已同步三项静态状态图标。两端测试及 Android、HarmonyOS 本地构建通过，同一会话的真机冷启动待复验。详见[验证记录](docs/testing/2026-11-25-mobile-test-result.md#11月26日修复与验证)。
- **有序离线缓存与消息时间**：三端原生缓存已升级为 v2，有序时间线和有界步骤索引共用单个 2 MiB 文件。共享 PC 宿主已支持 v15 消息时间，旧宿主继续降级。Go、React、ArkTS 自动化和 Android、HarmonyOS 调试构建通过。HarmonyOS 调试包 1.1.2（1000018）已安装并在手机启动，真机会话联调待复验。详见 [D136](DECISIONS.md) 和[验证记录](docs/testing/2026-11-25-mobile-test-result.md#有序缓存与消息时间验证)。
- **HarmonyOS 隐私政策与发布**：1.0.0（1000015）Release APP 已验签，开发者中心显示正在审核且已选择隐私托管。托管协议 `2090871319812905728` 的网页正文可打开。用户从 AppTest 重新安装 1.1.2（1000017）后，已确认应用内隐私政策入口可打开。详见[构建记录](docs/testing/2026-11-25-harmony-1.0.0-b1000015-release-build.md)和[正式发包手册](docs/harmony-release.md)。
- **1.1.2 三端测试发包**：Apple 已接收 iOS 18 并开始处理，HarmonyOS 1000017 邀请测试 APP 与 Android 9 华为推送 APK 已本地签名并归档。详见[发包记录](docs/testing/2026-11-25-mobile-1.1.2-test-release.md)和[测试发包手册](docs/mobile-test-release.md)。

## 待验收与后续

- **移动端联调**：用含 D136 的客户端和已接入 v15 的 PC 复验长任务中途扫码、锁屏与退出重进。以原验收会话检查关闭 PC 服务后的冷启动顺序和步骤位置，再启动 PC 验证重连及工具详情。iOS 由用户构建。Android、HarmonyOS 连接新版 PC 的后台续答与新轮 FIFO、实时工具详情和旧 PC 降级，以及 iPhone 与 Windows 的事件顺序复验和 C2-D VPN 门禁仍待确认。
- **HarmonyOS 上架**：等待 1.0.0（1000015）审核结果，并在可通过正式渠道安装后验证「设置 → 关于 → 隐私政策」。上架隐私标签仍待核对。
- **iOS 分发**：确认 TestFlight 1.1.2（18）处理结果、测试群组和实机安装。
- **较早的待核对项**：图片重启后由 PC 按需恢复、SDK 超限后的发送异常、Intel Mac CLI 验证、WhaleTerm 稳定 PC 标识接线、断线 replay。接到相关任务时先核对源码和最新证据，再安排复测。平台审核和离线通知按对应模块查阅[阶段计划](docs/plans/2026-10-27-remaining-work-and-offline-notifications-plan.md)及后续决策。

## 当前执行

- **三端测试发包手册修订（已完成）**：按用户确认，指定显示版本后依据本地记录递增构建号，无需发包前核对开发者后台；明确 T7 HarmonyOS 工具链路径并检查与脚本一致。已核对手册命令与脚本的 `HARMONY_TOOLS_HOME` 一致，保留上传结果不明确时的恢复处理；`git diff --check` 通过。本次仅修改文档，未重新发包。

- **1.1.3 三端测试发包（已交付）**：源码 `961bbcd`，用户要求不核对后台；iOS 19 已上传并获 Apple 接收，HarmonyOS 1000019 发布 APP 与 Android 10 华为推送 APK 已签名验签并归档。共享测试 726 项、三份产物 SHA-256 复核通过。HarmonyOS 使用 T7 现有 6.1.1 工具链，先前仅检查默认路径导致误报缺失。TestFlight 处理和实际安装待确认。详见 `docs/testing/2026-11-26-mobile-1.1.3-test-release.md`。

- **手机端验证与提交（2026-11-26）**：用户确认目前手机端验证已通过；记录该反馈并提交当前改动。既有自动化和构建结果见下方及验证记录，其他未明确验收的项目仍按待验收清单跟进。

- **HarmonyOS 有序缓存真机装机（已完成）**：在隔离工程将含 D136 的代码编译为 1.1.2（1000018）API 24 Debug HAP，ZIP 检查、华为验签和既有设备调试 Profile 比对通过。安装前设备当前用户未查到同包名应用。USB 安装成功，设备确认 1.1.2（1000018）及 `debug` 签发，EntryAbility 已启动并查到进程。待用实际会话复验离线冷启动、重连与工具详情。详见[装机记录](docs/testing/2026-11-25-mobile-test-result.md#harmonyos-有序缓存真机装机)。

- **会话缓存分层与消息时间（已实施，待真机复验）**：Go 全量 race、React 726 项、HarmonyOS 114 项及 22 文件逻辑检查通过。Web 生产资源、Android Debug APK、HarmonyOS API 24 未签名 Debug HAP 构建及两包 ZIP 检查通过。iOS Swift 语法检查通过，原生构建由用户执行。已验证旧缓存读取、大型 Thinking 与 MCP 的正文省略、两处步骤占位、裁剪与重连保序，以及 v15 时间的传输和落盘。历史消息缺少真实时间时留空。详见[验证记录](docs/testing/2026-11-25-mobile-test-result.md#有序缓存与消息时间验证)。

- **11 月 26 日真机反馈（已实施，待真机复验）**：React 724 项、HarmonyOS 113 项和 22 文件逻辑检查通过，覆盖旧缓存、长轮次截断、100 个子工具、旧宿主快照、跨轮续答和队列。Web 生产资源、Android Debug APK 与 HarmonyOS API 24 未签名 Debug HAP 构建通过，两包通过 ZIP 检查。三项 HarmonyOS 静态图标与来源逐字节一致，运行加载沿用现有动画。下一步复验原会话的离线冷启动及重连。详见[验证记录](docs/testing/2026-11-25-mobile-test-result.md#11月26日修复与验证)。

- **HarmonyOS 隐私政策核查（2026-11-26，已完成）**：比对两份签名 APP 的应用内点击逻辑及隐私配置。两包的查询、HTTPS 链接过滤、浏览器跳转和失败提示一致，均依赖平台下发链接。用户确认 AppTest 重新下载安装后的入口可用，最初报错的安装疑为此前本地调试部署，原构建身份未核实。1.0.0 送审包完成静态与后台核查，正式分发环境的真机验证待完成。

- **三端测试发包手册已修订**。HarmonyOS 以同包名本地可核实的最高构建号递增，并在本机完成签名、验签和归档。已核对取号和交付步骤均无需访问 HarmonyOS 开发者后台，后台上传与邀请测试由发包人处理。详见[测试发包手册](docs/mobile-test-release.md)。

- **ccBuddy 1.1.2 三端测试发包（已交付）**：隔离构建源码 `30d1974`。iOS 18 已上传 TestFlight 并收到 Apple 回执，HarmonyOS 1000017 和 Android 9 的本地发布签名包均已验签。React 720 项、HarmonyOS 108 项及 22 文件逻辑检查、三个文件 SHA-256 复核均通过。归档 `.local-tools/releases/20261125-v1.1.2-test/172408/` 附更新列表及校验文件。详见[发包记录](docs/testing/2026-11-25-mobile-1.1.2-test-release.md)。

- **11 月 25 日移动端真机反馈（已实施，待功能验收）**：Go 全量 race、React 720 项、Web 构建、Android Debug APK、HarmonyOS 108 项和 22 文件逻辑检查均通过。HarmonyOS 以既有调试 Profile 构建、验签、USB 安装并启动，手机确认版本 1000016。此次构建号仅用于隔离调试包。收到安排前启动的 iOS 模拟器构建已完成，最终 iOS 真机版本由用户构建。已记录真机验收步骤和旧 PC 回退边界。详见 [测试记录](docs/testing/2026-11-25-mobile-test-result.md)。

- HarmonyOS MyPhone 发布签名已配置自动读取本机密码文件，正式版和邀请测试手册已接入默认配置。配置权限为 0600，已在关闭标准输入的情况下使用私钥生成验证用 CSR。本轮未重新构建签名 APP。

- 交接文档已精简为当前状态和未闭环项。原文逐字节归档，引用路径已核对。此次只整理文档。
- 新任务开始前按 AGENTS.md 登记范围和通过条件，结束后更新对应状态。后续将已被替代的检查点移入本地归档。

## 本地资料

- `docs/` 已在 `main` 的 `7cd98f9` 移出 Git 跟踪并设为忽略。新克隆需要从已有本地工作区取得设计、测试证据和归档；遇到缺失资料时记录待核对项。
- 历史按需回查 [早期归档](docs/HANDOFF-archive.md) 和 [2026-11-25 交接快照](docs/HANDOFF-archive-2026-11-25.md)。
