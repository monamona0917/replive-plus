# Chat UI 多房间、游标分页与翻译开发计划

## 当前补充（2026-08-11）

当前实际前端为 `replive-web-pro/`，本阶段仅处理进入房间后的消息列表底部定位与媒体首屏加载：

- 首次进入或切换到一个房间后，消息列表应保持在最新消息底部；图片等媒体完成加载并改变列表高度时也继续贴底。
- 用户主动向上滚动后，解除该自动贴底，不能妨碍浏览历史消息。
- 日期跳转、搜索结果定位和相册定位消息必须保留其指定消息的居中定位，不受自动贴底影响。
- Fandom 聊天图片和视频优先使用后端按消息 ID 暴露的本地 `/media/...` 地址；本地文件缺失或请求失败时，前端只回退一次到 SQLite 保存的原始远程 URL。当前页可见图片优先解码；聊天视频允许预取 metadata，以保留 poster 或首帧预览；相册视频仍不预读。
- Fandom 相册同样使用本地优先、远程回退，但不能一次把全屋媒体同时挂到 DOM：首批渲染有限数量的格子，用户滚动到底部后再追加下一批。该策略不使用浏览器原生 loading="lazy"。
- 后端仅为 Fandom 维护本地媒体映射：同步下载时写入路径；启动时按消息 ID、远程文件名和归档目录回填旧记录。Prime 不接入本地媒体路径。
- 保留已经完成的 roomKey 状态隔离、房间最近消息排序、Fandom 发送方判断和待发送消息去重逻辑；本阶段不改后端接口或同步链路。
- 修改完成后仅执行构建，新的 `_prime` 产物输出到仓库根目录 `dist/`，不覆盖旧的非 `_prime` 可执行文件。
## 目标

把 `replive-web` 接到本地 Hertz 后端和 SQLite 聊天数据，实现：

- 改一下，别叫 replive-web 了，而是改成 replive_web。对应的水印也要显示为 chat room 的 display_name。
- 在左上方通过 `chat_rooms.display_name` 切换聊天对象。
- 从最新消息开始加载，再用游标向更早消息分页。
- 用户向上拖动/滚动到顶部附近时，拉取更早的历史消息。
- 图片和视频直接使用 SQLite 里保存的原始 `image_url` / `video_url`，不要走本地媒体服务器。
- 每条文字消息旁边增加翻译按钮。点击后调用免费的 Google Translate 兼容接口，把翻译内容显示在原文下方；再次点击隐藏翻译内容。
- 右上提供一个快捷按时间跳转的选项，用户可以点击跳转到任意天数的消息，选定时间后按时间查询那一天的消息，定位到那天的第一条，然后可以往上翻也可以往下翻来双向查询。
- 右上角提供一个搜索按钮，输入关键字后搜索，选中后跳转到指定时间，然后往上翻下滚动消息。
- 翻译请求通过后端代理，并使用系统代理或 `config.yaml` 中的代理配置访问 Google Translate。
- 消息时间展示完整年月日时分秒；日期跳转入口整块可点击；聊天列表提供回到底部按钮。
- 同步 chat room 时如果头像等资料图片 URL 更新，需要更新 DB 中的最新值，并把资料图片按日期归档保存到独立 media 目录。
- 直播录制文件按年月分目录保存，避免 `media/live` 下文件过多。
- “滑到最底下”需要先拉取后端最新消息，再滚动到最新消息下方。
- 通过解包确认的 `ListMyOshis` 接口定期同步关注列表到 SQLite 新表，并每 10 分钟刷新一次，同时归档关注对象的头像、背景图等资料图片。

## 非目标

- 不重设计整个 UI。
- 不为本功能新增本地图片/视频服务器。
- 除非用户后续要求，否则不把翻译结果写入 SQLite。
- 除非 API 正确性需要，否则不改消息同步链路。

## 后端计划

### 路由

在 `main.go` 中启用或新增本地 HTTP 路由：

- `GET /api/chat/rooms`
- `GET /api/chat/messages`
- 可选：`GET /api/translate`

如果前端开发时使用 Vite dev server，优先使用 Vite proxy 转发到后端；如果不用 proxy，再考虑给后端加本地开发 CORS。后续同源部署时应尽量保持简单。

### Chat Rooms API

可以复用或精简 `handler.HandleGetChatRooms`。

建议响应形态：

```json
{
  "success": true,
  "data": [
    {
      "user_id": "...",
      "chat_room_id": "...",
      "display_name": "...",
      "avatar_url": "..."
    }
  ]
}
```

排序建议：

- 优先按 `display_name ASC`
- 必要时再按 `id ASC`

### Messages API

当前 handler 已支持 `display_name`、`cursor_id`、`page_size`，并按 `id DESC` 查询。历史分页继续使用这个方向。

必须满足：

- 不传 `cursor_id`：返回最新一页，查询条件按 `id DESC`。
- 传 `cursor_id`：返回更早一页，查询条件 `id < cursor_id`，仍按 `id DESC`。
- 响应包含：
  - `messages`：明确约定返回顺序。建议后端返回 `id DESC`，前端入库前转成时间升序渲染。
  - `next_cursor_id`：本页最小 `id`；没有更多数据时为 `0`。
  - `has_more`：建议新增显式布尔值。如果暂时不加，前端可用 `next_cursor_id > 0 && messages.length === page_size` 推断。

推荐 DTO 字段：

```json
{
  "id": 123,
  "user_id": "...",
  "display_name": "...",
  "chat_room_id": "...",
  "chat_message_id": "...",
  "msg_type": 1,
  "content": "...",
  "image_url": "...",
  "video_url": "...",
  "send_time": 1710000000,
  "time_str": "..."
}
```

媒体规则：

- 图片使用 `image_url`。
- 视频如果继续展示，则使用 `video_url`。
- 本次 UI 不要返回或使用 `image_local_url` / `video_local_url`。

### 翻译 API

不要直接把浏览器端调用免费接口作为首选，因为可能遇到 CORS。建议后端加一个轻量代理：

- `GET /api/translate?text=<urlencoded>&source=ja&target=zh-CN`

实现候选：

- 调用 Google Translate 公共接口：`https://translate.googleapis.com/translate_a/single?client=gtx&sl=<source>&tl=<target>&dt=t&q=<text>`
- 防御式解析嵌套数组响应。
- 超时时间建议 10 秒左右。
- 响应 JSON：

```json
{
  "success": true,
  "data": {
    "source": "ja",
    "target": "zh-CN",
    "translated_text": "..."
  }
}
```

注意：

- 该接口是非官方接口，可能被限流或变更。UI 需要显示明确错误。
- 空文本不要请求翻译。
- 前端可用内存缓存，key 用 `message.id + target`，避免重复点击反复请求。

## 前端计划

### 数据模型

修改 `replive-web/src/types/chat.ts`，支持 chat room 和后端消息 DTO 映射。

建议类型：

```ts
export interface ChatRoom {
  userId: string;
  chatRoomId: string;
  displayName: string;
  avatarUrl?: string;
}

export interface Message {
  id: string;
  backendId: number;
  chatMessageId: string;
  content: string;
  type: "text" | "image" | "video";
  createdAt: string;
  mediaUrl?: string;
}
```

映射规则：

- `msg_type === 1` -> `text`
- `msg_type === 2` -> `image`，`mediaUrl = image_url`
- `msg_type === 3` -> `video`，`mediaUrl = video_url`
- `createdAt` 优先由 `send_time` 转换；没有时再 fallback 到 `time_str`。
- `id` 优先使用 `chat_message_id`；没有时 fallback 到后端数字 `id`。

### API Client

替换 `fetch-data.ts` 中的静态 `https://p.chilfish.top/replive/data.json`。

新增函数：

- `fetchChatRooms()`
- `fetchChatMessages({ displayName, cursorId, pageSize })`
- `translateText({ text, source, target })`

开发环境 base URL 策略：

- 优先使用相对路径 `/api/...`，并在 Vite 配置里 proxy 到后端。
- 如果暂时不加 proxy，再使用 `VITE_API_BASE_URL`。

### Store

把 `chat-store.ts` 从单人静态 `ChatData` 改成 room-aware 的分页状态。

建议状态：

- `rooms: ChatRoom[]`
- `selectedRoom: ChatRoom | null`
- `messagesByRoom: Record<chatRoomId, Message[]>`
- `cursorByRoom: Record<chatRoomId, number>`
- `hasMoreByRoom: Record<chatRoomId, boolean>`
- `isLoadingRooms`
- `isLoadingMessages`
- `isLoadingMore`
- `translationByMessageId: Record<string, { text?: string; loading?: boolean; error?: string; visible?: boolean }>`

建议 actions：

- `loadRooms()`
- `selectRoom(room)`
- `loadLatestMessages(room)`
- `loadOlderMessages(room)`
- `toggleTranslation(message)`
- `clearSearch()`

状态规则：

- 后端第一页返回最新消息，store 里应统一转换为时间升序，便于渲染。
- 更早页加载后 prepend 到已有消息前面，并保持滚动锚点。
- 按 `chatMessageId` 或 `backendId` 去重。
- 切换 room 时清空搜索状态。

### Room 切换器

修改 `ChatPage.tsx` 的 header：

- 左上方打开 room 切换控件，列表展示 `displayName`。
- 可以把现有返回箭头替换为 popover/dropdown 触发按钮。
- header 中展示当前选中的 display name。
- 如果有头像，可以一起展示。
- 选择 room 后加载该 room 最新消息，并滚动到底部。

### 游标滚动加载

修改 `ChatList.tsx`：

- 不再使用本地 `renderCount` 截取静态全量数据。
- 使用 store 中当前 room 的 messages。
- 滚动到顶部附近时：
  - 记录当前可视区域的锚点消息 id 和偏移
  - 调用 `loadOlderMessages`
  - 新消息渲染后恢复滚动位置，避免列表跳动
- 首次加载和切换 room 后滚动到底部。
- 搜索本阶段可以只搜索“已加载消息”；如果这样做，需要在 UI 或代码注释中说明。

### 翻译按钮

修改 `MessageBubble.tsx`：

- 每条非空文字消息都显示翻译按钮。
- 第一次点击：
  - 如果没有缓存翻译，调用翻译 API。
  - 显示 loading 状态。
  - 翻译完成后在原文下方显示译文。
- 第二次点击：
  - 隐藏译文，但不删除缓存。
- 之后再次点击：
  - 直接显示缓存译文。
- 失败时：
  - 显示一行小错误提示，并允许再次点击重试。

按钮文案/图标：

- 当前 `Languages` 图标可以继续使用。
- title 可用：`翻译 / 隐藏翻译`。

### 媒体展示

修改 `MessageBubble.tsx`：

- 图片直接渲染 `message.mediaUrl`，来源是后端 `image_url`。
- 视频如果保留播放器，直接渲染 `message.mediaUrl`，来源是后端 `video_url`。
- 不要请求后端 `/media/...`。
- 全屏弹窗也继续使用原始 URL。

## 后端验收标准

- `GET /api/chat/rooms` 能返回 SQLite 中所有 room。
- `GET /api/chat/messages?display_name=<name>&page_size=20` 返回最新 20 条。
- `GET /api/chat/messages?display_name=<name>&cursor_id=<next_cursor_id>&page_size=20` 返回下一页更早消息。
- 图片 DTO 包含 `image_url`，前端不依赖本地 URL 字段。
- 翻译路由返回译文，失败时返回明确错误。
- `CGO_ENABLED=0 go test ./...` 通过。

## 前端验收标准

- 页面从后端加载 room 列表。
- 用户可以从左上方/header 切换 room。
- 切换 room 后加载该 room 最新消息并滚动到底部。
- 向上滚动能拉取更早消息，且列表不明显跳动。
- 文字消息旁边有翻译按钮。
- 点击翻译后，译文显示在原文下方；再次点击隐藏。
- 图片使用 DB 中的原始远程 URL 展示。
- 如果本地 Bun 和依赖可用，`bun run lint` 和 `bun run build` 通过。

## 建议实现顺序

1. 后端：启用路由，整理 chat rooms/messages 响应结构。
2. 后端：新增翻译代理路由，设置超时并防御式解析。
3. 前端：把静态 JSON 数据源换成本地 API client。
4. 前端：重构 store，支持 rooms、selected room、cursor pagination、translation cache。
5. 前端：header 增加 room 切换器。
6. 前端：把向上加载历史改为调用后端 cursor API。
7. 前端：增加翻译按钮和显示/隐藏逻辑。
8. 前端：确认图片和视频使用原始 URL。
9. 跑后端和前端验证命令。

## 需要用户确认的问题

- 翻译目标语言是否默认使用简体中文 `zh-CN`？
- 前端开发时是否使用 Vite proxy 转发到 Hertz 后端，还是让后端直接托管构建后的前端文件？
- 搜索是否暂时只搜索已加载消息，还是要新增后端搜索接口？
- room 切换列表按什么顺序展示：插入顺序、字母顺序，还是最近消息时间？

