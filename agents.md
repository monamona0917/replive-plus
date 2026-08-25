# replive-plus 仓库指南

## 项目结构与模块组织

本项目名称为 `replive-plus`。仓库根目录是 Go/Hertz 后端，包含 SQLite 持久化和本地同步逻辑。核心包按职责划分：`api/` 负责 HTTP/API 接线，`handler/` 负责请求处理，`service/` 负责业务逻辑，`dal/` 负责数据库模型和持久化，`rep_api/` 负责 Replive API 客户端与解析器，`utils/` 放共享工具。`config/`、`login/` 和 `model/` 存放支持性运行时及领域代码；`cmd/replive-live-recorder/` 保留为独立的直播轮询与录像兼容入口，主后端也会在启动后后台开启直播监听与录像。

当前 Web 前端为 `replive-web-pro/`（React/Vite）。构建产物位于 `dist/`，计划和项目交接文档位于 `plan/`。

## 构建、测试与开发命令

- `go build -mod=vendor -o dist/replive-plus.exe .`：构建主后端可执行文件。
- `go build -mod=vendor -o dist/replive-plus-web.exe ./replive-web-pro`：构建带 Web 的可执行文件。
- `go build -mod=vendor -o dist/replive-live-recorder.exe ./cmd/replive-live-recorder`：构建独立直播录像进程。
- `go test -mod=vendor ./...`：运行全部后端测试。
- `cd replive-web-pro && npm install`：安装当前 Web UI 的依赖。
- `cd replive-web-pro && npm run build`：构建生产前端资源。

涉及本项目时统一使用 `replive-plus`，构建产物使用 `replive-plus.exe` 和 `replive-plus-web.exe`。以本节列出的 Go 与 Vite 命令作为默认构建方式。`build.sh`、`build_mac.sh`、`bootstrap_rep.sh` 与 `stop.sh` 尚未确认过时，但也不是默认工作流。

主后端会在 `service.Init()` 完成后后台轮询直播状态并启动 ffmpeg 录像；独立运行 `replive-live-recorder.exe` 仍然可用。两者复用同一份 `config.yaml`，且必须先完成登录。

中国网络环境优先使用 `GOPROXY=https://goproxy.cn,direct` 和 `GOSUMDB=sum.golang.google.cn`。根目录已有 `vendor/`，优先使用 `-mod=vendor`，避免依赖网络下载。

## 代码风格与命名

Go 代码使用 `gofmt` 格式化；包名保持简短、符合 Go 惯例，只为公共 API 导出标识符。GORM 推断可能偏离既有 SQLite schema 时，特别是时间字段，必须明确声明数据库列名。

React/Vite 代码遵循 `replive-web-pro/` 现有 TypeScript 风格：组件使用 PascalCase，函数和变量使用 camelCase，房间和聊天状态以房间标识为键，避免跨房间串状态。当前 UI 开发只在 `replive-web-pro/` 进行。

## 测试指南

Go 测试与被测包放在同一目录，文件名使用 `*_test.go`。优先覆盖解析器、数据库迁移、分页和消息排序。前端变更后运行 `npm run build`；仅在项目已有匹配测试设施时再补充组件或工具测试。

## 提交与 Pull Request 指南

提交信息使用简短的祈使句，例如 `fix fandom message ordering` 或 `add prime timing fields`。尽量让每个提交只包含一个行为变更。Pull Request 应说明用户可见变更、受影响的前后端区域、SQLite schema 变动，并为可见 UI 变更附上截图。

## Agent 专项规则

修改前先执行 `git status --short`，保留无关的用户改动。不要覆盖 `dist/` 中无关的可执行文件；当前默认构建产物是 `replive-plus.exe` 和 `replive-plus-web.exe`。

Prime 聊天时间字段在受控测试验证前仅可作为诊断信息；不得将其表述为已读回执，也不得据此增加已读 UI。除非用户明确要求，否则不要调用 Prime 的已读状态更新接口。Prime 使用远程原始媒体 URL；Fandom 可优先使用本地媒体 URL，失败后只回退一次到远程原始 URL。

Fandom 聊天状态必须按 `roomKey` 隔离。不要只依据 SQLite `user_id` 判断消息方向，因为它可能指向主播；除非受控测试确认更可靠来源，否则沿用既有的显示名称判定逻辑。

实现功能变更前，先阅读 `AGENTS.md` 以及与本次任务相关的最新交接或计划文档。历史计划可能包含已删除旧版前端的引用；以当前源码和当前行为为准。

Spend time on thinking; you do not need to use the commentary channel to report progress to me.

DO NOT send optional commentary.
