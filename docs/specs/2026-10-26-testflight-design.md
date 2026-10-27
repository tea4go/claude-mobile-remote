# iOS TestFlight外部测试设计

状态：2026-10-26用户选择“按设计继续”，设计已确认。

## 目标与范围

按D96建立可重复的iOS Release归档、App Store Connect分发、TestFlight外部测试流程，交付公开邀请链接及PNG二维码。保留现有应用内调试与远程检查设置。不增加Relay、离线推送或其他平台功能，不改共用契约，不自动提交Git。

## 流程

1. 用户登录Apple Developer与App Store Connect；核实会员为有效Apple Developer Program、当前组织/Team与Bundle ID对应、无阻塞协议。上传要求Account Holder/Admin/App Manager/Developer；外测管理要求Account Holder/Admin/App Manager。待接受法律协议由用户操作。
2. 查找或创建com.tea4go.claudemobileremote应用记录，避免重复建App。确认展示名、主语言、SKU、权限，以及后台现存版本/构建号。
3. 独立发布脚本，显式区分build/export/upload，默认只本地构建。每次要求指定递增build号，Release、generic iOS、app-store-connect导出，非testFlightInternalTestingOnly。前端先build，仅cap copy ios，避免无必要重写SPM；独立DerivedData与archive/export目录，拒绝覆盖既有产物。拒绝CAP_SERVER_URL入包；凭证不进脚本、日志或仓库。
4. 本机验证实际导出包Bundle ID/版本/build、分发签名、无开发server.url、内置Web资源与dist一致。现有debugging导出的IPA不作为外测上传包。沿用Xcode登录上传；如后续自动化使用API密钥，仅由仓库外文件或系统安全存储读取，不索取明文密码。
5. 上传Apple并等待处理。录入Beta描述/测试重点/反馈邮箱、审核联系人及准确的加密合规信息，不凭空填报。应用需连接PC局域网/VPN才能使用，审核备注需可操作的验证方式；不能上传个人真实会话凭证供审核。若需要演示环境，单独确认受限测试会话方案。
6. 创建外测组、关联构建、提交首次Beta审核。待通过后启用公开邀请链接；启用时确认实际公开范围/人数上限。后续更新向同组添加获准构建，可复用该组链接；构建有效期90天，链接本身不保证始终有可用构建。
7. 使用Apple实际返回的https://testflight.apple.com/join/...链接，本地生成二维码PNG及简短安装说明，解码核对原链接并打开验证。用户扫码后安装TestFlight并加入测试，无需登记UDID。二维码内不含会话配对信息。

## 二维码与托管

Apple提供公开邀请网页，PNG可直接分享，不需要服务器。若用户需要稳定在线二维码图片或自定义安装页，再上传至其指定服务器；不假定有现成服务器权限，不把IPA放到页面绕过TestFlight。

## 验收和边界

独立记录本地构建、Apple接收/处理、Beta审核、公开链接、实际扫码安装，不能互相替代。本阶段不承诺Apple审核时长或结果；账号登录/角色/协议/审核是外部依赖。外测发布不是App Store正式上架。签名材料/私钥/API密钥/产物不入Git。

## 官方依据（2026-10-26核查）

- https://developer.apple.com/help/app-store-connect/manage-builds/upload-builds
- https://developer.apple.com/help/app-store-connect/test-a-beta-version/invite-external-testers
- https://developer.apple.com/help/app-store-connect/test-a-beta-version/testflight-overview/
