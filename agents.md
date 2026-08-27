# Agents 指南

本仓库包含两个相关应用：

- 仓库根目录是 Go/Hertz 后端：负责登录 Replive、同步 chat room/chat message/media 元数据到 SQLite，自动监听直播并录制视频，并提供本地 HTTP API。
- `replive-web/` 是 React/Vite 前端：负责渲染类似 Replive 的聊天页面。

所有 AI agent 在实现功能前必须先阅读 `plan/README.md`。除非用户明确变更范围，否则以 `plan/` 里的计划作为执行约束。

## 原始 app
本仓库基于原始 app 的解包 `C:\Users\hwg2529\soft\src\replive_app_unpack` 而做。

## 工作规则

- 修改前先看 `git status --short`，不要覆盖用户已有改动。
- 搜索文件和文本优先用 `rg` / `rg --files`。
- 手工改文件必须用 `apply_patch`。
- 不要执行破坏性命令，例如 `git reset --hard`、`git checkout --`、`rm`，除非用户明确要求。
- 后端默认保持无 CGO 构建，使用 `CGO_ENABLED=0`。
- 前端展示 chat 图片/视频时，不要新增本地媒体服务器路径。应直接使用 SQLite 里保存的原始 `image_url` / `video_url`，除非用户明确改变这个决定。
- 调用 replive 的 http 接口的工作方式是：基于解包明确proto协议、生成proto代码，参考已有的http请求复用 rep_api 的流程和使用proto生成的结构体进行调用。
- agent 工作时优先使用中文。

## 全局编译验证
```powershell
build_all.bat
```

## 后端结构

- SQLite schema 和 DB 访问在 `dal/`。
- HTTP handler 在 `handler/`。
- Replive API client 在 `rep_api/`。
- 同步 chat/live/media 的业务逻辑在 `service/`。
- `main.go` 负责启动流程、登录准备、DB 初始化、worker 启动和路由注册。
- 现有 chat handler 可复用：
  - `HandleGetChatRooms`
  - `HandleGetChatMessages`
  - 历史本地媒体 handler 仍存在，但一般不要使用它。

## 前端结构

- `replive-web/` 使用 React 19、Vite、Zustand、Tailwind/shadcn 风格组件、Biome 和 Bun。
- 主要聊天 UI 文件：
  - `replive-web/src/components/chat/ChatPage.tsx`
  - `replive-web/src/components/chat/ChatList.tsx`
  - `replive-web/src/components/chat/MessageBubble.tsx`
  - `replive-web/src/stores/chat-store.ts`
  - `replive-web/src/utils/fetch-data.ts`
  - `replive-web/src/types/chat.ts`
- 除非计划明确要求重设计，否则保持现有视觉风格。

## 验证命令

后端：

```bash
env GOCACHE=/private/tmp/replive-go-build CGO_ENABLED=0 /Users/bytedance/go/bin/go1.24.10 test ./...
env GOCACHE=/private/tmp/replive-go-build CGO_ENABLED=0 /Users/bytedance/go/bin/go1.24.10 build -o /private/tmp/replive-build-check .
```

前端：

```bash
cd replive-web
bun run lint
bun run build
```

如果因为本地工具缺失、沙箱限制或网络限制导致命令不能运行，需要在最终回复里明确说明。
