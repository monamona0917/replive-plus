# 计划索引

这个目录用于维护 AI agent 在本仓库里的执行计划。

## 当前活跃计划

- `chat-ui-multi-room-cursor-translation.md`：当前需要执行的计划。目标是把 `replive-web` 接到本地 Hertz/SQLite 后端，并实现 chat room 切换、游标式历史消息分页、翻译按钮和原始媒体 URL 展示。

## 历史上下文

- `legacy-maintenance-login-plan.md`：从旧的根目录 `plan.md` 中整理出来的历史上下文，包括后端稳定性修复、登录行为、token 刷新失败处理、纯 Go SQLite、panic 日志等。
- 原始未跟踪的根目录 `plan.md` 仍保留在原位。除非用户要求，不要删除或覆盖。

## 执行策略

- 先读 `agents.md`。
- 再读当前活跃计划。
- 在用户确认本次计划前，不要开始实现 chat UI 相关代码。
- 如果需求范围变化，先更新对应计划文件，再继续实现。
- 实现过程中新增注释时优先使用中文。
