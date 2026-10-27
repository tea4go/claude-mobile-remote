# E3 iOS 极光 HTTPS 离线通知

用户确认完整范围；依据 D100 恢复 D83 暂停的 iOS 接收端。本轮交付代码与部署包；不自动部署、发布 TestFlight 或操作 Git。

## 链路

WhaleTerm 已有 remote.Config.EnablePush=true。公共组件独立订阅宿主事件，经有界异步 worker 向 edo POST；Standalone 复用。edo 持有极光 AppKey/Master Secret 并调用 v3 API，宿主只持有专用随机 bearer token（至少32字节）和 HTTPS URL。无 Session 中心、正文或命令存储。点击通知仍使用 LAN/VPN。

## 独立 HTTP 契约

POST /v1/events；公网代理可映射 /verfiles/notify/v1/events。Authorization bearer，JSON 上限2KiB，拒绝未知字段/额外JSON。仅 eventId（随机128位十六进制）、topic、kind、expiresAt；5分钟短TTL且不得晚于配对到期。kind 固定 approval/question/completed/failed/stopped，文案由服务端固定，不发送 Session ID/Pair Token/工具参数/正文。

沿用 D82 topic：cmr_ + SHA256("cmr-push-v1" + NUL + sessionId + NUL + pairToken)前16字节的小写十六进制。D102 修订：极光 tag 推送在当前账号被 2009 拒绝，手机改向同一 edo HTTPS 精确路径以 PUT 登记 topic、极光 Registration ID 与配对到期时间；DELETE 尽力撤销。edo 私有持久化 topic→RID/到期，多设备、多 Session 隔离；有限条目/每 topic 上限、短请求体和速率约束，过期清理。topic 是不可猜测的通知登记能力，不是 Remote 登录凭证，也不能向 edo 发送通知事件；PC 的 POST 仍需专用 bearer。极光按 RID 投递；APNs extras 只有 topic/kind/eventId，手机本地映射回 Session。

edo 对认证请求全局限速和并发限制；内存最多4096条幂等记录，保存到事件到期，容量满拒绝新请求而不逐出有效记录。相同ID不同内容拒绝，并发重复只允许一次平台调用，平台失败允许重试。宿主最多两次，仅网络错误/429/5xx重试，保持同一 eventId；不持久化积压。服务重启清空去重记录，网络结果未知时仍可能重复，不承诺exactly-once。APNs TTL使用剩余有效秒数。

鉴权失败不调用平台，错误正文固定，不记录 bearer、请求体、topic、RID或平台响应正文；上游失败仅记录 HTTP 状态及数字错误码。私有配置0600，不放下载目录。服务只监听loopback，由现有HTTPS代理暴露精确路径；GET /healthz仅代表本机进程存活。

## iOS

链接现有 noidfa JPush6.2.2/JCore5.5.1及隐私资源，恢复Push capability和aps-environment。AppKey由忽略入库的Push.local.xcconfig注入；Debug development、Release production，实际环境必须与profile一致。只做alert通知，不以静默推送维持WebSocket。

用户启用通知后初始化SDK（旧版只开启本地提醒的用户需显式点击“启用后台通知”，不因升级自动初始化极光）、注册APNs、以本机 RID 向 edo 登记有效配对 topic，界面说明使用极光；启动时恢复已启用偏好。关闭SDK日志。关闭通知时turnOffPush并尽力撤销登记；删除/过期时同步登记，失败显示固定状态，前台恢复重试。手机仅持久化 topic→Session/到期映射用于冷启动点击；不复制保存 Pair Token。移除后拒绝旧点击；离线无法即时撤销旧 RID，短TTL旧通知仍可能显示，这是撤销边界。宿主停止/凭证轮换后不发旧topic。

前台远程通知不弹横幅，仍由实时事件触发本地提醒，避免双链路重复；后台/退出显示APNs通知。冷启动缓存点击，Web配对恢复后导航。设置显示授权、SDK配置和设备就绪；设备注册成功不等价于端到端送达。

## 三端与验收

不改变WebSocket、Session/Turn、配对或共用命令。Android/Harmony保持本地提醒，后续独立实现同样hash和点击映射。通知状态为原生端内能力，不进入三端wire。

验证鉴权、超大/未知载荷、过期/topic、并发幂等、失败重试、平台隐私、HTTPS与禁止重定向、发送失败隔离；移动设置/旧端兼容、iOS编译、Linux双架构包。真实门禁需Apple Push capability/profile、极光APNs配置及真机后台/退出审批通知、点击、多Session/设备和禁用/移除；编译不可替代。

参考：[iOS集成](https://docs.jiguang.cn/jpush/client/iOS/ios_guide_new)、[Push v3](https://docs.jiguang.cn/jpush/server/push/rest_api_v3_push)。

## 通知点击修复（2026-10-27）

用户已验证系统通知送达。Capacitor 初始化会覆盖 AppDelegate 中提前安装的通知代理，因此在自定义插件 load（Bridge 初始化之后）重新安装 SessionNotificationRouter；Scene 冷启动 connectionOptions.notificationResponse 也交给同一个点击入口。入口继续只解析本地有效 topic 映射，事件通过 retainUntilConsumed 等待 Web 配对恢复后导航，不改变协议、配对或 edo。验证后台点击、冷启动点击及不存在的本地 Session 不串线；本轮不编译 App。
