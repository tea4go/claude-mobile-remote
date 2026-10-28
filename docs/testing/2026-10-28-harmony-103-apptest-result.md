# Harmony 1.0.3 签名包验证

- 用户授权：重建签名版1.0.3，由用户上传。
- 版本：1.0.3 / 1000003，包名com.tea4go.claudemobileremote，Release / API24。
- 构建：沿用仓库外发布签名，使用HVIGOR_USER_HOME=.local-tools/hvigor-icon-cache运行5_harmony_apptest.sh，退出0。首次空缓存构建停在依赖下载，已终止并改用既有缓存。
- verify-profile、verify-app、证书/Profile一致性与APP/HAP ZIP校验通过。
- 包内module.json和pack.info版本均核对。应用及Ability沿用layered_image，分层JSON前后景资源引用存在。
- 源前后景1024×1024；包内两层512×512，符合工具链图标编译行为，参见[华为官方FAQ](https://developer.huawei.com/consumer/cn/doc/doccenter-dev-faq/faqs-arkui-1149)。包内背景全像素不透明白色，前景有透明及可见像素，解包视觉核对为新蓝色无网格主体；启动图1024px像素与新源完全一致。编译后图像不采用文件摘要相等作为校验。
- 产物：`.local-tools/harmony-apptest/signed/ClaudeMobileRemote-1.0.3-signed.app`，229024字节。
- SHA256：`b5401b5c56648a5b45ec8f5710c6b45a4fb2662f47e4dc769732d2f6bc436dc5`。
- diff检查通过。未执行Git写操作、未上传、未真机安装，华为自检/审核仍待用户反馈。
