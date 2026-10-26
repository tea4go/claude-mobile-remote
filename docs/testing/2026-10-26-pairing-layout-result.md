# 调试入口与配对界面优化验证

## 范围

D94：日志默认开启、右侧中下入口，输入与键盘避让；WhaleTerm扫码和连接管理分视图，精简文案，记录倒序、有界滚动。菜单和扫码页摘要均可进入管理。连接数只表示认证订阅连接，不表示配对设备数。现有hello仅凭证，未持有手机元信息；设备识别/设备列表未实施，未变更共用协议。

## 已执行

- 移动端35文件530项测试通过，包含默认开关、关闭入口、视口偏移与键盘/输入避让；生产build通过。lint仅既有scripts/cdp-console.mjs警告。
- WhaleTerm现有配对回归28/28；完整前端生产构建和Wails应用打包通过。既有前端构建体积/弃用警告，无阻断。Wails复用临时modfile引用本地Remote，未改真实依赖清单。
- 两仓git diff --check通过；未执行其他Git操作。
- 仅cap copy ios更新内置资源，没有原生依赖变化，不执行sync；Release archive/export通过。最终IPA内11个Web文件逐字节匹配dist，iOS检查器开启、桥接logging none、无server.url；codesign深度严格验证通过。

## 产物

- IPA：apps/mobile/ios/DerivedData/pairing-layout-20261026/export/App.ipa
- SHA256：74bb3e074f5dffc920d6d9556ae510530a38fac571a3583b76f2cf74fb91b1ad
- PC：/Users/zhangym/code/work/whale-term/build/bin/WhaleTerm.app
- 日志：/tmp/cmr-layout-*

## 待人工验收

1. 新启动App默认可见右侧中下调试入口；弹出键盘不遮挡输入，设置可关闭。
2. 扫码页简洁可扫码，在线摘要和菜单可进入连接管理，两视图可切换。
3. 连接记录最新在上、可滚动；空状态和刷新失败提示可见。

未安装或启动设备/新PC应用，未上传。Android/Harmony原生未同步。此前GUI依赖图错误本轮未恢复验证；命令行归档成功不等同GUI状态已恢复。
