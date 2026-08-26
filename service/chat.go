package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"replive/config"
	"replive/dal"
	"replive/model"
	"replive/rep_api"
	"replive/utils"
	"sort"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"gorm.io/gorm"
)

var (
	chatRoomList    []*dal.ChatRoom = make([]*dal.ChatRoom, 0)
	chatRoomLocker  sync.RWMutex    = sync.RWMutex{}
	getChatRooms                    = rep_api.GetChatRooms
	getChatMessages                 = rep_api.GetChatMessages
	downloadImage                   = DownloadImage
	downloadVideo                   = DownloadVideo
	sendChatEmailFn                 = sendChatEmail
)

type ChatSenderSummary struct {
	NewMessages      int
	NewImageMessages int
}

type ChatSyncSummary struct {
	NewMessages      int
	NewImageMessages int
	DownloadedImages int
	SkippedImages    int
	BySender         map[string]ChatSenderSummary
}

func newChatSyncSummary() ChatSyncSummary {
	return ChatSyncSummary{BySender: make(map[string]ChatSenderSummary)}
}

func (s *ChatSyncSummary) ensureSenderMap() {
	if s.BySender == nil {
		s.BySender = make(map[string]ChatSenderSummary)
	}
}

func (s *ChatSyncSummary) merge(other ChatSyncSummary) {
	if s == nil {
		return
	}
	s.ensureSenderMap()
	s.NewMessages += other.NewMessages
	s.NewImageMessages += other.NewImageMessages
	s.DownloadedImages += other.DownloadedImages
	s.SkippedImages += other.SkippedImages
	for name, sender := range other.BySender {
		current := s.BySender[name]
		current.NewMessages += sender.NewMessages
		current.NewImageMessages += sender.NewImageMessages
		s.BySender[name] = current
	}
}

var chatMessageLogState struct {
	sync.Mutex
	initialized      bool
	hadMessages      bool
	lastNoMessageLog time.Time
}

const noChatMessageLogCooldown = time.Hour

var chatRoomTimingLogState struct {
	sync.Mutex
	values map[string]int64
}

const (
	maxChatMessagePages          = 200
	initialChatMessageFetchTries = 3
	initialChatMessageRetryDelay = 500 * time.Millisecond
)

type chatMessageFetcher func(ctx context.Context, uid, roomId string, cursorMsgId *string, backward bool, size int32) ([]*model.ListChatMessages, string, error)

type initialChatMessageFetchStrategy struct {
	name     string
	backward bool
	size     int32
}

var initialChatMessageFetchStrategies = []initialChatMessageFetchStrategy{
	{name: "latest-one", backward: false, size: 1},
	{name: "latest-page", backward: false, size: 100},
	{name: "older-page", backward: true, size: 100},
	{name: "older-one", backward: true, size: 1},
}

func saveChatRooms() error {
	_, err := saveChatRoomsWithSummary()
	return err
}

func saveChatRoomsWithSummary() (ChatSyncSummary, error) {
	summary := newChatSyncSummary()
	chatRooms, err := getChatRooms()
	if err != nil {
		hlog.Errorf("GetChatRooms failed, err: %v", err)
		return summary, err
	}
	innerChatRooms, err := dal.GetChatRooms()
	if err != nil {
		hlog.Errorf("GetChatRooms failed, err: %v", err)
		return summary, err
	}
	hlog.Infof("GetChatRooms success, room_count: %d", len(chatRooms))
	chatRoomMap := make(map[string]*dal.ChatRoom)
	for _, chatRoom := range innerChatRooms {
		chatRoomMap[chatRoom.ChatRoomId] = chatRoom
	}
	newRooms := make([]*dal.ChatRoom, 0)
	currentRooms := make([]*dal.ChatRoom, 0)
	err = dal.WithWriteDB(func(db *gorm.DB) error {
		for _, chatRoom := range chatRooms {
			if chatRoom == nil || chatRoom.UserProfile == nil {
				hlog.Warnf("Skip malformed Fandom chat room while syncing timing probe")
				continue
			}
			timing, timingErr := rep_api.ParseChatRoomTiming(chatRoom)
			if timingErr != nil {
				hlog.Warnf("Fandom room timing probe parse failed, chatRoomId: %s, err: %v", chatRoom.ChatRoomId, timingErr)
			}
			logChatRoomTimingProbe(chatRoom, timing)
			innerChatRoom := &dal.ChatRoom{
				UserId:              chatRoom.UserId,
				UniqueId:            chatRoom.UserProfile.UniqueId,
				DisplayName:         chatRoom.UserProfile.DisplayName,
				ChatRoomId:          chatRoom.ChatRoomId,
				AvatarUrl:           chatRoom.UserProfile.AvatarUrl,
				TalentLastCheckTime: chatRoomTimingUnix(timing.TalentLastCheckTime, timing.HasTalentLastCheckTime),
			}
			currentRooms = append(currentRooms, innerChatRoom)
			if existing, ok := chatRoomMap[chatRoom.ChatRoomId]; ok {
				updates := changedChatRoomFields(existing, innerChatRoom)
				if len(updates) > 0 {
					if err := db.Model(&dal.ChatRoom{}).
						Where("chat_room_id = ?", innerChatRoom.ChatRoomId).
						Updates(updates).Error; err != nil {
						hlog.Errorf("UpdateChatRoom failed, err: %v", err)
						return err
					}
					hlog.Infof("UpdateChatRoom success, chatRoomId: %v, updates: %v", innerChatRoom.ChatRoomId, updates)
				}
				continue
			}
			if err := db.Create(innerChatRoom).Error; err != nil {
				hlog.Errorf("CreateChatRoom failed, err: %v", err)
				return err
			}
			hlog.Infof("CreateChatRoom success, result: %v", innerChatRoom)
			newRooms = append(newRooms, innerChatRoom)
		}
		return nil
	})
	if err != nil {
		return summary, err
	}
	if len(currentRooms) == 0 {
		chatRoomLocker.Lock()
		chatRoomList = currentRooms
		chatRoomLocker.Unlock()
		hlog.Warnf("chat room subscription list is empty")
		return summary, nil
	}
	shuffleChatRooms(currentRooms)
	shuffleChatRooms(newRooms)
	chatRoomLocker.Lock()
	chatRoomList = currentRooms
	chatRoomLocker.Unlock()
	hasSuccess := len(newRooms) == 0
	for _, chatRoom := range newRooms {
		_, err := refreshOldChatMessagesWithSummary(chatRoom.UserId, chatRoom.ChatRoomId)
		if err != nil {
			hlog.Errorf("refreshOldChatMessages failed, err: %v", err)
			continue
		}
		hasSuccess = true
		hlog.Infof("refreshOldChatMessages done, chatRoomId: %v", chatRoom.ChatRoomId)
	}
	if !hasSuccess {
		return summary, errors.New("refresh all new chat rooms failed")
	}
	return summary, nil
}

// checkChatRoomTimings only checks the server-provided room timestamps. It
// deliberately does not update room records or refresh historical messages.
func checkChatRoomTimings() error {
	chatRooms, err := getChatRooms()
	if err != nil {
		return fmt.Errorf("get Fandom room timing: %w", err)
	}
	for _, chatRoom := range chatRooms {
		if chatRoom == nil || chatRoom.UserProfile == nil {
			continue
		}
		timing, timingErr := rep_api.ParseChatRoomTiming(chatRoom)
		if timingErr != nil {
			hlog.Warnf("Fandom room timing probe parse failed, chatRoomId: %s, err: %v", chatRoom.ChatRoomId, timingErr)
			continue
		}
		logChatRoomTimingProbe(chatRoom, timing)
	}
	return nil
}

func changedChatRoomFields(existing *dal.ChatRoom, next *dal.ChatRoom) map[string]any {
	updates := make(map[string]any)
	if existing.UserId != next.UserId {
		updates["user_id"] = next.UserId
	}
	if existing.UniqueId != next.UniqueId {
		updates["unique_id"] = next.UniqueId
	}
	if existing.DisplayName != next.DisplayName {
		updates["display_name"] = next.DisplayName
	}
	if existing.AvatarUrl != next.AvatarUrl {
		updates["avatar_url"] = next.AvatarUrl
	}
	if existing.TalentLastCheckTime != next.TalentLastCheckTime {
		updates["talent_last_check_time"] = next.TalentLastCheckTime
	}
	return updates
}

func logChatRoomTimingProbe(chatRoom *model.ChatRoom, timing rep_api.ChatRoomTiming) {
	if chatRoom == nil || chatRoom.UserProfile == nil {
		return
	}
	value := chatRoomTimingLogValue(timing.TalentLastCheckTime, timing.HasTalentLastCheckTime)
	if !shouldLogChatRoomTiming(chatRoom.ChatRoomId, value) {
		return
	}
	hlog.Infof(
		"Fandom room timing probe: display_name=%q chat_room_id=%s talent_last_check_time=%s",
		chatRoom.UserProfile.DisplayName,
		chatRoom.ChatRoomId,
		formatChatRoomProbeTime(timing.TalentLastCheckTime, timing.HasTalentLastCheckTime),
	)
}

func shouldLogChatRoomTiming(roomID string, value int64) bool {
	chatRoomTimingLogState.Lock()
	defer chatRoomTimingLogState.Unlock()
	if chatRoomTimingLogState.values == nil {
		chatRoomTimingLogState.values = make(map[string]int64)
	}
	previous, seen := chatRoomTimingLogState.values[roomID]
	chatRoomTimingLogState.values[roomID] = value
	return !seen || previous != value
}

func formatChatRoomProbeTime(value time.Time, present bool) string {
	if !present {
		return "unset"
	}
	return fmt.Sprintf("%s (unix=%d)", value.In(utils.JapanLocation()).Format(time.RFC3339Nano), value.Unix())
}

func chatRoomTimingUnix(value time.Time, present bool) int64 {
	if !present {
		return 0
	}
	return value.Unix()
}

func chatRoomTimingLogValue(value time.Time, present bool) int64 {
	if !present {
		return 0
	}
	return value.UnixNano()
}

func refreshNewMessages() error {
	_, err := refreshNewMessagesWithSummary()
	return err
}

func refreshNewMessagesWithSummary() (ChatSyncSummary, error) {
	return refreshNewMessagesWithSummaryOptions(true)
}

func refreshNewMessagesWithSummaryOptions(emitNotification bool) (ChatSyncSummary, error) {
	summary := newChatSyncSummary()
	ctx := context.Background()
	chatRooms := make([]*dal.ChatRoom, 0)
	chatRoomLocker.RLock()
	chatRooms = append(chatRooms, chatRoomList...)
	chatRoomLocker.RUnlock()
	if len(chatRooms) == 0 {
		hlog.Warnf("chat room subscription list is empty, skip refreshNewMessages")
		return summary, nil
	}
	shuffleChatRooms(chatRooms)

	hasSuccess := false

	if len(chatRooms) == 0 {
		hlog.Warnf("订阅数为空")
		return summary, nil
	}

	for _, chatRoom := range chatRooms {
		roomSummary, err := updateNewMessagesWithSummary(ctx, chatRoom.UserId, chatRoom.ChatRoomId, chatRoom.DisplayName)
		if err != nil {
			hlog.Errorf("[%v]updateNewMessages failed, err: %v", chatRoom.DisplayName, err)
			continue
		} else {
			hasSuccess = true
			summary.merge(roomSummary)
		}
	}
	if !hasSuccess {
		return summary, errors.New("refresh all failed")
	}
	if emitNotification {
		logChatSyncMessages(summary)
	}
	return summary, nil
}

func shuffleChatRooms(chatRooms []*dal.ChatRoom) {
	rand.Shuffle(len(chatRooms), func(i, j int) {
		chatRooms[i], chatRooms[j] = chatRooms[j], chatRooms[i]
	})
}

func updateNewMessages(ctx context.Context, uid, roomId, displayName string) error {
	_, err := updateNewMessagesWithSummary(ctx, uid, roomId, displayName)
	return err
}

func updateNewMessagesWithSummary(ctx context.Context, uid, roomId, displayName string) (ChatSyncSummary, error) {
	summary := newChatSyncSummary()
	existMessages := make([]dal.ChatMessage, 0)
	err := dal.ReadDB().Table(dal.ChatMessage{}.TableName()).
		Where("user_id = ? AND chat_room_id = ?", uid, roomId).
		Order("id desc").
		Limit(1).
		Find(&existMessages).Error
	if err != nil {
		hlog.Errorf("Failed to get chat messages: %v", err)
		return summary, err
	}
	if len(existMessages) == 0 {
		_, err := refreshOldChatMessagesWithSummary(uid, roomId)
		if err != nil {
			return summary, err
		}
		return summary, nil
	}
	totalMsg, err := collectChatMessages(ctx, getChatMessages, uid, roomId, &existMessages[0].ChatMessageId, false, 100)
	if err != nil {
		hlog.Errorf("Failed to get chat messages: %v", err)
		return summary, err
	}
	if len(totalMsg) == 0 {
		return summary, nil
	}
	hlog.Infof("updateNewMessages, uid: %v, roomId: %v, name: %v, msg size: %v", uid, roomId, displayName, len(totalMsg))
	summary, err = saveMessageWithSummary(totalMsg)
	if err != nil {
		hlog.Errorf("Failed to save chat message: %v", err)
		return summary, err
	}
	return summary, nil
}

func refreshOldChatMessages(uid, roomId string) error {
	_, err := refreshOldChatMessagesWithSummary(uid, roomId)
	return err
}

func refreshOldChatMessagesWithSummary(uid, roomId string) (ChatSyncSummary, error) {
	summary := newChatSyncSummary()
	// 1. 先获取最新一条
	ctx := context.Background()
	messages, cursor, err := getInitialChatMessages(ctx, uid, roomId)
	if err != nil {
		hlog.Errorf("Failed to get chat messages: %v", err)
		return summary, err
	}
	if len(messages) == 0 {
		return summary, fmt.Errorf("failed to get initial chat messages, len(messages) == 0, uid: %s, roomId: %s", uid, roomId)
	}
	if cursor == "" {
		return summary, fmt.Errorf("failed to get initial chat messages cursor, uid: %s, roomId: %s, len(messages): %d", uid, roomId, len(messages))
	}
	// 2. 基于初始游标双向拉取，直到向前和向后都确认结束
	totalMsg, err := collectChatMessagesAroundCursor(ctx, uid, roomId, cursor)
	if err != nil {
		hlog.Errorf("Failed to get chat messages around cursor: %v", err)
		return summary, err
	}
	totalMsg = append(messages, totalMsg...)
	// 3. 保存
	summary, err = saveMessageWithSummary(totalMsg)
	if err != nil {
		hlog.Errorf("Failed to save chat message: %v", err)
		return summary, err
	}
	return summary, nil
}

func collectChatMessagesAroundCursor(ctx context.Context, uid, roomId, cursor string) ([]*model.ListChatMessages, error) {
	if cursor == "" {
		return nil, errors.New("cursor message id is empty")
	}
	totalMsg := make([]*model.ListChatMessages, 0)
	olderMessages, err := collectChatMessages(ctx, getChatMessages, uid, roomId, &cursor, true, 100)
	if err != nil {
		return nil, fmt.Errorf("collect older chat messages failed: %w", err)
	}
	totalMsg = append(totalMsg, olderMessages...)

	newerMessages, err := collectChatMessages(ctx, getChatMessages, uid, roomId, &cursor, false, 100)
	if err != nil {
		return nil, fmt.Errorf("collect newer chat messages failed: %w", err)
	}
	totalMsg = append(totalMsg, newerMessages...)
	return totalMsg, nil
}

func getInitialChatMessages(ctx context.Context, uid, roomId string) ([]*model.ListChatMessages, string, error) {
	var messages []*model.ListChatMessages
	var cursor string
	var err error
	for attempt := 1; attempt <= initialChatMessageFetchTries; attempt++ {
		for _, strategy := range initialChatMessageFetchStrategies {
			messages, cursor, err = getChatMessages(ctx, uid, roomId, nil, strategy.backward, strategy.size)
			if err != nil {
				return nil, "", fmt.Errorf("initial chat message fetch failed, strategy=%s backward=%v size=%d: %w", strategy.name, strategy.backward, strategy.size, err)
			}
			if len(messages) > 0 {
				hlog.Infof("GetChatMessages initial page success, uid: %v, roomId: %v, strategy: %v, size: %v, len: %v", uid, roomId, strategy.name, strategy.size, len(messages))
				return messages, cursor, nil
			}
			hlog.Warnf("GetChatMessages returned empty initial page, uid: %v, roomId: %v, strategy: %v, backward: %v, size: %v, attempt: %v", uid, roomId, strategy.name, strategy.backward, strategy.size, attempt)
		}
		if attempt < initialChatMessageFetchTries {
			time.Sleep(initialChatMessageRetryDelay)
		}
	}
	return messages, cursor, nil
}

func collectChatMessages(ctx context.Context, fetcher chatMessageFetcher, uid, roomId string, cursorMsgID *string, backward bool, size int32) ([]*model.ListChatMessages, error) {
	if fetcher == nil {
		return nil, errors.New("chat message fetcher is nil")
	}
	if cursorMsgID == nil || *cursorMsgID == "" {
		return nil, errors.New("cursor message id is empty")
	}

	nextID := *cursorMsgID
	seenCursors := map[string]struct{}{nextID: {}}
	totalMsg := make([]*model.ListChatMessages, 0)

	for page := 0; page < maxChatMessagePages; page++ {
		messages, cursor, err := fetcher(ctx, uid, roomId, &nextID, backward, size)
		if err != nil {
			return nil, err
		}
		if len(messages) == 0 {
			return totalMsg, nil
		}
		totalMsg = append(totalMsg, messages...)

		if cursor == "" {
			hlog.Warnf("collectChatMessages stop: empty cursor, uid=%s roomId=%s backward=%v page=%d", uid, roomId, backward, page+1)
			return totalMsg, nil
		}
		if cursor == nextID {
			hlog.Warnf("collectChatMessages stop: cursor not advancing, uid=%s roomId=%s cursor=%s backward=%v page=%d", uid, roomId, cursor, backward, page+1)
			return totalMsg, nil
		}
		if _, ok := seenCursors[cursor]; ok {
			hlog.Warnf("collectChatMessages stop: repeated cursor, uid=%s roomId=%s cursor=%s backward=%v page=%d", uid, roomId, cursor, backward, page+1)
			return totalMsg, nil
		}
		if samePageAsCursor(messages, nextID) {
			hlog.Warnf("collectChatMessages stop: page still contains current cursor, uid=%s roomId=%s cursor=%s backward=%v page=%d", uid, roomId, nextID, backward, page+1)
			return totalMsg, nil
		}

		seenCursors[cursor] = struct{}{}
		nextID = cursor
	}

	return totalMsg, fmt.Errorf("collect chat messages exceeded max pages: uid=%s roomId=%s backward=%v limit=%d", uid, roomId, backward, maxChatMessagePages)
}

func samePageAsCursor(messages []*model.ListChatMessages, cursor string) bool {
	if cursor == "" {
		return false
	}
	for _, msg := range messages {
		if msg != nil && msg.ChatMessageId == cursor {
			return true
		}
	}
	return false
}

func saveMessage(messages []*model.ListChatMessages) error {
	_, err := saveMessageWithSummary(messages)
	return err
}

func saveMessageWithSummary(messages []*model.ListChatMessages) (ChatSyncSummary, error) {
	summary := newChatSyncSummary()
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].GetTimestamp().GetSeconds() < messages[j].GetTimestamp().GetSeconds()
	})

	savedMessages := make([]*dal.ChatMessage, 0)
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		exists, err := chatMessageExists(msg.GetChatMessageId())
		if err != nil {
			return summary, err
		}
		if exists {
			hlog.Infof("Skip existing chat message, msgId: %v", msg.GetChatMessageId())
			continue
		}

		dbMsg, mediaResult, err := buildChatMessageWithMedia(msg)
		if err != nil {
			return summary, err
		}
		saved, err := saveChatMessageIfAbsent(dbMsg)
		if err != nil {
			return summary, err
		}
		if saved {
			savedMessages = append(savedMessages, dbMsg)
			summary.NewMessages++
			summary.ensureSenderMap()
			sender := summary.BySender[dbMsg.DisplayName]
			sender.NewMessages++
			if dbMsg.MsgType == int32(model.ChatMessageType_Image) {
				summary.NewImageMessages++
				sender.NewImageMessages++
				switch mediaResult {
				case MediaDownloaded:
					summary.DownloadedImages++
				case MediaSkipped:
					summary.SkippedImages++
				}
			}
			summary.BySender[dbMsg.DisplayName] = sender
		}
	}

	for _, msg := range savedMessages {
		sendChatEmailFn(msg)
	}
	return summary, nil
}

func buildChatMessage(msg *model.ListChatMessages) (*dal.ChatMessage, error) {
	dbMsg, _, err := buildChatMessageWithMedia(msg)
	return dbMsg, err
}

func buildChatMessageWithMedia(msg *model.ListChatMessages) (*dal.ChatMessage, MediaResult, error) {
	msgTime := time.Unix(msg.GetTimestamp().GetSeconds(), 0)
	displayName := msg.GetUserProfile().GetDisplayName()
	mediaResult := MediaDownloaded
	dbMsg := &dal.ChatMessage{
		UserId:        msg.GetUserId(),
		DisplayName:   displayName,
		ChatRoomId:    msg.GetChatRoomId(),
		ChatMessageId: msg.GetChatMessageId(),
		MsgType:       msg.GetType(),
		Content:       msg.GetContent(),
		ImageUrl:      msg.GetImageUrl(),
		VideoUrl:      msg.GetVideoUrl(),
		TimeStr:       msg.GetTimeStr(),
		SendTime:      msg.GetTimestamp().GetSeconds(),
	}
	switch msg.GetType() {
	case int32(model.ChatMessageType_Text):
		// do nothing
	case int32(model.ChatMessageType_Image):
		hlog.Infof("DownloadImage, imageUrl: %v, time: %v, path: %v, msgId: %v", msg.GetImageUrl(), msgTime, getMediaPath(displayName), msg.GetChatMessageId())
		imgPath, result, err := downloadChatImage(msg.GetImageUrl(), msgTime, getMediaPath(displayName), msg.GetChatMessageId())
		if err != nil {
			hlog.Errorf("Failed to download image: %v", err)
			return nil, MediaFailed, err
		}
		mediaResult = result
		dbMsg.ImagePath = imgPath
	case int32(model.ChatMessageType_Video):
		hlog.Infof("DownloadVideo, videoUrl: %v, time: %v, path: %v, msgId: %v", msg.GetVideoUrl(), msgTime, getMediaPath(displayName), msg.GetChatMessageId())
		videoPath, err := downloadVideo(msg.GetVideoUrl(), msgTime, getMediaPath(displayName), msg.GetChatMessageId())
		if err != nil {
			hlog.Errorf("Failed to download video: %v", err)
			return nil, MediaFailed, err
		}
		dbMsg.VideoPath = videoPath
	default:
		hlog.Warnf("Unknown chat message type: %v, msgId: %v", msg.GetType(), msg.GetChatMessageId())
	}
	return dbMsg, mediaResult, nil
}

func downloadChatImage(mediaURL string, imgTime time.Time, pathPrefix string, msgID string) (string, MediaResult, error) {
	name, path, err := getImgFileName(mediaURL, imgTime, msgID)
	if err != nil {
		return "", MediaFailed, err
	}
	fullPath := filepath.Join(pathPrefix, path, name)
	wasPresent := mediaFileExists(fullPath)
	imagePath, err := downloadImage(mediaURL, imgTime, pathPrefix, msgID)
	if err != nil {
		return "", MediaFailed, err
	}
	if wasPresent {
		return imagePath, MediaSkipped, nil
	}
	return imagePath, MediaDownloaded, nil
}

func mediaFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func logChatSyncMessages(summary ChatSyncSummary) {
	chatMessageLogState.Lock()
	defer chatMessageLogState.Unlock()

	if summary.NewMessages > 0 {
		names := make([]string, 0, len(summary.BySender))
		for name := range summary.BySender {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			sender := summary.BySender[name]
			if sender.NewImageMessages > 0 {
				hlog.Infof("%s 给你发新消息了！而且有 %d 张新照片！⸜(*ˊᗜˋ*)⸝", name, sender.NewImageMessages)
			} else {
				hlog.Infof("%s 给你发新消息了！⸜(*ˊᗜˋ*)⸝", name)
			}
		}
		chatMessageLogState.hadMessages = true
	} else if !chatMessageLogState.initialized || chatMessageLogState.hadMessages ||
		(!chatMessageLogState.lastNoMessageLog.IsZero() && time.Since(chatMessageLogState.lastNoMessageLog) >= noChatMessageLogCooldown) {
		hlog.Infof("还没有女声优给你发新消息(´；ω；｀)")
		chatMessageLogState.hadMessages = false
		chatMessageLogState.lastNoMessageLog = time.Now()
	}
	chatMessageLogState.initialized = true
}

func chatMessageExists(chatMessageID string) (bool, error) {
	var count int64
	err := dal.ReadDB().Table(dal.ChatMessage{}.TableName()).
		Where("chat_message_id = ?", chatMessageID).
		Limit(1).
		Count(&count).Error
	if err != nil {
		hlog.Errorf("Failed to check chat message existence: %v", err)
		return false, err
	}
	return count > 0, nil
}

func saveChatMessageIfAbsent(dbMsg *dal.ChatMessage) (bool, error) {
	saved := false
	err := dal.WithWriteDB(func(db *gorm.DB) error {
		var count int64
		if err := db.Table(dal.ChatMessage{}.TableName()).
			Where("chat_message_id = ?", dbMsg.ChatMessageId).
			Limit(1).
			Count(&count).Error; err != nil {
			hlog.Errorf("Failed to check chat message existence: %v", err)
			return err
		}
		if count > 0 {
			hlog.Infof("Skip existing chat message before save, msgId: %v", dbMsg.ChatMessageId)
			return nil
		}
		if err := db.Create(dbMsg).Error; err != nil {
			hlog.Errorf("Failed to save chat message: %v", err)
			return err
		}
		saved = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return saved, nil
}

func getMediaPath(name string) string {
	return config.GetMediaPath() + "/" + name
}
