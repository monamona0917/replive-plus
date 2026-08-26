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
	ResumeAttempts int
}

type LiveSummary struct {
	Name    string
	Title   string
	State   LiveState
	RtmpURL string
}

type LiveState int

const (
	FandomOnly LiveState = iota
	RecordingReady
	RtmpParseFailed
)

type LiveCheckSummary struct {
	Lives       []LiveSummary
	QueryFailed bool
}

func (s LiveCheckSummary) HasLive() bool {
	return len(s.Lives) > 0
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
	activeLives  map[string]struct{}
	pendingLives []*NsyLiveInfo
	initialized  bool
	queryFailed  bool
}

var liveMonitorOnce sync.Once
var liveMonitorStartupReleaseOnce sync.Once
var liveMonitorStartupRelease = make(chan struct{})

var preparedLiveMonitor struct {
	sync.Mutex
	state *liveMonitorState
}

// StartLiveMonitor starts the live polling and recording loop in the backend.
func StartLiveMonitor() {
	liveMonitorOnce.Do(func() {
		state := takePreparedLiveMonitorState()
		startFfmpegWatcher()
		hlog.Infof("继续观察有没有女声优正在直播(:3_ヽ)_")
		go func() {
			<-liveMonitorStartupRelease
			if err := runLiveRecorderLoop(context.Background(), 0, state, state != nil, false); err != nil {
				hlog.Errorf("直播监听已停止：%v", err)
			}
		}()
	})
}

func CheckLiveAtStartup() (LiveCheckSummary, error) {
	initEmailSender()
	startFfmpegWatcher()
	state := &liveMonitorState{activeLives: make(map[string]struct{})}
	summary, err := checkLiveWithStateOptions(state, false, false)
	if err != nil {
		summary.QueryFailed = true
		logLiveStatusFailure(err)
		state.queryFailed = true
	}
	preparedLiveMonitor.Lock()
	preparedLiveMonitor.state = state
	preparedLiveMonitor.Unlock()
	return summary, err
}

func takePreparedLiveMonitorState() *liveMonitorState {
	preparedLiveMonitor.Lock()
	defer preparedLiveMonitor.Unlock()
	state := preparedLiveMonitor.state
	preparedLiveMonitor.state = nil
	return state
}

func RunLiveRecorder(ctx context.Context, interval time.Duration) error {
	return runLiveRecorderLoop(ctx, interval, takePreparedLiveMonitorState(), false, true)
}

func ReleaseLiveMonitorStartup() {
	liveMonitorStartupReleaseOnce.Do(func() {
		close(liveMonitorStartupRelease)
	})
}

func runLiveRecorderLoop(ctx context.Context, interval time.Duration, monitorState *liveMonitorState, preparedByStartup bool, announce bool) error {
	if ctx == nil {
		ctx = context.Background()
	}
	initEmailSender()
	startFfmpegWatcher()
	if monitorState == nil {
		monitorState = &liveMonitorState{activeLives: make(map[string]struct{})}
	}
	if announce {
		hlog.Infof("继续观察有没有女声优正在直播(:3_ヽ)_")
	}
	if preparedByStartup {
		queued := make(map[string]struct{}, len(monitorState.pendingLives))
		for _, live := range monitorState.pendingLives {
			if live == nil {
				continue
			}
			key := liveRecordKey(live)
			if _, exists := queued[key]; exists {
				continue
			}
			if err := enqueueLiveRecord(live); err != nil {
				hlog.Errorf("直播录制任务启动失败，请检查本地环境")
				continue
			}
			queued[key] = struct{}{}
			knownLives.Store(key, live)
		}
		monitorState.pendingLives = nil
	}

	poll := func() {
		if err := checkLiveWithState(monitorState); err != nil {
			logLiveStatusFailure(err)
			monitorState.queryFailed = true
		}
	}
	if !preparedByStartup {
		poll()
	}
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
	info, _, err := buildLiveInfoWithState(live, nsyInfo, now)
	if err != nil {
		return nil, err
	}
	return info, nil
}

func buildLiveInfoWithState(live *model.LiveStream, nsyInfo *model.LiveUser, now time.Time) (*NsyLiveInfo, LiveState, error) {
	if live == nil || nsyInfo == nil || nsyInfo.Info == nil {
		return nil, RtmpParseFailed, fmt.Errorf("live or user profile is missing")
	}
	info := &NsyLiveInfo{
		LiveStream:     live,
		Name:           nsyInfo.Info.DisplayName,
		RecordBaseName: fmt.Sprintf("%s_%s", nsyInfo.Info.DisplayName, now.Format("200601021504")),
		SegmentIndex:   1,
	}
	if live.WebrtcUrl == "" {
		return info, FandomOnly, nil
	}
	rtmpUrl := ""
	parsedRtmpUrl, err := parseRtmpUrl(live.WebrtcUrl)
	if err != nil {
		return info, RtmpParseFailed, fmt.Errorf("failed to parse url: %w", err)
	}
	rtmpUrl = parsedRtmpUrl
	info.RtmpUrl = rtmpUrl
	return info, RecordingReady, nil
}

func cloneLiveInfoForResume(current *NsyLiveInfo) *NsyLiveInfo {
	if current == nil {
		return nil
	}
	next := *current
	next.SegmentIndex++
	next.ResumeAttempts++
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
		nextInfo.ResumeAttempts = current.ResumeAttempts + 1
		return nextInfo, true, nil
	}
	return nil, false, nil
}

func getResumeLiveInfo(current *NsyLiveInfo) (*NsyLiveInfo, bool, error) {
	msgResp, err := getStreamingLive()
	if err != nil {
		return nil, false, err
	}
	if msgResp == nil {
		return nil, false, fmt.Errorf("streaming live response is empty")
	}
	logLiveStatusRecovered()
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
			hlog.Errorf("直播录制任务启动失败，请检查本地环境")
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
	_, err := checkLiveWithStateOptions(state, true, true)
	return err
}

func checkLiveWithStateOptions(state *liveMonitorState, emitStateLogs bool, queueRecording bool) (LiveCheckSummary, error) {
	summary := LiveCheckSummary{Lives: make([]LiveSummary, 0)}
	clearLiveCacheIfNeeded(time.Now())
	msgResp, err := getStreamingLive()
	if err != nil {
		return summary, fmt.Errorf("failed to get streaming live: %v", err)
	}
	if msgResp == nil {
		return summary, fmt.Errorf("failed to get streaming live: empty response")
	}
	if state != nil && state.queryFailed && emitStateLogs {
		logLiveStatusRecovered()
		state.queryFailed = false
	} else if state != nil && state.queryFailed {
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
		nsyInfo := msgResp.UserProfile[i]
		if nsyInfo == nil || nsyInfo.Info == nil {
			hlog.Errorf("streaming live response missing user profile data for index %d", i)
			continue
		}
		nsyLiveInfo, liveState, buildErr := buildLiveInfoWithState(live, nsyInfo, time.Now())
		if buildErr != nil && liveState != RtmpParseFailed {
			hlog.Errorf("failed to build live info, user: %s, err: %v", nsyInfo.Info.DisplayName, buildErr)
			continue
		}
		summary.Lives = append(summary.Lives, LiveSummary{
			Name:    nsyLiveInfo.Name,
			Title:   live.Title,
			State:   liveState,
			RtmpURL: nsyLiveInfo.RtmpUrl,
		})
		key := liveRecordKey(nsyLiveInfo)
		currentLives[key] = struct{}{}

		isNewLive := false
		if state != nil {
			_, wasActive := state.activeLives[key]
			isNewLive = !state.initialized || !wasActive
			if isNewLive && emitStateLogs {
				logNewLive(summary.Lives[len(summary.Lives)-1])
			}
		}
		if isNewLive {
			if _, ok := sendLives.Load(key); !ok {
				sendLiveEmail(live, nsyInfo, nsyLiveInfo.RtmpUrl, liveState == FandomOnly)
				sendLives.Store(key, true)
			}
		}
		if liveState != RecordingReady {
			continue
		}
		if !queueRecording {
			if state != nil {
				state.pendingLives = append(state.pendingLives, nsyLiveInfo)
			}
			continue
		}
		if _, exist := knownLives.Load(key); exist {
			continue
		}
		if err := enqueueLiveRecord(nsyLiveInfo); err != nil {
			hlog.Errorf("直播录制任务启动失败，请检查本地环境")
			continue
		}
		knownLives.Store(key, nsyLiveInfo)
	}

	if state != nil {
		if emitStateLogs && len(currentLives) == 0 && (!state.initialized || len(state.activeLives) > 0) {
			hlog.Infof("还没有女声优在直播(:3_ヽ)_")
		}
		state.activeLives = currentLives
		state.initialized = true
	}
	return summary, nil
}

var liveStatusLogState struct {
	sync.Mutex
	failed bool
}

func logLiveStatusFailure(_ error) {
	liveStatusLogState.Lock()
	if liveStatusLogState.failed {
		liveStatusLogState.Unlock()
		return
	}
	liveStatusLogState.failed = true
	liveStatusLogState.Unlock()
	hlog.Errorf("直播状态检查失败...请检查网络或配置问题(´；ω；｀)")
}

func logLiveStatusRecovered() {
	liveStatusLogState.Lock()
	wasFailed := liveStatusLogState.failed
	liveStatusLogState.failed = false
	liveStatusLogState.Unlock()
	if wasFailed {
		hlog.Infof("直播检测已恢复⸜(ˊᗜˋ)⸝")
	}
}

func logNewLive(live LiveSummary) {
	hlog.Infof("发现%s正在直播！⸜(ˊᗜˋ)⸝", live.Name)
	hlog.Infof("直播标题：%s", live.Title)
	switch live.State {
	case FandomOnly:
		hlog.Infof("这次直播属于Fandom only，但你还没有加入她的Fandom(´；ω；｀)")
	case RecordingReady:
		hlog.Infof("已获取录制地址：RTMP 地址：%s", live.RtmpURL)
	case RtmpParseFailed:
		hlog.Infof("RTMP 地址解析失败，请检查本地环境")
	}
}

func parseRtmpUrl(webrtcUrl string) (string, error) {
	u, err := url.Parse(webrtcUrl)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("WebrtcUrl 格式无效")
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
