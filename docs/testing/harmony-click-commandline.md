# Harmony 点击响应命令行复测

先连接实际鸿蒙手机、开启开发者模式/USB 调试、手机确认调试授权。工具使用项目既有官方 API24 Command Line Tools；不要求安装完整 DevEco Studio。脚本不自动安装应用或清除现有数据。

```bash
bash scripts/macos/6_harmony_click_check.sh --probe
# 先在手机打开待测页面，再执行；采集启动后等待 1–2 秒手动点击一次。
bash scripts/macos/6_harmony_click_check.sh --measure "进入离线演示"
```

多个设备用 `--device` 选择。默认报告保存在忽略目录 `.local-tools/harmony-performance`，包含设备工具原始输出与结构化结果；`--out` 可指定其他本地目录。采集使用设备自带 `SP_daemon -editor timeDelay`，系统版本或权限不支持时会失败，不将无数据视为通过。每条结果仅代表该次系统测量，不能替代云端审核。

本次审核详情：HarmonyOS 6.1 / Pura 80 Pro+，319.0ms，优化目标250.0ms，涉及演示界面操作和扫码；以用户文字描述定位，不读取审核视频。

退出码：0=取得 <=250ms 的单次响应结果（或探测成功）；1=实测 >250ms；2=缺设备、无有效数据、超时或工具错误。可用 `--threshold-ms 100` 另行观察更严格的体验目标。探测成功不代表 `timeDelay` 子命令或应用性能达标。脚本不使用主机侧命令执行时长作点击时延。

对当前修复包至少测这些场景，每个重复5次，分别记录冷/暖状态；每次先回到同一起始页面。初始进入演示、演示发送/停止/审批/问答/退出、打开/关闭调试、真实配对的扫码/连接、列表进出长会话、抽屉打开关闭、清除缓存。长会话/大附件和后台回前台也需复测。输入、权限对话框和局域网/VPN连接必须使用真机，不以 Previewer 代替。

结果超标时保留原始输出并使用 DevEco Profiler/云测试取得对应 trace、控件和测试机型，继续区分点击至首次反馈与后续网络/操作完成时间。设备工具对系统打点/首帧有适用限制，不把单次测量解释为全应用通过。

官方参考：[应用性能体验建议—时延](https://developer.huawei.com/consumer/cn/doc/harmonyos-guides/performance-delay)、[SmartPerf 指导](https://developer.huawei.com/consumer/cn/doc/harmonyos-guides/smartperf-guidelines)、[OpenHarmony UI 测试命令行指导](https://github.com/openharmony/docs/blob/master/zh-cn/application-dev/application-test/uitest-guidelines.md)。
