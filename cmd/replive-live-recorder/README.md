# Replive Live Recorder

这是独立的直播状态轮询和 ffmpeg 录像进程。主后端不会再启动它，也不会再请求直播状态接口。

先使用主程序完成一次登录，使同目录的 `config.yaml` 包含 `refresh_token`。构建后单独运行：

```powershell
.\replive-live-recorder.exe
```

它复用 `config.yaml` 中的代理、媒体目录、ffmpeg 与邮件设置。默认每 2 秒检查一次直播；可按需调整：

```powershell
.\replive-live-recorder.exe -interval 5s
```

按 `Ctrl+C` 停止轮询进程。
