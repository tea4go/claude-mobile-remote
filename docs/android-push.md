# Android 极光与厂商离线通知

> 2026-10-29归档状态：系统通知推送开发已按用户要求暂停，本文件记录此前实现及旧开发包配置，不代表新发布包已配置或厂商送达已通过。下一步Android发布目标为`Whale Remote Control` / `com.tea4go.claudemobileremote.android`，当前源码仍使用下文旧包名；新包签名、配置迁移和发布时推送隔离尚未实施。此次仅归档既有源码与配置模板。

应用通过极光统一接口收取固定文字提醒；PC→edo沿用HTTPS，手机以本机RID登记匿名topic。点击只从本地未过期配对映射进入详情并重连。启用前不主动初始化SDK；前台抑制系统提醒，无WebSocket本地重复通知。

极光统一了SDK适配与投递路由，但不能代替开发者开通各家厂商的应用/推送权限。当前已集成JPush 6.3.0及同版本小米、华为、荣耀、OPPO、vivo插件；Redmi使用小米通道。HarmonyOS NEXT与Android华为通道是不同应用配置，不可互用。

## 配置

1. 在各厂商后台为Android包`com.tea4go.claudemobileremote`开通推送。华为、荣耀以及厂商要求的其他场景需登记实际安装包的签名证书指纹；Debug与Release证书不同。
2. 将`apps/mobile/android/app/push.local.properties.example`复制为同目录`push.local.properties`，只填客户端参数。OPPO前缀`OP-`自动补齐。文件已忽略Git。
3. 华为使用该Android应用的`agconnect-services.json`放在同目录，Gradle验证包名并应用AGC插件。文件已忽略Git，不复制Harmony配置。
4. 在极光同一应用的Android集成设置填写服务端凭据并启用对应厂商。小米/Redmi：AppID、AppKey、AppSecret；华为：Android AppID、AppSecret或对应服务账号；荣耀：AppID、AppSecret；OPPO：客户端参数及服务端MasterSecret/AppServerSecret；vivo：AppID、AppKey、AppSecret。入口Activity为`com.tea4go.claudemobileremote.MainActivity`，默认标题为`Claude Mobile Remote`。具体字段以平台当前表单为准。
5. 服务端Secret仅留厂商/极光后台，极光Master Secret仅留edo，禁止放入App、仓库或下载目录。OPPO平台称AppSecret的客户端参数为SDK必需，与服务端MasterSecret/AppServerSecret不同。

缺参数仍可编译并使用普通极光通道，设置显示当前品牌离线通道尚未配置；本机RID就绪只代表登记成功，不代表厂商送达。后台的厂商通道当前核对为0/8，不能据此宣称各品牌退出送达已完成。厂商实名认证、上架/测试资格、服务类分类、渠道/配额仍由对应平台决定；不冒用消息分类或开通付费服务。

## 验收

用户自行构建安装，设置启用极光通知并重新扫码或等待有效配对同步。每家至少一台真机测试：前台无系统提醒；后台权限/提问/完成文字提醒；划走App后厂商送达；冷启动点击直接进入对应详情且重连；删除配对后旧点击不恢复；关闭后不再投递（离线撤销与已在途消息不能即时保证）。系统“强行停止”或厂商禁用自启动等策略不承诺送达，不使用常驻前台服务规避系统限制。

通知点击只携带不可猜测topic，不含PairToken、Session ID或会话内容；edo与App不打印厂商载荷/RID。登记请求禁止重定向，10秒超时，失败有界重试并于前台/重新同步时重试。

官方依据：[厂商SDK集成](https://docs.jiguang.cn/jpush/client/Android/android_3rd_guide)、[厂商参数申请](https://docs.jiguang.cn/jpush/client/Android/android_3rd_param)、[创建推送API](https://docs.jiguang.cn/jpush/server/push/rest_api_v3_push)。
