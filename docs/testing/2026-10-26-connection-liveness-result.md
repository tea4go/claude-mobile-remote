# 连接存活检测验证（2026-10-26）

## 原因与改动

PC已有10秒控制帧ping和30秒无pong回收，WebView自动处理控制帧，移动JS无法观察。移动端之前只处理open/message/error/close及操作超时；offline只写日志，前台刷新只读取内部stage，未探测链路。因此网络变化后存在系统socket未通知而界面仍在线的窗口。

按D95扩展v6应用层ping/pong（同Session，seq=0，空correlation/payload），PC仅已订阅态接受，业务队列不接收心跳。手机10秒发送、5秒未响应断开；offline立即断开，online/前台复查。旧v2–v5兼容，无心跳能力在日志说明，不自动发送不兼容帧。保留配对与消息、不重发操作，未受理结果沿用未知语义。

## 实际验证

- 移动端35文件537/537：静默断网、心跳正常空闲、pending命令未知、前台超时、offline、错误Session、重连迟到响应、v5降级与旧端不发ping。
- Go全部12包通过；transport全套race通过，最终新增心跳race通过。真实WebSocket验证pong及Session隔离、无Claude输入；非法版本/seq/相关ID/payload拒绝。
- Web生产构建通过；lint仅既有scripts/cdp-console.mjs:76警告。
- WhaleTerm使用临时modfile引用本地Remote构建成功，build/bin/WhaleTerm.app；未更新正式发布依赖。
- cap copy ios通过，未重写SPM依赖；Release archive/export通过，实际IPA11个Web资源与dist一致、检查配置正确、深度严格验签通过（需沙箱外macOS信任服务）。两仓git diff --check通过。
- 初次沙箱Go测试因禁止本地监听失败，授权环境重跑通过；沙箱签名信任查询失败，授权环境实际验签通过。

IPA：apps/mobile/ios/DerivedData/connection-liveness-20261026/export/App.ipa

SHA256：046f7d17c7e59d667ae2eae1f81effc643cc4ce7cff1b6bf89f9b33ddd2bd8ff

日志：/tmp/cmr-liveness-*。

## 待人工验收

1. 更新并重启新WhaleTerm，安装新IPA，配对后不发送消息，切换Wi-Fi或关闭网络：前台静默断网约15秒内改为断开，offline信号可更快；调试日志应见ping/pong或heartbeat_timeout/network_offline。
2. 网络恢复后手动重连，连接和snapshot恢复，不重复提交消息；空闲正常联网不误断。
3. 后台切网后恢复前台，旧失效连接及时更新；系统暂停期间不保证定时器或日志实时运行。

未安装/启动设备、未重启现有WhaleTerm、未上传、无Git写操作。Android/Harmony原生未同步。原主副路由可达性问题没有由本轮证明解决；此前Xcode GUI依赖问题也未由命令行归档成功证明恢复。

## 人工验收反馈

2026-10-26，用户反馈“OK，验证通过”，并授权提交 claude-mobile-remote。本轮连接存活检测人工验收记为通过；用户未提供逐项日志或检测耗时，不补充未经观测的数值，也不据此认定原跨路由网络问题已解决。Android/Harmony原生同步及正式Remote依赖发布仍未执行。本次仅本仓库本地提交，不推送；测试截图与构建缓存不纳入提交。
