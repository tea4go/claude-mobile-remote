# Android华为签名、草稿及发布进展（2026-10-30）

用户最初授权本地修改与华为测试发布，随后选择“先完成签名和草稿，暂不发布”；最新反馈已完成APK发布配置、正在等待审核，并明确授权本地提交。本记录保留AI实际操作的签名/草稿证据，并区分用户后续反馈。推送开发继续暂停。

## 本地结果

- 名称Whale Remote Control；applicationId `com.tea4go.claudemobileremote.android`；首次版本1.0.0（1），minSDK26/targetSDK36。Java namespace保持原值，Manifest组件使用完整类名，Provider使用新applicationId。
- Android界面/桌面及同步配置使用新身份，sync:native分开同步iOS/Android，iOS已生成配置身份仍为原值，Harmony不修改。Windows启动入口同步完整类名，未在Windows执行。
- 默认编译src/noPush，既有实现保留于src/push；Release显式cmrPushEnabled=false。APK无极光/厂商推送SDK、通知服务/接收器、POST_NOTIFICATIONS或edo登记。设置显示暂未开放系统通知。available仅为原生桥接可选状态，不改跨端wire。
- 独立RSA3072/JKS发布密钥有效10000天，私有文件0600、本机副本一致性检查通过。另机/离线介质备份未完成，同磁盘副本不等于灾备。
- Release脚本通过：550项Web测试/37文件，lint仅既有CDP警告，TS/Vite通过（既有大chunk提示）。assembleRelease成功；离线testDebugUnitTest9项零失败。日志`/tmp/cmr-android-release-build.log`、`/tmp/cmr-android-release-native-tests.log`。
- apksigner v2/v3验签、zipalign -P16通过，最低API26无需v1。ZIP完整，3个DEX无极光/华为推送/OPPO推送类，合并Manifest无推送权限/服务。包内身份正确，server.url缺席、WebView调试false、日志none，内置正式资源，无默认模拟数据入口。
- 上传APK`.local-tools/android-release/WhaleRemoteControl-1.0.0-android-signed.apk`，32335779字节，SHA256 `7d443557b7358d641b5071678db3b91217eb3caf9d9d43e510a90af937396d65`。
- Python脚本编译、bash语法及git diff --check通过；上述构建验证时未执行Git写操作，后续归档提交见下文。

## AGC结果

APP ID `119552119`，包名正确。APK已接收，显示1.0.0（1）/30.84MB，上传时间2026-10-30 10:11:19。

按D106完成方式二。PEPK来自Google官方`https://www.gstatic.com/play-apps-publisher-rapid/signing-tool/prod/pepk.jar`，ZIP完整，SHA256 `aaccc0774b240aa5304bdad2a49865e92f229ca73209ecb6eaafe75dc858e24e`。首次下载超时后续传成功；普通stdin因java.io.Console为空失败，改用禁回显PTY后成功，密码未进入命令参数或日志。

PEPK按AGC页面固定公钥加密，输出ZIP3942字节，仅含encryptedPrivateKey与certificate.pem，证书DER与本地一致。仅上传加密ZIP，未上传原始JKS/密码，未单设上传密钥。后台明确显示“此应用已加入应用签名计划”；SHA256为`7F:E5:7B:05:B3:FF:6E:80:CB:D3:49:7D:FB:6A:B7:E8:2E:53:3F:76:D9:97:4B:4B:25:01:0E:F4:27:B0:44:B7`，与APK一致。

草稿保存提示“保存成功”，版本`v2072443135153186113`。重新进入后状态准备提交、保存按钮禁用、APK与反馈邮箱保留。中国大陆、开放式测试、2026-10-30 10:13:37至2026-11-29 00:00:00、邀请有效30天；权限说明补充主动语音、扫码/拍照用途。自动审批曾拒绝未经具体授权填写私人反馈邮箱；告知用户并获得具体授权后填写保存，无绕过。私人联系方式不入文档。

Android后台内部测试最多100人，取消免审核后最多5000人。创建名单只有手动账号/文件导入，未见公开加入链接。不能把HarmonyOS公开测试文档当成Android相同入口。用户选择只保留签名和草稿；最终草稿未勾内部免审核，名单为空，未提交审核、发布或发送邀请。

## 用户后续反馈与归档

2026-10-30用户确认“发布到华为平台apk配置完毕，正在等待审核”，替代此前仅保存草稿、暂不发布的当前状态。本轮未重新打开后台，审核状态、提交时间、最终分发类型及最终资料配置均未由AI独立核实；此前开放式测试名单为空等内容仅记录当时草稿，不能作为最终配置结论。未记录审核通过或下载可用。

用户授权本地提交Android相关源码、脚本与必要文档，不推送。归档不新增功能、不重建APK；已复核既有550项Web测试/Release日志、9项Android JUnit报告、APK摘要、脚本语法、推送源码迁移一致性与Git差异；25个暂存文件敏感内容/文件与空白检查通过，本轮未重跑功能测试。密钥、密码、签名导出、APK及无关iOS文档/测试图片不纳入提交。

## 未验证与后续

等待华为审核结果；若退回，按实际驳回项修复。真实Android设备扫码/语音/附件/VPN、云测、审核通过和可下载链接仍未验证。此前应用信息、隐私政策、个人信息/AI声明、权限截图及单机APP分类已列为待复核项，用户后续后台最终修改未由AI核实。推送继续暂停，独立介质密钥备份待用户完成。
