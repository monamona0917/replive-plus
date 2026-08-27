# 定时发送 chat 消息计划

## 背景

本次需求基于原始 app 解包目录 `C:\Users\hwg2529\soft\src\replive_app_unpack` 反推发送协议，并在 Go/Hertz 后端里实现自动发送。

## 解包结论

- 普通 chat 消息发送接口：
  - `POST /user.v1.ChatService/SendChatMessage`
  - 解包类：`sq0.n30` / adapter `sq0.m30`
  - 请求字段：
    - `user_id = 1`
    - `chat_room_id = 2`
    - `content = 3`
    - `image_path = 4`
    - `user_upload_video_id = 5`
    - `chat_message_id = 6`
    - `confirm_contains_forbidden_words_warning = 7`
- 付费 card 创建接口：
  - `POST /user.v1.LiveService/CreateCard`
  - 解包类：`sq0.C17681g3` / adapter `sq0.C17644f3`
  - 请求字段：
    - `user_id = 1`
    - `content = 2`
    - `coin_amount = 3`
    - `live_id = 4`
    - `confirm_contains_forbidden_words_warning = 5`

## 实现范围

1. 新增 `model/chat_send.proto`，写入上述两个接口的 request/response，并生成 Go protobuf 代码。
2. 在 `config.Config` 中新增 `scheduled_chat_message` 配置：
   - 普通消息：是否启用、目标 `display_name`、`user_id`、`chat_room_id`、消息内容。
   - 付费 card：是否启用、目标 `display_name`、`user_id`、`live_id`、内容和 coin 数。
3. 在 `rep_api` 新增：
   - `SendChatMessage`
   - `CreateCard`
4. 在 `service` 新增定时任务：
   - 使用 `Asia/Shanghai` 时区。
   - 每周日 23:00 触发一次普通消息发送。
   - 启动后只调度，不立即发送。
   - card 函数实现好，但默认不启用；只有配置显式启用时才发送。

## 验证

- `go test ./... -run '^$'`
- `CGO_ENABLED=0 go build ./...`

