# HarmonyOS 真机测试问题记录

### 背景

harmonyOS7版本，PuraX机器测试

### 修复与验收记录（2026-11-11）

- 步骤详情抽屉右上角关闭图标改为居中矢量图形；Debug 包构建、签名并覆盖安装至 Pura X，用户回复“OK”，该关闭按钮验收通过。
- 本轮换行处理与其他步骤抽屉样式仍待逐项真机验收；问题记录3中尚未完成的项目见 HANDOFF.md。配对失效按用户要求忽略。
- 本地截图用于问题对照，不作为会话内容提交。

### 问题列表

1. harmony真机UI和设计稿差异较大，需要对齐，目前iOS/android已经和设计稿对齐了。
   harmony截屏：docs/testing/images/11-11-harmony_screenshot
   设计稿：docs/testing/images/11-03-2

2. 切屏不自然，没有任何动画平移过渡效果，且 返回按钮不统一，需要固定在左上方

### 问题记录2

1. 切换界面动画效果改得过于炫酷了，可以模仿harmony的系统设置，平滑切换即可;返回按钮样式不对，"<"没有垂直居中
2. 进入sessionDetail详情后，滚动条没有自动滚到最底部
3. sessionDetail内的askuserquestion，claudeTool等抽屉，应当从界面最底部弹出，遮罩层应当覆盖整个界面屏幕，现在只覆盖了内容区域
4. sessionDetail内的执行步骤 抽屉打开后，左侧没有step步骤条样式；只能点击工具标题进入详情；没有进入详情的图标和从详情返回工具的图标
5. sessionDetail内的claude回复，有的是空内容，只有一个空外壳，这是为何？并且claude回复的图标样式不对
6. sessionDetail内的用户消息 复制有问题，长按无法复制；claude回复 若是普通的markdown文本 复制则没问题，如果是askuserQuestion这样的面板，长按复制则无反应
7. sessionDetail内markdown渲染有问题，没识别出换行，文字全部挤在一起了

### 问题记录3

1. 界面切换的动画还是不对，我感觉是方向反了；而且 从设置界面进入里面的PC管理，动画也没了；
2. sessionList的目录图标不对，它的展开/收缩 均有各自的图标，请和iOS对齐
3. sessionList的列表选中的卡片样式不对，请和iOS对齐
4. sessionDetail内的 用户提问和claude回复 都带了一个"复制"，这个不需要，删除即可；claude回复的结果状态，放在该回复的最下方；
5. "问题记录2"内的6 验证不通过，并且 claude回复长按全选后，点击claude回复面板内的空白区域无法取消选中，但是点击整个界面的空白区域可以取消选中
6. sessionDetail内的 工具栏弹出层的 stepItem间隔太大了；步骤顶部缺少内容，样式不对，请和iOS对齐。问题截图：docs/testing/images/11-11-harmony_screenshot/session_detail_steps.jpg，docs/testing/images/11-11-harmony_screenshot/session_step_title.jpg
7. sessionDetail内的 askuserquestion抽屉，把 "复制标题"去掉；只能点击每一项的checkbox/radio才能选中，点文本不行；checkbox样式和iOS差异很大
8. sessionDetail头部的连接状态，文字提示和iOS需要保持一致，而不是"配对失败"这种说明；多余的说明不需要，否则头部会占用很大的空间，把中间内容挤没了,问题截图：docs/testing/images/11-11-harmony_screenshot/session_detail_title.jpg
9. "问题记录2"内的7 验证不通过，且iOS也有同样的问题，session_id=af88cb86-c62e-4f7a-a3f5-78ed822207c3 这个会话的用户提问在PC的whale-term内渲染换行正常，同步到手机后则换行失效。建议优先修复iOS和android，再修复harmony
10. sessionDetail内 内容区域的右侧滚动条应该在界面的最右侧，目前是和界面右侧还有段距离，不美观；steps工具步骤的滚动条也是一样的问题；请检查 sessionDetail内抽屉的滚动条，要求贴边
11. 我发现harmony的配对码很快就失效了，下次使用，需要重新扫码配对，检查一下，需要和iOS对齐
12. sessionDetail的图片上传 预览区域应当 左对齐
13. 我发现harmony所有的界面 顶部和底部都没有贴边，是因为界面的背景色的原因吗？问题截图：docs/testing/images/11-11-harmony_screenshot/session_preview.jpg
