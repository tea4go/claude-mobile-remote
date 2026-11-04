# 移动端 UI-2/UI-3 验证记录

- React 详情、输入、问答、步骤入口与抽屉已按新版设计实施；沿用既有会话数据和操作。
- 前端全量 39 文件 574 项通过；随后新增调试面板与步骤抽屉交互测试，定向 3 项通过。lint、生产构建通过；有既有脚本 warning 和大 chunk 提示。
- Capacitor iOS 资源同步及 Xcode Debug generic iOS 无签名构建通过。未安装到设备。
- 尚未进行 iPhone 视觉、键盘、附件、问答、步骤与生命周期人工验收；Android 平台验收和 Harmony ArkUI 同步后续进行。不能将自动化结果视为真机验收。

### 验收结果

1. 所有的界面顶部都应当固定，内容区域滚动，目前"设置"界面需要修改

2. 界面顶部的"返回"图标太小了，建议直接放弃使用该svg，换一个吧

3. askUserQuestion抽屉面板里，多选的checkbox和单选的需要区分，前者应该是方形选择框
   docs/testing/images/11-04/askuserquestion.png

4. 录音进行时，它的动效需要改一下，参考docs/testing/images/11-04/microphone_ing.png，外圈转动。

5. 录音进行时，录音图标和发送图标快重叠了，并且发送图标太大了，最好和其他的保持一致
   docs/testing/images/11-04/microphone.png

6. session详情面板里，claude回复方的图标有背景；claude回复区域的背景和界面整体背景不一致；展示用户选择的答案 tag应当有圆角。请参考设计稿
   docs/testing/images/11-04/session_detail.png

7. 主页的列表里，点击目录行 收缩/展开 下面的session列表

8. 主页的列表里，点击上方的 PC区域，打开抽屉显示目前已配对过的PC机器列表，选择可切换PC下的会话列表 + 展示全部，是一个筛选操作；点击下方的"新会话"，配对后，如果新增的配对会话不在当前筛选的PC机器里，则直接默认切换到此会话所在的PC。

9. 会话列表内，如果目录下的第一个session是选中状态，它的上边框消失了；sess_item右侧的 更多 action图标太小了，手指很难触碰到；session_item右下方的时间应当和右上方的action图标垂直对齐；目录项右侧的plus 图标，使用primary颜色。请参考设计稿。
   docs/testing/images/11-04/session_list.png

10. 配对界面内，"粘贴 Service 输出的配对 JSON 或完整 #pair Fragment"这样的说明tip不要换行，我测试的手机是iphone17 promax大屏幕了，还是换行了，请注意兼容。
    docs/testing/images/11-04/paring.png

11. 设置 界面与设计稿差异较大：1. item右侧的箭头图标 与设计稿不一致 2. 调试日志 item左侧不需要图标；左右应当顶部对齐；3. 会话通知 和设计稿不一致
    docs/testing/images/11-04/setting.png

### 反馈修复记录（2026-11-04）

- 最终复验：用户反馈本轮问题已全部测试通过，包括目录首项上边框和加号 primary 配色；该反馈未提供逐项设备或平台明细。Android 平台验收及 Harmony 同步仍待后续。
- 用户随后反馈上述 11 项测试通过；未提供逐项场景或设备细节。Android 平台及 Harmony UI 同步仍待后续。
- 目录定向配对追加实现：React 全量 39 文件 581 项通过，错误电脑、错误目录、字段缺失及重试定向回归通过；lint、前端构建、iOS 资源同步与无签名 Debug 构建通过（既有 lint 和包大小提示）。最后调整 Toast 文案后重新构建前端并同步；该功能尚未真机验收。

- 已按 11 项调整：设置/配对顶部固定及返回图标、问答多选方框、录音旋转外圈与按钮间距、详情底色及头像与答案标签、目录折叠和 PC 筛选抽屉、列表边框与操作触点和时间、配对提示、设置通知区。PC 筛选指定机器时，配对另一台 PC 后切换至新配对所属 PC；选择全部时继续显示全部。
- 目录折叠和跨 PC 配对筛选新增回归；React 全量 39 文件 577 项通过，lint 和生产构建通过（既有脚本 warning 和 chunk 提示）；Capacitor iOS 同步及 Xcode Debug generic iOS 无签名构建通过。首次沙箱构建受缓存权限限制，使用授权的既有缓存后通过。
- 以上为代码及构建验证，尚未在 iPhone 复验 11 项视觉、触摸、录音、键盘或真实多 PC 场景；Android 平台及 Harmony UI 同步仍待 iOS 验收后。
