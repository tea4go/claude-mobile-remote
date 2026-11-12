# IP 快速配对实施记录

日期：2026-11-12。源码完成，真实 PC/手机交互待验收。

## 实现

WhaleTerm 新增 backend/quickpair、services/claude_quick_pair.go 和全局 QuickPairDialog；服务启停接入既有 Remote 生命周期。首个 Remote 成功启动后监听进程内 7330，最后一个关闭后停止；端口冲突不影响扫码。后台查询种子与 Registry，包含未订阅草稿；前端选中会话优先，否则本机下拉选择。六位码两分钟有效，最多五次错误，同一 PC 仅一条待处理请求；目标关闭及取消使连接失效。

iOS/Android 共用 QuickPairForm，Harmony 独立 QuickPairForm，默认 IP 入口及验证码输入；原手动导入改为次级入口，扫码继续保留。交付原 Pairing 后沿用认证/订阅/保存/通知登记；地址采用用户连接的主机和目标会话端口。后台/组件销毁清理临时连接。

未修改公共组件依赖或 Session 协议；新入口实现位于 WhaleTerm，因此不需要为本功能发布公共 Go 模块。

## 实际验证

- WhaleTerm `go test -race ./backend/quickpair` 通过：前导零、未选目标、错误验证码、正确会话交付、并发 busy、取消、监听关闭及重新监听。
- WhaleTerm `GOFLAGS=-buildvcs=false go build ./backend/services` 通过；前端生产构建通过。存在既有 macOS API 弃用与前端构建警告。
- 移动端配对组件测试 7/7 通过，包含原手动导入与新握手返回的多网卡地址替换；TypeScript 与 Web 生产构建通过。
- iOS/Android 执行 cap copy，同步最新 Web 资源；本轮未编译这两个原生壳。
- Harmony API 24 Debug 未签名 HAP 构建、ZIP 与最低 API 检查通过；最终 SHA256 c1533dffc38ec5b27b23555a9535c7f890a246189a34594a1b4a53bbb13873aa。存在既有编译警告。
- 两仓库 git diff --check 通过。未执行 Git 写操作、安装或发布。

## 人工验收

需重新构建并运行 WhaleTerm 桌面应用、安装移动端新包：

1. 未开启 Remote 时快速连接失败并提示先启动服务；启动一个后输入 IP，PC 显示验证码，手机验证进入正确会话。
2. 切换到未开启 Remote 的会话，确认弹窗可选已开启目标及待配对草稿；切换主界面不悄悄改变弹窗目标。
3. 两个 Remote 下关闭一个不影响入口，最后一个关闭后 7330 不再监听；重启不恢复。
4. 验证错误、取消、超时、多手机 busy、前后台及多网卡；原扫码/导入仍可连接，完整会话交互与推送需真机回归。

本轮未完整运行 Wails 应用及真机，不将编译成功视为双端验收通过。IP 输入支持系统文本粘贴，设计稿中的独立剪贴板图标尚未添加。
