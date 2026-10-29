# Android / Harmony 极光基础通知

> 2026-10-29状态：D106已暂停系统通知推送开发，优先Android APK上架准备；本设计保留此前实现依据。归档提交不启用Harmony推送、不代表Android新包配置完成，也不替代厂商真机送达验收。

依据用户确认D103，复用D100/D102链路与现有iOS通知HTTP契约。PC发送组件不变；edo极光载荷扩展Android/Harmony平台及各自点击字段，不记录正文/PairToken/RID。固定文字、300秒TTL、Registration ID目标不变。

Android原生极光SDK经用户显式启用后初始化，保存通知偏好与不含PairToken的topic→Session/到期映射，利用现有HTTPS PUT/DELETE登记设备RID。通知点击先校验本地有效映射，再交既有notificationOpened入口，React运行时负责重连。前台抑制展示，不再叠加WebSocket本地提醒。厂商通道需对应厂商凭据，不能以普通长连接承诺进程结束后送达。

HarmonyOS NEXT独立接入极光鸿蒙SDK及华为Push Kit所需配置；同样登记RID并本地映射。EntryAbility接收冷/热启动通知，再等待安全配对恢复后进入详情；前台抑制，关闭与移除尽力撤销，过期拒绝点击。无通知SDK初始化前的隐式数据采集，不在App保存Master Secret。

设备标识登记失败应显示未就绪并前台重试；禁止重定向、超时有界。以单测/类型或平台编译检查验证代码，真实送达、杀进程、冷启动与撤销由真机验收，平台强制停止不作送达保证。保留iOS行为且不改三端wire。

参考：https://docs.jiguang.cn/jpush/client/Android/android_guide 和 https://docs.jiguang.cn/jpush/client/HarmonyOS/hmos_guide 。

## 当前范围修订

用户因Harmony签名变更要求暂停该端，保留未完成改动待后续复核；本轮edo只启用ios/android，不启用hmos。Android接入小米（含Redmi）、华为、荣耀、OPPO、vivo配套6.3.0插件；客户端参数从忽略的push.local.properties注入，华为AGC文件校验Android包名，后台Secret只填对应控制台。厂商参数缺失时不宣称厂商离线送达，设置显示当前机型通道未配置。详见../../docs/android-push.md。

通知清理只消除已展示提醒，不撤销有效配对；配对完整集合同步才执行登记/解绑，避免用户点击通知后失去后续提醒。PUT请求超时也保留待确认登记，关闭/移除时仍发送DELETE，防止服务端已成功而客户端丢失登记记录。
