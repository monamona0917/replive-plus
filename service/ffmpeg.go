package service

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"replive/config"
	"replive/utils"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
)

var ffmpegPath string
var ffmpegPathAvailable bool
var ffmpegPathFailure string
var ffmpegWatcherOnce sync.Once

const maxFfmpegResumeAttempts = 3

type ffmpegStartFailure struct {
	message string
}

func (e *ffmpegStartFailure) Error() string {
	return e.message
}

func initFfmpeg() {
	configuredPath := strings.TrimSpace(config.Conf.FfmpegPath)
	if configuredPath != "" {
		info, err := os.Stat(configuredPath)
		if err == nil && !info.IsDir() {
			ffmpegPath = configuredPath
			ffmpegPathAvailable = true
			return
		}
		ffmpegPathFailure = "ffmpeg 启动失败，请在 config.yaml 中配置正确路径后重启后端"
		return
	}
	if _, err := os.Stat("./ffmpeg.exe"); err == nil {
		ffmpegPath = "./ffmpeg.exe"
		ffmpegPathAvailable = true
	} else if _, err := os.Stat("./ffmpeg"); err == nil {
		ffmpegPath = "./ffmpeg"
		ffmpegPathAvailable = true
	} else {
		if utils.IsWindows() {
			ffmpegPath = "ffmpeg.exe"
		} else {
			ffmpegPath = "ffmpeg"
		}
		if _, err := exec.LookPath(ffmpegPath); err == nil {
			ffmpegPathAvailable = true
		} else {
			ffmpegPathFailure = "ffmpeg 启动失败，请在 config.yaml 中配置正确路径后重启后端"
		}
	}
}

func startFfmpegWatcher() {
	ffmpegWatcherOnce.Do(func() {
		liveRecordChannel = make(chan *NsyLiveInfo, 1000)
		initFfmpeg()
		go func() {
			for nsyLiveInfo := range liveRecordChannel {
				if err := startFfmpegRecord(nsyLiveInfo); err != nil {
					logFfmpegStartFailure(err)
				}
			}
		}()
	})
}

var ffmpegFailureLogOnce sync.Once

func logFfmpegStartFailure(err error) {
	ffmpegFailureLogOnce.Do(func() {
		if ffmpegPathFailure != "" {
			hlog.Errorf("%s", ffmpegPathFailure)
			return
		}
		var startFailure *ffmpegStartFailure
		if errors.As(err, &startFailure) {
			hlog.Errorf("%s", startFailure.message)
			return
		}
		hlog.Errorf("ffmpeg 启动失败，请检查录像目录、文件权限和本地运行环境")
	})
}

func startFfmpegRecord(nsyLiveInfo *NsyLiveInfo) error {
	defer func() {
		if r := recover(); r != nil {
			hlog.Errorf("ffmpeg 录像任务发生异常，请检查本地环境")
		}
	}()
	if nsyLiveInfo == nil {
		return &ffmpegStartFailure{message: "ffmpeg 启动失败，请检查录像任务配置"}
	}
	if !ffmpegPathAvailable {
		return &ffmpegStartFailure{message: ffmpegPathFailure}
	}
	baseName := nsyLiveInfo.RecordBaseName
	if baseName == "" {
		baseName = fmt.Sprintf("%s_%s", nsyLiveInfo.Name, time.Now().Format("200601021504"))
	}
	outputName := baseName + ".mp4"
	if nsyLiveInfo.SegmentIndex > 1 {
		outputName = fmt.Sprintf("%s_part%02d.mp4", baseName, nsyLiveInfo.SegmentIndex)
	}
	path := config.GetLiveMonthPath(time.Now())
	if len(path) > 0 {
		_ = os.MkdirAll(path, 0755)
	}
	outputFile := filepath.Join(path, outputName)
	if path == "" {
		outputFile = outputName
	}
	for i := 1; i < 100; i++ {
		if _, err := os.Stat(outputFile); err == nil {
			outputFile = fmt.Sprintf("%s_retry_%d.mp4", strings.TrimSuffix(outputFile, ".mp4"), i)
		} else {
			break
		}
	}
	logFileName := strings.TrimSuffix(outputFile, ".mp4") + ".txt"
	logFile, err := os.Create(logFileName)
	if err != nil {
		return &ffmpegStartFailure{message: "ffmpeg 启动失败，请检查录像目录、文件权限和本地运行环境"}
	}
	cmd := exec.Command(ffmpegPath, buildFfmpegRecordArgs(nsyLiveInfo.RtmpUrl, outputFile)...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return &ffmpegStartFailure{message: "ffmpeg 启动失败，请检查录像目录、文件权限和本地运行环境"}
	}
	if nsyLiveInfo.ResumeAttempts > 0 {
		hlog.Infof("重启 ffmpeg 成功，录像将会继续保存为另一个分段...")
	} else {
		hlog.Infof("开始录制“%s”的直播！", nsyLiveInfo.Name)
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				hlog.Errorf("ffmpeg 录像任务发生异常，请检查本地环境")
			}
		}()
		defer logFile.Close()

		waitErr := cmd.Wait()

		if waitErr != nil {
			hlog.Infof("直播录制意外中断，正在确认直播状态...")
		}
		nextInfo, stillLive, checkErr := getResumeLiveInfo(nsyLiveInfo)
		if checkErr != nil {
			logLiveStatusFailure(checkErr)
			if nsyLiveInfo.ResumeAttempts < maxFfmpegResumeAttempts {
				scheduleLiveResume(cloneLiveInfoForResume(nsyLiveInfo), resumeDelay(nsyLiveInfo.SegmentIndex+1))
			} else {
				knownLives.Delete(liveRecordKey(nsyLiveInfo))
				hlog.Errorf("录制恢复失败，请检查网络、磁盘空间和本地运行环境")
			}
			return
		}
		if stillLive {
			if waitErr == nil {
				hlog.Infof("直播录制意外中断，正在确认直播状态...")
			}
			if nsyLiveInfo.ResumeAttempts >= maxFfmpegResumeAttempts {
				knownLives.Delete(liveRecordKey(nsyLiveInfo))
				hlog.Errorf("录制恢复失败，请检查网络、磁盘空间和本地运行环境")
				return
			}
			hlog.Infof("ffmpeg 意外退出，正在尝试重新启动...")
			scheduleLiveResume(nextInfo, resumeDelay(nextInfo.SegmentIndex))
			return
		}
		knownLives.Delete(liveRecordKey(nsyLiveInfo))
		hlog.Infof("%s的直播结束啦！", nsyLiveInfo.Name)
		sendLiveEndEmail(nsyLiveInfo.Name, outputFile)
	}()
	return nil
}

func buildFfmpegRecordArgs(inputURL, outputFile string) []string {
	args := []string{
		"-nostdin",
		"-rw_timeout", "30000000",
	}
	if supportsFfmpegReconnect(inputURL) {
		args = append(args,
			"-reconnect", "1",
			"-reconnect_streamed", "1",
			"-reconnect_at_eof", "1",
			"-reconnect_on_network_error", "1",
			"-reconnect_delay_max", "10",
		)
	}
	args = append(args,
		"-i", inputURL,
		"-c", "copy",
		outputFile,
	)
	return args
}

func supportsFfmpegReconnect(inputURL string) bool {
	u, err := url.Parse(inputURL)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return true
	default:
		return false
	}
}
