# 会话进度与宿主诊断同步验证

2026-10-26，需求：移动端对齐WhaleTerm MsgLoading，并将会话异常同步到已有日志面板。

## 实现

- 协议v5新增turn_progress/session_diagnostic，活动轮次快照保存进度；旧v2–v4通过同seq unsupported通知保持序列，快照移除新增字段，新手机可以降级旧PC。
- WhaleTerm emitStatus改走显式Turn事件出口，保留现有PC状态行为；Host映射固定phase与受限工具名，不转发包含API正文的label。支持请求/思考/生成/工具及结果/子任务及结果/压缩/重试/等待。
- 移动端显示状态、按宿主起始时间计算耗时、输出token及估算标记；审批优先，断线隐藏实时指示，完成后停表，重连由快照恢复，迟到进度不污染下一轮。
- SDK API重试、查询发送失败、接收失败、响应提前结束、assistant/result错误，以及既有任务/工具失败事件进入PC类别日志。仅固定代码、级别及允许的数值字段，附脱敏目标和进程内C编号；不拦截全部后台日志、不包含原始错误/正文/凭证/Session ID；沿用默认关闭、最近200条内存和复制/导出。

## 验证结果

- `npm --prefix apps/mobile test`：35文件529/529；新增覆盖v5解码/旧版本拒绝、轮次隔离、完成清理、服务器起始时间/估算token/审批优先及计时器释放、日志开关/隐私、真实ProtocolClient→Runtime→日志链路。
- 移动生产构建通过；lint仅既有scripts/cdp-console.mjs:76 no-unused-expressions警告；diff检查通过。
- `GOFLAGS=-buildvcs=false go test ./...`全套通过；protocol/transport/remote race通过。新增测试覆盖v2–v5事件序列兼容、元数据拒绝、进度快照拷贝和迟到状态清理。
- WhaleTerm chat定向race通过，覆盖真实emitStatus出口、快照与诊断转发/隐私；默认vet遇到既有deepwiki/gemini格式化错误，使用-vet=off，未将这些既有错误标成修复。
- WhaleTerm后端Wails应用构建通过，复用既有前端产物；临时modfile与编译包装器引用当前本地Remote，实际go.mod/.base未修改。go version -m已核对产物引用本地Remote。产物 `/Users/zhangym/code/work/whale-term/build/bin/WhaleTerm.app`。首次把modfile放全局GOFLAGS时Wails辅助命令失败，改为仅编译步骤注入后成功。
- 仅同步iOS，Release archive/export成功，实际IPA的11个Web资源逐文件与最终dist一致，远程检查开启、桥接日志none、无server.url；实际IPA深度严格签名校验通过。沿用开发测试签名，未分发/上传。

## 交付与复验

IPA：`apps/mobile/ios/DerivedData/session-progress-20261026/export/App.ipa`。
SHA256：`08cbf7f69ab9f19d1595fb08b81644e21d2b7061a5bab6d22987acc76f109d11`。
日志：`/tmp/cmr-progress-mobile-test.log`、`cmr-progress-go-all.log`、`cmr-progress-go-test.log`、`cmr-progress-whale-test.log`、`cmr-progress-whale-build.log`、`cmr-progress-ios-archive.log`、`cmr-progress-ios-export.log`。

两端均需使用新构建。用户验收：发送任务观察状态/耗时/token；工具、审批与结束时正确切换；活动任务重连恢复；开启调试后API重试/任务失败出现PC记录，HTTP码/重试信息可用时显示。未实际触发在线API错误、未安装/运行真机、未重启当前WhaleTerm；正式依赖版本发布、Android/Harmony原生同步未执行。跨路由可达性问题不因此宣告解决。
