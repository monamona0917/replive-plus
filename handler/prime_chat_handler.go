package handler

import (
	"context"
	"fmt"
	"replive/dal"
	"replive/utils"
	"sort"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"gorm.io/gorm"
)

// Prime Chat is intentionally served from a separate local API surface. It
// never shares the Fandom room or message queries.
type PrimeChatRoomDTO struct {
	Id                        int64  `json:"id"`
	ChatRoomId                string `json:"chat_room_id"`
	TalentUserId              string `json:"talent_user_id"`
	TalentUniqueId            string `json:"talent_unique_id"`
	TalentDisplayName         string `json:"talent_display_name"`
	TalentAvatarUrl           string `json:"talent_avatar_url"`
	TalentAvatarLocalURL      string `json:"talent_avatar_local_url,omitempty"`
	MemberUserId              string `json:"member_user_id"`
	MemberBackgroundImageUrl  string `json:"member_background_image_url"`
	TalentLastCheckTimeMillis int64  `json:"talent_last_check_time_ms"`
	MemberLastCheckTimeMillis int64  `json:"member_last_check_time_ms"`
	SyncedAt                  int64  `json:"synced_at"`
	LastMessageTime           int64  `json:"last_message_time"`
	LastMessageContent        string `json:"last_message_content"`
	LastMessageType           string `json:"last_message_type"`
}

type PrimeChatMessageDTO struct {
	Id                   int64  `json:"id"`
	MessageId            string `json:"message_id"`
	ChatRoomId           string `json:"chat_room_id"`
	ChatRoomOwnerUserId  string `json:"chat_room_owner_user_id"`
	MemberUserId         string `json:"member_user_id"`
	Sender               string `json:"sender"`
	BodyType             string `json:"body_type"`
	Content              string `json:"content"`
	ImageUrl             string `json:"image_url"`
	VideoUrl             string `json:"video_url"`
	VideoThumbnailUrl    string `json:"video_thumbnail_url"`
	CoinAmount           int64  `json:"coin_amount"`
	ReactionEmoji        string `json:"reaction_emoji"`
	IsDeleted            bool   `json:"is_deleted"`
	CreateUnixTimeMillis int64  `json:"create_unix_time_ms"`
}

type GetPrimeChatMessagesResp struct {
	Messages     []*PrimeChatMessageDTO `json:"messages"`
	NextCursorId int64                  `json:"next_cursor_id"`
	PrevCursorId int64                  `json:"prev_cursor_id"`
	HasMore      bool                   `json:"has_more"`
	HasOlder     bool                   `json:"has_older"`
	HasNewer     bool                   `json:"has_newer"`
	AnchorId     int64                  `json:"anchor_id"`
}

func HandleGetPrimeChatRooms(ctx context.Context, c *app.RequestContext) {
	rooms, err := dal.GetPrimeChatRooms()
	if err != nil {
		c.JSON(consts.StatusOK, BadResp(err.Error()))
		return
	}

	data := make([]*PrimeChatRoomDTO, 0, len(rooms))
	for _, room := range rooms {
		if room == nil {
			continue
		}
		data = append(data, primeChatRoomDTO(room))
	}
	c.JSON(consts.StatusOK, &Resp{Data: data})
}

func HandleGetPrimeChatMessages(ctx context.Context, c *app.RequestContext) {
	chatRoomID := strings.TrimSpace(string(c.Query("chat_room_id")))
	if chatRoomID == "" {
		c.JSON(consts.StatusOK, BadResp("chat_room_id cannot be empty"))
		return
	}
	exists, err := primeChatRoomExists(chatRoomID)
	if err != nil {
		c.JSON(consts.StatusOK, BadResp(err.Error()))
		return
	}
	if !exists {
		c.JSON(consts.StatusOK, &Resp{Data: emptyPrimeMessagesResp()})
		return
	}

	direction := strings.TrimSpace(string(c.Query("direction")))
	bodyType := strings.TrimSpace(string(c.Query("body_type")))
	date := strings.TrimSpace(string(c.Query("date")))
	pageSize, err := parsePrimePageSize(c)
	if err != nil {
		c.JSON(consts.StatusOK, BadResp(err.Error()))
		return
	}
	cursorID, err := parseInt64Query(c, "cursor_id", 0)
	if err != nil {
		c.JSON(consts.StatusOK, BadResp(err.Error()))
		return
	}
	anchorID, err := parseInt64Query(c, "anchor_id", 0)
	if err != nil {
		c.JSON(consts.StatusOK, BadResp(err.Error()))
		return
	}

	if date != "" && anchorID == 0 {
		anchorID, err = findFirstPrimeMessageIDByDate(chatRoomID, bodyType, date)
		if err != nil {
			c.JSON(consts.StatusOK, BadResp(err.Error()))
			return
		}
		if anchorID == 0 {
			c.JSON(consts.StatusOK, &Resp{Data: emptyPrimeMessagesResp()})
			return
		}
	}
	if anchorID > 0 && direction == "" {
		direction = "around"
	}

	messages, hasOlder, hasNewer, err := queryPrimeChatMessages(chatRoomID, bodyType, cursorID, anchorID, pageSize, direction)
	if err != nil {
		hlog.Errorf("query prime_chat_messages failed room=%s: %v", chatRoomID, err)
		c.JSON(consts.StatusOK, BadResp(err.Error()))
		return
	}

	respMessages, prevCursor, nextCursor := buildPrimeChatMessageResp(messages)
	c.JSON(consts.StatusOK, &Resp{Data: &GetPrimeChatMessagesResp{
		Messages:     respMessages,
		NextCursorId: nextCursor,
		PrevCursorId: prevCursor,
		HasMore:      hasOlder,
		HasOlder:     hasOlder,
		HasNewer:     hasNewer,
		AnchorId:     anchorID,
	}})
}

func HandleGetPrimeChatDates(ctx context.Context, c *app.RequestContext) {
	chatRoomID := strings.TrimSpace(string(c.Query("chat_room_id")))
	if chatRoomID == "" {
		c.JSON(consts.StatusOK, BadResp("chat_room_id cannot be empty"))
		return
	}
	exists, err := primeChatRoomExists(chatRoomID)
	if err != nil {
		c.JSON(consts.StatusOK, BadResp(err.Error()))
		return
	}
	if !exists {
		c.JSON(consts.StatusOK, &Resp{Data: []string{}})
		return
	}

	var createTimes []int64
	err = primeBaseMessageQuery(chatRoomID, "").
		Where("create_unix_time_ms > 0").
		Pluck("create_unix_time_ms", &createTimes).Error
	if err != nil {
		c.JSON(consts.StatusOK, BadResp(err.Error()))
		return
	}

	dateSet := make(map[string]struct{}, len(createTimes))
	for _, createTime := range createTimes {
		dateSet[time.UnixMilli(createTime).In(utils.LocalLocation()).Format("2006-01-02")] = struct{}{}
	}
	dates := make([]string, 0, len(dateSet))
	for dateKey := range dateSet {
		dates = append(dates, dateKey)
	}
	sort.Strings(dates)
	c.JSON(consts.StatusOK, &Resp{Data: dates})
}

func HandleSearchPrimeChatMessages(ctx context.Context, c *app.RequestContext) {
	chatRoomID := strings.TrimSpace(string(c.Query("chat_room_id")))
	keyword := strings.TrimSpace(string(c.Query("keyword")))
	if chatRoomID == "" || keyword == "" {
		c.JSON(consts.StatusOK, BadResp("chat_room_id and keyword cannot be empty"))
		return
	}
	if !ensurePrimeChatRoom(c, chatRoomID) {
		return
	}

	pageSize, err := parsePrimePageSize(c)
	if err != nil {
		c.JSON(consts.StatusOK, BadResp(err.Error()))
		return
	}
	cursorID, err := parseInt64Query(c, "cursor_id", 0)
	if err != nil {
		c.JSON(consts.StatusOK, BadResp(err.Error()))
		return
	}

	query := primeBaseMessageQuery(chatRoomID, "").
		Where("text_content LIKE ?", "%"+keyword+"%")
	if cursorID > 0 {
		cursor, err := getPrimeMessageCursor(chatRoomID, cursorID)
		if err != nil {
			c.JSON(consts.StatusOK, BadResp(err.Error()))
			return
		}
		if cursor == nil {
			c.JSON(consts.StatusOK, &Resp{Data: emptyPrimeMessagesResp()})
			return
		}
		query = primeBefore(query, *cursor)
	}

	messages := make([]dal.PrimeChatMessage, 0, int(pageSize)+1)
	if err := query.Order("create_unix_time_ms DESC, id DESC").Limit(int(pageSize) + 1).Find(&messages).Error; err != nil {
		c.JSON(consts.StatusOK, BadResp(err.Error()))
		return
	}
	hasOlder := len(messages) > int(pageSize)
	if hasOlder {
		messages = messages[:pageSize]
	}
	reversePrimeMessages(messages)
	respMessages, prevCursor, nextCursor := buildPrimeChatMessageResp(messages)
	c.JSON(consts.StatusOK, &Resp{Data: &GetPrimeChatMessagesResp{
		Messages:     respMessages,
		NextCursorId: nextCursor,
		PrevCursorId: prevCursor,
		HasMore:      hasOlder,
		HasOlder:     hasOlder,
	}})
}

func ensurePrimeChatRoom(c *app.RequestContext, chatRoomID string) bool {
	exists, err := primeChatRoomExists(chatRoomID)
	if err != nil {
		c.JSON(consts.StatusOK, BadResp(err.Error()))
		return false
	}
	if !exists {
		c.JSON(consts.StatusOK, &Resp{Data: emptyPrimeMessagesResp()})
		return false
	}
	return true
}

func primeChatRoomExists(chatRoomID string) (bool, error) {
	room, err := dal.GetPrimeChatRoomByChatRoomID(chatRoomID)
	if err != nil {
		return false, err
	}
	return room != nil, nil
}

func parsePrimePageSize(c *app.RequestContext) (int32, error) {
	pageSize, err := parseInt32Query(c, "page_size", 30)
	if err != nil {
		return 0, err
	}
	if pageSize <= 0 {
		pageSize = 30
	}
	if pageSize > 1000 {
		pageSize = 1000
	}
	return pageSize, nil
}

func emptyPrimeMessagesResp() *GetPrimeChatMessagesResp {
	return &GetPrimeChatMessagesResp{Messages: []*PrimeChatMessageDTO{}}
}

func findFirstPrimeMessageIDByDate(chatRoomID, bodyType, date string) (int64, error) {
	start, err := time.ParseInLocation("2006-01-02", date, utils.LocalLocation())
	if err != nil {
		return 0, fmt.Errorf("invalid date %q; expected yyyy-MM-dd: %w", date, err)
	}
	end := start.AddDate(0, 0, 1)
	var message dal.PrimeChatMessage
	err = primeBaseMessageQuery(chatRoomID, bodyType).
		Where("create_unix_time_ms >= ? AND create_unix_time_ms < ?", start.UnixMilli(), end.UnixMilli()).
		Order("create_unix_time_ms ASC, id ASC").
		Limit(1).
		Find(&message).Error
	return message.Id, err
}

func queryPrimeChatMessages(chatRoomID, bodyType string, cursorID, anchorID int64, pageSize int32, direction string) ([]dal.PrimeChatMessage, bool, bool, error) {
	limit := int(pageSize) + 1

	if direction == "newer" {
		if cursorID <= 0 {
			return []dal.PrimeChatMessage{}, false, false, nil
		}
		cursor, err := getPrimeMessageCursor(chatRoomID, cursorID)
		if err != nil || cursor == nil {
			return []dal.PrimeChatMessage{}, false, false, err
		}
		messages := make([]dal.PrimeChatMessage, 0, limit)
		query := primeAfter(primeBaseMessageQuery(chatRoomID, bodyType), *cursor)
		if err := query.Order("create_unix_time_ms ASC, id ASC").Limit(limit).Find(&messages).Error; err != nil {
			return nil, false, false, err
		}
		hasNewer := len(messages) > int(pageSize)
		if hasNewer {
			messages = messages[:pageSize]
		}
		hasOlder, err := hasPrimeMessageBefore(chatRoomID, bodyType, firstPrimeMessage(messages))
		return messages, hasOlder, hasNewer, err
	}

	if direction == "around" {
		if anchorID <= 0 {
			return []dal.PrimeChatMessage{}, false, false, nil
		}
		anchor, err := getPrimeMessageCursor(chatRoomID, anchorID)
		if err != nil || anchor == nil {
			return []dal.PrimeChatMessage{}, false, false, err
		}
		// 保证至少为 anchor 自身预留 1 条配额，前半段最多取 (pageSize - 1) / 2 条
		targetTotal := int(pageSize)
		if targetTotal < 1 {
			targetTotal = 1
		}
		halfBefore := (targetTotal - 1) / 2

		// 1. 取 anchor 前面的消息（按倒序查出再翻转）
		beforeMsgs := make([]dal.PrimeChatMessage, 0, halfBefore)
		if halfBefore > 0 {
			if err := primeBefore(primeBaseMessageQuery(chatRoomID, bodyType), *anchor).Order("create_unix_time_ms DESC, id DESC").Limit(halfBefore).Find(&beforeMsgs).Error; err != nil {
				return nil, false, false, err
			}
			reversePrimeMessages(beforeMsgs)
		}

		// 2. 取 anchor 及之后的消息（保证 neededAfter >= 1，必定包含 anchor 自己）
		neededAfter := targetTotal - len(beforeMsgs)
		if neededAfter < 1 {
			neededAfter = 1
		}
		atOrAfterMsgs := make([]dal.PrimeChatMessage, 0, neededAfter+1)
		if err := primeAtOrAfter(primeBaseMessageQuery(chatRoomID, bodyType), *anchor).Order("create_unix_time_ms ASC, id ASC").Limit(neededAfter + 1).Find(&atOrAfterMsgs).Error; err != nil {
			return nil, false, false, err
		}
		hasNewer := len(atOrAfterMsgs) > neededAfter
		if hasNewer {
			atOrAfterMsgs = atOrAfterMsgs[:neededAfter]
		}

		// 3. 若向后不够，且向前有更多记录，向前多取补足至 targetTotal
		if len(beforeMsgs)+len(atOrAfterMsgs) < targetTotal {
			missing := targetTotal - (len(beforeMsgs) + len(atOrAfterMsgs))
			oldestAnchor := *anchor
			if len(beforeMsgs) > 0 {
				oldestAnchor = beforeMsgs[0]
			}
			extraBefore := make([]dal.PrimeChatMessage, 0, missing)
			if err := primeBefore(primeBaseMessageQuery(chatRoomID, bodyType), oldestAnchor).Order("create_unix_time_ms DESC, id DESC").Limit(missing).Find(&extraBefore).Error; err != nil {
				return nil, false, false, err
			}
			if len(extraBefore) > 0 {
				reversePrimeMessages(extraBefore)
				beforeMsgs = append(extraBefore, beforeMsgs...)
			}
		}

		messages := append(beforeMsgs, atOrAfterMsgs...)
		hasOlder, err := hasPrimeMessageBefore(chatRoomID, bodyType, firstPrimeMessage(messages))
		if err != nil {
			return nil, false, false, err
		}
		if !hasNewer {
			hasNewer, err = hasPrimeMessageAfter(chatRoomID, bodyType, lastPrimeMessage(messages))
			if err != nil {
				return nil, false, false, err
			}
		}
		return messages, hasOlder, hasNewer, nil
	}

	query := primeBaseMessageQuery(chatRoomID, bodyType)
	if cursorID > 0 {
		cursor, err := getPrimeMessageCursor(chatRoomID, cursorID)
		if err != nil || cursor == nil {
			return []dal.PrimeChatMessage{}, false, false, err
		}
		query = primeBefore(query, *cursor)
	}
	messages := make([]dal.PrimeChatMessage, 0, limit)
	if err := query.Order("create_unix_time_ms DESC, id DESC").Limit(limit).Find(&messages).Error; err != nil {
		return nil, false, false, err
	}
	hasOlder := len(messages) > int(pageSize)
	if hasOlder {
		messages = messages[:pageSize]
	}
	reversePrimeMessages(messages)
	hasNewer, err := hasPrimeMessageAfter(chatRoomID, bodyType, lastPrimeMessage(messages))
	return messages, hasOlder, hasNewer, err
}

func primeBaseMessageQuery(chatRoomID, bodyType string) *gorm.DB {
	query := dal.ReadDB().Table(dal.PrimeChatMessage{}.TableName()).Where("chat_room_id = ?", chatRoomID)
	if bodyType != "" {
		query = query.Where("body_type = ?", bodyType)
	}
	return query
}

func getPrimeMessageCursor(chatRoomID string, id int64) (*dal.PrimeChatMessage, error) {
	if id <= 0 {
		return nil, nil
	}
	var message dal.PrimeChatMessage
	err := primeBaseMessageQuery(chatRoomID, "").Where("id = ?", id).Limit(1).Find(&message).Error
	if err != nil {
		return nil, err
	}
	if message.Id == 0 {
		return nil, nil
	}
	return &message, nil
}

func primeBefore(query *gorm.DB, pivot dal.PrimeChatMessage) *gorm.DB {
	return query.Where("(create_unix_time_ms < ? OR (create_unix_time_ms = ? AND id < ?))", pivot.CreateUnixTimeMillis, pivot.CreateUnixTimeMillis, pivot.Id)
}

func primeAfter(query *gorm.DB, pivot dal.PrimeChatMessage) *gorm.DB {
	return query.Where("(create_unix_time_ms > ? OR (create_unix_time_ms = ? AND id > ?))", pivot.CreateUnixTimeMillis, pivot.CreateUnixTimeMillis, pivot.Id)
}

func primeAtOrAfter(query *gorm.DB, pivot dal.PrimeChatMessage) *gorm.DB {
	return query.Where("(create_unix_time_ms > ? OR (create_unix_time_ms = ? AND id >= ?))", pivot.CreateUnixTimeMillis, pivot.CreateUnixTimeMillis, pivot.Id)
}

func hasPrimeMessageBefore(chatRoomID, bodyType string, message *dal.PrimeChatMessage) (bool, error) {
	if message == nil {
		return false, nil
	}
	var count int64
	err := primeBefore(primeBaseMessageQuery(chatRoomID, bodyType), *message).Limit(1).Count(&count).Error
	return count > 0, err
}

func hasPrimeMessageAfter(chatRoomID, bodyType string, message *dal.PrimeChatMessage) (bool, error) {
	if message == nil {
		return false, nil
	}
	var count int64
	err := primeAfter(primeBaseMessageQuery(chatRoomID, bodyType), *message).Limit(1).Count(&count).Error
	return count > 0, err
}

func firstPrimeMessage(messages []dal.PrimeChatMessage) *dal.PrimeChatMessage {
	if len(messages) == 0 {
		return nil
	}
	return &messages[0]
}

func lastPrimeMessage(messages []dal.PrimeChatMessage) *dal.PrimeChatMessage {
	if len(messages) == 0 {
		return nil
	}
	return &messages[len(messages)-1]
}

func reversePrimeMessages(messages []dal.PrimeChatMessage) {
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
}

func buildPrimeChatMessageResp(messages []dal.PrimeChatMessage) ([]*PrimeChatMessageDTO, int64, int64) {
	data := make([]*PrimeChatMessageDTO, 0, len(messages))
	for i := range messages {
		data = append(data, primeChatMessageDTO(&messages[i]))
	}
	if len(messages) == 0 {
		return data, 0, 0
	}
	// Messages are always returned oldest-to-newest. SQLite IDs are opaque
	// cursors; creation time is the actual chronological ordering.
	return data, messages[0].Id, messages[len(messages)-1].Id
}

func primeChatRoomDTO(room *dal.PrimeChatRoom) *PrimeChatRoomDTO {
	return &PrimeChatRoomDTO{
		Id:                        room.Id,
		ChatRoomId:                room.ChatRoomId,
		TalentUserId:              room.TalentUserId,
		TalentUniqueId:            room.TalentUniqueId,
		TalentDisplayName:         room.TalentDisplayName,
		TalentAvatarUrl:           room.TalentAvatarUrl,
		TalentAvatarLocalURL:      localProfileMediaURL(room.TalentAvatarPath),
		MemberUserId:              room.MemberUserId,
		MemberBackgroundImageUrl:  room.MemberBackgroundImageUrl,
		TalentLastCheckTimeMillis: room.TalentLastCheckTimeMillis,
		MemberLastCheckTimeMillis: room.MemberLastCheckTimeMillis,
		SyncedAt:                  room.SyncedAt,
		LastMessageTime:           room.LastMessageTime,
		LastMessageContent:        room.LastMessageContent,
		LastMessageType:           room.LastMessageType,
	}
}

func primeChatMessageDTO(message *dal.PrimeChatMessage) *PrimeChatMessageDTO {
	return &PrimeChatMessageDTO{
		Id:                   message.Id,
		MessageId:            message.MessageId,
		ChatRoomId:           message.ChatRoomId,
		ChatRoomOwnerUserId:  message.ChatRoomOwnerUserId,
		MemberUserId:         message.MemberUserId,
		Sender:               message.Sender,
		BodyType:             message.BodyType,
		Content:              message.TextContent,
		ImageUrl:             message.ImageUrl,
		VideoUrl:             message.VideoUrl,
		VideoThumbnailUrl:    message.VideoThumbnailUrl,
		CoinAmount:           message.CoinAmount,
		ReactionEmoji:        message.ReactionEmoji,
		IsDeleted:            message.IsDeleted,
		CreateUnixTimeMillis: message.CreateUnixTimeMillis,
	}
}
