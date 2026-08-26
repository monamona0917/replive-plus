# Replive 项目交接说明（2026-08-26）

> 源码行为优先于历史交接文字。本文件记录 2026-08-25/26 按启动与日志方案完成的实现。

## 当前实现

### 启动顺序

主后端启动后依次完成：

1. 配置、登录、API 和 SQLite 初始化。
2. Fandom 房间同步。
3. 一次 Fandom 新消息同步。
4. 一次 Prime Chat 房间、背景媒体和消息同步。
5. 一次直播状态检查，并收集直播名称、标题和录制地址可用性。
6. 输出固定顺序的初始化总结。
7. 启动聊天后台 worker，并启动直播监控。

后台 worker 在初始化总结输出完成后才启动。worker 的第一次执行先等待自己的周期，避免与启动阶段重复请求：

- `refreshNewMessages`：约 3--6.9 秒。
- `checkChatRoomTimings`：每 5 分钟读取一次 Fandom 房间时间字段，只输出首次或变化的房间。
- 直播监控：约 5.9--10 秒。
- `listening...`：主程序每 1 分钟检查一次，但仅以约 2% 概率输出。

`saveChatRooms` 和 `syncOshiProfiles` 只在初始化阶段执行一次。`checkChatRoomTimings` 不更新数据库、`chatRoomList` 或聊天消息。

### 聊天日志

`ChatSyncSummary` 按主播统计新消息和新图片数量，并统计聊天图片的下载/跳过结果。新消息提示格式为：

```text
xxx 给你发新消息了！⸜(*ˊᗜˋ*)⸝
xxx 给你发新消息了！而且有 2 张新照片！⸜(*ˊᗜˋ*)⸝
```

普通轮询成功不输出。没有新消息只在首次、从有消息恢复为空，或冷却时间达到一小时后输出。启动阶段的历史初始化不会误报为新消息。

后台聊天轮询不再随机输出 `sync refreshNewMessages done`。

### 直播和 FFmpeg

- 主后端自动启动直播监控；独立 `replive-live-recorder` 入口仍可用。
- 默认直播轮询为随机 `5.9`--`10` 秒。
- 新直播按三种状态处理：无 `WebrtcUrl` 时为 Fandom only；地址存在且 RTMP 转换成功时加入录制；地址存在但转换失败时只显示主播、标题和解析失败提示，不加入录制队列，后续轮询继续重试。
- 新直播提示只输出一次，包含主播名称和直播标题；录制地址可用时输出完整 RTMP 地址。
- 没有直播只在首次空结果或从有直播恢复为空时提示。
- 无直播提示统一为 `还没有女声优在直播(:3_ヽ)_`；后台监听启动提示为 `继续观察有没有女声优正在直播(:3_ヽ)_`。
- 连续直播查询失败只记录一次，恢复时记录一次。
- FFmpeg 启动成功只输出一次；不再运行 30 秒录制心跳。
- FFmpeg 退出后查询直播状态：仍在直播时有限次数恢复，并使用新的分段文件名；确认结束时输出结束提示并发送结束邮件。
- 配置路径不存在时只输出一次配置错误，不循环重试。
- 录制结束提示为 `%s的直播结束啦！`，文件名冲突自动换名且不输出英文提示，录像 goroutine、恢复和入队异常使用中文日志。

### Media 日志

下载函数返回 `MediaDownloaded`、`MediaSkipped` 或 `MediaFailed`。聊天图片、Prime Chat 背景图、Oshi 资料图片和关注列表资料图片分别汇总，任务结束时输出 `downloaded/skipped/failed` 统计；不会把不同任务混在一条统计中。

启动时会检查数据库中已有 Fandom 聊天消息的图片和视频：本地文件存在则跳过，文件缺失则按原有文件名从数据库保存的远程 URL 重新下载，并更新数据库路径。

### 时间日志

Fandom `talent_last_check_time` 按 `chat_room_id` 去重：启动完整房间同步时每个房间首次输出；之后由每 5 分钟的轻量检查任务读取，值变化时再次输出，值不变时不输出。Prime 时间字段仍然只是诊断信息，不作为已读状态，也不调用已读更新接口。

日志去重状态使用 `UnixNano()` 保存完整时间精度；SQLite 的 `talent_last_check_time` 仍通过 `chatRoomTimingUnix()` 保存 Unix 秒，日志展示格式不变。不存在时间的首次 `unset` 会输出一次，后续连续 `unset` 不重复输出。

Prime Chat 的 `ListPrimeChatRooms response received` 日志已删除；Prime 时间字段仍保留在数据库中供现有数据使用。

## 验证

- 已执行 `gofmt`。
- 已执行 `go test -mod=vendor ./...`，全部通过。
- 已用单元测试验证 Fandom only、RecordingReady 和 RtmpParseFailed 三态分类；尚未用真实账号启动后端验证直播、FFmpeg、媒体缺失恢复和日志数量。
- 尚未构建或覆盖 `dist/` 中的可执行文件；用户可按 `AGENTS.md` 中的命令自行构建。
- 已增加 timing 去重、纳秒级变化和 `unset` 稳定性的单元测试。

## 注意事项

- 保留用户已有的 Git 改动、分支和 `vendor/`。
- 不要将 Prime `talent_last_check_time` 或 `member_last_check_time` 表述为对方已读。
- 修改启动或日志行为后，继续更新本文件并在 `plan/README.md` 指向最新日期文件。
