# HarmonyOS NEXT

独立的 HarmonyOS NEXT Stage 工程，使用 ArkTS + ArkUI。

## 当前范围

- D1 Task 2：建立基础工程、固定应用标识、声明 `ohos.permission.INTERNET`、替换默认模板首页。
- 不包含：协议模块、Session 状态、扫码、安全存储、推送、真机 VPN 与发布验证。

## 常用命令

```bash
devecocli build
devecocli run --module entry --device <target>
devecocli device list
devecocli emulator list
```

## 说明

- 本工程通过 `devecocli create` 生成后做了最小项目化修正。
- DevEco 模板实际生成的页面清单位于 `entry/src/main/resources/base/profile/main_pages.json`。
