# iOS 1.0(5) TestFlight发布记录

用户确认`2026-10-30-ios-test-result.md`四项界面及语音修复验证通过，要求打包上传TestFlight。本轮沿用既有Release/App Store分发配置、Team及应用身份，不新增功能、不执行Git写操作。

## 构建与包核验

- Chrome刷新核实后台最新构建为1.0(4)，正在Public Beta测试，本次采用1.0(5)。现有Public Beta公开链接`https://testflight.apple.com/join/uCHWhPEK`，上限50人、当前5人，公开范围不变。
- 执行`5_ios_testflight.sh build --build-number 5`，独立输出到`apps/mobile/ios/DerivedData/testflight-20261030-b5`，复用现有SourcePackages缓存。
- Web37文件555项测试、lint、TS/Vite构建、仅Capacitor iOS资源复制通过；lint只有既有CDP warning，Vite只有既有大chunk提示。Release Archive及App Store导出通过，codesign严格验签、IPA资源与最新dist逐字节核对通过。
- 版本1.0(5)，Bundle`com.tea4go.claudemobileremote`，Team`2WTXVQNV99`；无开发server.url，Capacitor身份正确。App Store Profile无设备名单/企业分发、get-task-allow关闭；Info.plist、Profile和实际签名APNs均production；JPush配置存在且格式合法，JPush/JCore隐私资源和新版录音动效资源在包内，ZIP完整性通过。补充核验首次受沙箱security CMS权限限制，获准重跑通过，未输出凭据/Profile全文。
- IPA：`apps/mobile/ios/DerivedData/testflight-20261030-b5/export/App.ipa`，2904989字节，SHA256`3d1c1f50f28852ad083cf7f7e97e00c5947b553a9bbe385676109f9506f7ac8d`。同级保留`App.xcarchive`。日志`/tmp/cmr-testflight-b5-build.log`。

## 上传与平台状态

已使用既有Xcode账号执行`5_ios_testflight.sh upload`。2026-10-30 11:07:52 Apple返回`Uploaded package is processing`和`Upload succeeded`，命令退出0；日志`/tmp/cmr-testflight-b5-upload.log`。Apple已接收，继续核实后台处理、合规及新构建外测关联，不以上传成功代替可安装结论。

- 后台出现1.0(5)，构建ID`1b3645c8-f7ab-4afb-95ad-9f3c6811e43d`。先显示缺少出口合规证明；尝试打开管理时工具提示用户已操作Chrome，刷新后已为“准备提交”。AI没有填写/保存该问卷，不推断用户最终选择的声明。
- 在build5“添加群组”选择现有Public Beta后打开测试内容表单。自动审批拒绝填写测试说明并调整通知选项，理由为具体功能/LAN/VPN说明发送给Apple和自动通知未获明确授权；实际拟关闭表单预选的自动通知，不主动发送通知。未绕过拒绝，已向用户呈现精确说明、通知关闭及提交Beta审核范围，等待授权。当前未点击“提交以供审核”，不标记新构建外测发布完成；未重传构建。

系统通知推送开发继续暂停；既有iOS通知实现及生产配置沿用，不部署或修改edo，不修改Android/Harmony分发。新TestFlight构建实际安装仍待验证。
