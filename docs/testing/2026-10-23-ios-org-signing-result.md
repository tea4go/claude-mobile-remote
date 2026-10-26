# iOS 组织签名与真机验证（2026-10-23）

## 配置与授权

- 用户按 D45 确认 Bundle ID `com.tea4go.claudemobileremote`，接受作为新 App 安装并重新配对。
- 组织 Guangzhou Haojing Cloud Technology Co., Ltd.，Team `2WTXVQNV99`。
- 普通 Chrome 为组织 Admin，用户个人账号在无痕浏览器；Xcode 使用个人开发账号的组织开发证书。
- 用户明确授权登记当前 iPhone、占用年度测试设备名额，并在中止及咨询后再次确认继续。
- 管理员后台注册 App ID、登记当前 iPhone；设备登记页面显示 Registration Complete。未将 UDID 或签名私钥写入仓库。

## 最终产物

- 路径：`apps/mobile/ios/DerivedData/org-signing/export-registered/App.ipa`。
- 版本：1.0(1)。
- SHA256：`734d52b5144982abd4b1831ada7744822da58931159107351ee59ccb9707c6e8`。
- 归档：`apps/mobile/ios/DerivedData/org-signing/App-registered.xcarchive`。
- method=debugging，get-task-allow=true，为组织开发测试签名。不能据此认定 Ad Hoc/App Store 分发通过。
- Profile 含 4 台设备，已比对包含当前 iPhone；有效至 2027-10-23 09:22:27 UTC（北京时间17:22:27）。

## 实际验证

1. 安装 iOS 组件后 Any iOS Device 可用；上轮 Web build、cap sync ios 成功。
2. 原 IPA 仅含 3 台设备，安装失败 0xe8008012；核对确认不包含当前 iPhone。带登记参数的 archive 虽退出0，但仍使用旧 Profile，未据此认定设备登记完成。
3. 管理员后台手动登记设备后，对实际 iPhone destination 执行 Release build，生成包含当前设备的4台设备Profile；随后 generic/platform=iOS archive、debugging export 均成功。
4. 对最终 IPA 解包执行 `codesign --verify --deep --strict --verbose=2`，结果 valid on disk / satisfies its Designated Requirement。核对 Profile、Info.plist 和签名 entitlements 的 Team、Bundle ID、application-identifier 一致。
5. Capacitor 配置无外部开发服务器 URL；内置 public 的 11 个 dist 文件逐文件字节一致。
6. devicectl 确认 iPhone connected、Developer Mode enabled、DDI服务可用；最终 IPA 解包 App 安装成功。
7. `devicectl device process launch` 成功前台启动目标 Bundle ID；约19秒后 `device info apps` 核对安装版本1.0(1)，`device info processes`确认同一目标App进程仍运行。
8. 沙箱内签名信息/信任读取受限，获准访问系统信任后复验通过；`git diff --check`通过。

## 验收边界与下一步

组织签名、本机验签、当前 iPhone 安装及进程启动已通过。手机画面未直接观察，不据进程存在认定页面渲染或功能验收通过。

待用户在新 App 检查：

- 首页正常显示；重新扫码配对。
- 消息收发、审批与所需原生扫码/语音等能力。

未上传蒲公英、TestFlight 或 App Store。开发/Ad Hoc包仍要求目标设备登记并包含于Profile；免逐台登记测试可另行采用TestFlight，尚未切换渠道。无Git写操作；签名材料和产物不入库。
