# TestFlight 1.0(3) 发布记录

用户确认离线演示及iOS安全区域真机通过，授权继续外部测试审核。

## 构建和上传

- 运行5_ios_testflight.sh build，build-number 3，全量Web门禁、Release归档、app-store-connect导出与脚本签名/资源验证通过。
- 产物：apps/mobile/ios/DerivedData/testflight-20261027-b3/export/App.ipa，版本1.0(3)，Bundle com.tea4go.claudemobileremote，Team 2WTXVQNV99。
- SHA256：6a5902d87d4d5b6efb2bdcc75ab2be3af23c27b07b02e98642df1ef63edcb744。
- 上传命令2026-10-27 10:37:00成功，Apple返回Uploaded package is processing / Upload succeeded。日志/tmp/cmr-testflight-b3-build.log、/tmp/cmr-testflight-b3-upload.log。
- 上传前后台仅1.0(2)，状态缺少出口合规证明、无群组。尚未核实1.0(3)处理完成。

## 待完成

## 后台续办结果：已提交Beta审核

- 用户解锁前台Chrome后UI恢复；确认build 3已处理完成。
- Beta描述及英文审核备注已保存，包含无需账户/网络的演示入口、完整操作步骤、模拟限制和真实LAN/VPN依赖；后台显示已保存，联系人保留原已确认资料。
- 建立Internal内部组（无测试员、关闭自动分发）作为外测流程前置；建立Public Beta外部组。
- 按代码核对系统WSS/Keychain/CryptoKit用途及Apple系统加密指导，build 3出口合规选择“不属于上述的任意一种算法”，保存后状态变为准备提交。未提供虚构证明文件，旧build 2未使用、未修改。
- 将1.0(3)加入Public Beta，无需登录，填写中英文测试重点，按用户授权点击“提交以供审核”。UI确认Public Beta有1个构建，1.0(3)状态“正在等待审核”。
- build 3 ID：4d2435a5-22ef-4a53-9005-da9dc360a5de；Public Beta组ID：fbf4e68d-d2c6-44a3-98cf-b4ae8a7015e6。
- 尚未审核通过，未启用公开邀请或生成真实邀请二维码。下一步等待Apple审核结果，若要求额外真实验证材料再按反馈补齐；通过后启用邀请并生成二维码。

以下为此前中间状态，已由上述续办结果替代：

当前浏览器UI读取空白，截图出现ScreenCaptureKit -3811错误；已请求用户解锁Mac并前台打开Chrome后台。未把本地上传成功当作Beta审核提交。

待后台恢复：核对新构建处理结果及出口合规问卷，更新测试/审核资料为离线演示实际步骤，建立测试组，加入build 3并提交Beta审核；审核通过后启用公开邀请链接、生成二维码。无Git写操作，无真实配对或私人录屏上传。
