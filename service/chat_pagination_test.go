package service

import (
	"context"
	"errors"
	"replive/dal"
	"replive/model"
	"testing"
)

func TestCollectChatMessagesStopsWhenCursorDoesNotAdvance(t *testing.T) {
	callCount := 0
	fetcher := func(ctx context.Context, uid, roomId string, cursorMsgId *string, backward bool, size int32) ([]*model.ListChatMessages, string, error) {
		callCount++
		return []*model.ListChatMessages{{ChatMessageId: "msg-2"}}, *cursorMsgId, nil
	}

	msgs, err := collectChatMessages(context.Background(), fetcher, "u", "r", stringPtr("msg-1"), false, 100)
	if err != nil {
		t.Fatalf("collectChatMessages() error = %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %d, want 1", len(msgs))
	}
	if callCount != 1 {
		t.Fatalf("callCount = %d, want 1", callCount)
	}
}

func TestCollectChatMessagesStopsOnRepeatedCursor(t *testing.T) {
	callCount := 0
	fetcher := func(ctx context.Context, uid, roomId string, cursorMsgId *string, backward bool, size int32) ([]*model.ListChatMessages, string, error) {
		callCount++
		switch callCount {
		case 1:
			return []*model.ListChatMessages{{ChatMessageId: "msg-2"}}, "cursor-2", nil
		case 2:
			return []*model.ListChatMessages{{ChatMessageId: "msg-3"}}, "cursor-1", nil
		default:
			t.Fatalf("unexpected extra fetch, call=%d", callCount)
			return nil, "", nil
		}
	}

	msgs, err := collectChatMessages(context.Background(), fetcher, "u", "r", stringPtr("cursor-1"), false, 100)
	if err != nil {
		t.Fatalf("collectChatMessages() error = %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("len(msgs) = %d, want 2", len(msgs))
	}
	if callCount != 2 {
		t.Fatalf("callCount = %d, want 2", callCount)
	}
}

func TestCollectChatMessagesStopsWhenPageContainsCurrentCursor(t *testing.T) {
	callCount := 0
	fetcher := func(ctx context.Context, uid, roomId string, cursorMsgId *string, backward bool, size int32) ([]*model.ListChatMessages, string, error) {
		callCount++
		return []*model.ListChatMessages{{ChatMessageId: *cursorMsgId}}, "cursor-2", nil
	}

	msgs, err := collectChatMessages(context.Background(), fetcher, "u", "r", stringPtr("cursor-1"), true, 100)
	if err != nil {
		t.Fatalf("collectChatMessages() error = %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %d, want 1", len(msgs))
	}
	if callCount != 1 {
		t.Fatalf("callCount = %d, want 1", callCount)
	}
}

func TestCollectChatMessagesReturnsFetcherError(t *testing.T) {
	wantErr := errors.New("boom")
	fetcher := func(ctx context.Context, uid, roomId string, cursorMsgId *string, backward bool, size int32) ([]*model.ListChatMessages, string, error) {
		return nil, "", wantErr
	}

	_, err := collectChatMessages(context.Background(), fetcher, "u", "r", stringPtr("cursor-1"), false, 100)
	if !errors.Is(err, wantErr) {
		t.Fatalf("collectChatMessages() error = %v, want %v", err, wantErr)
	}
}

func TestSaveChatRoomsAllowsEmptySubscriptions(t *testing.T) {
	restoreCWD := chdirTemp(t)
	defer restoreCWD()

	if err := dal.InitDB(); err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}

	oldGetChatRooms := getChatRooms
	defer func() {
		getChatRooms = oldGetChatRooms
	}()

	getChatRooms = func() ([]*model.ChatRoom, error) {
		return nil, nil
	}

	if err := saveChatRooms(); err != nil {
		t.Fatalf("saveChatRooms() error = %v", err)
	}
	chatRoomLocker.RLock()
	got := len(chatRoomList)
	chatRoomLocker.RUnlock()
	if got != 0 {
		t.Fatalf("len(chatRoomList) = %d, want 0", got)
	}
}

func TestRefreshNewMessagesAllowsEmptySubscriptions(t *testing.T) {
	chatRoomLocker.Lock()
	oldChatRoomList := chatRoomList
	chatRoomList = nil
	chatRoomLocker.Unlock()
	defer func() {
		chatRoomLocker.Lock()
		chatRoomList = oldChatRoomList
		chatRoomLocker.Unlock()
	}()

	if err := refreshNewMessages(); err != nil {
		t.Fatalf("refreshNewMessages() error = %v", err)
	}
}

func TestRefreshOldChatMessagesSavesInitialMessageWhenNoOlderPages(t *testing.T) {
	restoreCWD := chdirTemp(t)
	defer restoreCWD()

	if err := dal.InitDB(); err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}

	oldGetChatMessages := getChatMessages
	oldSendChatEmailFn := sendChatEmailFn
	defer func() {
		getChatMessages = oldGetChatMessages
		sendChatEmailFn = oldSendChatEmailFn
	}()

	sendChatEmailFn = func(*dal.ChatMessage) {}
	getChatMessages = func(ctx context.Context, uid, roomId string, cursorMsgId *string, backward bool, size int32) ([]*model.ListChatMessages, string, error) {
		if cursorMsgId == nil {
			return []*model.ListChatMessages{testTextMessage("msg-latest", 10)}, "msg-latest", nil
		}
		if *cursorMsgId == "msg-latest" {
			return nil, "", nil
		}
		t.Fatalf("unexpected fetch cursor=%v backward=%v", cursorMsgId, backward)
		return nil, "", nil
	}

	if err := refreshOldChatMessages("user-1", "room-1"); err != nil {
		t.Fatalf("refreshOldChatMessages() error = %v", err)
	}
	assertChatMessageCount(t, 1)
	assertChatMessageExists(t, "msg-latest")
}

func TestRefreshOldChatMessagesFallsBackToInitialPageStrategies(t *testing.T) {
	restoreCWD := chdirTemp(t)
	defer restoreCWD()

	if err := dal.InitDB(); err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}

	oldGetChatMessages := getChatMessages
	oldSendChatEmailFn := sendChatEmailFn
	defer func() {
		getChatMessages = oldGetChatMessages
		sendChatEmailFn = oldSendChatEmailFn
	}()

	sendChatEmailFn = func(*dal.ChatMessage) {}
	var calls []string
	getChatMessages = func(ctx context.Context, uid, roomId string, cursorMsgId *string, backward bool, size int32) ([]*model.ListChatMessages, string, error) {
		if cursorMsgId == nil {
			calls = append(calls, initialFetchCall(backward, size))
			if !backward && size == 100 {
				return []*model.ListChatMessages{
					testTextMessage("msg-old", 1),
					testTextMessage("msg-latest", 10),
				}, "msg-latest", nil
			}
			return nil, "", nil
		}
		if *cursorMsgId == "msg-latest" {
			return nil, "", nil
		}
		t.Fatalf("unexpected fetch cursor=%v backward=%v", cursorMsgId, backward)
		return nil, "", nil
	}

	if err := refreshOldChatMessages("user-1", "room-1"); err != nil {
		t.Fatalf("refreshOldChatMessages() error = %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("initial fetch calls = %v, want 2 calls", calls)
	}
	assertChatMessageCount(t, 2)
	assertChatMessageExists(t, "msg-old")
	assertChatMessageExists(t, "msg-latest")
}

func TestRefreshOldChatMessagesCollectsBothDirectionsFromInitialCursor(t *testing.T) {
	restoreCWD := chdirTemp(t)
	defer restoreCWD()

	if err := dal.InitDB(); err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}

	oldGetChatMessages := getChatMessages
	oldSendChatEmailFn := sendChatEmailFn
	defer func() {
		getChatMessages = oldGetChatMessages
		sendChatEmailFn = oldSendChatEmailFn
	}()

	sendChatEmailFn = func(*dal.ChatMessage) {}
	var backwardCursorCalls int
	var forwardCursorCalls int
	getChatMessages = func(ctx context.Context, uid, roomId string, cursorMsgId *string, backward bool, size int32) ([]*model.ListChatMessages, string, error) {
		if cursorMsgId == nil {
			return []*model.ListChatMessages{testTextMessage("msg-anchor", 10)}, "msg-anchor", nil
		}
		if *cursorMsgId != "msg-anchor" {
			t.Fatalf("unexpected cursor=%v", cursorMsgId)
		}
		if backward {
			backwardCursorCalls++
			return []*model.ListChatMessages{testTextMessage("msg-older", 1)}, "", nil
		}
		forwardCursorCalls++
		return []*model.ListChatMessages{testTextMessage("msg-newer", 20)}, "", nil
	}

	if err := refreshOldChatMessages("user-1", "room-1"); err != nil {
		t.Fatalf("refreshOldChatMessages() error = %v", err)
	}
	if backwardCursorCalls != 1 {
		t.Fatalf("backward cursor calls = %d, want 1", backwardCursorCalls)
	}
	if forwardCursorCalls != 1 {
		t.Fatalf("forward cursor calls = %d, want 1", forwardCursorCalls)
	}
	assertChatMessageCount(t, 3)
	assertChatMessageExists(t, "msg-older")
	assertChatMessageExists(t, "msg-anchor")
	assertChatMessageExists(t, "msg-newer")
}

func TestRefreshOldChatMessagesReturnsErrorAfterAllInitialStrategiesEmpty(t *testing.T) {
	oldGetChatMessages := getChatMessages
	defer func() {
		getChatMessages = oldGetChatMessages
	}()

	callCount := 0
	getChatMessages = func(ctx context.Context, uid, roomId string, cursorMsgId *string, backward bool, size int32) ([]*model.ListChatMessages, string, error) {
		if cursorMsgId != nil {
			t.Fatalf("unexpected cursor fetch cursor=%v", cursorMsgId)
		}
		callCount++
		return nil, "", nil
	}

	err := refreshOldChatMessages("user-1", "room-1")
	if err == nil {
		t.Fatal("refreshOldChatMessages() error = nil, want initial empty error")
	}
	wantCalls := initialChatMessageFetchTries * len(initialChatMessageFetchStrategies)
	if callCount != wantCalls {
		t.Fatalf("initial fetch call count = %d, want %d", callCount, wantCalls)
	}
}

func stringPtr(s string) *string {
	return &s
}

func initialFetchCall(backward bool, size int32) string {
	if backward {
		return "backward"
	}
	return "forward"
}

func testTextMessage(msgID string, sendTime int64) *model.ListChatMessages {
	return &model.ListChatMessages{
		UserId:        "user-1",
		ChatRoomId:    "room-1",
		ChatMessageId: msgID,
		UserProfile: &model.UserProfile{
			DisplayName: "alice",
		},
		Type:      int32(model.ChatMessageType_Text),
		Timestamp: &model.Timestamp{Seconds: sendTime},
	}
}
