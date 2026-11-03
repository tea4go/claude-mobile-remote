# iOS 1.0(4) TestFlight签名测试包

用户要求生成签名IPA供其自行上传，包含已验证的iOS极光系统通知。TestFlight采用Release/App Store分发签名。

## 完成与验证

- 使用5_ios_testflight.sh build --build-number 4，复用既有SourcePackages缓存；未执行upload。
- 37文件549项Web测试、lint、TypeScript/Vite构建、Capacitor资源复制通过。Release Archive及app-store-connect导出通过。
- codesign --verify --deep --strict通过；IPA资源与最新dist逐字节一致，无开发server.url。
- 最终IPA版本1.0(4)、Bundle com.tea4go.claudemobileremote、Team 2WTXVQNV99；App Store Profile无设备名单/企业分发，get-task-allow关闭。
- Info.plist中的极光SDK环境、分发Profile与签名aps-environment均production。AppKey存在且格式合法，JPush/JCore隐私资源存在，ZIP完整性通过。未输出凭据或Profile全文。

产物：`apps/mobile/ios/DerivedData/testflight-20261028-b4/export/App.ipa`，2903314字节。SHA256：`dad6b4551e595a92a1fb3c2a1cc0e3f39153969fa08e59a0bb1e9c109afdf233`。归档保存在同级`App.xcarchive`。构建日志`/tmp/cmr-testflight-b4-build.log`。

## edo环境

原CMR_JPUSH_PRODUCTION=false；原配置以root私有权限备份至`/etc/cmr-notify/backups/20261028T034840Z-production`。仅将该开关改为true，保留原密钥与Android投递代码，重启服务。systemd active、healthz204；从运行进程环境单独核对该开关为true，未输出其他环境变量。未改Nginx或端口。服务现与TestFlight production环境一致，Xcode开发签名包的APNs环境与其不同。

## 待验收

2026-10-28续办：用户随后明确授权上传发布TestFlight。使用原1.0(4)归档执行upload，12:30:23 Apple返回Uploaded package is processing / Upload succeeded，命令成功。日志`/tmp/cmr-testflight-b4-upload.log`。继续在App Store Connect核实处理、合规及Public Beta关联；此时尚未将上传成功视为外测可安装。

## 上传后的后台核验及阻塞

### 续办最终结果（2026-10-28）

用户已开通App管理权限，原阻塞解除。保存出口合规问卷“不属于上述的任意一种算法”，build4转为准备提交；将1.0(4)加入既有Public Beta并填写中文测试说明（后台/退出文字推送、点击对应会话及重连、前台抑制、删除配对、LAN/VPN条件和离线演示）。保留自动通知测试员勾选，点击“提交以供审核”。提交后群组页面明确显示两个构建：1.0(4)“正在测试”、90天后过期；1.0(3)仍正在测试。外测发布已完成，非待审核状态。现有公开链接https://testflight.apple.com/join/uCHWhPEK及50人上限保留。未重传、未重新构建、未执行Git写操作。最终生产推送与点击验收由用户安装TestFlight版本完成；下文阻塞记录仅保留过程。

- Apple处理完成，build4 ID：`9fe23515-d48d-4734-aa0b-abf94e14d5bf`，版本1.0(4)，当前状态“缺少出口合规证明”，群组0。
- 当前Chrome账号个人设置明确角色为“开发者”。Beta测试信息不可编辑，Public Beta构建页没有添加控件；未擅自改变账号权限。
- Apple官方要求账户持有人、管理员或App管理者处理[外测发布](https://developer.apple.com/help/app-store-connect/test-a-beta-version/invite-external-testers/)及[出口合规](https://developer.apple.com/help/app-store-connect/manage-app-information/determine-and-upload-app-encryption-documentation/)。已请求用户切换对应角色账号；无需重传build4。
- Public Beta仍关联旧build3，既有公开链接及50人上限未修改。build4尚未提交Beta审核或开放安装。
- 准备的测试重点：极光后台/退出文字提醒、通知点击对应会话及自动重连、前台抑制、删除配对后旧点击不恢复；通知需用户主动启用，点击需手机与PC在LAN/VPN可互通。离线演示入口沿用既有审核说明。
- 为填写新SDK相关问卷，核对了App使用系统CryptoKit/Keychain/URLSession；本机JCore静态库引用系统CommonCrypto与Security，未发现AES/RSA/EVP/SSL自定义加密导出符号。该检查不替代平台问卷或厂商对未公开内部实现的说明。本轮尚未填写问卷。

用户通过Transporter或Xcode上传IPA，待Apple处理后将build4用于TestFlight测试；本轮未上传、未提交审核、未发布。真机验后台/退出通知、点击具体会话及重连、前台抑制、删除配对后的旧点击。签名及环境检查不代表APNs生产送达或Beta审核通过。无Git写操作。Android厂商key由用户自行配置；未变更同期Harmony签名工作。
