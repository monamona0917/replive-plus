package service

import (
	"context"
	"fmt"
	"net/url"
	"replive/model"
	"replive/rep_api"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
)

type NsyLiveInfo struct {
	*model.LiveStream
	Name           string
	RtmpUrl        string
	RecordBaseName string
	SegmentIndex   int
}

var (
	knownLives        sync.Map
	sendLives         sync.Map
	liveRecordChannel chan *NsyLiveInfo
	liveCacheClearMu  sync.Mutex
	liveCacheClearDay string
	getStreamingLive  = rep_api.GetStreamingLive
)

// RunLiveRecorder owns live-status polling and ffmpeg recording. It is kept
// outside the normal chat worker lifecycle so the main backend does not poll
// live status unless this standalone worker is started.
func RunLiveRecorder(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	initEmailSender()
	startFfmpegWatcher()
	hlog.Infof("live recorder started, polling interval: %s", interval)

	if err := checkLive(); err != nil {
		hlog.Errorf("initial live check failed: %v", err)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			hlog.Infof("live recorder stopped")
			return nil
		case <-ticker.C:
			if err := checkLive(); err != nil {
				hlog.Errorf("live check failed: %v", err)
			}
		}
	}
}

func enqueueLiveRecord(nsyLiveInfo *NsyLiveInfo) error {
	if liveRecordChannel == nil {
		return fmt.Errorf("ffmpeg watcher not started")
	}
	select {
	case liveRecordChannel <- nsyLiveInfo:
		return nil
	default:
		return fmt.Errorf("live record channel full")
	}
}

func liveRecordKey(nsyLiveInfo *NsyLiveInfo) string {
	if nsyLiveInfo == nil || nsyLiveInfo.LiveStream == nil {
		return ""
	}
	if nsyLiveInfo.LiveId != "" {
		return "live:" + nsyLiveInfo.LiveId
	}
	if nsyLiveInfo.UserId != "" {
		return "user:" + nsyLiveInfo.UserId
	}
	return "rtmp:" + nsyLiveInfo.RtmpUrl
}

func newLiveInfo(live *model.LiveStream, nsyInfo *model.LiveUser, now time.Time) (*NsyLiveInfo, error) {
	rtmpUrl, err := parseRtmpUrl(live.WebrtcUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse url: %v", err)
	}
	return &NsyLiveInfo{
		LiveStream:     live,
		Name:           nsyInfo.Info.DisplayName,
		RtmpUrl:        rtmpUrl,
		RecordBaseName: fmt.Sprintf("%s_%s", nsyInfo.Info.DisplayName, now.Format("200601021504")),
		SegmentIndex:   1,
	}, nil
}

func cloneLiveInfoForResume(current *NsyLiveInfo) *NsyLiveInfo {
	if current == nil {
		return nil
	}
	next := *current
	next.SegmentIndex++
	return &next
}

func matchesActiveLive(current *NsyLiveInfo, live *model.LiveStream) bool {
	if current == nil || current.LiveStream == nil || live == nil {
		return false
	}
	if current.LiveId != "" && live.LiveId == current.LiveId {
		return true
	}
	if current.UserId != "" && live.UserId == current.UserId {
		return true
	}
	return false
}

func getResumeLiveInfoFromResponse(current *NsyLiveInfo, msgResp *model.CheckStreamLiveResponse) (*NsyLiveInfo, bool, error) {
	if current == nil || msgResp == nil {
		return nil, false, nil
	}
	for i, live := range msgResp.LiveInfo {
		if !matchesActiveLive(current, live) {
			continue
		}
		if i >= len(msgResp.UserProfile) {
			return nil, false, fmt.Errorf("live response user profile missing for index %d", i)
		}
		if live.WebrtcUrl == "" {
			return cloneLiveInfoForResume(current), true, nil
		}
		nextInfo, err := newLiveInfo(live, msgResp.UserProfile[i], time.Now())
		if err != nil {
			return nil, false, err
		}
		nextInfo.RecordBaseName = current.RecordBaseName
		nextInfo.SegmentIndex = current.SegmentIndex + 1
		return nextInfo, true, nil
	}
	return nil, false, nil
}

func getResumeLiveInfo(current *NsyLiveInfo) (*NsyLiveInfo, bool, error) {
	msgResp, err := getStreamingLive()
	if err != nil {
		return nil, false, err
	}
	return getResumeLiveInfoFromResponse(current, msgResp)
}

func scheduleLiveResume(nsyLiveInfo *NsyLiveInfo, delay time.Duration) {
	if nsyLiveInfo == nil {
		return
	}
	key := liveRecordKey(nsyLiveInfo)
	knownLives.Store(key, nsyLiveInfo)
	go func() {
		time.Sleep(delay)
		if err := enqueueLiveRecord(nsyLiveInfo); err != nil {
			knownLives.Delete(key)
			hlog.Errorf("failed to resume live record, key: %s, err: %v", key, err)
			return
		}
		hlog.Infof("resume live record queued, key: %s, delay: %s, segment: %d", key, delay, nsyLiveInfo.SegmentIndex)
	}()
}

func resumeDelay(segmentIndex int) time.Duration {
	if segmentIndex <= 1 {
		return 5 * time.Second
	}
	delay := time.Duration(segmentIndex*5) * time.Second
	if delay > time.Minute {
		return time.Minute
	}
	return delay
}

func clearLiveCacheIfNeeded(now time.Time) {
	if now.Hour() != 4 {
		return
	}
	day := now.Format("2006-01-02")

	liveCacheClearMu.Lock()
	defer liveCacheClearMu.Unlock()

	if liveCacheClearDay == day {
		return
	}
	knownLives.Clear()
	sendLives.Clear()
	liveCacheClearDay = day
	hlog.Infof("live cache cleared at %s", now.Format("2006-01-02 15:04:05"))
}

func checkLive() error {
	clearLiveCacheIfNeeded(time.Now())
	msgResp, err := rep_api.GetStreamingLive()
	if err != nil {
		return fmt.Errorf("failed to get streaming live: %v", err)
	}
	for i, live := range msgResp.LiveInfo {
		if i >= len(msgResp.UserProfile) {
			hlog.Errorf("streaming live response missing user profile for index %d", i)
			continue
		}
		isFandomOnly := len(live.WebrtcUrl) == 0
		nsyInfo := msgResp.UserProfile[i]
		nsyLiveInfo, err := newLiveInfo(live, nsyInfo, time.Now())
		if err != nil {
			hlog.Errorf("failed to build live info, user: %s, err: %v", nsyInfo.Info.DisplayName, err)
			continue
		}
		key := liveRecordKey(nsyLiveInfo)
		if _, exist := knownLives.Load(key); exist {
			continue
		}
		if _, ok := sendLives.Load(key); !ok {
			sendLiveEmail(live, nsyInfo, nsyLiveInfo.RtmpUrl, isFandomOnly)
			sendLives.Store(key, true)
		}
		if isFandomOnly {
			hlog.Warnf("live %s is fandom only", nsyInfo.Info.DisplayName)
			continue
		}
		hlog.Infof("%s start live, title: %s, \nrtmp url: %s", nsyInfo.Info.DisplayName, live.Title, nsyLiveInfo.RtmpUrl)
		if err := enqueueLiveRecord(nsyLiveInfo); err != nil {
			hlog.Errorf("queue live record failed, name: %s, err: %v", nsyInfo.Info.DisplayName, err)
			continue
		}
		knownLives.Store(key, nsyLiveInfo)
	}
	return nil
}
func parseRtmpUrl(webrtcUrl string) (string, error) {
	u, err := url.Parse(webrtcUrl)
	if err != nil {
		return "", err
	}
	u.Scheme = "rtmp"
	queryParams := u.Query()
	keepParams := map[string]bool{
		"txSecret": true,
		"txTime":   true,
	}
	newQueryParams := url.Values{}
	for key, values := range queryParams {
		if keepParams[key] {
			for _, value := range values {
				newQueryParams.Add(key, value)
			}
		}
	}
	u.RawQuery = newQueryParams.Encode()
	return u.String(), nil
}
