# Harmony Mac 预览与 AppTest 准备验证

日期：2026-10-23。无Git写操作，不代表原生设备门禁通过。

## 实际完成

- SDK6.1.1/API24、原生Previewer引擎，隔离工程编译模拟界面；无完整模拟器。
- `bash scripts/macos/3_harmony_preview.sh --build-only` 退出0，实际PreviewBuild成功并生成modules.abc。因Hvigor预览worker会常驻，脚本检测首次编译成功后仅清理本次编译进程组，随后启动独立引擎。
- 原生引擎与本机命令宿主连接成功。最初规范化模块寻址和缺失loader配置导致白屏；仅在副本中适配并传入loader.json、完整帧刷新后解决。
- 通过Chrome原生电脑控制实际看到会话列表，点击进入详情看到审批抽屉，点击“本次允许”后抽屉关闭、显示模拟操作完成及Markdown内容；返回列表并进入设置看到PC管理和提醒/缓存入口。以上操作使用虚构数据，不连接真实PC、不执行实际审批。
- 修正预览容器分配，顶部模拟提示不占用子页面的100%高度；修正后预览编译退出0。暂不支持键盘输入转发、热更新或断点调试。
- 初次预览编译因外接盘迁移AppleDouble文件失败；核验`00051607`头及对应原文件后，依次清理组件目录149项、ets-loader子树13851项附属文件，未改真实SDK资源；本轮未清理其他SDK目录。

## 构建与签名准备

- `bash scripts/macos/5_harmony_apptest.sh --unsigned` 实际Release assembleApp成功，最终退出0。
- 产物：`.local-tools/harmony-apptest/unsigned/ClaudeMobileRemote-unsigned.app`。
- 最后一次SHA256：`eff8fd9a25669a4cbb9ff1763198ffa7f2ea5648a3458b3f102c1491ae8bace9`；临时工程路径不同，构建摘要可能变化，不宣称可复现构建。
- ZIP检查、pack.info与HAP存在、内嵌HAP包名、Release模式/非debug及最低API60101024检查通过。该包未签名，不能上传分发。
- 发布入口在仓库外一次性私有工程生成签名配置，密码从权限受限的外部文件读取、工具输出脱敏、进程退出清理临时工程。签名模式实现Profile类型/包名/渠道限制及内嵌HAP签名核验；因无发布证书/Profile，成功路径未实际执行。
- 已执行发布材料初始化：仓库外`~/.config/claude-mobile-remote/harmony-signing/release`，目录0700、文件0600；EC secp256r1 / SHA256withECDSA，别名cmr-release，随机密码不回显；CSR自签名核验通过。现有Debug材料未覆盖。
- 签名配置缺少release.cer时实际拒绝构建，不回退未签名包。
- 用户截图确认APP ID `6917619813829911575`、包名`com.tea4go.claudemobileremote`、名称Claude Mobile Remote。后台声明手机/平板，当前工程只声明phone。

## 自动检查与限制

- Harmony测试42/42、14逻辑文件严格检查通过。
- Shell语法、Python语法、帮助入口和git diff --check通过。
- 初次隔离构建因用户级Hvigor缓存写权限受限失败，获自动审批后真实构建成功。
- 尚未取得发布.cer/.p7b，未验证签名APP、后台上传/AppTest开通与创建任务、同事安装及扫码/语音/通知等原生能力。不能标记AppTest发布完成。

下一步：在华为后台使用release.csr申请发布证书，下载应用市场发布Profile；接收后验证并执行签名构建，再核对AppTest测试入口和实际提交要求。

## 发布材料与签名构建续验（2026-10-23）

- 用户提供的发布证书三张PEM链中叶证书匹配既有Release CSR，当前有效，链内签名通过；Profile CMS签名通过，类型`release`、渠道`app_gallery`、包名`com.tea4go.claudemobileremote`、APP ID`6917619813829911575`，绑定同一发布证书。未独立验证华为根信任。材料原样保存于仓库外私有目录，权限0600，原下载文件保留。
- 初次签名构建因Hvigor要求加密签名密码失败；修复脚本在一次性工程生成加密材料后通过配置校验。现有JDK21.0.0对有效ZIP64元数据报错，签名子进程加`jdk.util.zip.disableZip64ExtraFieldValidation=true`后通过；脚本独立进行ZIP结构检查。
- 常规APP自带发布签名，包内HAP默认未签名；原逐个验证内嵌HAP的假设已按华为构建配置说明修正为验证APP本身。华为`verify-profile`及`verify-app`成功，APP内Profile字节一致、APP签名证书匹配指定发布证书。
- 最终命令`bash scripts/macos/5_harmony_apptest.sh --config <仓库外signing.json>`退出0。APP ZIP及内嵌HAP ZIP检查通过，包名、Release模式、最低API60101024、versionCode1000000/versionName1.0.0均核对。产物`.local-tools/harmony-apptest/signed/ClaudeMobileRemote-signed.app`，SHA256=`b0cc8f8657749f2936af7c48c34a2b4a7f32348a36865dcfd250764c4aa0a484`。
- Chrome开发者后台可见当前应用“应用测试→测试版本列表”入口；点击后内容区未加载，未完成后台上传、AppTest邀请任务或设备安装，不能据此宣称分发与原生功能门禁通过。

## 后台上传续验（2026-10-23，用户操作与截图）

- 用户已上传上述签名APP。版本选取截图中的软件包摘要与本机SHA-256一致，软件包合法性显示“已达标”、用途为“仅测试”；该行同时显示“不可选”，当时上架自检为“未检测”。截图未显示不可选的确切原因，不能将其归因于某一项检查。
- 用户随后启动“上架自检”，最新反馈仍为“检测中”；尚无完成时间或结果证据。邀请测试任务、下载链接和Harmony真机安装均未完成。
- 蒲公英仅作为待决备选：其现行Harmony文档要求已签名HAP、包含设备UDID的“指定设备发布”Profile，并向平台提供签名P12及密码以签署安装元数据；现有AppGallery发布APP及app_gallery Profile不能直接复用。未生成该渠道产物、未上传私钥，也未切换分发决策。
