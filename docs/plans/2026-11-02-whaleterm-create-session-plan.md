# WhaleTerm 同目录远程新建会话实施计划

- 日期：2026-11-02；最高优先级，归属已由用户确认。
- 设计：../specs/2026-11-02-whaleterm-create-session-design.md

1. 扩展公共 remote.Server 可选 SessionCreator，保持单 Session、旧宿主和能力轮换兼容，补回归。
2. WhaleTerm 创建事务：完整配置种子、规范身份、固定 cwd、独立 Server、串行/幂等/上限/限流/回滚，补 Go 测试。
3. 订阅成功回调把目录入口的预留会话正式加入 PC 列表；手机 + 成功响应直接同步。PC 本地列表合并首轮前新会话、空详情及无敏感数据刷新事件。
4. Claude 列表目录操作新增“启动远程 CLI”，独立展示新二维码；复用手机 +。
5. gofmt、公共 Go race/vet/build、WhaleTerm 定向测试、前端回归/构建及 diff --check；记录实际通过/未验和人工清单。不执行 Git 写操作或发布。

2026-11-02 复用审查：用户要求仔细核对已有能力，禁止重复实现。已核对现有 Registry 端口递增、SDK 自定义 ID/FIFO、配对/二维码、三端 +、PC 列表/详情及 W1 实时监听；删除本轮重复的端口分配器和未使用的 Host 工厂注入。新增创建路径的测试改走真实 WebSocket hello/subscribe/create_session，覆盖既有端口分配与 PC 元数据同步。复用边界见设计表格；测试结果见同日 testing 记录。

2026-11-02 用户验收修订：目录扫码及手机 + 新建已由用户复测通过；新建完成后 PC 需复用既有 selectClaudeLocalSession 打开、touchClaudeLocalSession 更新最近活动，继续使用原目录排序与激活后展开/定位。不再创建前端草稿或分配新 ID；本机事件增加首次创建标记，普通 Turn 活动仍只刷新，防止抢焦点。不改变三端共享契约。
