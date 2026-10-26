# iOS 网络反馈修复计划

沿用 D80 和本轮确认，设计见 ../specs/2026-10-26-ios-network-design.md。

1. 登记 HANDOFF，记录三端兼容结论；保留原始真机失败记录。
2. ProtocolClient 加入各阶段/总预算与命令受理超时、固定错误码、有限脱敏诊断，补假时钟测试。
3. 候选配对复用同一连接，snapshot 后保存并导航；取消/失败回收，新增配对反馈与诊断面板，覆盖原生与浏览器回归。
4. Transport/remote 增加宿主查询统计和脱敏诊断，测试多连接与关闭；接入 WhaleTerm 查询及界面。
5. 执行移动端/Go/WhaleTerm/iOS 门禁；更新 HANDOFF 与测试记录，提供实际主副网与权限复验清单。原生 Android/Harmony 后续同步。

6. 根据跨路由复现补充配对页目标PC地址，失败后保留，定向回归、构建并仅同步iOS资源。

## 平台调试替代（用户后续修订）

移除自建网络诊断面板及入口。开发调试采用iOS Safari Web Inspector/Xcode、Android Chrome DevTools/Android Studio。保留配对目标地址、阶段进度、超时及就地错误；内部脱敏记录仅用于连接错误归因，不向产品界面展示记录面板。不为此开启Release包远程检查。此前面板展示要求由本条替代。

## Debug/Release远程检查补充

用户明确要求开发及Release构建均可检查WebView；将iOS/Android webContentsDebuggingEnabled设为true，替代此前不启用Release远程检查的安排。iOS先行重建签名Release IPA并核验入包配置，保留loggingBehavior:none。Safari/Chrome检查器在电脑上使用，不在App内新增面板。三端协议及配对契约不变。

## 应用内调试面板（用户确认的后续修订）

用户确认增加设置开关与全局悬浮入口，开发/Release共用，替代此前禁止App内面板的部分。保留Safari检查。最近200条仅内存保存，固定动作/连接阶段/异常类别和脱敏scheme/host/port，不收集Session ID、凭证、正文、协议帧或原始错误/堆栈。支持清空、复制、导出和失败提示。默认关闭，关闭停止新增记录，已有记录退出进程清除；开关不持久化。

实施：有界存储及异常监听 → 复用连接记录、补动作记录 → 设置与悬浮面板 → 隐私/容量/交互测试、生产构建、仅同步iOS。不拦截console或底层网络，不完整映射Safari，无新依赖，无协议/状态/Pairing契约变化；其他原生端待iOS验收。

iOS复制通过系统剪贴板（仅本机、5分钟过期），导出通过UIActivityViewController分享临时UTF-8文本文件，完成/取消后删除临时目录；用户明确保存的导出文件由用户管理。接口可选，不改共用协议；非iOS保留浏览器复制/下载路径，其他原生端未验收。

## 真机日志反馈优化

依据images/1026/mobile_log.png：日志增加固定事件码/级别、进程内连接编号与尝试次数、阶段及阶段耗时、总耗时、协议版本、超时阈值、WebSocket readyState、close code/wasClean及固定关闭码解释。不收集close.reason或异常原文。记录主动关闭、手动重连、协议降级和错误后的close事件，旧连接延迟事件只写其原尝试记录、不改变新连接。现有客户端无自动重试，断开记录明确等待手动重连，不实现或暗示新重试策略。

面板列表倒序实时更新、毫秒时间、紧凑工具栏与行距；左下入口根据可见输入组件和visualViewport动态避让，键盘/输入区变化时更新。验证覆盖关闭元数据、error→close、重连关联、超时、敏感字段过滤、倒序和入口布局计算，构建同步iOS并重打IPA。
