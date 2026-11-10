# iOS TestFlight 1.0(9) 发布记录

- 2026-11-09：用户要求编译并上传 TestFlight；后台确认 1.0(8) 在 Public Beta 测试，本次使用构建号 9。
- Web 生产构建、iOS 资源复制、Xcode Release 归档和 App Store 导出通过；按用户要求未运行 npm lint/test。归档有 AppIntents 元数据跳过提示。
- 已验证 IPA 的 App Store 分发描述文件、严格签名、Bundle ID、版本和内置 Web 资源；IPA 位于 `apps/mobile/ios/DerivedData/testflight-20261109-b9/export/App.ipa`，SHA-256 `e1ace73712b3c33d3e86092a6bfdec37a2c1402e51254611a1cc494d9ee47947`。
- 16:37 Apple 返回 Upload succeeded 和 Uploaded package is processing。后台刷新后显示空白，已请用户恢复页面；处理完成、Public Beta 关联及外测审核状态尚未核实，不代表测试员已可安装。
- 未执行 Git 写操作；下一步恢复后台后核实构建并关联既有 Public Beta 群组。
