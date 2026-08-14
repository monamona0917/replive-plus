package service

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"replive/config"
	"replive/utils"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
)

var ffmpegPath string
var ffmpegWatcherOnce sync.Once

func initFfmpeg() {
	if _, err := os.Stat(config.Conf.FfmpegPath); err == nil {
		ffmpegPath = config.Conf.FfmpegPath
	} else if _, err := os.Stat("./ffmpeg.exe"); err == nil {
		ffmpegPath = "./ffmpeg.exe"
	} else if _, err := os.Stat("./ffmpeg"); err == nil {
		ffmpegPath = "./ffmpeg"
	} else {
		if utils.IsWindows() {
			ffmpegPath = "ffmpeg.exe"
		} else {
			ffmpegPath = "ffmpeg"
		}
	}
}

func startFfmpegWatcher() {
	ffmpegWatcherOnce.Do(func() {
		liveRecordChannel = make(chan *NsyLiveInfo, 1000)
		initFfmpeg()
		hlog.Infof("ffmpeg path: %s, Starting ffmpeg watcher...", ffmpegPath)
		go func() {
			for nsyLiveInfo := range liveRecordChannel {
				hlog.Info("start, receive")
				if err := startFfmpegRecord(nsyLiveInfo); err != nil {
					hlog.Errorf("Error starting ffmpeg record: %v", err)
					go func(info *NsyLiveInfo) {
						time.Sleep(5 * time.Second)
						if err := enqueueLiveRecord(info); err != nil {
							hlog.Errorf("Error requeueing ffmpeg record: %v", err)
						}
					}(nsyLiveInfo)
				}
			}
		}()
	})
}

func startFfmpegRecord(nsyLiveInfo *NsyLiveInfo) error {
	defer func() {
		if r := recover(); r != nil {
			hlog.Errorf("Recovered in ffmpeg record: %v", r)
		}
	}()
	baseName := nsyLiveInfo.RecordBaseName
	if baseName == "" {
		baseName = fmt.Sprintf("%s_%s", nsyLiveInfo.Name, time.Now().Format("200601021504"))
	}
	outputFile := baseName + ".mp4"
	if nsyLiveInfo.SegmentIndex > 1 {
		outputFile = fmt.Sprintf("%s_part%02d.mp4", baseName, nsyLiveInfo.SegmentIndex)
	}
	for i := 1; i < 100; i++ {
		if _, err := os.Stat(outputFile); err == nil {
			hlog.Warnf("file %s already exist? err: %v, try %d", outputFile, err, i)
			outputFile = fmt.Sprintf("%s_retry_%d.mp4", strings.TrimSuffix(outputFile, ".mp4"), i)
		} else {
			break
		}
	}
	path := config.GetLiveMonthPath(time.Now())
	logFileName := strings.TrimSuffix(outputFile, ".mp4") + ".txt"
	if len(path) > 0 {
		_ = os.MkdirAll(path, 0755)
		outputFile = path + "/" + outputFile
		logFileName = path + "/" + logFileName
	}
	logFile, err := os.Create(logFileName)
	if err != nil {
		return fmt.Errorf("创建日志文件失败: %v", err)
	}
	cmd := exec.Command(ffmpegPath, buildFfmpegRecordArgs(nsyLiveInfo.RtmpUrl, outputFile)...)
	hlog.Infof("start recording by command: %s", cmd.String())
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("ffmpeg start failed: %v", err)
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				hlog.Errorf("Recovered in ffmpeg record: %v", r)
			}
		}()
		defer logFile.Close()
		hlog.Infof("ffmpeg 日志: %s", logFile.Name())

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()

		hlog.Infof("[%s] 录制开始:", outputFile)

		for {
			select {
			case <-ticker.C:
				hlog.Infof("[%s] 录制中...日志: %s", outputFile, logFile.Name())
			case err := <-done: // 进程结束处理
				hlog.Infof("[%s] 录制结束:", outputFile)
				if err != nil {
					hlog.Errorf("[%s] 录制错误: %v", outputFile, err)
				}
				nextInfo, stillLive, checkErr := getResumeLiveInfo(nsyLiveInfo)
				if checkErr != nil {
					hlog.Errorf("[%s] check live after ffmpeg exit failed: %v", outputFile, checkErr)
					scheduleLiveResume(cloneLiveInfoForResume(nsyLiveInfo), resumeDelay(nsyLiveInfo.SegmentIndex+1))
					return
				}
				if stillLive {
					hlog.Warnf("[%s] ffmpeg exited while live still active, resume recording", outputFile)
					scheduleLiveResume(nextInfo, resumeDelay(nextInfo.SegmentIndex))
					return
				}
				knownLives.Delete(liveRecordKey(nsyLiveInfo))
				sendLiveEndEmail(nsyLiveInfo.Name, outputFile)
				return
			}
		}
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
