# Replive 项目交接说明（2026-08-13）

> 供下一次 Codex 会话使用。先阅读本文件、`AGENTS.md`、`plan/README.md` 与当前活动计划 `plan/chat-ui-multi-room-cursor-translation.md`，再执行任何修改。

## 1. 工作区与外部资料

- 仓库根目录：`D:\Tencentt\Tencent Files\1528760842\文件\MobileFile\nsy_chat_live-master`
- 根目录应用：Go/Hertz 后端、SQLite 同步与本地 HTTP API。直播轮询与录像已拆到 `cmd/replive-live-recorder/`，只在独立进程启动时运行。
- 当前新版 Web：`replive-web-pro/`（React 19 + Vite + Zustand + Tailwind）。
- `replive-web/`：旧版 Web，已在 2026-08-14 的遗留清理中移除；当前仅维护 `replive-web-pro/`。
- 最新 APK：`C:\Users\Fu\Desktop\Replive_4.7.7.apk`
- JADX 源码：`C:\Users\Fu\Documents\replive\jadx_4.7.7\sources`
- 已解包 DEX：`C:\Users\Fu\Documents\replive\apk_dex\`
- 用户常用运行目录：`F:\迅雷下载\replive\new\`
  - 数据库：`F:\迅雷下载\replive\new\sqlite.db`
  - 日志：`F:\迅雷下载\replive\new\replive_*.log`
  - 手动复制运行的产物：`replive-plus.exe`、`replive-plus-web.exe`

## 2. 项目结构

### 后端

- `main.go`：程序启动、路由注册。
- `dal/`：SQLite schema、迁移和读写。
- `handler/`：本地 HTTP handler。
- `rep_api/`：Replive 官方 HTTP/protobuf 客户端；新接口必须先由 APK 确认协议再调用。
- `service/`：房间、消息、媒体、Prime、直播同步逻辑。
- `model/`：现有 protobuf 生成结构。

主要本地路由：

- `GET /api/chat/rooms`：Fandom 房间。
- `GET /api/chat/messages`：Fandom 消息。
- `GET /api/prime/chat/rooms`：Prime 房间。
- `GET /api/prime/chat/messages`：Prime 消息。

### 前端

新版主要文件：

- `replive-web-pro/src/components/chat/`
- `replive-web-pro/src/components/drawers/MediaGalleryDrawer.tsx`
- `replive-web-pro/src/components/modals/MediaLightbox.tsx`
- `replive-web-pro/src/stores/chat-store.ts`
- `replive-web-pro/src/utils/fetch-data.ts`
- `replive-web-pro/src/types/chat.ts`

`fetch-data.ts` 已能区分 Fandom 与 Prime。Prime 使用远程原始媒体 URL；Fandom 当前可优先使用后端本地媒体 URL，失败后只回退一次到远程 URL。这是用户后来明确授权的例外，虽然 `AGENTS.md` 的旧规则默认建议媒体直接使用原 URL。

## 3. 已完成的功能性改动

以下改动来自此前会话，应保留，除非用户明确要求回退：

- Fandom 和 Prime 的消息状态按 `roomKey` 隔离，切换房间不会串消息。
- 房间列表按最后消息时间排序；后端房间接口提供最后消息摘要。
- Fandom 头像右下角图标已移除；水印只显示对方 `display_name`。
- Fandom 左右消息判断：`message.senderName !== room.displayName` 代表“我发送”。
  - 不可用 SQLite `user_id` 或 `senderId === userProfile.userId` 判断，因为 Fandom 本地记录的 `user_id` 会指向主播。
- Fandom 发送去重：待发送消息用官方返回消息 ID；后续同步同 ID 消息覆盖，而非重复插入。
- 后端 Fandom 消息排序/分页使用 `send_time, id`，不只用 SQLite 自增 ID。
- 新版 Web 已处理首屏图片/相册缩略图的 eager 加载与并发控制；相册不是一次性挂载所有媒体。
- 历史图片加载改变高度时，进入/切房间应保持贴底；用户主动上滑后解除贴底，日期跳转、搜索定位、相册跳转不可被贴底逻辑覆盖。后续改 UI 时必须回归验证该行为。
- Fandom 本地媒体映射、旧媒体回填、失败回退等实现已在当前开发线中出现；Prime 不使用本地媒体路径。

## 4. 当前时间字段工作：Fandom

### 当前源码行为

Fandom 现在只保留一个诊断/持久化字段：

- `talent_last_check_time`
  - `ListChatRooms` 响应的 protobuf 未知字段 `101`。
  - 在 `rep_api/chat_room_timing.go` 解析。
  - 保存至 `chat_rooms.talent_last_check_time`，单位 Unix 秒。
  - 每次 Fandom 房间同步日志：

```text
Fandom room timing probe: display_name="..." chat_room_id=... talent_last_check_time=...
```

用户已要求删除以下内容，当前源码已删除其解析/写入/接口输出/日志：

- `last_check_chat_message_create_time`（字段 102）。用户确认其更像“当前账号最后发送消息”的时间，对判断对方已读没有价值。
- `latest_chat_room_join_time=request:unset ...`。这只是此前人为添加的日志说明，不是响应字段；APK 中它是 `ListChatRoomsRequest` 的本客户端请求游标，不能当对方进房时间。

兼容策略：旧 SQLite 中的 `last_check_chat_message_create_time` 列不删除，避免破坏性迁移；新代码不再读写它。

### 部署状态提醒

用户最后运行过的含字段 101 Prime 诊断日志版本是约 `2026-08-13 15:17`。随后源码又移除了 Fandom 字段 102 和 `latest_chat_room_join_time` 日志说明，但这最后一次清理**尚未重新构建/部署**。新日志在重新构建后才会只有 Fandom 的 `talent_last_check_time`。

## 5. 当前时间字段工作：Prime

### 官方协议已确认

Prime 与 Fandom 是独立接口和独立表：

- `user.v1.ChatService/ListPrimeChatRooms`
- `user.v1.ChatService/UpdatePrimeChatRoomLastCheckTime`
- `user.v1.ChatService/UpdateTalentPrimeChatRoomsLastCheckTime`

官方 APK 依据：

- Prime 房间模型：`sq0/ry.java`
- 序列化器：`sq0/qy.java`
- 单房间已读更新请求：`sq0/j90.java`

`PrimeChatRoom` 的官方字段名：

| Protobuf 字段 | APK 名称 | 代码中保存列 | 目前结论 |
| --- | --- | --- | --- |
| 100 | `user_last_check_time` | `talent_last_check_time_ms` | 与 `user` / field 1、3 一方关联。该方在当前房间列表中是声优，但它是否可靠等于“声优进入房间/已读”的真实时间**尚未证实**。 |
| 101 | `member_user_last_check_time` | `member_last_check_time_ms` | 与 `member_user` / field 2、4 一方关联，即当前登录会员。仅作对照，绝不能在 UI 中称为对方已读。 |

当前实现位置：

- `rep_api/prime_chat.go`：解析字段 100、101，统一转换为 Unix 毫秒。
- `dal/main.go`：`prime_chat_rooms` 两列及兼容迁移。
  - 两个 Go 字段都有明确 GORM `column:` 标签。此前遗漏该标签时，GORM 错误使用了 `talent_last_check_time_millis`，导致 Prime 保存失败；该问题已修复。
- `service/prime_chat.go`：同步保存；Prime 日志只对当前 Fandom 房间中同一主播 ID 的 Prime 房间输出。
- `handler/prime_chat_handler.go`：Prime 房间 API 返回 `talent_last_check_time_ms` 和 `member_last_check_time_ms`。

当前 Prime 日志格式：

```text
Prime Chat timing probe: display_name="..." talent_user_id=... member_user_id=... talent_last_check_time=... member_user_last_check_time=... (member/current account diagnostic only; only logged because this talent has a Fandom chat room for the current account)
```

### Prime 字段语义：当前不能下结论

用户实际观察到紫月杏朱彩的字段 100 与 101 完全相同，并且该时间正好等于用户自己最后一条 Prime 消息的发送时间。原始响应中两个字段的 protobuf bytes 也完全相同，所以不是解析错误。

因此，禁止把 Prime 的字段 100 表述为“对方已读时间”或制作已读 UI。当前最严谨的结论：

- 字段名与官方更新接口证明它们是某种“检查/水位”相关状态。
- 但字段具体在哪个服务端行为中更新尚未被证明；可能涉及发信、初始化、同步、双方水位对齐，或实际进入房间。
- 项目只读同步，不调用 Prime 的两个已读更新接口，不会主动改变这两个官方值。
- 真正验证必须做控制变量实验：记录初值，分别执行“仅自己打开房间、不发送”“仅发送”“等待对方操作/不操作”，对比 100/101 与消息 ID 的变化。

Prime 保持“未订阅 Fandom 也可同步”的能力。Fandom/Prime ID 交集只用于限制日志，**不可**用来限制 Prime 同步。

## 6. 最近日志和已修复问题

- `F:\迅雷下载\replive\new\replive_202608131445.log`
  - Prime 同步曾失败：`no such column: talent_last_check_time_millis`。
  - 根因：GORM 由 Go 字段名推导列名，和实际 `talent_last_check_time_ms` 不一致。
  - 源码已通过 `gorm:"column:talent_last_check_time_ms"` 修复。
- `F:\迅雷下载\replive\new\replive_202608131508.log`
  - Prime 同步成功，5 个房间保存成功。
  - 该日志没有字段 101，因为相应可执行文件仍是字段 101 接入之前的版本。
  - 原始 `ListPrimeChatRooms` 响应已显示某些房间只有 101，紫月杏朱彩有 100 和 101。

## 7. 工作区当前状态（必须保留）

截至本交接文件创建时，工作树不是干净状态：

```text
 M dal/main.go
 M handler/prime_chat_handler.go
 D nsy_chat_live-master.zip
 M plan/chat-ui-multi-room-cursor-translation.md
 M rep_api/prime_chat.go
 M service/chat.go
 M service/prime_chat.go
?? rep_api/chat_room_timing.go
?? vendor/
```

说明：

- `D nsy_chat_live-master.zip` 是已有用户改动，绝不能恢复或删除其他文件来“清理”它。
- 上述 Go/plan 改动包含 Fandom 时间字段、Prime 字段 100/101、GORM 映射修复及日志改动；尚未提交。
- `vendor/` 是用户在本轮生成的本地 Go 依赖副本，尚未纳入 Git。是否提交由用户决定；不要擅自删除。
- 交接文件本身也会是新的未跟踪改动。
- 修改前总是先执行 `git status --short`；不要覆盖未知改动。

## 8. 构建与依赖

### 用户当前偏好

用户明确要求：之后由用户自己重新构建。Agent 修改代码后只做静态检查，除非用户明确再次授权构建。

构建产物约定：

- 输出到仓库根 `dist/`。
- 默认构建产物：`dist\replive-plus.exe` 与 `dist\replive-plus-web.exe`。
- `dist/` 是生成目录，不纳入 Git；构建前确认不会覆盖用户正在使用的文件。
- 不启动程序。
- 用户此前还要求不要额外跑 Go 测试、文件大小/差异检查。

供用户手动构建的参考命令（当前后端+新版 Web）：

```powershell
cd D:\Tencentt\Tencent Files\1528760842\文件\MobileFile\nsy_chat_live-master\replive-web-pro
npm run build

cd D:\Tencentt\Tencent Files\1528760842\文件\MobileFile\nsy_chat_live-master
$env:CGO_ENABLED = "0"
go build -mod=vendor -o dist\replive-plus.exe .
go build -mod=vendor -o dist\replive-plus-web.exe .\replive-web-pro
go build -mod=vendor -o dist\replive-live-recorder.exe .\cmd\replive-live-recorder
```

已删除旧版 `replive-web/` 及其专用 Windows 构建脚本；使用上述命令构建当前版本。

`replive-live-recorder.exe` 使用与主程序相同的 `config.yaml` 和登录令牌，不启动聊天同步或本地 HTTP API。需要录像时单独运行；可用 `-interval 2s` 调整直播状态轮询间隔。

### Go 环境与国内网络

- Go 安装：`D:\Go\bin\go.exe`，当前查到版本为 Go 1.26.4；`go.mod` 声明 Go 1.24.0 / toolchain 1.24.5。
- 已将当前 Windows 用户 Go 配置为：

```text
GOPROXY=https://goproxy.cn,direct
GOSUMDB=sum.golang.google.cn
```

- 以后涉及 Go 下载依赖，优先使用国内镜像，避免 `proxy.golang.org`。
- 根目录 `vendor/` 已存在，并已在离线模式下成功解析 `.` 和 `./replive-web-pro`。优先 `-mod=vendor`，不依赖网络或 `pkg/mod` 下载元数据。
- 用户曾将 `D:\Go\bin` 加到用户 `PATH`；修改 PATH 后需重开 Codex/终端才会刷新当前进程环境。

## 9. Codex Windows 沙箱问题

当前工作区曾反复出现：

```text
windows sandbox: helper_unknown_error: apply deny-read ACLs
```

表现：普通受限读取和 `apply_patch` 可能失败，甚至 Codex 内部浏览器/Node 进程启动也触发同一错误。项目目录 ACL 本身可见 `CodexSandboxUsers` 修改权限；这是 Codex Windows 沙箱初始化问题，未证实是代码或 SQLite 问题。

处理原则：

- 不要执行 `icacls /reset`，不要破坏现有权限。
- 先完全退出、重开 Codex；如果仍有问题，建议把工作区迁到非 Tencent Files 同步目录，例如 `D:\Dev\...`。
- 在问题存在时，必要的读写可用受控的、经用户同意的 PowerShell 单文件替换作为 `apply_patch` 的 fallback；每次限定具体文件与替换文本。
- 常规情况下仍优先 `apply_patch`，符合 `AGENTS.md`。

## 10. 后续待办与风险

1. 用户重新构建后，确认新 Fandom 日志只剩 `talent_last_check_time`，Prime 日志仍有字段 100/101 对照。
2. 对 Prime 做控制变量实测；在有结论前，绝不增加 Prime “对方已读”前端显示。
3. 若继续改善新版 Web 媒体体验，重点实测：进入含大量历史图片的房间后是否持续贴底、用户上滑是否解除贴底、日期/搜索/相册跳转是否不被自动贴底干扰。
4. 聊天视频预览图曾出现黑屏问题，用户报告“上一个版本有预览图”；后续改媒体加载时需检查 `video_thumbnail_url` / poster、聊天和相册两处，不要只改图片。
5. 使用 APK 做新协议工作时，先在 JADX 确认模型、序列化器、字段号、调用路径；复用 `rep_api` 的请求方式，不能猜测字段。
6. 未提交或推送。用户此前想将全部改动更新到 GitHub，但本交接时没有完成 commit/push。提交前先让用户确认是否包含 `vendor/`。

## 11. 日常规则

- 工作语言优先中文。
- 修改前先读 `AGENTS.md`、`plan/README.md`、本文件和活动计划。
- 搜索优先 `rg` / `rg --files`。
- 不执行 `git reset --hard`、`git checkout --`、`rm` 等破坏性操作。
- 后端默认 `CGO_ENABLED=0`。
- 不启动程序，除非用户明确要求。
- 当前用户希望自己负责重新构建；不要擅自构建。
- 写新日志字段时，清楚区分“协议字段名称”“实体归属”“已被实测证明的行为语义”。三者不能混为一谈。
