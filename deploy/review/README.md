# 简单审核演示（Ubuntu x86_64）

这是模拟服务：不调用Claude、不执行命令。手机使用真实配对、WebSocket、审批和问答流程。所有连入手机共享一个演示Session；只供审核，请勿输入私人内容。不能用它证明真实模型/附件读取能力，也不保证Apple接受。

## 1. 安装与启动

将压缩包上传到服务器（可暂放 /home/share/verfiles，公开包不含凭证），以tony执行：

```sh
mkdir -p /home/tony/cmr-review/private
chmod 700 /home/tony/cmr-review/private
# 将压缩包路径改为实际路径；不要在公开下载目录运行服务
tar -xzf /home/share/verfiles/cmr-review-linux-amd64.tar.gz -C /home/tony/cmr-review
cd /home/tony/cmr-review
sha256sum -c SHA256SUMS
./review-server --data ./private
```

显示 REVIEW_READY 后保持进程运行。仅监听127.0.0.1:18765；首次启动生成私有pairing.json、pairing.png和credential.json。默认7天到期后自动生成新二维码；须重新向审核提供。重启保留凭证，Session消息不持久化，手机重连后以当前快照为准。

长期运行可使用附带systemd文件（与上面的前台进程二选一）：

```sh
sudo cp cmr-review.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now cmr-review
sudo systemctl status cmr-review
```

## 2. HTTPS/WSS代理

在 **edo.iwhalecloud.com实际终止HTTPS的网关** 的现有 `server {}` 中加入 nginx-location.conf 的location。若域名经其他网关转发，网关还需将此路径和WebSocket Upgrade转发到本服务器；仅复制到静态目录不会生效。不要替换现有站点或占用SSH的6443端口。

确认可信HTTPS证书已有配置，然后：

```sh
sudo nginx -t
sudo systemctl reload nginx
curl -i http://127.0.0.1:18765/ws
curl -i https://edo.iwhalecloud.com/verfiles/review/ws
```

普通curl未带WebSocket Upgrade时返回400是正常情况；404/502/证书错误需先修复。以上仅检查路由，最终需手机使用移动数据扫码、发送消息及审批成功。不要关闭App的证书验证。

## 3. 获取二维码（在Mac执行）

```sh
scp -P 6443 tony@8.210.19.98:/home/tony/cmr-review/private/pairing.png ~/Downloads/review-pairing.png
```

将PNG显示在另一块屏幕上，用当前iOS App扫码。它是会话访问凭证，不是TestFlight安装二维码。**不放到/home/share/verfiles、Git、日志或公共网页**。准备正式Apple资料时私下提交该专用演示二维码及review-notes.md中的已验证步骤。

## 4. 停止与撤销

```sh
sudo systemctl stop cmr-review
```

前台运行则Ctrl+C。停止立即断开连接。需要更换凭证时，先停止，再移除private下credential.json、pairing.json和pairing.png，重新启动并取得新二维码。旧凭证不再可用。审核结束停止服务、移除专用Nginx location并检查配置后reload。

## 交付验证范围

包中二进制为Linux静态构建，无Claude账号/配置、私钥或预生成凭证。构建时本机测试与服务器公网验收分别记录；上传成功并不等于演示服务已部署。域名网关、TLS和手机验证由部署后执行。
