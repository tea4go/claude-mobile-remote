# Android 华为 / 新极光应用配置记录

日期：2026-10-30。范围依据 D107：恢复 Android 配置，旧 iOS 应用保持不变，Harmony 推送暂停；不发布 APK、不部署 edo、不执行 Git 写操作。

## 已完成

- Android：Whale Remote Control，包名 `com.tea4go.claudemobileremote.android`；华为 APP ID `119552119`，极光应用 ClaudeRemoteControlNew。
- 用户明确授权将该华为应用 Client Secret 传至新极光应用并启用华为通道。后台保存并单独开启后，已复核华为“已启用”、开关 on、厂商通道1/8；默认标题 Whale Remote Control，入口为 `com.tea4go.claudemobileremote.MainActivity`。未改旧 iOS 应用，未开通付费能力。
- 华为基本信息 → API 管理：推送服务（Push Kit）开关 on，已独立核实；无需额外切换。
- 用户提供的 AGC JSON 已复制至忽略的 `apps/mobile/android/app/agconnect-services.json`，内容一致；包名、APP ID 正确。本机 `push.local.properties` 使用新 Android AppKey。两文件权限0600，不记录 Secret 或完整配置。
- Gradle 显式推送构建从本机 properties 读取并校验24位十六进制 AppKey，不再硬编码旧 iOS AppKey；AGC 文件校验包名及非空 APP ID。普通构建仍默认关闭推送。

## 验证

在 `apps/mobile/android` 执行：

```sh
JAVA_HOME="$HOME/jdk" ./gradlew --offline -PcmrPushEnabled=true testDebugUnitTest compileReleaseJavaWithJavac --console=plain
JAVA_HOME="$HOME/jdk" ./gradlew --offline :app:processReleaseManifest --console=plain
```

- 启用推送：BUILD SUCCESSFUL，9项原生测试、0失败/错误，Release Java 编译通过。最终 `merged_manifests/release/processReleaseManifest/AndroidManifest.xml` 包名正确、AppKey与本机新应用一致、存在通知权限、华为 metadata 为 `appid=119552119`。须检查最终复数目录的清单，AGC 注入前的单数目录不能代表最终结果。
- 默认关闭推送：Manifest 任务通过，包名正确，无 POST_NOTIFICATIONS，无 JPush/HMS/AGC 组件。该命令会覆盖上述最终清单，启用态结论来自覆盖前独立核对。
- 初次构建因沙箱不允许访问 Gradle 缓存而失败；获准读取缓存后成功。既有 SDK XML / Gradle 弃用警告仍存在。
- 已核对本机发布证书，SHA-256 与已发布 APK / AGC 托管签名一致。未更换密钥、未新增 Debug 证书。

## 待完成及下一步

1. 发布签名指纹：`7F:E5:7B:05:B3:FF:6E:80:CB:D3:49:7D:FB:6A:B7:E8:2E:53:3F:76:D9:97:4B:4B:25:01:0E:F4:27:B0:44:B7`。后台仍显示“添加证书指纹”，尚未登记。用户已对该指纹及 APP ID 的登记明确授权；自动审批最初因目标不清及缺具体授权拦截，获授权后原生坐标点击仍报 `noWindowsAvailable`，入口不支持键盘焦点，已请用户手动打开表单后继续。不将授权等同保存成功。
2. edo 当前 `push/jpush.go` 只加载一套凭据，`push/devices.go` 保存 topic→RID→有效期，无极光应用维度。新 Android RID 不属于旧 iOS 应用，不能直接混发或覆盖旧凭据；须先设计/实现兼容隔离。Android intent 的旧包名及组件短写也需随新应用发送路径修正。
3. 新 Android Master Secret 尚未配置到 edo；未部署、未请求真实推送、未做真机后台/划走/冷启动点击/撤销验收。华为服务类权益未申请，不推定已具备分类权限。
4. 已上架1.0.0 APK默认不含推送；本轮未生成、签名或上传新 APK。后续需使用既有发布证书构建推送版本并安装验收。
