# iOS TestFlight外部测试发布

## 权限和首次准备

- 使用有效Apple Developer Program组织会员，TestFlight无额外套餐费用。
- 上传构建可用Developer；创建App和管理外测需Account Holder/Admin/App Manager。当前已登录账号经后台核实为Developer，需管理员协助。
- App记录应对应`com.tea4go.claudemobileremote`，Team为`2WTXVQNV99`。若当前账号只看到其他App，不据此认定组织内不存在目标App；管理员须先核实记录和访问范围。
- Xcode 26+登录同组织，安装iOS平台；准备App Store分发证书/Profile。不得把密钥/签名文件提交仓库。

## 本地构建（不会上传）

```bash
bash scripts/macos/5_ios_testflight.sh build \
  --build-number 2 \
  --output apps/mobile/ios/DerivedData/testflight-b2
```

构建号必须在后台核实后递增（脚本支持1–9999），输出目录必须全新。可用`--packages /绝对路径/SourcePackages`复用已解析依赖。脚本执行Web测试/lint/build、cap copy ios、Release归档、app-store-connect本地导出，检查Bundle ID、分发Profile、签名、无开发server.url和Web资源。日志仅保存在本地，不提交完整构建日志。原`4_ios_build.sh`仍是模拟器Debug构建。

## 上传与审核

```bash
bash scripts/macos/5_ios_testflight.sh upload \
  --output apps/mobile/ios/DerivedData/testflight-b2
```

使用Xcode现有登录，不在脚本里保存密码。上传只对已通过验证的输出开放；命令结束后仍须在App Store Connect确认处理完成，不能据此认定发布完成。失败重试前先检查后台是否已收到同一build。

在TestFlight填写真实测试资料、审核联系人与加密合规信息，创建外测组，选择构建并提交Beta审核。应用需要连接PC，审核说明须提供可实际验证的方案；不得上传个人配对凭证代替演示。此流程不会发布App Store正式版本。构建可测试90天。

审核通过后启用该组公开邀请链接（需确认人数上限和公开范围）。Apple托管邀请页面，不需要服务器存放IPA或二维码。

## 生成二维码

从后台复制实际公开链接后执行：

```bash
bash scripts/macos/5_ios_testflight.sh qr \
  --url 'https://testflight.apple.com/join/实际邀请码' \
  --output /tmp/claude-mobile-remote-testflight.png
```

示例中的中文占位符必须换成Apple实际返回的邀请码。工具仅接受无查询参数的TestFlight公开邀请URL，使用macOS CoreImage生成PNG，并由Vision解码比对。不要用于会话配对二维码。图片可直接分享；如需在线图片地址，再部署到指定服务器。后续将新构建加到同组时通常可继续使用同一邀请链接。

验证二维码：扫码打开Apple邀请页 → 安装TestFlight → 加入测试 → 安装App → 配对PC。图片生成成功不代表邀请已启用或Apple已审核通过。

官方依据：[上传构建](https://developer.apple.com/help/app-store-connect/manage-builds/upload-builds)、[外部测试与公开链接](https://developer.apple.com/help/app-store-connect/test-a-beta-version/invite-external-testers)、[TestFlight流程](https://developer.apple.com/help/app-store-connect/test-a-beta-version/testflight-overview/)。
