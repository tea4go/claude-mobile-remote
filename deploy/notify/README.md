# edo 极光通知服务部署

目标：Ubuntu20.04 Linux amd64（另附arm64包）。链路只有HTTPS，不使用WebSocket。2026-10-27 已在 edo 的现有入口部署 D102 RID 服务；以下步骤用于重装或其他环境。所有示例值都必须替换。不要将配置/密钥放到公开下载目录，也不要把真实密钥贴到聊天、日志或Git。

## 1. 准备极光与Apple

- 极光创建/选择与App一致的iOS应用，Bundle ID目前为 `com.tea4go.claudemobileremote`，取得AppKey/Master Secret。
- Apple Developer的对应App ID启用Push Notifications，刷新开发/分发Provisioning Profile；在极光后台配置该Bundle ID的APNs密钥或证书。证书/私钥由账号持有人在平台配置，不打进服务包。
- Debug开发签名使用sandbox（CMR_JPUSH_PRODUCTION=false）；Release/TestFlight使用production（true）。不能仅按手机系统判断环境；以实际签名为准。并行测试两种环境应部署两个独立实例、端口及鉴权token。
- 当前极光账号按 tag 推送会返回 2009；D102 改用 Registration ID。手机通过同一路径 PUT/DELETE 登记/撤销匿名 topic 与自身 ID；POST 事件仍只接受 PC 专用 bearer。无需新增公网端口或 Nginx 路由。
- iOS配置：复制 `apps/mobile/ios/App/Push.local.xcconfig.example` 为同目录 `Push.local.xcconfig`，填写AppKey（不是Master Secret），重新构建并安装。SDK依赖本机已导入；其他工作区用 `scripts/macos/ios_import_jpush.sh` 导入同版本noidfa SDK。SDK二进制/本地配置不入库。

## 2. 安装服务

将对应架构包解压到私有临时目录并执行 `sha256sum -c SHA256SUMS`。以管理员身份：

```sh
sudo useradd --system --no-create-home --shell /usr/sbin/nologin cmr-notify
sudo install -d -m 755 /opt/cmr-notify
sudo install -m 755 notify-server /opt/cmr-notify/notify-server
sudo install -d -m 700 /etc/cmr-notify
sudo install -m 600 notify.env.example /etc/cmr-notify/notify.env
sudoedit /etc/cmr-notify/notify.env
```

用 `openssl rand -hex 32` 生成独立token，私下同时配置到edo和可信PC。notify.env还需填写极光AppKey/Master Secret与production值。已有用户不重复useradd。不能使用示例占位符上线。

```sh
sudo install -m 644 cmr-notify.service /etc/systemd/system/cmr-notify.service
sudo systemctl daemon-reload
sudo systemctl enable --now cmr-notify
curl --fail http://127.0.0.1:18767/healthz
sudo systemctl status cmr-notify --no-pager
```

healthz返回204只说明进程启动；不代表已配置APNs或送达手机。接口固定错误，不打印请求/平台正文。密钥应从受控环境文件加载，不写到命令参数。
服务单元通过 `StateDirectory=cmr-notify` 创建私有可写状态目录；RID 映射保存在 `/var/lib/cmr-notify/devices.json`，由服务账号持有且权限0600。升级已有服务时一并替换单元文件并执行 `daemon-reload` 后重启，否则新版本无法写入映射。

## 3. 映射现有HTTPS路径

将nginx-location.conf精确location合入现有HTTPS站点，测试配置后reload，不覆盖原下载服务。外部路径 `/verfiles/notify/v1/events` 转为服务 `/v1/events`。普通未认证POST应返回401，GET该路径返回405；不需要Upgrade头或WebSocket转发。

注意edo既有Nginx可能运行于Docker：容器内127.0.0.1不是宿主机。此时须复用该服务器已批准的容器→宿主桥接方式（原审核服务使用systemd socket-proxyd），仅把这个精确路径指向通知进程；不要直接照搬loopback代理地址，也不要把通知进程改成0.0.0.0。该网络映射需部署时核实实际容器/网桥，模板不自动改网络。

公网HTTPS证书、WAF路径规则需允许 PC 的 POST/Authorization 与 App 的 PUT/DELETE（无 bearer）通过同一精确路径。PC→edo已能HTTPS出站；新接口需分别验证。

## 4. 配置WhaleTerm/Standalone

公共Remote Server已使用新出口；WhaleTerm既有EnablePush=true接线无需另建SDK Client，但必须重新编译引用本次公共组件的WhaleTerm并重启Remote Server。已有运行中的旧二进制不会自动获得新出口。

在运行WhaleTerm的同一系统账号下创建私有 `notifications.json`，内容按notifications.json.example；URL指向edo精确HTTPS路径，token与服务一致。路径：

- macOS：`~/Library/Application Support/claude-mobile-remote/notifications.json`
- Linux：`$XDG_CONFIG_HOME/claude-mobile-remote/notifications.json`，未设置时 `~/.config/claude-mobile-remote/notifications.json`
- Windows：使用下述进程环境变量；本轮JSON私有文件权限检查仅支持Unix，避免未经ACL验证读取凭证。

Unix执行 `chmod 600`，不使用符号链接。也支持环境变量 `CMR_NOTIFY_URL`/`CMR_NOTIFY_TOKEN`，适用于服务进程；macOS从Finder启动的GUI不保证继承shell环境，优先文件配置。旧PC端jpush.json/CMR_JPUSH_MASTER_SECRET不再被宿主默认出口读取；Master Secret迁至edo。

无配置时不发送；格式/权限错误时停用通知并输出固定诊断，不影响会话启动。发送错误/限流只做一次有界重试，不阻塞宿主输入队列，不提供跨重启补发。

## 5. 真机验收

1. 安装含本次修改且APNs签名正确的新iOS构建，在设置启用通知并允许系统授权（旧版本地提醒用户需另点“启用后台通知”），确认“本机远程通知已就绪”。这表示本机 APNs 与 edo RID 登记完成，仍不代表系统通知已送达。
2. 扫码配对并保持有效，手机退到后台或退出App；PC触发真实工具审批/问答。验系统横幅/通知中心，正文只有固定提醒。
3. 点击通知冷启动，进入对应本地配对会话并按LAN/VPN连回PC，PC最新状态为准；测试第二个Session和第二台手机，不串线。
4. 前台看当前Session不弹提醒，其他Session只一条本地提醒；关闭通知后新事件无系统提醒，移除配对后旧点击不能恢复访问。
5. 暂停edo服务时PC的消息/审批/停止仍正常；恢复后只收后续事件。轮换token后旧token返回401；同步更新可信PC并重启Remote Server。

APNs不是必达通道。事件TTL至多5分钟且不晚于配对到期；相同Session通知可能被APNs合并。edo仅内存去重，不持久化正文/设备/Session。服务重启或平台结果未知时仍可能重复；设备离线时无法即时删除极光tag，已入队短TTL提醒仍可能显示，旧点击仍需有效本地配对。关闭开关依赖SDK注销与APNs传播，不撤回平台已交付的历史通知。

更新：替换二进制后 `sudo systemctl restart cmr-notify`；停用：`sudo systemctl disable --now cmr-notify`，移除精确代理location并reload。轮换Master Secret只需更新edo环境文件并重启，无需分发到PC或App。

参考： https://docs.jiguang.cn/jpush/client/iOS/ios_guide_new ，https://docs.jiguang.cn/jpush/server/push/rest_api_v3_push 。
