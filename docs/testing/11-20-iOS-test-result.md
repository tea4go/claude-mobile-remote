# 11月20日 iOS测试问题

### 背景

iOS版本：1.0.9(11月19日发布)
whale-term：windows最新版

### 问题记录

1. 在手机上，已知目录下已存在连接成功的session，点击目录plus报错：操作失败请重试。问题截图：docs/testing/11-20-error/add-session-error.png
   1.1. 报错提示不友好，定位不出原因
   1.2. 可能是网络断开了，但是列表的状态还没有刷新，用户没有主动刷新；可能是点击plus很频繁；

2. 进入sessionDetail后，标题显示：已连接 就绪 Turn: 失败.但是还是能在底部输入，发送后报错。问题截图：docs/testing/11-20-error/connect-claude-error.png

3. 在PC whale-term上新建了一个claude会话，点击 启动remote service报错了，问题截图：docs/testing/11-20-error/start-remote-service-error.png
   3.1. 在启动此新会话的remote service之前，已存在了一个 启动的remote cli session


### 修复与验证

1. 目录加号按实际结果提示。连接未同步或已断开时提醒刷新，重复点击只提交一次。电脑返回来源不可用、创建限频或容量上限时显示对应指引，创建失败时引导检查电脑上的远程服务。旧版本的通用错误无法仅凭截图追溯到具体原因。
2. WhaleTerm 在开放远程服务和执行 Claude CLI 恢复前校验会话 UUID。不可恢复的旧会话保留桌面历史，打开远程时提示在同目录新建。普通轮次失败后仍可发送下一条消息。
3. Windows 监听失败时识别 Winsock 地址占用错误，并顺延到可用端口。配对使用顺延后的地址。

移动端 703 项回归和生产 Web 构建通过。补充普通轮次失败后可发送的断言后，reducer 85 项复跑通过。WhaleTerm 定向回归通过，包括真实回环端口占用顺延及目录配对后通过 WebSocket 新建第二个会话。Windows x64 交叉构建通过。

真机复验清单（用户已确认通过）

- iPhone 在断线或快速连点时使用目录加号，确认提示与连接状态一致，电脑只创建一个会话。
- 在 Windows 尝试为不可恢复的旧会话启动远程服务，确认电脑提示在同目录新建。用新会话配对并在 iPhone 发送消息。
- 保持首个远程服务开启，在 Windows 启动第二个远程服务，确认两条会话都能在 iPhone 独立配对和收发。
