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

工程保持已有 `6.1.1(24)` SDK 基准，使用安装了对应 SDK 的 DevEco Studio 打开此目录，配置本机签名后构建。已有 Windows `build-unsigned.ps1`、`build-signed.ps1`、`deploy.ps1` 可继续使用。若已配置 devecocli：

```bash
devecocli build --modules entry@default
devecocli device list
devecocli run --module entry --device <target>
```

2026-10-22 本次 Mac 环境未安装可用的 devecocli/hvigor/ohpm，构建命令返回 command not found；本轮没有生成 HAP，也没有完成 ArkUI/原生 API 编译与真机验收。平台编译错误仍可能存在，不能仅凭逻辑测试将 D3 标记完成。

真机清单和实际验证结果见 [同步验证记录](../../docs/testing/2026-10-22-harmony-parity-result.md)。
