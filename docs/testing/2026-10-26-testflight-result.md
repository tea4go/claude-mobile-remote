# TestFlight 外部测试发布记录

## 已完成

- 用户检查通过发布设计；组织后台创建 Claude Mobile Remote，Apple ID 6827042755，对应 com.tea4go.claudemobileremote。
- 本机 Release 1.0(2) App Store Connect 归档/导出、严格深度验签、分发 Profile 和内置 Web 资源验证通过；移动端537项测试及发布脚本5项测试通过。二维码脚本已用示例链接完成生成及解码，尚无真实邀请二维码。
- IPA：apps/mobile/ios/DerivedData/testflight-20261026-b2-r2/export/App.ipa；SHA256：86750fdebef33d686cc8121b8d559799712941984e64bbb9dd6524c75802afbb。
- 首次上传因开发者仅可访问其他 App，FetchAppRecord 返回 missingApp；用户完成 App 授权后，因原签名资源权限关闭而失败。按用户明确确认恢复两项原有权限，Xcode 下载描述文件刷新后重试成功。
- 16:32:22 上传日志明确 Upload succeeded / EXPORT SUCCEEDED：/tmp/cmr-testflight-upload-b2-r4.log。上传成功不等于处理完成或外测获批。
- Beta 描述、反馈邮箱和审核联系人在后台显示“已保存”；私人联系方式仅在 Apple 后台。

## 未完成与下一步

- 后台处理结果、加密合规、外测组及首次 Beta 审核尚待核实/完成；公开邀请未启用，尚不能扫码安装。
- 用户确认没有专用外网审核 PC，要求评估审核方案。现有 CLI interactive 假 Host 可展示固定文本、工具审批与问答，使用真实配对和传输链路，不运行 Claude；它并非现成的全功能审核环境。
- 需要形成可复现 PC 安装/配对步骤或独立审核演示环境，并明确向 Apple 披露演示限制。审核专用凭证不得写入本记录或 Git。
- 没有执行 Git 写操作。
