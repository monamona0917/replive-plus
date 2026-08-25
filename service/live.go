package service

import (
	"context"
	"fmt"
	"math/rand/v2"
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

const (
	LivePollingMinInterval = 5900 * time.Millisecond
	LivePollingMaxInterval = 10 * time.Second
)

type liveMonitorState struct {
	activeLives map[string]struct{}
	initialized bool
	queryFailed bool
}

var liveMonitorOnce sync.Once

// StartLiveMonitor starts the live polling and recording loop in the backend.
func StartLiveMonitor() {
	liveMonitorOnce.Do(func() {
		go func() {
			if err := RunLiveRecorder(context.Background(), 0); err != nil {
				hlog.Errorf("直播监听已停止：%v", err)
			}
		}()
	})
}

func RunLiveRecorder(ctx context.Context, interval time.Duration) error {
	if ctx == nil {
		ctx = context.Background()
	}
	initEmailSender()
	startFfmpegWatcher()
	monitorState := &liveMonitorState{activeLives: make(map[string]struct{})}
	intervalLabel := fmt.Sprintf("%s", interval)
	if interval <= 0 {
		intervalLabel = fmt.Sprintf("%s~%s", LivePollingMinInterval, LivePollingMaxInterval)
	}
	hlog.Infof("正在查看有没有女声优正在直播！（轮询间隔：%s）", intervalLabel)

	poll := func() {
		if err := checkLiveWithState(monitorState); err != nil {
			if !monitorState.queryFailed {
				hlog.Errorf("直播状态查询失败：%v", err)
				monitorState.queryFailed = true
			}
		}
	}
	poll()
	for {
		wait := interval
		if wait <= 0 {
			wait = nextLivePollingInterval()
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return nil
		case <-timer.C:
			poll()
		}
	}
}

func nextLivePollingInterval() time.Duration {
	span := int(LivePollingMaxInterval - LivePollingMinInterval)
	return LivePollingMinInterval + time.Duration(rand.IntN(span+1))
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
}

func checkLive() error {
	return checkLiveWithState(nil)
}

func checkLiveWithState(state *liveMonitorState) error {
	clearLiveCacheIfNeeded(time.Now())
	msgResp, err := getStreamingLive()
	if err != nil {
		return fmt.Errorf("failed to get streaming live: %v", err)
	}
	if msgResp == nil {
		return fmt.Errorf("failed to get streaming live: empty response")
	}
	if state != nil && state.queryFailed {
		hlog.Infof("直播状态查询已恢复")
		state.queryFailed = false
	}

	currentLives := make(map[string]struct{})
	for i, live := range msgResp.LiveInfo {
		if live == nil {
			hlog.Errorf("streaming live response contains nil live at index %d", i)
			continue
		}
		if i >= len(msgResp.UserProfile) {
			hlog.Errorf("streaming live response missing user profile for index %d", i)
			continue
		}
		isFandomOnly := len(live.WebrtcUrl) == 0
		nsyInfo := msgResp.UserProfile[i]
		if nsyInfo == nil || nsyInfo.Info == nil {
			hlog.Errorf("streaming live response missing user profile data for index %d", i)
			continue
		}
		nsyLiveInfo, err := newLiveInfo(live, nsyInfo, time.Now())
		if err != nil {
			hlog.Errorf("failed to build live info, user: %s, err: %v", nsyInfo.Info.DisplayName, err)
			continue
		}
		key := liveRecordKey(nsyLiveInfo)
		currentLives[key] = struct{}{}

		isNewLive := false
		if state != nil {
			_, wasActive := state.activeLives[key]
			isNewLive = !state.initialized || !wasActive
			if isNewLive {
				logNewLive(nsyLiveInfo.Name)
			}
		}
		if isNewLive {
			if _, ok := sendLives.Load(key); !ok {
				sendLiveEmail(live, nsyInfo, nsyLiveInfo.RtmpUrl, isFandomOnly)
				sendLives.Store(key, true)
			}
		}
		if isFandomOnly {
			continue
		}
		if _, exist := knownLives.Load(key); exist {
			continue
		}
		if err := enqueueLiveRecord(nsyLiveInfo); err != nil {
			hlog.Errorf("queue live record failed, name: %s, err: %v", nsyInfo.Info.DisplayName, err)
			continue
		}
		knownLives.Store(key, nsyLiveInfo)
	}

	if state != nil {
		if len(currentLives) == 0 && (!state.initialized || len(state.activeLives) > 0) {
			hlog.Infof("暂无女声优在直播！")
		}
		state.activeLives = currentLives
		state.initialized = true
	}
	return nil
}

func logNewLive(name string) {
	for i := 0; i < 3; i++ {
		hlog.Infof("发现“%s”正在直播！", name)
	}
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
