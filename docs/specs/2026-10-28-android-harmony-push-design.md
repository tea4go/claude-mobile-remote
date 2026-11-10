# Android / Harmony 极光基础通知

> 2026-10-30状态：D107恢复 Android 华为推送配置，使用已发布 Android 包名及新极光应用；旧极光应用继续服务 iOS，Harmony 推送保持暂停。后台配置和构建不替代厂商真机送达验收。

依据用户确认D103，复用D100/D102链路与现有iOS通知HTTP契约。PC发送组件不变；edo极光载荷扩展Android/Harmony平台及各自点击字段，不记录正文/PairToken/RID。固定文字、300秒TTL、Registration ID目标不变。

Android原生极光SDK经用户显式启用后初始化，保存通知偏好与不含PairToken的topic→Session/到期映射，利用现有HTTPS PUT/DELETE登记设备RID。通知点击先校验本地有效映射，再交既有notificationOpened入口，React运行时负责重连。前台抑制展示，不再叠加WebSocket本地提醒。厂商通道需对应厂商凭据，不能以普通长连接承诺进程结束后送达。

HarmonyOS NEXT独立接入极光鸿蒙SDK及华为Push Kit所需配置；同样登记RID并本地映射。EntryAbility接收冷/热启动通知，再等待安全配对恢复后进入详情；前台抑制，关闭与移除尽力撤销，过期拒绝点击。无通知SDK初始化前的隐式数据采集，不在App保存Master Secret。

设备标识登记失败应显示未就绪并前台重试；禁止重定向、超时有界。以单测/类型或平台编译检查验证代码，真实送达、杀进程、冷启动与撤销由真机验收，平台强制停止不作送达保证。保留iOS行为且不改三端wire。

参考：https://docs.jiguang.cn/jpush/client/Android/android_guide 和 https://docs.jiguang.cn/jpush/client/HarmonyOS/hmos_guide 。

## 当前范围修订

用户因Harmony签名变更要求暂停该端，保留未完成改动待后续复核；本轮edo只启用ios/android，不启用hmos。Android接入小米（含Redmi）、华为、荣耀、OPPO、vivo配套6.3.0插件；客户端参数从忽略的push.local.properties注入，华为AGC文件校验Android包名，后台Secret只填对应控制台。厂商参数缺失时不宣称厂商离线送达，设置显示当前机型通道未配置。详见../../docs/android-push.md。

通知清理只消除已展示提醒，不撤销有效配对；配对完整集合同步才执行登记/解绑，避免用户点击通知后失去后续提醒。PUT请求超时也保留待确认登记，关闭/移除时仍发送DELETE，防止服务端已成功而客户端丢失登记记录。

## Android 新发布身份配置（2026-10-30）

Android 使用 `com.tea4go.claudemobileremote.android`、华为 APP ID `119552119` 和新极光应用 ClaudeRemoteControlNew；Java namespace/入口仍为 `com.tea4go.claudemobileremote.MainActivity`。AppKey 从忽略入库的本机 properties 注入，不复用旧 iOS AppKey；推送仍为显式构建开关及用户端内同意，未配置时拒绝生成推送包，普通发布包保留无 SDK 的构建路径。

AGC 文件必须校验包名和 APP ID，华为登记证书应与实际安装签名一致。服务端 Secret 不进入客户端。新极光 RID 不能直接发送给旧 AppKey，edo 需支持新旧应用目标隔离后才能联调；在此之前不得宣称端到端就绪，不能通过覆盖旧凭据迁移现有 iOS。后台华为通道填写已获用户明确授权，服务类权益、消息分类及真实后台/退出送达按平台和真机结果另行记录。

## 2026-11-10 华为单厂商续做

本轮仅验收华为 Android。登记接口增加可选平台标识：缺省解释为原 iOS，Android 明确标识；服务端持久化隔离并按各自极光应用凭据投递，旧登记文件兼容。Android 点击使用当前包名和完整 Activity。PC 事件、三端 WebSocket、配对及状态不变。分别投递成功的目标在同一事件重试时不重复发送；服务重启后沿用既有去重边界。

## 2026-11-10 HarmonyOS 恢复

用户要求恢复 Harmony 推送，设备登记携带 `hmos`，edo 隔离平台与凭据并保留 iOS/Android 兼容；客户端使用异步私有存储保存通知路由，恢复配对后仅允许定位本地有效会话。华为 Push Kit 接收及极光 Harmony SDK 配置以官方当前要求和实际编译验证为准；服务类消息权益与真机送达分别核实。
