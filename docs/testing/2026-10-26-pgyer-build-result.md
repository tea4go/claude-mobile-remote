# 今日蒲公英安装包构建

- 用户要求先生成Android APK与iOS IPA，继续蒲公英分发。
- Web：35文件537测试通过，lint仅既有cdp-console警告，生产build通过。
- Android：同步当前Web和原生配置，assembleDebug成功；可安装debug签名APK，包名com.tea4go.claudemobileremote，1.0(1)，minSDK26/target36，apksigner v2验证通过。
- iOS：复用已验证1.0(2)归档，以debugging重新导出；开发签名Profile包含4台已登记设备，Team/Bundle匹配，严格深度验签通过。不是TestFlight分发包，蒲公英不解除设备限制。
- 两包全部11个Web资源与最新dist逐文件相同，均无开发server.url。未安装或上传，真机验收待用户。
- APK：apps/mobile/android/app/build/outputs/apk/debug/ClaudeMobileRemote.apk；SHA256 6fbcb6d6788b3bdeff50447a4742b0e3a2d0a4738708873c278a217997ac2ff1。
- IPA：apps/mobile/ios/DerivedData/pgyer-20261026-b2/export/App.ipa；SHA256 75279724e095f2b288319014565e636db289079af9ccf1ee6277f36ef95f07f3。
- 日志/tmp/cmr-pgyer-web.log、cmr-pgyer-android-sync.log、cmr-pgyer-android-build.log、cmr-pgyer-ios.log。首次沙箱Profile解码失败，获准系统签名服务访问后核验通过。无Git写操作。
