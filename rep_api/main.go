package rep_api

import (
	"context"
	"fmt"
	"replive/model"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"golang.org/x/time/rate"
	"google.golang.org/protobuf/proto"
)

// akh
// userId: 481074e0-5f19-4cdb-90d2-21ff2e9544ac
// chatroomId: 395b8889-204d-460a-8c91-60bee4c9ba2d

// eri
//     UserId:      "950c24bb-6cf2-49a4-b728-7b14fd623caa",
//     ChatRoomId:  "d646d12f-2221-4267-ad45-f4640d665206",

var (
	chatApiLimiter = rate.NewLimiter(10, 10)
)

func GetChatMessages(ctx context.Context, uid, roomId string, cursorMsgId *string, backward bool, size int32) ([]*model.ListChatMessages, string, error) {
	uri := "user.v1.ChatService/ListChatMessages"
	req := &model.ListChatMessagesRequest{
		UserId:      uid,
		ChatRoomId:  roomId,
		MaxPageSize: size,
		Backward:    backward,
	}
	if cursorMsgId != nil {
		req.CursorChatMessageId = *cursorMsgId
	}
	for {
		if chatApiLimiter.Allow() {
			break
		}
		hlog.Warnf("GetChatMessages rate limit, query uid: %s, msgId: %v, backward: %v", uid, cursorMsgId, backward)
		time.Sleep(time.Millisecond * 200)
	}
	resp, err := GetReplive(uri, req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get list chat message: %v", err)
	}
	msgResp := new(model.ListChatMessagesResponse)
	if err := proto.Unmarshal(resp, msgResp); err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal ListChatMessagesResponse: %v", err)
	}
	for _, msg := range msgResp.Messages {
		timeVal := time.Unix(msg.Timestamp.Seconds, msg.Timestamp.Nanos).In(time.Local)
		msg.TimeStr = timeVal.Format("2006-01-02 15:04:05.000")
	}
	return msgResp.Messages, msgResp.NextPageCursorMessageId, nil
}

func GetChatRooms() ([]*model.ChatRoom, error) {
	uri := "user.v1.ChatService/ListChatRooms"
	req := &model.ListChatRoomsRequest{
		MaxPageSize: 32,
	}
	resp, err := GetReplive(uri, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat rooms: %v", err)
	}
	msgResp := new(model.ListChatRoomsResponse)
	if err := proto.Unmarshal(resp, msgResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ListChatRoomsResponse: %v", err)
	}
	return msgResp.ChatRooms, nil
}

func ListMyOshis(maxPageSize int64, pageToken string) (*model.ListMyOshisResponse, error) {
	uri := "user.v1.UserService/ListMyOshis"
	req := &model.ListMyOshisRequest{
		MaxPageSize: maxPageSize,
		PageToken:   pageToken,
	}
	resp, err := GetReplive(uri, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list my oshis: %v", err)
	}
	msgResp := new(model.ListMyOshisResponse)
	if err := proto.Unmarshal(resp, msgResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ListMyOshisResponse: %v", err)
	}
	return msgResp, nil
}

func ListFollowings(maxPageSize int64, pageToken string) (*model.ListFollowingsResponse, error) {
	uri := "user.v1.UserService/ListFollowings"
	req := &model.ListFollowingsRequest{
		UserId:      "me",
		MaxPageSize: maxPageSize,
		PageToken:   pageToken,
		Type:        model.FollowTargetType_FOLLOW_TARGET_TYPE_OSHI,
	}
	resp, err := GetReplive(uri, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list followings: %v", err)
	}
	msgResp := new(model.ListFollowingsResponse)
	if err := proto.Unmarshal(resp, msgResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ListFollowingsResponse: %v", err)
	}
	return msgResp, nil
}

func GetUserPrivate() (*model.UserPrivate, error) {
	uri := "user.v1.UserService/GetUserPrivate"
	req := &model.GetUserPrivateRequest{}
	resp, err := GetReplive(uri, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user private: %v", err)
	}
	msgResp := new(model.GetUserPrivateResponse)
	if err := proto.Unmarshal(resp, msgResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal GetUserPrivateResponse: %v", err)
	}
	return msgResp.GetUser(), nil
}

func GetStreamingLive() (*model.CheckStreamLiveResponse, error) {
	uri := "user.v1.LiveService/CheckStreamingLive"
	req := &model.CheckStreamLiveRequest{}
	resp, err := GetReplive(uri, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat streaming live: %v", err)
	}
	msgResp := new(model.CheckStreamLiveResponse)
	if err := proto.Unmarshal(resp, msgResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal CheckStreamLiveResponse: %v", err)
	}
	return msgResp, nil
}
