# 审核演示服务交付验证

## 本机完成

- 独立review-server只创建interactive FakeHost，真实Controller/Channel/Transport；模拟PC名称和合成项目标识，不调用真实Claude。无新wire或移动App改动。
- 回环监听/WSS地址与有效期校验，7天默认、最长30天；私有目录0700，配对JSON/PNG及恢复文件0600，不打印凭证或输入。停止关闭连接，重启恢复未过期凭证；会话消息不恢复。
- 新测试覆盖非回环/明文/查询凭证地址拒绝、真实WS认证/订阅/快照、重启凭证一致及文件权限。
- 首次测试因沙箱禁止监听回环失败；获准后cmd/review-server、internal/claude、internal/pairing、internal/transport全套race通过。假Host原有测试覆盖审批/问答/取消，未重复写同逻辑测试。go vet及shell语法检查通过，git diff --check通过。
- Linux amd64与arm64 CGO关闭交叉构建成功，file确认对应静态ELF。包内容仅服务、配置、说明和SHA256，无运行凭证/视频/签名材料。
- 产物.local-tools/review-dist-20261026/cmr-review-linux-amd64.tar.gz及arm64对应包；校验清单同目录。构建脚本scripts/macos/6_review_server_build.sh可重复生成新输出目录。

## 待服务器/真机验证

- 用户自行部署；未上传文件、未改服务器或Nginx。用户提供的域名路径在网页读取工具中不可访问，不能声称公网可达。
- WSS必须由域名实际TLS网关映射/verfiles/review/ws到127.0.0.1:18765/ws；静态/home/share/verfiles仅用于放无凭证的下载包。用户需确认网关与该主机的实际路由。
- 待部署后从移动数据扫码、审批/问答/取消、重连验收；本机Mac测试不替代Ubuntu执行和公网TLS核验。
- 此模拟Host不支持真实Claude回复或附件理解，审核材料必须披露；英文说明仍为待部署验证的草案，不直接提交Apple。
- 未提交Beta审核、未启用TestFlight邀请、未执行Git写操作。
