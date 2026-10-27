# E3 edo 通知服务部署检查点

PC续办结果：实际WhaleTerm replace与EnablePush核实，当前macOS账号私有配置0600已写，仅URL/独立bearer；公共Go组件加载与本机公网鉴权空JSON400通过。WhaleTerm backend/remote测试、主程序临时路径编译通过，模块元数据确认本地replace。未替换运行App，临时编译不等同Wails分发包。iOS本地AppKey配置完成，现有开发/Store Profile均缺aps-environment；Apple App ID页面当前空白，未更改后台能力。未发送真实通知，无Git写操作。

2026-10-27，用户明确授权 SSH 部署并确认路径与内部桥接；用户确认尚未取得极光凭据。当前为已安装、待配置启用，不能称已上线或已送达。

## 实际部署

- 主机：用户指定 edo（SSH 6443），Linux amd64；Nginx 容器 whale_term_nginx，现有 8118 转发保持不变。未新增公网端口/转发。
- 二进制 `/opt/cmr-notify/notify-server`，SHA256 `a2352d59b703fad764a189bb9574d3a1273b47c041ee7e620d8ed1d1da633803`；归属专用系统用户 cmr-notify 的服务。
- `/etc/cmr-notify/notify.env` 为 root:root 0600，父目录0700；已在服务器生成随机独立 bearer，未输出或复制至文档。AppKey/Master Secret仍为 REPLACE_ME，production=true。
- systemd：cmr-notify.service监听127.0.0.1:18767；cmr-notify-bridge.socket/service沿用既有socket-proxyd方式，从Docker桥172.17.0.1:18768接至loopback。两者inactive/disabled，没有新增监听。
- `/etc/nginx_docker/conf.d/fileserver.conf` 仅增加一条server内include，引用 `/etc/nginx/cmr-notify-locations.conf`（宿主文件 `/etc/nginx_docker/cmr-notify-locations.conf`）。该文件含 `/verfiles/notify/v1/events`、`/notify/v1/events` 两条精确路由，兼容公网剥前缀；代理目标为172.17.0.1:18768/v1/events，当前显式return503，待凭据齐备后移除。
- 原配置备份 `/etc/cmr-notify/backups/20261027T071645Z/fileserver.conf`。旧WS配置未改。

## 实测

- 上传tar及包内SHA256均通过，安装二进制hash一致。GNU tar忽略macOS provenance扩展属性，不影响内容校验。
- systemd-analyze verify无新增单元错误，仅输出既有cloudmonitor配置警告。
- Docker nginx -t及reload成功；配置diff仅新增include。
- 本机8118两条新路由503；公网 `https://edo.iwhalecloud.com/verfiles/notify/v1/events` GET/POST均503。
- 公网 `/verfiles/` 部署前后均200，响应体185字节且逐字节相同。
- 进程与bridge均inactive/disabled，18767/18768没有监听。

## 后续启用

由账号持有人准备极光应用AppKey/Master Secret及对应iOS Bundle ID的APNs配置。通过SSH执行 `sudoedit /etc/cmr-notify/notify.env` 填写，保留已生成token；不把密钥发至聊天或公开下载目录。Release/TestFlight为production=true，开发签名需独立sandbox配置。

凭据完成后由AI继续：验证私有配置非占位符；启动并启用cmr-notify.service及cmr-notify-bridge.socket；验证loopback healthz204与桥接；备份通知location文件，移除两条return503；nginx -t后reload；公网无凭据POST应401、GET应405。失败则恢复维护响应并记录。未经认证接口测试不得触发真实推送。

PC还需私下配置URL与同一bearer，并重建引用新公共组件的WhaleTerm；iOS仍需Push能力/profile和AppKey的新构建。真实后台/退出通知、点击、多Session和禁用/撤销尚未验收。无Git写操作。

## 启用结果（2026-10-27 15:44 CST）

用户提供极光凭据并明确授权继续配置。已直接通过SSH写入服务器root0600环境文件，保留既有bearer与production=true；未写入项目或公开目录。

- cmr-notify.service与cmr-notify-bridge.socket已启动并开机启用；loopback及桥接healthz均204。首次即时健康探测因进程尚未就绪拒绝连接；复查active且204后才修改Nginx。
- 维护配置备份 `/etc/cmr-notify/backups/20261027T074353Z-activation/locations.conf`；移除两条return503，nginx -t/reload通过。
- 公网通知GET405、无鉴权POST401；服务器私有token经公网提交无效空JSON返回400，证明鉴权头透传与载荷校验生效，未调用极光发送通知。
- 原公网下载入口仍200，与部署前响应体完全一致。
- 极光后台APNs已上传（用户授权），Bundle ID和Sandbox & Production核对正确。尚未验证极光REST实际调用、APNs真实送达、App注册及通知点击；PC私有配置和新组件版本、iOS新签名构建仍待完成。

本节替代上文停用/503的历史状态。当前服务接口已启用，不等价于完整离线通知闭环验收通过。无Git写操作。
