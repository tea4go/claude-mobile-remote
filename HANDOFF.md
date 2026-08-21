# HANDOFF.md — Claude Mobile Remote 交接文档

> 新会话先读 `AGENTS.md`，再读本文件、`DECISIONS.md`、`REQUIREMENTS.md` 和总体设计。完成阅读后先复述当前状态和下一步，确认后再修改代码。

## 0. 一句话现状

项目已完成需求边界与总体架构确认，正在建立独立仓库的长期文档；业务代码尚未开始。

## 1. 当前目标

首期完成“简单 CLI + SDK 托管 Claude + 单 `sessionId` + 局域网 WebSocket + 移动浏览器”的最小双向通信闭环。

```text
终端 ──> CLI Service ──> claude-agent-sdk-go ──> Claude
                 └────局域网 WebSocket────> 手机浏览器
```

## 2. 已确认事项

- 项目仓库：`/Users/zhangym/code/work/utils/claude-mobile-remote`；
- 与 WhaleTerm 无关，旧方案、阶段和记忆不继承；
- 使用 `github.com/tea4go/claude-agent-sdk-go` 托管 Claude；
- CLI 使用简单行输入输出，不复刻 Claude Code 完整终端；
- 扫码绑定真实 `sessionId`；
- 手机不展示扫码前完整历史，只展示当前状态和后续消息；
- 首期局域网与浏览器调试优先；
- iOS / Android 使用 Capacitor，HarmonyOS NEXT 独立开发；
- 心跳、完整重连、推送、公网中转、多会话和多机器后置。

## 3. 文档状态

| 文件 | 状态 | 作用 |
| --- | :---: | --- |
| `REQUIREMENTS.md` | ✅ | 产品需求、阶段范围和验收标准 |
| `AGENTS.md` | ✅ | 长期约束和协作规则 |
| `CLAUDE.md` | ✅ | Claude Code 阅读入口 |
| `DECISIONS.md` | ✅ | 新项目决策日志 |
| 总体设计 | ✅ 待用户复核 | 架构、协议和测试边界 |
| 实施计划 | ⬜ | 总体设计复核后编写 |
| 业务代码 | ⬜ | 实施计划确认后开始 |

## 4. 下一步

1. 用户复核 `REQUIREMENTS.md` 和总体设计；
2. 根据反馈修订文档；
3. 编写阶段一、阶段二实施计划与门禁；
4. 搭建 Go Service 和 React 移动 Web 骨架；
5. 用 SDK 假实现和真实 Claude 逐层验证单会话闭环；
6. 每个重要检查点更新本文件，新增决策时更新 `DECISIONS.md`。

## 5. 首期关键验证

- SDK 能否在独立最小程序中稳定提供 session、流式消息、错误和取消事件；
- 真实 `sessionId` 何时可用，以及二维码生成时机；
- 终端与手机输入如何在单一 Session Controller 中串行；
- 新连接的当前状态快照需要包含哪些最小字段；
- macOS、Windows 的局域网地址、防火墙与二维码连接差异。

## 6. 参考提醒

WhaleTerm 只用于查阅 SDK 已验证用法，不读取或同步其 `AGENTS.md`、`HANDOFF.md`、`DECISIONS.md` 和远程控制设计。
