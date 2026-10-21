# 移动端附件补全验证（2026-10-21）

## 实际完成

- 相机、照片、文件选择器分离；图片预览，追加、移除及清空。
- 单附件与文字组合发送；PC确认受理后清空，拒绝、超时、断线保留当前页面草稿且不重发。离开详情会卸载草稿，不承诺跨页面/重启保留。
- 4项/每项4 MiB/总8 MiB限制，服务端再次验证名称和base64；共享命令不含平台依赖。
- HEIC/HEIF与超限照片尝试缩放转JPEG（最长边2048、质量0.85，原图20 MiB上限）；普通文件保持原字节。
- PC临时目录 cmr-attachments 内随机目录/文件，0700/0600权限；拒绝清理，受理后保留供原会话读取，7天后下次上传清理，容量128 MiB。

## 已执行

- `GOFLAGS=-buildvcs=false go test ./...` 与 `go vet ./...` 通过；真实Claude的环境依赖测试不因此视为已验证。
- transport附件定向race通过，包含超过旧1 MiB预算的真实WebSocket传输、文件字节一致、未鉴权/跨Session拒绝、缓存权限、拒绝清理、过期清理和非法数据/大小限制。
- 移动端27文件486/486测试通过，含选择/移除/编码、PC受理/拒绝/断线、照片转换与释放资源。照片转换采用模拟解码器，不等于真机HEIC通过。
- TypeScript/build通过；lint仅既有cdp-console.mjs警告；build仅既有chunk提示。
- 最终cap sync ios和iOS Simulator xcodebuild通过，产物 /tmp/cmr-attachments-ios/Build/Products/Debug-iphonesimulator/App.app。本轮未重新安装模拟器或真机。
- WhaleTerm实际Registry/Service/Controller/W1/RemotePairingResponse测试通过，完整 `wails build -m -nosyncgomod -nopackage`通过，产物 /Users/zhangym/code/work/whale-term/build/bin/WhaleTerm。首轮误用宽泛Remote匹配触发既有无关TestGetRemoteConn初始化panic；已收窄实际测试名。
- 两仓库diff检查通过，未执行Git写操作。Android/Harmony原生工程未同步。

## 用户验收

1. 启动新构建的WhaleTerm，使用更新后的iOS工程配对；旧PC服务不支持新命令，需要重建/重启。
2. 分别拍照、选相册JPEG/HEIC、选择PDF/文本，确认预览/移除/仅附件与文字组合发送；由Claude描述图片或读取文件，按原权限处理工具审批。
3. 取消选择、拒绝相机权限、超限文件和转换失败有明确反馈；恢复权限后再试。
4. 上传时断网或队列拒绝，草稿不丢失、不自动重发；结果未知先检查会话避免重复。切换会话不串线。

实际相机、系统授权、WebView HEIC解码、VPN大附件与Claude读取结果均未在本轮操作，不标记门禁通过。附件是PC文件读取工具输入，不是直接SDK视觉content block。
