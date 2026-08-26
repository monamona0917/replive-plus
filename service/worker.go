package service

import (
	"math/rand/v2"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
)

type syncWorker struct {
	Name     string
	Handle   func() error
	Interval func() time.Duration
}

func (w *syncWorker) run() error {
	defer func() {
		if err := recover(); err != nil {
			hlog.Errorf("sync %v panic, err: %v", w.Name, err)
		}
	}()
	return w.Handle()
}

func (w *syncWorker) Start() {
	go func() {
		hlog.Infof("start worker: %s", w.Name)
		time.Sleep(w.Interval())
		for {
			if err := w.run(); err != nil {
				hlog.Errorf("sync %v failed, err: %v", w.Name, err)
			}
			time.Sleep(w.Interval())
		}
	}()
}

var (
	syncWorkers = []*syncWorker{
		{
			Name:     "refreshNewMessages",
			Handle:   refreshNewMessages,
			Interval: func() time.Duration { return time.Duration(rand.IntN(3900)+3000) * time.Millisecond },
		},
		{
			Name:     "checkChatRoomTimings",
			Handle:   checkChatRoomTimings,
			Interval: func() time.Duration { return 5 * time.Minute },
		},
	}
)

type StartupSummary struct {
	Chat  ChatSyncSummary
	Live  LiveCheckSummary
	Prime PrimeChatStartupSummary
}

func Init() StartupSummary {
	summary := StartupSummary{}
	initEmailSender()
	chatRoomsSummary, err := saveChatRoomsWithSummary()
	if err != nil {
		hlog.Errorf("saveChatRooms failed, err: %v", err)
		panic(err)
	}
	summary.Chat.merge(chatRoomsSummary)
	hlog.Infof("saveChatRooms done, then refresh new")
	newMessagesSummary, err := refreshNewMessagesWithSummaryOptions(false)
	if err != nil {
		hlog.Errorf("refreshNewMessages failed, err: %v", err)
		panic(err)
	}
	summary.Chat.merge(newMessagesSummary)
	if err := syncOshiProfiles(); err != nil {
		hlog.Errorf("syncOshiProfiles failed, err: %v", err)
	}
	primeSummary, err := syncPrimeChatAtStartupWithSummary()
	if err != nil {
		hlog.Errorf("syncPrimeChatAtStartup failed, err: %v", err)
	}
	summary.Prime = primeSummary
	liveSummary, err := CheckLiveAtStartup()
	if err != nil {
		hlog.Errorf("initial live check failed, err: %v", err)
	}
	summary.Live = liveSummary
	return summary
}

func LogStartupSummary(summary StartupSummary) {
	hlog.Infof("================================")
	if summary.Live.QueryFailed {
		hlog.Infof("直播状态检查失败...请检查网络或配置问题(´；ω；｀)")
	} else if summary.Live.HasLive() {
		for _, live := range summary.Live.Lives {
			logNewLive(live)
		}
	} else {
		hlog.Infof("还没有女声优在直播(:3_ヽ)_")
	}

	logChatSyncMessages(summary.Chat)
	if summary.Chat.NewMessages > 0 {
		hlog.Infof("收到 %d 条新消息，其中 %d 张图片", summary.Chat.NewMessages, summary.Chat.NewImageMessages)
	}
	hlog.Infof("================================")
}

func StartBackgroundWorkers() {
	startWorkers()
	StartLiveMonitor()
	ReleaseLiveMonitorStartup()
}

func startWorkers() {
	hlog.Info("StartWorkers")
	for _, worker := range syncWorkers {
		worker.Start()
	}
}
