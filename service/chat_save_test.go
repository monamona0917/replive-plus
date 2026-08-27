package service

import (
	"errors"
	"os"
	"replive/dal"
	"replive/model"
	"testing"
	"time"
)

func TestSaveMessagePersistsProcessedMessagesWhenLaterDownloadFails(t *testing.T) {
	restoreCWD := chdirTemp(t)
	defer restoreCWD()

	if err := dal.InitDB(); err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}

	oldDownloadImage := downloadImage
	oldDownloadVideo := downloadVideo
	oldSendChatEmailFn := sendChatEmailFn
	defer func() {
		downloadImage = oldDownloadImage
		downloadVideo = oldDownloadVideo
		sendChatEmailFn = oldSendChatEmailFn
	}()

	sendChatEmailFn = func(*dal.ChatMessage) {}
	downloadVideo = func(string, time.Time, string, string) (string, error) {
		t.Fatal("downloadVideo should not be called")
		return "", nil
	}

	callCount := map[string]int{}
	downloadImage = func(mediaURL string, imgTime time.Time, pathPrefix string, msgID string) (string, error) {
		callCount[msgID]++
		if msgID == "msg-2" {
			return "", errors.New("timeout")
		}
		return pathPrefix + "/" + msgID + ".jpeg", nil
	}

	messages := []*model.ListChatMessages{
		testImageMessage("msg-1", 1),
		testImageMessage("msg-2", 2),
	}
	if err := saveMessage(messages); err == nil {
		t.Fatal("saveMessage() error = nil, want timeout")
	}
	assertChatMessageCount(t, 1)
	assertChatMessageExists(t, "msg-1")

	downloadImage = func(mediaURL string, imgTime time.Time, pathPrefix string, msgID string) (string, error) {
		callCount[msgID]++
		return pathPrefix + "/" + msgID + ".jpeg", nil
	}
	if err := saveMessage(messages); err != nil {
		t.Fatalf("saveMessage() retry error = %v", err)
	}
	assertChatMessageCount(t, 2)
	assertChatMessageExists(t, "msg-2")
	if callCount["msg-1"] != 1 {
		t.Fatalf("msg-1 download count = %d, want 1", callCount["msg-1"])
	}
	if callCount["msg-2"] != 2 {
		t.Fatalf("msg-2 download count = %d, want 2", callCount["msg-2"])
	}
}

func TestSaveMessageWithSummaryExcludesOwnMessagesFromNotification(t *testing.T) {
	restoreCWD := chdirTemp(t)
	defer restoreCWD()

	if err := dal.InitDB(); err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}

	oldDownloadImage := downloadImage
	oldDownloadVideo := downloadVideo
	oldSendChatEmailFn := sendChatEmailFn
	defer func() {
		downloadImage = oldDownloadImage
		downloadVideo = oldDownloadVideo
		sendChatEmailFn = oldSendChatEmailFn
	}()

	downloadImage = func(string, time.Time, string, string) (string, error) {
		return "talent-image.jpeg", nil
	}
	downloadVideo = func(string, time.Time, string, string) (string, error) {
		return "own-video.mp4", nil
	}
	emailedMessageIDs := make([]string, 0)
	sendChatEmailFn = func(msg *dal.ChatMessage) {
		emailedMessageIDs = append(emailedMessageIDs, msg.ChatMessageId)
	}

	talentMessage := testImageMessage("talent-image", 1)
	ownImage := testImageMessage("own-image", 2)
	ownImage.UserProfile.DisplayName = "me"
	ownVideo := &model.ListChatMessages{
		UserId:        "user-1",
		ChatRoomId:    "room-1",
		ChatMessageId: "own-video",
		UserProfile: &model.UserProfile{
			DisplayName: "me",
		},
		Type:      int32(model.ChatMessageType_Video),
		VideoUrl:  "https://example.com/own-video.mp4",
		Timestamp: &model.Timestamp{Seconds: 3},
	}

	summary, err := saveMessageWithSummaryForTalent([]*model.ListChatMessages{
		talentMessage,
		ownImage,
		ownVideo,
	}, "alice")
	if err != nil {
		t.Fatalf("saveMessageWithSummaryForTalent() error = %v", err)
	}
	assertChatMessageCount(t, 3)
	if summary.NewMessages != 1 {
		t.Fatalf("summary.NewMessages = %d, want 1", summary.NewMessages)
	}
	if summary.NewImageMessages != 1 {
		t.Fatalf("summary.NewImageMessages = %d, want 1", summary.NewImageMessages)
	}
	if summary.NewVideoMessages != 0 {
		t.Fatalf("summary.NewVideoMessages = %d, want 0", summary.NewVideoMessages)
	}
	if got := summary.BySender["alice"].NewMessages; got != 1 {
		t.Fatalf("alice message count = %d, want 1", got)
	}
	if _, ok := summary.BySender["me"]; ok {
		t.Fatalf("own messages should not appear in summary.BySender: %#v", summary.BySender)
	}
	if len(emailedMessageIDs) != 1 || emailedMessageIDs[0] != "talent-image" {
		t.Fatalf("emailed message IDs = %v, want [talent-image]", emailedMessageIDs)
	}
}

func chdirTemp(t *testing.T) func() {
	t.Helper()
	oldCWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	return func() {
		if err := dal.CloseDB(); err != nil {
			t.Fatalf("CloseDB() error = %v", err)
		}
		if err := os.Chdir(oldCWD); err != nil {
			t.Fatalf("restore Chdir() error = %v", err)
		}
	}
}

func testImageMessage(msgID string, sendTime int64) *model.ListChatMessages {
	return &model.ListChatMessages{
		UserId:        "user-1",
		ChatRoomId:    "room-1",
		ChatMessageId: msgID,
		UserProfile: &model.UserProfile{
			DisplayName: "alice",
		},
		Type:      int32(model.ChatMessageType_Image),
		ImageUrl:  "https://example.com/" + msgID + ".jpeg",
		Timestamp: &model.Timestamp{Seconds: sendTime},
		TimeStr:   time.Unix(sendTime, 0).Format(time.RFC3339),
	}
}

func assertChatMessageCount(t *testing.T, want int64) {
	t.Helper()
	var got int64
	if err := dal.ReadDB().Table(dal.ChatMessage{}.TableName()).Count(&got).Error; err != nil {
		t.Fatalf("count chat messages error = %v", err)
	}
	if got != want {
		t.Fatalf("chat message count = %d, want %d", got, want)
	}
}

func assertChatMessageExists(t *testing.T, msgID string) {
	t.Helper()
	var got int64
	if err := dal.ReadDB().Table(dal.ChatMessage{}.TableName()).
		Where("chat_message_id = ?", msgID).
		Count(&got).Error; err != nil {
		t.Fatalf("count chat message %s error = %v", msgID, err)
	}
	if got != 1 {
		t.Fatalf("chat message %s count = %d, want 1", msgID, got)
	}
}
