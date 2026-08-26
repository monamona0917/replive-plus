# Replive Live Recorder

这是独立的直播状态轮询和 ffmpeg 录像进程。主后端启动时也会自动执行同样的直播监听；只有需要单独运行录像进程时才使用本目录的可执行文件。

先使用主程序完成一次登录，使同目录的 `config.yaml` 包含 `refresh_token`。构建后单独运行：

```powershell
.\replive-live-recorder.exe
```

它复用 `config.yaml` 中的代理、媒体目录、ffmpeg 与邮件设置。默认以 `5.9` 到 `10` 秒之间的随机间隔检查直播；可按需指定固定间隔：

```powershell
.\replive-live-recorder.exe -interval 5s
```

按 `Ctrl+C` 停止轮询进程。
