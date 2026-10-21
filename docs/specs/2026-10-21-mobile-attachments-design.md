# 移动端附件发送

用户要求补全图片、拍照和文件入口，沿用无需中间确认的实施授权。

## 契约

增加独立命令 send_attachments，payload 为 text 与 attachments 数组；每项 name、mediaType、data（标准 base64）。send_text 不变；旧服务拒绝未知命令时保留草稿。最多 4 项、每项 4 MiB、总计 8 MiB，文本上限仍 1 MiB。服务端再次验证文件名、格式和大小。二进制不进入事件、日志或通知。

只有已鉴权且订阅所属 Session 可发送。PC 使用系统临时目录内的私有缓存，随机请求目录与文件名，目录 0700、文件 0600。写入后把路径随文字经现有 SubmitText 和 FIFO 交给唯一 Host，Claude 使用现有读取工具和权限读取图片、PDF 等。不是直接注入视觉 content block。受理失败立即删除该次文件；关闭 Remote 不删除已受理附件，以免影响运行中的 Turn。缓存满 7 天在下次上传时清理，总容量上限 128 MiB；不作为消息历史。

## 三端核对

JSON、base64 和 WebSocket 无平台依赖，React/ArkTS 可独立实现。三个选择器分离；iOS 使用系统文件选择器与 capture，已有相机权限说明。Android/Harmony 原生体验待 iOS 验收后同步。公共 Input 与宿主 SDK 契约不变，WhaleTerm 和 Standalone 复用同一传输实现，PC 必须重新构建。旧服务拒绝时保留草稿；断线结果未知，不自动重发。

## 界面

照片仅选图片，文件独立选择；支持追加、单项移除、清空与图片预览。仅附件也可发送；服务端受理后清空，拒绝或断线保留。预览资源在移除或卸载释放。

照片补充：HEIC/HEIF 和超过 4 MiB 的图片在客户端尝试转换 JPEG，最长边缩至 2048、质量 0.85；原始图片超过 20 MiB 拒绝。转换失败明确提示导出 JPEG/PNG 后重试。普通受限文件保持原字节。转换依赖系统 WebView 解码，实际 HEIC 和拍照需真机验证。
