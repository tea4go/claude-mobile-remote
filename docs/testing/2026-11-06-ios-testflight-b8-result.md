# iOS TestFlight 1.0(8) 发布记录

- 日期：2026-11-06；用户明确要求编译并发布到 TestFlight，浏览器已登录。
- 后台基线：App Store Connect 中 1.0(7) 已在 Public Beta 正在测试；本轮递增为 1.0(8)，沿用既有应用、Team 和测试组，不修改公开链接或人数上限。
- 源码基础：`faf9f31` 的 iOS 验收修复；打包时另修正 `SelectableMessage.test.tsx` 的测试辅助函数目标类型，从 `EventTarget` 收窄至实际使用的 `Element | Document`，不改变运行时行为。本轮未执行 Git 写操作。

## 构建与本地校验

- 首次必要 Web 构建被上述 TypeScript 类型错误阻塞，修正后生产构建通过；仍有既有大包提示。遵守用户要求，未运行 npm lint/test；本次明确打包请求包含必需的生产构建。
- 已执行 cap copy ios，仅同步新构建资源，不执行 Android 同步或无必要的 SPM 重写。
- Xcode 26.2 Release / generic iOS 归档成功；构建号由命令行指定为 8，不修改工程默认构建号。
- App Store 导出成功；Bundle ID `com.tea4go.claudemobileremote`、版本 `1.0`、构建号 `8` 及 Team 与既有应用一致。
- App Store 分发 Profile、codesign 严格校验、无开发 server.url、内置全部 Web 文件与 dist 逐字节一致的检查通过。归档日志有 1 条 warning、无 error，导出无 warning/error；未重新运行自动化测试或真机交互验收。
- IPA：`apps/mobile/ios/DerivedData/testflight-20261106-b8/export/App.ipa`，2,958,917 字节。
- SHA-256：`10781419b3ed1cebf896ab5c40a81c159260839c53109f7ea51366b1fcdd2de2`。
- 归档/导出/上传日志与产物保留在该独立 DerivedData 目录，不入库，不覆盖旧产物或上传旧包。

## Apple 与测试组状态

- 已通过既有发布脚本使用 Xcode 账号上传；17:41:07 日志显示 Upload succeeded / Uploaded App / EXPORT SUCCEEDED，命令返回 0，Apple 已接收并开始处理。上传成功不能替代处理、外测审核或分发完成。
- 后台空白期间已尝试刷新和独立标签；用户协助刷新后页面恢复。iOS 列表已显示构建 8，状态“准备提交”，90 天后过期；说明 Apple 处理完成，尚未关联群组或开始外测。
- 构建详情：<https://appstoreconnect.apple.com/teams/10fc6e67-2c5f-4450-b831-4054e2f0f8d0/apps/6827042755/testflight/ios/d9d5dc60-ed4c-48aa-9de2-2778e25e140c>。
- 现有 Public Beta 测试组公开邀请链接：<https://testflight.apple.com/join/uCHWhPEK>；本轮不创建新群组、不增加人数上限。
- 后续：核实上传结果与 Apple 处理状态，填写本轮测试重点并关联构建；如需 Beta 审核则准确记录审核状态，不提前标记测试员可安装。

## 本轮测试重点（待填写后台）

1. 消息长按全选、拖动选区复制及一次轻触觉反馈，修复选区反复切换和菜单冲突。
2. 优化步骤抽屉、状态标签、会话标题和列表时间显示。
3. 修复附件删除、批量清空、全屏预览及权限/附件菜单布局。
4. 优化目录下新建会话的来源选择。

请重点验证长文本、代码/表格的选区与滚动，以及切换页面、输入区聚焦或进入后台后的选区和菜单清理。真实会话仍需手机与电脑网络互通；可在配对或设置页进入离线演示体验基本交互。
