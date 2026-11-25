# ccBuddy HarmonyOS 正式版发包手册

在仓库根目录对 Codex 说「按 docs/harmony-release.md 构建 HarmonyOS 正式版」。流程从确认版本开始，到交付本地已验签的 Release APP 为止。AppGallery Connect 上传和提审属于后续操作。

## 1. 确认版本并生成更新列表

1. 请发包人输入或确认显示版本号，格式为 `X.Y.Z`。默认取 `apps/harmony/AppScope/app.json5` 中的 `versionName`，当前为 `1.0.0`。
2. 核对 ccBuddy 在 AppGallery Connect 的正式发布与邀请测试记录，以及最近的本地归档，再确认本轮递增的 `versionCode`。同一显示版本重打包也使用新构建号。后台记录无法读取时，请发包人提供已用的最高构建号。
3. 确认本轮源码快照和上次正式版的对比基线。编译前生成 `更新列表.md` 并发给发包人，写明显示版本、构建号、用户可感知的变化及对应验证项。基线无法确认时，在列表中注明并列出能核实的变更。
4. 在独立构建副本的 `apps/harmony/AppScope/app.json5` 设置确认后的 `versionName` 和 `versionCode`。核对目标包名为 `com.tea4go.claudemobileremote1028`。在原仓库新建归档目录 `.local-tools/releases/<日期>-harmony-v<版本>-release-b<构建号>/`，保存更新列表和本轮产物。

## 2. 构建正式签名 APP

使用 ccBuddy 的 MyPhone 发布签名。实际签名材料为 `MyPhoneCSR.cer`、配套私钥库 `MyPhoneKey.p12` 和应用市场发布 Profile `ccBuddy-ReleaseRelease.p7b`。证书和 Profile 位于 `~/.config/claude-mobile-remote/harmony-signing/myphone-release/`，私钥库位于上一级目录。原始 `MyPhoneCSR.csr` 用于证书申请。

在 macOS 构建副本中确认 HarmonyOS SDK 6.1.1 / API 24 工具链和 JDK 可用。将 `HARMONY_SIGNING_CONFIG` 设为仓库外的签名配置绝对路径。配置格式沿用 [HarmonyOS 构建说明](harmony-preview-apptest.md#构建)，证书、私钥和 Profile 使用本节列出的文件，`keyAlias` 使用私钥库的实际别名。配置、材料及密码文件权限设为 `0600`。

```bash
bash scripts/macos/5_harmony_apptest.sh --config "$HARMONY_SIGNING_CONFIG"
```

仓库现有脚本通过 `assembleApp` 构建 Release APP 并执行验签。成功产物位于构建副本的 `.local-tools/harmony-apptest/signed/ClaudeMobileRemote-signed.app`。构建或签名失败时停止在当前步骤，保留诊断结果，仅将本轮成功产物用于交付。

## 3. 验签、归档并结束

- 检查 APP 和内嵌 HAP 的完整性，从包内 `pack.info` 或 HAP 的 `module.json` 核对目标包名、显示版本和构建号，确认内嵌 HAP 为 Release 模式。
- 确认华为工具对 APP 验签成功，内嵌 Profile 与 `ccBuddy-ReleaseRelease.p7b` 一致，并核对发布叶证书为 `MyPhoneCSR.cer` 中对应的发布证书。
- 将已验签产物归档为 `ccBuddy-<版本>-b<构建号>-release-signed.app`。回报更新列表、APP 路径、版本、构建号和 SHA-256。交付后结束本轮流程。
