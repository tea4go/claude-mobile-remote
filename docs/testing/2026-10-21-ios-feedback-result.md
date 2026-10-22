# iOS 人工反馈修复验证

来源：C-ios-result.md。真机项不以编译通过替代。

2026-10-22 更新：按用户要求，iOS 极光推送暂时停用以解除 Apple 签名阻塞。下文极光配置与离线推送用例保留为恢复接入时的历史参考，当前不得据此认为极光仍在 App 中运行。在线事件本地通知继续可用；退出 App 后不接收新事件通知。

2026-10-21 用户安排：本轮先提交保存，真机/系统通知验收延至 2026-10-22，今天不继续验证；未将人工门禁标记为通过。截图保留本地，不纳入本次代码提交。

## 对应修复

| 原反馈 | 实现 | 人工复验 |
| --- | --- | --- |
| 1.1 图片仅名称/路径 | 手机已受理消息保留本次进程内图片预览；PC 既有图片组件接收经过上传校验的图片 data URL，执行提示与显示正文分离 | 发送图片、纯图片、拍照 |
| 1.2 上传后断开、重连无正文 | 修复附件显示文字与执行提示不同导致的协议误判；WhaleTerm 回复块从 32 扩大到 4096 rune，减少突发溢出；快照累积最新一轮正文并避免刷新清空 | 发图后继续多轮、长回复、中断网络后重连最新一轮 |
| 2 工具与详情 UI | v4 下发最多 16 KiB 纯文本工具结果并提示截断；v3/v2 兼容；抽屉限高内部滚动；运行中停止替换发送、加载状态、删除常驻技术提示 | Bash/Read/问答/MCP 结果与长审批抽屉 |
| 3/4 机器与首页 | 配对携带稳定 pcId 和 macOS ComputerName；首页机器/项目背景/会话白卡分层 | 重启新版 WhaleTerm、重新打开 Remote，并重扫原 Session 更新旧配对元数据；核对首页与 PC 管理 |
| 5 系统通知 | 原极光实现已暂时从 iOS App 取消链接和注册；保留在线本地提醒与通知点击定位，PC 发送实现保留 | 极光离线推送验收暂停；真机本地提醒仍待复验 |

## 极光配置（暂停接入，恢复时参考）

### iOS

本机 SDK 已从用户下载的 `/Users/zhangym/Downloads/jpush-ios-6.2.2-release` 导入。其他工作区运行：

```bash
bash scripts/macos/ios_import_jpush.sh /path/to/jpush-ios-6.2.2-release
cp apps/mobile/ios/App/Push.local.xcconfig.example apps/mobile/ios/App/Push.local.xcconfig
```

在 `Push.local.xcconfig` 填入极光控制台该 iOS 应用的 AppKey。文件已忽略，不包含 Master Secret。Bundle ID 必须与极光应用、Apple App ID、APNs 凭证匹配。在极光控制台配置该应用的 APNs 凭证，使用具有 Push Notifications 能力的签名。Debug 使用开发环境，Release 使用生产环境；工程已添加相应 entitlement。

2026-10-22 真机 Build 排查：Xcode 26.2 对当前 `App` 工程执行 Debug `generic/platform=iOS` 有签名构建，签名阶段报自动签名的开发 Provisioning Profile 缺少 Push Notifications capability 和 `aps-environment` entitlement。临时清空 `CODE_SIGN_ENTITLEMENTS` 仍报缺 Push Notifications，因为项目同时声明了该 capability。此前无签名 arm64 编译通过不能代表真机签名通过。需要在 Apple Developer 中为 `com.lingsizhang.claudemobileremote` 对应的 App ID 启用 Push Notifications，重新生成或刷新开发 Provisioning Profile，并在 Xcode 的 Signing & Capabilities 确认 Team、Bundle ID 与自动签名，重新 Build/Run。当前仓库不能代替 Apple 账号更新签名资料；此次没有物理 iPhone 安装或推送送达结论。

2026-10-22 暂停处理：移除 iOS 工程 Push capability、entitlement 构建设置、JPush/JCore 链接与原生注册；脚本不再要求下载 SDK。Web 构建及 `cap sync ios` 后，以 Xcode 26.2 执行 Debug `generic/platform=iOS` 有签名构建 **BUILD SUCCEEDED**，签名使用原 `iOS Team Provisioning Profile: com.lingsizhang.claudemobileremote`，签名 entitlements 无 `aps-environment`。未安装/操作物理 iPhone；极光和 APNs 送达未验证。恢复极光时需重新接入 SDK 和 Push capability，并准备含该能力的签名资料。

2026-10-22 Xcode 真机复核：在用户已打开的 Xcode 中，选中其 iPhone 时出现 5 个 `Missing package product` 错误（AparajitaCapacitorSecureStorage、CapacitorApp、CapacitorBarcodeScanner、CapacitorKeyboard、CapacitorPreferences）。各本地 Package.swift 均存在；通过 File > Packages > Resolve Package Versions 重建依赖图后，Xcode 对该设备显示 `Build Succeeded`，随后 Run 显示 `Running App on 我从来都不是你的热点`，控制台出现 App 运行日志。`scripts/macos/3_ios_device.sh` 已在 `cap sync ios` 后增加依赖解析；命令成功列出 9 个 Swift Package，脚本语法/help 与 diff 检查通过。当前仅证实构建、安装和启动，功能体验及本地通知仍待用户真机验收；极光推送保持暂停。

### PC（WhaleTerm / Standalone CLI）

创建 macOS 用户配置文件 `~/Library/Application Support/claude-mobile-remote/jpush.json`，权限 `0600`：

```json
{ "appKey": "YOUR_APP_KEY", "masterSecret": "YOUR_MASTER_SECRET", "production": false }
```

或使用 `CMR_JPUSH_APP_KEY`、`CMR_JPUSH_MASTER_SECRET`、`CMR_JPUSH_PRODUCTION=true/false` 环境变量。环境变量优先；Finder 启动通常使用配置文件。修改后重启 Remote/CLI。真实 Master Secret 仅留 PC，不放进 App、仓库或测试记录。未配置时不请求极光，错误/部分配置明确报错。

### 真机门禁

1. 手机启用通知，等待设备注册；已有配对会话自动绑定通知 tag。新扫码、移除或配对失效更新 tag；设置页的原生注册状态不代表 PC 发送端已配置。
2. 前台查看当前会话不弹横幅；其他会话待审批/待回答/完成/失败触发系统通知。
3. 退后台与退出 App 后，由仍运行且联网的 PC 触发事件，核对 APNs 系统通知。
4. 点击通知冷启动打开对应已配对会话；移除配对后不能打开已删除 Session；关闭通知后停止推送。
5. Relay 就绪后补公网恢复测试。PC 出站到极光的通知可以独立于 Relay 工作；点击后连接会话仍需 VPN/可达地址或 Relay。

## 边界

最新 Turn 快照正文最多 1 MiB，超限显示提示；不补偿跨 Turn 的完整离线历史和工具事件。通知队列有界并合并积压，失败只输出不含凭证/正文的诊断；没有持久推送重试队列，不将 APNs 视作必达服务。原生 SDK 二进制和平台签名不纳入 Git。Android/Harmony 原生本轮未同步，仍可按 v3/v2 连接更新后的 PC。

## 自动化记录

- 移动端：491/491 测试通过；build 通过；lint 只有既有 `scripts/cdp-console.mjs` warning；最终 `cap sync ios` 通过。
- Go：`GOFLAGS=-buildvcs=false go test ./...`、`go vet ./...` 通过；push/remote/transport 的 race 通过；hostinfo 与 CLI race 通过（含并发首次创建 PC ID）。
- 回归覆盖：附件受理后执行提示差异不再误判断线、图片预览 MIME 白名单、工具结果 HTML 纯文本及截断、v4→v3 降级与 v2/v3 输出剥离、最新 Turn 完整快照恢复与文本去重、无手机 WebSocket 时仍产生通知、极光请求鉴权与通知隐私。
- iOS：实际链接用户提供的 JPush/JCore，无签名 Debug 模拟器与真机 arm64 编译均通过；App 包内确认包含两个 SDK 的 PrivacyInfo.xcprivacy。没有安装或操作物理 iPhone，没有实际向极光发送通知。
- WhaleTerm：Remote 包全套、chat/services Remote 定向测试通过；原有无关 vet 错误使默认测试失败，定向测试明确使用 `-vet=off`。Remote 前端 22/22 通过，完整 Wails 构建通过，产物 `build/bin/WhaleTerm`；构建有既有弃用/重复链接警告。
- 两仓库 `git diff --check`、相关 Bash 语法通过。本轮没有 Git 写操作。
