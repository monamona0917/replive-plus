package service

import (
	"fmt"
	"replive/model"
	"sync"
	"testing"
	"time"
)

func TestClearLiveCacheIfNeededClearsOncePerDay(t *testing.T) {
	oldDay := liveCacheClearDay
	t.Cleanup(func() {
		liveCacheClearDay = oldDay
		knownLives = syncMapWithNoData()
		sendLives = syncMapWithNoData()
	})

	knownLives = syncMapWithEntry("live", true)
	sendLives = syncMapWithEntry("mail", true)
	liveCacheClearDay = ""

	now := time.Date(2026, 4, 30, 4, 0, 0, 0, time.Local)
	clearLiveCacheIfNeeded(now)

	if _, ok := knownLives.Load("live"); ok {
		t.Fatal("knownLives was not cleared")
	}
	if _, ok := sendLives.Load("mail"); ok {
		t.Fatal("sendLives was not cleared")
	}
	if liveCacheClearDay != "2026-04-30" {
		t.Fatalf("liveCacheClearDay = %q, want %q", liveCacheClearDay, "2026-04-30")
	}

	knownLives.Store("live2", true)
	clearLiveCacheIfNeeded(now.Add(10 * time.Minute))
	if _, ok := knownLives.Load("live2"); !ok {
		t.Fatal("cache was cleared more than once on the same day")
	}
}

func TestClearLiveCacheIfNeededDoesNotClearAtOtherHours(t *testing.T) {
	oldDay := liveCacheClearDay
	t.Cleanup(func() {
		liveCacheClearDay = oldDay
		knownLives = syncMapWithNoData()
		sendLives = syncMapWithNoData()
	})

	knownLives = syncMapWithEntry("live", true)
	sendLives = syncMapWithEntry("mail", true)
	liveCacheClearDay = ""

	clearLiveCacheIfNeeded(time.Date(2026, 4, 30, 3, 59, 0, 0, time.Local))

	if _, ok := knownLives.Load("live"); !ok {
		t.Fatal("knownLives was cleared outside the target hour")
	}
	if _, ok := sendLives.Load("mail"); !ok {
		t.Fatal("sendLives was cleared outside the target hour")
	}
	if liveCacheClearDay != "" {
		t.Fatalf("liveCacheClearDay = %q, want empty", liveCacheClearDay)
	}
}

func TestGetResumeLiveInfoFromResponseUsesFreshLiveInfo(t *testing.T) {
	current := &NsyLiveInfo{
		LiveStream: &model.LiveStream{
			LiveId: "live-1",
			UserId: "user-1",
			Title:  "old",
		},
		Name:           "tester",
		RtmpUrl:        "rtmp://old",
		RecordBaseName: "tester_202604301200",
		SegmentIndex:   1,
	}

	resp := &model.CheckStreamLiveResponse{
		LiveInfo: []*model.LiveStream{{
			LiveId:    "live-1",
			UserId:    "user-1",
			Title:     "new",
			WebrtcUrl: "https://example.com/live?txSecret=abc&txTime=def",
		}},
		UserProfile: []*model.LiveUser{{
			Info: &model.UserProfile{DisplayName: "tester"},
		}},
	}

	next, stillLive, err := getResumeLiveInfoFromResponse(current, resp)
	if err != nil {
		t.Fatalf("getResumeLiveInfoFromResponse() error = %v", err)
	}
	if !stillLive {
		t.Fatal("expected live to still be active")
	}
	if next == nil {
		t.Fatal("expected resumed live info")
	}
	if next.RecordBaseName != current.RecordBaseName {
		t.Fatalf("RecordBaseName = %q, want %q", next.RecordBaseName, current.RecordBaseName)
	}
	if next.SegmentIndex != 2 {
		t.Fatalf("SegmentIndex = %d, want 2", next.SegmentIndex)
	}
	if next.RtmpUrl != "rtmp://example.com/live?txSecret=abc&txTime=def" {
		t.Fatalf("RtmpUrl = %q", next.RtmpUrl)
	}
}

func TestGetResumeLiveInfoFromResponseFallsBackWhenWebrtcMissing(t *testing.T) {
	current := &NsyLiveInfo{
		LiveStream: &model.LiveStream{
			LiveId: "live-2",
			UserId: "user-2",
		},
		Name:           "tester",
		RtmpUrl:        "rtmp://old",
		RecordBaseName: "tester_202604301200",
		SegmentIndex:   2,
	}

	resp := &model.CheckStreamLiveResponse{
		LiveInfo: []*model.LiveStream{{
			LiveId: "live-2",
			UserId: "user-2",
		}},
		UserProfile: []*model.LiveUser{{
			Info: &model.UserProfile{DisplayName: "tester"},
		}},
	}

	next, stillLive, err := getResumeLiveInfoFromResponse(current, resp)
	if err != nil {
		t.Fatalf("getResumeLiveInfoFromResponse() error = %v", err)
	}
	if !stillLive {
		t.Fatal("expected live to still be active")
	}
	if next == nil {
		t.Fatal("expected resumed live info")
	}
	if next.SegmentIndex != 3 {
		t.Fatalf("SegmentIndex = %d, want 3", next.SegmentIndex)
	}
	if next.RtmpUrl != current.RtmpUrl {
		t.Fatalf("RtmpUrl = %q, want %q", next.RtmpUrl, current.RtmpUrl)
	}
}

func TestBuildLiveInfoWithStateClassifiesLiveResults(t *testing.T) {
	now := time.Date(2026, 8, 26, 3, 0, 0, 0, time.UTC)
	profile := &model.LiveUser{Info: &model.UserProfile{DisplayName: "tester"}}

	fandomInfo, fandomState, err := buildLiveInfoWithState(&model.LiveStream{Title: "fandom"}, profile, now)
	if err != nil {
		t.Fatalf("FandomOnly build error = %v", err)
	}
	if fandomState != FandomOnly || fandomInfo.Name != "tester" || fandomInfo.RtmpUrl != "" {
		t.Fatalf("FandomOnly result = %#v, state=%v", fandomInfo, fandomState)
	}

	readyInfo, readyState, err := buildLiveInfoWithState(&model.LiveStream{
		Title:     "ready",
		WebrtcUrl: "https://example.com/live?txSecret=abc&txTime=def",
	}, profile, now)
	if err != nil {
		t.Fatalf("RecordingReady build error = %v", err)
	}
	if readyState != RecordingReady || readyInfo.RtmpUrl != "rtmp://example.com/live?txSecret=abc&txTime=def" {
		t.Fatalf("RecordingReady result = %#v, state=%v", readyInfo, readyState)
	}

	failedInfo, failedState, err := buildLiveInfoWithState(&model.LiveStream{
		Title:     "invalid",
		WebrtcUrl: "not-a-valid-url",
	}, profile, now)
	if err == nil {
		t.Fatal("RtmpParseFailed build error = nil")
	}
	if failedState != RtmpParseFailed || failedInfo.Name != "tester" || failedInfo.RtmpUrl != "" {
		t.Fatalf("RtmpParseFailed result = %#v, state=%v", failedInfo, failedState)
	}
}

func TestLiveRecordKeyPrefersStableIdentifiers(t *testing.T) {
	info := &NsyLiveInfo{LiveStream: &model.LiveStream{LiveId: "live-3", UserId: "user-3"}, RtmpUrl: "rtmp://x"}
	if got := liveRecordKey(info); got != "live:live-3" {
		t.Fatalf("liveRecordKey() = %q", got)
	}

	info.LiveId = ""
	if got := liveRecordKey(info); got != "user:user-3" {
		t.Fatalf("liveRecordKey() = %q", got)
	}

	info.UserId = ""
	if got := liveRecordKey(info); got != fmt.Sprintf("rtmp:%s", info.RtmpUrl) {
		t.Fatalf("liveRecordKey() = %q", got)
	}
}

func syncMapWithEntry(key string, value any) sync.Map {
	var m sync.Map
	m.Store(key, value)
	return m
}

func syncMapWithNoData() sync.Map {
	return sync.Map{}
}
