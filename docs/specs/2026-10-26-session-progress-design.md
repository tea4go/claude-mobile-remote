# 会话进度与宿主诊断同步

沿用D80/D93。WhaleTerm emitStatus目前绕过Remote事件出口，Host也忽略onstatus。新增v5事件turn_progress与session_diagnostic，进度含phase、开始/更新时间、token统计；诊断只允许固定类别及HTTP码/重试次数/等待毫秒，不包含任意日志文本。状态标签由移动端固定映射，工具名/原始retry message不进诊断。活动轮次快照保存进度，重连恢复；终止/下一轮清理，旧轮迟到状态丢弃。审批优先展示等待审批，断线时隐藏实时进度。

v5客户端可与v2–v4服务端协商降级；旧客户端接收新事件时映射unknown_subtype通知，保持seq连续且不泄漏新字段。三端契约是纯JSON和数字，不改Session/Turn状态、输入串行或配对授权。Harmony新字段实现待iOS验收，现有客户端走旧协议。

移动端实现动态状态、秒级耗时、已知/估算token；会话API重试、SDK接收/发送失败、assistant/result失败及工具失败按安全类别记录在现有200条调试面板。禁止全局拦截后台日志，禁止原始错误进入面板。

验证：序列与旧版本兼容、快照恢复、轮次隔离、元数据校验、日志隐私和关停、UI计时与终止；Go定向/race、前端测试/build、WhaleTerm本地替换依赖构建、iOS Debug编译。真机网络/API重试与状态体验由用户验收。

实现细节：状态标签采用固定phase映射，工具名仅限128字符的标识符，不转发原始label（API重试label可能含响应正文）。诊断面板标识PC类别，附脱敏目标地址及C连接编号，沿用开关/容量/复制/导出；progress不写入本地会话缓存。旧端通过unsupported通知推进seq，不能显示新进度。WhaleTerm无须第二个SDK Client，仍复用现有Host事件桥。
