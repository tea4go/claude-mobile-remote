# HarmonyOS 同步 iOS / Android

用户已确认完整同步范围。本设计替代旧 D2 中单活动连接、无扫码/安全持久化、工具结果不展示等已过时限制；不改变已有服务端协议。

- ArkTS 独立实现既有 v4/v3/v2 协商、v4 output/truncated、附件命令与回显。附件沿用 4 项、单项 4 MiB、总量 8 MiB，受理后清空，拒绝/未知结果保留，不自动重发。
- 同步既有 D61 权限模式：snapshot/permission_mode_changed 为唯一状态来源，set_permission_mode 交 PC 校验执行；未收到服务端变更不提前改变当前模式，模式切换不自动处理已有审批。
- ArkUI 平铺会话行、PC 管理、固定状态、消息滚动、审批抽屉、运行中停止；保留按 Session 隔离连接和草稿。系统选择器提供图片/拍照/文件，图片可预览。
- 原生扫码解码同一配对负载；HUKS 非导出密钥加密配对凭证，应用私有目录保存密文。记录列表和凭证独立，凭证不写日志/明文文件。前台连接全部有效配对，后台断开并取消语音，回前台重新鉴权同步，不回放历史。
- 原生语音识别只生成可编辑草稿，显式发送；权限拒绝、取消、超时和生命周期清理有明确结果。
- 在线事件本地提醒默认关闭、隐藏正文、按 Session/seq 去重；前台当前详情抑制，通知点击定位已保存 Session，冷启动保留入口。系统挂起期间不承诺新事件送达；极光离线推送暂停。
- 私有 JSON 缓存只保存已收到的展示消息及标题/目录/时间；每 Session 200 项/2 MiB，总量 20 MiB，30 天。排除凭证、附件字节和可执行审批/连接/队列状态。重启静态恢复、snapshot 校正，移除配对同时清缓存，设置支持全部清理。不增加数据库和历史接口。

三端核对：使用已有 JSON/WebSocket/base64 数据契约；扫码、HUKS、文件、语音、通知及 ArkUI 为平台适配，不引入新 wire 字段或状态枚举。构建和平台 API 按实际 SDK 验证，不能将 TypeScript 转译检查视为 ArkTS/ArkUI 工程构建通过。

验收：协议降级/跨 Session 拒绝/附件受理与未知结果/工具输出、缓存隔离限额清理/静态恢复和生命周期逻辑；DevEco 构建；真机扫码、重启、权限拒绝、相机、语音、通知点击、多 Session/VPN。工具链不可用时保留准确未验证项。

## 平台接口依据

用户确认采用 Mac ARM64 官方 Command Line Tools 与匹配的 6.1.1(24) SDK 完成终端构建，暂不安装模拟器。工具链在项目忽略目录隔离安装，或以该目录内的本地符号链接接入用户已解压的官方目录，不修改 shell 配置；脚本设置进程级环境并生成未签名 HAP，设备签名和原生体验单独验收。下载需从华为官方源取得并核对官方校验值，不替换项目 SDK 基准。

- [华为 ScanKit 默认扫码](https://developer.huawei.com/consumer/en/doc/harmonyos-guides-V14/scan-scanbarcode-V14)。
- [华为官方 CoreSpeechKit 示例](https://gitee.com/harmonyos_codelabs/core-speech-kit-codelab-ark-ts-kit-asrdemo/blob/master/entry/src/main/ets/pages/Index.ets)：AudioCapturer 音频送入识别引擎，取消/完成后清理。
- [OpenHarmony HUKS 加解密官方示例](https://github.com/openharmony/docs/blob/master/en/application-dev/security/UniversalKeystoreKit/huks-encryption-decryption-arkts.md)：AES-GCM 使用随机 nonce、关联数据及认证标签。
- [OpenHarmony NotificationManager 类型声明](https://github.com/openharmony/interface_sdk-js/blob/master/api/@ohos.notificationManager.d.ts)：授权、本地发布和撤回；设备 SDK 仍需实际编译复核。
