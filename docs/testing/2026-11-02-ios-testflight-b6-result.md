# iOS 1.0(6) TestFlight 签名包

用户要求生成 IPA 自行上传并整理更新说明。App Store Connect 的 iOS 构建列表确认最新为 1.0(5)，本次采用 1.0(6)。使用当前工作区代码，其中包含尚未提交的 iOS 及移动端改动；未上传、未发布、未进行 Git 写操作。

- 产物：`apps/mobile/ios/DerivedData/testflight-20261102-b6/export/App.ipa`；SHA256：`66bf61ff102c494fa16362891d4517175818bfaad9c3a77a14aca039176aa90a`。同级保存归档，构建日志 `/private/tmp/cmr-testflight-b6-build.log`。
- 移动端全量 39 文件 568 项测试、lint、TypeScript/Vite 构建、TestFlight 脚本 5 项测试通过；lint 和 Vite 有既有提示。Release archive 和 App Store 导出成功；脚本验证 Bundle、分发 Profile、严格签名、无开发服务器地址及包内 Web 资源与构建结果一致。
- 用户此前确认会话列表滚动和消息复制测试通过；本次分发包尚未安装验证。当前输入同步需配套更新 WhaleTerm 正式依赖后再做端到端验收；11-02 反馈第 3、5 项按用户要求继续暂停，其他待复验项目见 HANDOFF。

2026-11-02 17:26：用户要求继续 TestFlight 发布。复用已验证归档上传，Apple 返回 `Uploaded package is processing` 和 `Upload succeeded`，日志 `/private/tmp/cmr-testflight-b6-upload.log`；上传成功不代表 Apple 处理完成或外测可安装。等待后台确认构建状态并关联既有测试组。

后台已显示 1.0(6) 创建于 11 月 2 日 17:26，状态“正在处理”；后续页面刷新未加载，尚未加入 Public Beta，尚不可断言可安装。1.0(5) 的后台创建时间为 10 月 30 日 11:07。
