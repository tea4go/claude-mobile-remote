# Android 同步实施计划

1. 已验收 iOS 与公共修改提交 5e4a0a7，不纳入真实截图。
2. 核对原生差距；实现 Android SessionExperience 的语音、通知、点击与无数据库缓存；复用 UI 与生命周期。
3. 原生缓存单元测试、共用 Web 全套测试/build/lint、cap sync android、Gradle test/assembleDebug 和可用模拟器检查。
4. 更新用户验收记录、HANDOFF 和使用说明；独立提交 Android。WhaleTerm 不修改、不暂存、不提交。

## 2026-10-30 华为Android测试发布

1. 核对后台新应用、既有密钥与D106，登记当前执行。
2. 同步Android名称/包名及平台脚本，默认隔离推送依赖和入口，保留原始实现。
3. 准备仓库外首次发布密钥和备份；构建内置资源Release，zipalign/apksigner签名并核对证书、包名、版本、非debug、推送缺席。
4. 执行相关Web测试/build/lint、Android单测/Release检查；记录验证证据。
5. Chrome完成方式二签名配置、测试版本资料及APK上传，核实平台状态；遇必要本人验证据实交接。不自动提交Git。
