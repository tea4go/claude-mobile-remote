# Harmony 同步 iOS 实施计划

依据 D111–D120 和既有 Harmony 同步设计，不改变共用契约。逐项核对新版界面、目录创建与配对、电脑筛选和偏好、调试、消息复制/触觉、附件、队列和生命周期。WhaleTerm 队列采用 v8 宿主权威状态、支持时展示输入区入口及取消/引导，断线禁用，异常提示；旧宿主按能力降级。检查长列表、消息流式更新和多会话状态开销，避免引入无界计算。同步完成后记录实际检查、平台构建和真机验证缺口；用户自行执行 npm lint/build/test。旧缓存问题及 Harmony 离线推送维持既有范围。

消息选择沿用 D118/D119：保留 ArkUI Markdown，按渲染段落、列表标记、代码及表格单元格建立统一可见文本偏移。TextController 提供字符范围矩形及触点命中，组件负责高亮、双手柄、复制菜单和约 500ms 全选，消息正文关闭系统选择。ComponentUtils/文本布局的 px 转为窗口 vp，水平滚动块与列表视口分别裁剪；拖动边缘滚动、失焦/后台/内容变化清理，轻触觉采用系统 soft 预设并静默降级。仅活动选区安排刷新和订阅生命周期；边界惰性缓存，流式解析保留已有 Text 绑定。真实行为、字体缩放和长内容性能列入设备验收。

接口依据：[Text 布局](https://developer.huawei.com/consumer/cn/doc/harmonyos-references/ts-text-common/)、[组件坐标](https://developer.huawei.com/consumer/cn/doc/harmonyos-references/js-apis-arkui-componentutils/)、[手势坐标](https://developer.huawei.com/consumer/cn/doc/harmonyos-references/ts-gesture-common/)、[原生触觉](https://developer.huawei.com/consumer/cn/doc/harmonyos-references/js-apis-vibrator/)。以当前本地 API 24 声明与实际 hvigor 编译为准。

实现检查与真机清单见 `../testing/2026-11-10-harmony-ios-sync-result.md`。源码同步完成不等于跨端行为、视觉或性能验收通过。
