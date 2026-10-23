# HarmonyOS NEXT

独立 ArkTS / ArkUI 工程，功能同步基线为 2026-10-22 iOS / Android。设计见 [Harmony 同步设计](../../docs/specs/2026-10-22-harmony-parity-design.md)。

## 已接入的代码

- v4/v3/v2 协议协商，权限模式读取/切换、工具结果正文、审批/问答、取消与排队、多 Session、新建/终止（按宿主能力）。
- 扫码及粘贴配对、中文机器名、平铺列表、PC 管理、列表滚动恢复、固定状态、审批抽屉与系统返回键、受限 Markdown 和表格。
- 图片、系统相机、文件选择；追加/移除/预览、附件单独发送，最多 4 项、单项 4 MiB、总量 8 MiB；PC 确认受理才清空，失败或未知保留草稿。超限/HEIC 图片尝试缩放转 JPEG。
- 系统语音识别生成草稿，用户手动发送；停止/取消、60 秒限时、权限及生命周期清理。
- 本地通知默认关闭，可在设置开启；只提醒收到的在线事件，隐藏正文、按 Session/seq 去重，点击定位支持冷启动。极光离线推送仍暂停。
- Pair Token 单独使用 HUKS AES-256-GCM 加密后写入私有文件，密钥不导出；列表元数据不含明文凭证。未声明备份扩展，不将应用数据接入备份/迁移。
- 已收到的展示消息使用私有 JSON 缓存，无数据库：每 Session 200 项/2 MiB、总量 20 MiB、30 天；不缓存附件字节、凭证及可执行审批/连接/排队状态。删除配对同时清缓存；设置可全部清理。
- 前台同步全部有效配对，切换详情不关闭连接；后台断开连接并取消录音，回前台按 snapshot 校正。系统挂起期间不保证提醒和消息送达，不补回离线期间历史。

## 本机逻辑验证

先确保 `apps/mobile/node_modules/typescript` 可用（沿用项目已有 TypeScript，不增加依赖），然后：

```bash
npm --prefix apps/harmony test
```

测试包含协议、状态、附件、缓存投影、文件顺序、加密封装、通知和语音生命周期。原生 API 使用 Node 测试适配器；TypeScript 检查只覆盖逻辑与原生文件语法，**不是 Harmony SDK / ArkUI 编译验证**。

## 工程构建与真机

### Mac ARM64 终端构建

用户已确认优先使用命令行工具链，不安装模拟器镜像。先在[华为官方下载页](https://developer.huawei.com/consumer/cn/download/command-line-tools-for-hmos)登录，选择 **macOS ARM64 / Command Line Tools 6.1.1**，保存 ZIP 与页面 SHA-256。暂未取得包时，脚本完成不代表环境安装或工程编译完成。

```bash
# 在仓库根目录执行；HASH 替换为官方下载页完整 SHA-256。
bash scripts/macos/1_install_harmony_tools.sh --archive /path/to/commandline-tools-mac-arm64-6.1.1.zip --sha256 HASH
bash scripts/macos/2_harmony_check.sh
bash scripts/macos/4_harmony_build.sh
```

默认安装到 `.local-tools/harmony/6.1.1`（Git 忽略），不会修改 shell 配置。已有独立工具链可用 `HARMONY_TOOLS_HOME` 指定解压后的 `command-line-tools` 目录。已实测 6.1.1.418 的 ArkTS HAP 构建不需要额外安装 JDK。环境检查使用配套 Node 和 SDK 元数据核对 API 24，缺失时直接报错，不自动升级 SDK。

构建会运行 `ohpm install --all` 与 `hvigorw assembleHap --no-daemon`，首次需要网络。目标产物为 `entry/build/default/outputs/default/entry-default-unsigned.hap`，检查 ZIP 完整性、最低 API `60101024` 并打印 SHA-256。未签名 HAP 用于工程编译验证，安装到手机需另行配置签名。2026-10-23 已用 ARM64 6.1.1.418 实际编译成功。当前机器以忽略目录中的符号链接接入用户已解压的工具链，当前指向 `/Volumes/T7/code/tools/harmony/command-line-tools`，连接 T7 外接硬盘后即可直接运行上述检查和构建命令；工具链约 6.1 GiB。迁移到外接盘若出现工具内部 `material/fd is illegal` 错误，应检查 macOS 生成的 `._*` AppleDouble 附属文件；本机已清理该资源目录的附属文件并复验构建通过。工具的用户级依赖缓存位于 `~/.ohpm` / `~/.hvigor`。

### DevEco / 设备验证

工程保持已有 `6.1.1(24)` SDK 基准，使用安装了对应 SDK 的 DevEco Studio 打开此目录，配置本机签名后构建。已有 Windows `build-unsigned.ps1`、`build-signed.ps1`、`deploy.ps1` 可继续使用。若已配置 devecocli：

```bash
devecocli build --modules entry@default
devecocli device list
devecocli run --module entry --device <target>
```

2026-10-22 本次 Mac 环境未安装可用的 devecocli/hvigor/ohpm，构建命令返回 command not found；本轮没有生成 HAP，也没有完成 ArkUI/原生 API 编译与真机验收。平台编译错误仍可能存在，不能仅凭逻辑测试将 D3 标记完成。

真机清单和实际验证结果见 [同步验证记录](../../docs/testing/2026-10-22-harmony-parity-result.md)。
