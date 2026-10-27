# Harmony 1.0.1 AppTest发布记录

## 范围

用户授权沿用已有发布签名构建1.0.1并上传测试审核；沿用既有AppTest设计，包含本次同步功能与离线演示，不执行Git写操作。

## 本机构建通过

- versionName 1.0.1，versionCode 1000001；仅更新Harmony AppScope版本。
- `bash scripts/macos/5_harmony_apptest.sh --config <仓库外既有signing.json>`：Release APP构建、ZIP、包名、API24、Profile及证书一致性检查通过，华为verify-profile/verify-app成功。
- 产物：`.local-tools/harmony-apptest/signed/ClaudeMobileRemote-signed.app`。
- SHA256：`a3cc98954b91b49cb62571d661c21b9b1a50eeca928fe878a257a15d513c02e5`。
- 日志：`/tmp/cmr-harmony-apptest-101.log`。
- pack.info独立读取确认1.0.1(1000001)，包名com.tea4go.claudemobileremote；diff --check通过。
- 本轮只修改版本，无功能变更；功能回归沿用mobile-parity记录，未重复运行或真机安装。

## 后台状态

已实际核实旧1.0.0合法性“已达标”、上架自检“通过”。进入上传流程，选择“仅测试”；原生文件选择器自动确认异常，已请求用户手动选择上述文件。新1.0.1上传、审核提交及测试分发尚未完成，不将旧版达标等同新版通过。

## 下一步

用户选取文件后继续核实上传及合法性处理，创建/提交邀请测试任务；完成后再记录后台实际状态。测试生效后验证安装和离线演示。
