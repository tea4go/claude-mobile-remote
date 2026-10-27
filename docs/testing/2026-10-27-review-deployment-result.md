# 审核环境服务器排查与修复

用户授权SSH修复；未提交Git，未读取私有配对文件。

- 演示服务systemd正常，loopback18765普通请求400。
- 8118实际由Docker whale_term_nginx提供，挂载/etc/nginx_docker；之前/etc/nginx修改不生效。
- 原站点备份fileserver.conf.bak-cmr-20261027094417，保留既有三条代理。
- systemd socket仅172.17.0.1:18766监听，经socket-proxyd转127.0.0.1:18765，容器精确路径转此桥接。
- 公网网关剥离/verfiles前缀：双前缀请求可达；补/review/ws精确映射后原公网地址普通请求400。
- nginx -t/reload通过；本机8118升级握手101。
- 公网相同升级请求400，尚未连通WebSocket；需HTTPS上游检查Upgrade/Connection与HTTP1.1转发，再做手机移动数据认证/交互验收。

当前不提交Apple审核，不把普通400当作WSS连接成功。

## 公网入口补充排查

服务器DNS解析：edo.iwhalecloud.com的别名位于aliyunwaf4.com，IPv4 47.75.234.205；服务器SSH公网为8.210.19.98。本机无443监听，Docker仅一个Nginx，Caddy监听80/8080/2087，完整路由未取得。可确认外部WAF参与公网入口，不能据此断言具体哪层移除了哪个握手头；需管理该域名的阿里云账号核实WAF与后续网关。只读检查，未进一步改远端。
