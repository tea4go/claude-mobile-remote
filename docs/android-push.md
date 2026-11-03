# Android 极光与厂商离线通知

> 2026-10-30：D107 恢复 Android 华为推送配置。应用为 `Whale Remote Control` / `com.tea4go.claudemobileremote.android`，使用新极光应用 ClaudeRemoteControlNew；旧极光应用继续服务 iOS。华为 Push Kit 已开启，极光华为通道已启用，本机 SDK 配置与启用推送的编译检查通过。发布签名指纹登记仍待完成，edo 尚缺新旧极光应用隔离，真机送达未验证。Harmony 推送继续暂停。

应用通过极光统一接口收取固定文字提醒；PC→edo沿用HTTPS，手机以本机RID登记匿名topic。点击只从本地未过期配对映射进入详情并重连。启用前不主动初始化SDK；前台抑制系统提醒，无WebSocket本地重复通知。

极光统一了SDK适配与投递路由，但不能代替开发者开通各家厂商的应用/推送权限。当前已集成JPush 6.3.0及同版本小米、华为、荣耀、OPPO、vivo插件；Redmi使用小米通道。HarmonyOS NEXT与Android华为通道是不同应用配置，不可互用。

## 配置

1. 在各厂商后台为Android包`com.tea4go.claudemobileremote.android`开通推送。华为 Android APP ID 为 `119552119`。华为、荣耀以及厂商要求的其他场景需登记实际安装包的签名证书指纹；Debug与Release证书不同，不更换既有发布密钥。
2. 将`apps/mobile/android/app/push.local.properties.example`复制为同目录`push.local.properties`，填新 Android 应用的 `JPUSH_APPKEY` 及所需客户端参数。OPPO前缀`OP-`自动补齐。文件已忽略Git，本机权限为0600。
3. 华为使用该Android应用的`agconnect-services.json`放在同目录，Gradle验证包名及非空 APP ID 并应用AGC插件；本机已核对 APP ID 与上述一致。文件已忽略Git，不复制Harmony配置。
4. 在新极光应用的Android集成设置填写服务端凭据并启用对应厂商。小米/Redmi：AppID、AppKey、AppSecret；华为：Android AppID、AppSecret或对应服务账号；荣耀：AppID、AppSecret；OPPO：客户端参数及服务端MasterSecret/AppServerSecret；vivo：AppID、AppKey、AppSecret。入口Activity为`com.tea4go.claudemobileremote.MainActivity`（Java namespace 保留），默认标题为`Whale Remote Control`。具体字段以平台当前表单为准。
5. 服务端Secret仅留厂商/极光后台，极光Master Secret仅留edo，禁止放入App、仓库或下载目录。OPPO平台称AppSecret的客户端参数为SDK必需，与服务端MasterSecret/AppServerSecret不同。

默认构建不含推送 SDK、服务、通知权限或 edo 登记。启用推送时显式传 `-PcmrPushEnabled=true`，缺少有效的 Android `JPUSH_APPKEY` 会拒绝构建；各厂商参数缺失时不能宣称该通道可用。本机 RID 就绪也不等于送达。极光后台已核对华为启用、厂商通道1/8，其余厂商尚未配置。厂商实名认证、上架/测试资格、服务类分类、渠道/配额仍由对应平台决定；不冒用消息分类或开通付费服务。

本轮编译命令（在 `apps/mobile/android` 下，使用本机已有 JDK）：

```sh
JAVA_HOME="$HOME/jdk" ./gradlew --offline -PcmrPushEnabled=true testDebugUnitTest compileReleaseJavaWithJavac --console=plain
```

edo 当前仅加载一套极光凭据，设备映射尚无应用维度，发送端 Android intent 仍使用旧包名。后续必须先完成新旧应用隔离和新包点击配置，保留旧 iOS 凭据及兼容登记，再配置新 Android Master Secret、部署和联调。不可直接替换旧 AppKey，或把新 Android RID 与旧 iOS RID 混发。市场现有1.0.0 APK未包含推送，本轮后台配置不会自动使已安装包获得推送；后续需另行构建并安装启用推送的版本。

当前证据及缺口见 [配置验证记录](testing/2026-10-30-android-huawei-push-result.md)。

## 验收

用户自行构建安装，设置启用极光通知并重新扫码或等待有效配对同步。每家至少一台真机测试：前台无系统提醒；后台权限/提问/完成文字提醒；划走App后厂商送达；冷启动点击直接进入对应详情且重连；删除配对后旧点击不恢复；关闭后不再投递（离线撤销与已在途消息不能即时保证）。系统“强行停止”或厂商禁用自启动等策略不承诺送达，不使用常驻前台服务规避系统限制。

通知点击只携带不可猜测topic，不含PairToken、Session ID或会话内容；edo与App不打印厂商载荷/RID。登记请求禁止重定向，10秒超时，失败有界重试并于前台/重新同步时重试。

官方依据：[厂商SDK集成](https://docs.jiguang.cn/jpush/client/Android/android_3rd_guide)、[厂商参数申请](https://docs.jiguang.cn/jpush/client/Android/android_3rd_param)、[创建推送API](https://docs.jiguang.cn/jpush/server/push/rest_api_v3_push)。
