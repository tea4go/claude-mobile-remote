# Mac ArkUI 预览与 AppTest

工具链仍使用 `.local-tools/harmony/6.1.1`，指向 T7 上用户提供的官方工具。需要挂载外接盘，不需要安装完整模拟器。

## 界面预览

```bash
bash scripts/macos/3_harmony_preview.sh
# 只编译，不启动服务
bash scripts/macos/3_harmony_preview.sh --build-only
```

脚本将工程输入复制到忽略目录 `.local-tools/harmony-preview/project`，仅在副本中替换 Index 入口并关闭规范化模块寻址以适配 Previewer。真实应用入口和构建配置不变。复用现有列表、详情、设置和审批组件，数据来自 `apps/harmony/preview/Index.ets`，无真实配对、网络连接或数据持久化。

编译后使用 SDK 原生 Previewer 渲染，以本机浏览器作为图像和鼠标操作前端；不是用 HTML 重写 Harmony 界面。仅绑定127.0.0.1，命令接口校验随机路径和来源。Ctrl+C退出，改代码后重新运行。暂不提供热更新、键盘输入转发和断点调试。扫码、语音、文件、安全存储及通知须设备验证。

顶部黄色提示可点击切换列表、工具审批、问题回答、运行中、录音中、断连和长标题场景，均为模拟数据。列表包含两台电脑、长目录和不同状态。

日志在 `.local-tools/harmony-preview/previewer.log`。SDK 迁移外接盘若生成 `._*.json` 会导致预览编译失败；不要删除原始SDK文件或自动忽略实际编译错误。

## AppTest 应用和签名

已由用户创建应用，截图核对：

- 应用名称：Claude Mobile Remote。
- 包名：`com.tea4go.claudemobileremote`。
- APP ID：`6917619813829911575`。
- 后台支持手机/平板；当前工程仅声明phone，不据此承诺平板适配。

已有 Debug 证书用于开发调试。AppTest 选择应用市场邀请测试流程，准备独立发布证书与应用市场发布Profile，不使用指定设备分发Profile。AppTest入口是否已开通、当前提交必填项仍需后台核实。

发布材料已生成并核验在仓库外：

`~/.config/claude-mobile-remote/harmony-signing/release/`

- `release.csr`：申请发布证书时上传的CSR。
- `release.p12`：本机私钥库，别名`cmr-release`，不要上传或提交。
- `password.txt`：本机密码，勿粘贴到聊天或日志。
- `signing.json`：本机文件路径配置，指向已核验的`release.cer`及`release.p7b`。

若在另一台Mac重新准备，可用以下命令创建一个全新私有目录；脚本拒绝覆盖已有目录：

```bash
export JAVA_HOME="$HOME/jdk" # 指向实际安装的JDK
bash scripts/macos/5_harmony_apptest.sh --init-signing "$HOME/.config/claude-mobile-remote/harmony-signing/new-release"
```

后台步骤：

1. 开发者管理 → ID、证书和Profile → 证书，新增HarmonyOS发布证书，上传`release.csr`。仅上传CSR，不上传p12或密码。
2. 下载签发的发布证书，保存为私有目录内`release.cer`，权限0600。
3. 在Profile页面选择该应用、发布类型、应用市场发布方式及上述发布证书，下载为`release.p7b`，权限0600。若页面选项不同，提供脱敏截图核对，不选择“指定设备发布”代替AppTest渠道。
4. 材料到齐后执行签名构建。脚本要求所有材料及配置位于仓库外，权限不对其他用户开放。

## 构建

```bash
# 只验证Release APP打包，不能上传分发
bash scripts/macos/5_harmony_apptest.sh --unsigned

# 有发布材料后：构建并核验内嵌HAP签名
bash scripts/macos/5_harmony_apptest.sh --config "$HOME/.config/claude-mobile-remote/harmony-signing/release/signing.json"
```

脚本在一次性私有工程副本配置签名，不改源码工程。密码只从本机文件读取，不放命令行参数；工具输出进行密码脱敏。临时副本退出清理，最终APP保留在`.local-tools/harmony-apptest/{unsigned,signed}`。不自动递增版本号；再次提交更高版本前，人工确认并更新工程版本。

华为审核反馈旧安装图标像系统图标后，Harmony 已改用现有 iOS/Android 的浅蓝 Capacitor 风格图案。后台上传图标为 `apps/harmony/branding/app-icon-216.png`（216×216）。同目录 `app-icon.svg` 是以原 iOS PNG 嵌入的独立 SVG 容器，保证与已有平台图案一致；原始资产不是矢量文件。应用图标分别来自 AppScope 和 entry 的分层资源，启动图标也已更新。修改图标后须重新生成并上传签名 APP，单独更新后台图片不会改变已安装应用的图标。

本地配置示例（仅路径，不含密码）：

```json
{
  "storeFile": "release.p12",
  "certpath": "release.cer",
  "profile": "release.p7b",
  "keyAlias": "cmr-release",
  "storePasswordFile": "password.txt",
  "keyPasswordFile": "password.txt"
}
```

本机校验包括APP ZIP结构、pack.info、内嵌HAP包名/Release模式/最低API24，签名模式再用华为签名工具验证APP本身，核对内嵌Profile与发布证书。常规APP内的HAP默认不带独立签名；Hvigor另生成单独的signed.hap。JDK 21.0.0在签名时触发ZIP64误报，脚本仅对此JDK临时加入兼容参数，并独立检查APP与内嵌HAP的ZIP结构。华为工具的验证不作为独立根信任或后台分发通过的声明。当前已成功生成本机验签的发布包；用户上传后后台显示合法性“已达标”，但版本仍不可选，上架自检仍在检测中，原因待结果报告确认。

## 发布与验收

进入应用管理的应用版本/测试入口，确认使用AppTest邀请测试而非旧开放式测试。用户已上传本轮签名APP；待上架自检和版本可选性明确后，再填写实际要求的测试信息。提交前核对应用、版本、测试用户范围和生效时间。当前脚本只本机构建，不上传、不创建测试任务、不向他人发送邀请。

测试版本生效后，由同事通过AppTest接受邀请安装。当前最低HarmonyOS6.1.1/API24；后台通过不能替代设备安装。验收列表：配对、消息、多Session隔离、审批、附件、权限拒绝、语音、本地通知点击和重启恢复。

官方依据：

- [Previewer源码说明](https://github.com/openharmony/ide_previewer/blob/master/README_zh.md)：原生渲染及宿主通信。
- [华为提交应用](https://developer.huawei.com/consumer/cn/app/submit)：APP发布包。
- [华为应用市场测试流程](https://developer.huawei.com/consumer/cn/appgallery)：测试版本、群组与邀请。
- [AppTest相关测试群组能力](https://developer.huawei.com/consumer/cn/doc/doccenter-submission/agc-help-test-api-add-test-group-user-0000002236201338)：AppTest与旧渠道入口可能不同，未切换时需后台确认。
