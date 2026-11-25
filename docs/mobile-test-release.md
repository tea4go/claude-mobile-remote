# ccBuddy 三端签名测试发包手册

在仓库根目录把本手册交给 Codex 执行，例如「按 docs/mobile-test-release.md 发一轮三端测试包」。先向发包人确认版本并生成更新列表，再依次打包 iOS、HarmonyOS、Android。iOS 完成 TestFlight 上传，另外两端交付本地签名产物。以下命令适用于 macOS，在当轮构建副本的根目录执行。

## 1. 确认版本和生成更新列表

1. 先请发包人输入或确认显示版本号，格式为 `X.Y.Z`。从 iOS App 的 Release `MARKETING_VERSION` 提取默认值，目前为 `1.1.1`。三端使用同一个确认值。
2. 查对应后台和最近的本地发包记录，分别确认三端的构建号。iOS 的脚本接受 1–9999。HarmonyOS 和 Android 分别用各自递增的 `versionCode`。显示版本相同也要为新构建分配新构建号。
3. 选定三端本次共同使用的源码快照，包含发包人确认纳入的本地变更。查上次发给同一测试渠道的源码基线，基线因平台而异时分别核对。在编译前把差异整理成 `更新列表.md`，放到本轮归档目录，并发给发包人。每项写清适用平台、用户可感知的变化和测试操作或预期结果。记录显示版本、各端构建号及对比基线。基线找不到时明确标注，按实际可核对的变更生成列表。
4. 在不含签名私钥的独立构建副本中准备版本参数和工具链。保留工作区现有改动，先检查 Xcode 登录、HarmonyOS 工具链和 MyPhoneCSR 签名材料、Android SDK Build Tools 36.0.0 和已有签名配置是否可用。复用已安装的工具和依赖。

建议归档至原仓库的 `.local-tools/releases/<日期>-v<版本>-test/<批次>/`。用新目录区分批次。下面的 `RUN_DIR` 取该目录的绝对路径，`IOS_BUILD` 取已确认的 iOS 构建号。

## 2. iOS 签名并上传 TestFlight

在构建副本的 `apps/mobile/ios/App/App.xcodeproj/project.pbxproj` 设置 App target 的 Release `MARKETING_VERSION`。构建号由脚本参数覆盖。

```bash
bash scripts/macos/5_ios_testflight.sh build \
  --build-number "$IOS_BUILD" --output "$RUN_DIR/ios"
```

检查导出的 IPA 和归档具有确认的显示版本及构建号。脚本会检查分发签名并生成 `verified.json`。确认后调用同一脚本的上传入口。

```bash
bash scripts/macos/5_ios_testflight.sh upload --output "$RUN_DIR/ios"
```

记录 Apple 接收结果。上传完成后可继续 HarmonyOS 构建，TestFlight 处理和测试组可安装状态分别在后台确认。上传结果不明确时，重试前先查后台是否已收到该构建。首次配置见 [iOS TestFlight](ios-testflight.md)。

## 3. HarmonyOS 签名邀请测试 APP

在构建副本的 `apps/harmony/AppScope/app.json5` 设置 `versionName` 和 `versionCode`。使用 ccBuddy 应用市场发布签名构建邀请测试 APP，包名为 `com.tea4go.claudemobileremote1028`。

`MyPhoneCSR.csr` 用于申请证书。实际签名使用签发的 `MyPhoneCSR.cer`、配套私钥库 `MyPhoneKey.p12` 和应用市场发布 Profile `ccBuddy-ReleaseRelease.p7b`。通常在 `~/.config/claude-mobile-remote/harmony-signing/` 找到私钥库，发布证书和 Profile 位于其 `myphone-release/` 子目录。

`HARMONY_SIGNING_CONFIG` 指向仓库外的 MyPhone 发布签名配置。若本机尚未保存此配置，按 [HarmonyOS AppTest 配置格式](harmony-preview-apptest.md#构建) 准备，`storeFile`、`certpath`、`profile` 使用上述材料，`keyAlias` 使用 MyPhone 私钥库的实际别名，密码字段指向本机私有文件。签名配置及材料限制为 0600，密码与密钥留在本机。

```bash
bash scripts/macos/5_harmony_apptest.sh --config "$HARMONY_SIGNING_CONFIG"
```

脚本输出 `.local-tools/harmony-apptest/signed/ClaudeMobileRemote-signed.app`。核对 APP 中内嵌 HAP 的包名、显示版本、构建号和 Release 模式，核对 APP 验签结果、内嵌 Profile 及发布证书。将本轮产物复制到归档目录，命名为 `ccBuddy-<版本>-b<构建号>-harmony-invite-test-signed.app`。

## 4. Android 签名华为推送测试 APK

先将 `JAVA_HOME` 和 `ANDROID_HOME` 指向本机已有 JDK 与 Android SDK。沿用 ccBuddy Android 发布签名和华为推送配置，applicationId 为 `com.tea4go.claudemobileremote.android`。在构建副本的 `apps/mobile/android/app/build.gradle` 设置 `versionName` 和 `versionCode`。

现有 `android_release.py` 在身份校验和文件名中固定了 1.1.1（8）。若本轮使用其他版本或构建号，先在构建副本中同步更新该脚本的 `expected` 版本字段及输出文件名，保留包名、签名、调试状态和通知权限校验。

```bash
bash scripts/macos/4_web_build.sh
(
  cd apps/mobile
  env -u CAP_SERVER_URL CMR_NATIVE_TARGET=android CMR_ANDROID_RELEASE=1 \
    npm exec -- cap sync android
  cd android
  ./gradlew -PcmrPushEnabled=true assembleRelease
)
python3 scripts/macos/android_release.py --push-enabled \
  --config "${CMR_ANDROID_SIGNING_CONFIG:-$HOME/.config/claude-mobile-remote/android-signing/signing.json}" \
  --apk apps/mobile/android/app/build/outputs/apk/release/app-release-unsigned.apk
```

根据脚本输出找到已验签 APK，核对版本、构建号和发布证书指纹，归档为 `ccBuddy-<版本>-b<构建号>-android-huawei-push-signed.apk`。首次配置见 [Android 华为测试发布](android-release.md)。

## 5. 交付与失败恢复

- 回报更新列表、三端版本和构建号、TestFlight 接收状态、两个本地签名包的路径及 SHA-256。
- 某端失败时停在当前步骤，保留此前已验证的产物和状态。从失败步骤续跑。iOS 已上传的构建在续跑时直接复用上传结果。若修复改变了源码，重新确认版本、更新列表和受影响的构建号。
- 本轮到 TestFlight 上传和两端本地签名包交付为止。AppTest 邀请、Android 后台上传及测试组管理由发包人另行处理。
