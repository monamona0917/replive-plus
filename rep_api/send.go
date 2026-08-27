package rep_api

import (
	"crypto/rand"
	"fmt"
	"replive/model"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
)

func SendChatMessage(userID, chatRoomID, content string) (*model.SendChatMessageResponse, error) {
	resp, _, err := sendChatMessage(userID, chatRoomID, content)
	return resp, err
}

// SendChatMessageWithID 返回本次请求提交给 Replive 的消息 ID。
// ListChatMessages 会携带相同 ID，以便调用方合并本地待同步消息。
func SendChatMessageWithID(userID, chatRoomID, content string) (string, error) {
	_, chatMessageID, err := sendChatMessage(userID, chatRoomID, content)
	return chatMessageID, err
}

func sendChatMessage(userID, chatRoomID, content string) (*model.SendChatMessageResponse, string, error) {
	userID = strings.TrimSpace(userID)
	chatRoomID = strings.TrimSpace(chatRoomID)
	content = strings.TrimSpace(content)
	if userID == "" || chatRoomID == "" || content == "" {
		return nil, "", fmt.Errorf("user_id, chat_room_id and content are required")
	}
	req := &model.SendChatMessageRequest{
		UserId:                               userID,
		ChatRoomId:                           chatRoomID,
		Content:                              content,
		ChatMessageId:                        newChatMessageID(),
		ConfirmContainsForbiddenWordsWarning: true,
	}
	resp, err := Post("user.v1.ChatService/SendChatMessage", req)
	if err != nil {
		return nil, "", fmt.Errorf("send chat message failed: %v", err)
	}
	out := new(model.SendChatMessageResponse)
	if err := proto.Unmarshal(resp, out); err != nil {
		return nil, "", fmt.Errorf("unmarshal SendChatMessageResponse failed: %v", err)
	}
	return out, req.ChatMessageId, nil
}

func newChatMessageID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
