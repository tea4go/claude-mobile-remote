# Android 华为测试发布

Android应用显示名称：CodeBuddy，applicationId `com.tea4go.claudemobileremote.android`，华为APP ID `119552119`。Java namespace保持`com.tea4go.claudemobileremote`；iOS和Harmony身份独立。本次华为推送构建版本1.0.3（4）；此前首次发布版本为1.0.0（1）。

默认 Release 脚本关闭系统通知。华为推送包需显式以 `cmrPushEnabled=true` 构建并使用签名脚本的 `--push-enabled` 选项；厂商推送仍需真机验收。

macOS首次准备独立密钥（已有目录时会拒绝，不能覆盖或用来轮换密钥）：

```sh
JAVA_HOME="$HOME/jdk" python3 scripts/macos/android_release.py --init-signing
```

私有目录`~/.config/claude-mobile-remote/android-signing/`包含JKS、0600的signing.json、公开证书和同磁盘local-backup。另行备份至可信离线介质；同磁盘副本不能抵御整机损坏。不能将这些文件或密码入库、写日志。

```sh
JAVA_HOME="$HOME/jdk" bash scripts/macos/4_android_build.sh --release
```

脚本执行Web测试/lint/build、清除live reload地址、只同步Android、关闭WebView调试、Release构建、16KiB zipalign、apksigner签名/验签及包内身份检查。使用既有私有配置，不隐式生成密钥。自定义配置使用`CMR_ANDROID_SIGNING_CONFIG=/仓库外/signing.json`。默认无推送APK输出至`.local-tools/android-release/WhaleRemoteControl-1.0.3-android-signed.apk`；本次华为推送签名包输出至`.local-tools/android-release/WhaleRemoteControl-1.0.3-huawei-push-signed.apk`。Windows现有Release命令仍只生成未签名APK，不能直接上传，需另行执行正式签名。

AGC按D106使用方式二：依据目标Android签名页当前公钥，用Google官方PEPK本地加密JKS，上传生成的ZIP。仅上传加密副本，不上传原始JKS或密码；本次未单设上传密钥，应用签名密钥兼作上传密钥。务必核对AGC签名指纹与本地公开证书一致。APK上传成功、测试提交和审核通过分别核实。

官方依据：[华为应用签名方式二](https://developer.huawei.com/consumer/en/doc/appgallery-connect-guides/agc-appsigning-releasedapp-0000001051818319)、[Android应用签名](https://developer.android.com/studio/publish/app-signing)。

2026-10-30已完成方式二托管，AGC证书指纹与APK一致。此前由AI保存测试草稿，Android当时后台要求指定华为账号，未见公开加入入口，不能与HarmonyOS后台混用。用户最新反馈已完成APK发布配置、正在等待审核；最终分发类型和审核状态未由AI重新核实，后续等待审核结果。详见`testing/2026-10-30-android-agc-draft-result.md`。
