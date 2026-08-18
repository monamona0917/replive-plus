package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"replive/config"
	"replive/dal"
	"replive/rep_api"
	"replive/utils"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"gorm.io/gorm"
)

func HandleGetChatRooms(ctx context.Context, c *app.RequestContext) {
	resp := &Resp{}
	rooms, err := dal.GetChatRooms()
	if err != nil {
		c.JSON(consts.StatusOK, BadResp(err.Error()))
		return
	}
	resp.Data = rooms
	c.JSON(consts.StatusOK, resp)
}

type ChatMessageDTO struct {
	Id            int64  `json:"id"`
	UserId        string `json:"user_id"`
	DisplayName   string `json:"display_name"`
	ChatRoomId    string `json:"chat_room_id"`
	ChatMessageId string `json:"chat_message_id"`
	MsgType       int32  `json:"msg_type"`
	Content       string `json:"content"`
	ImageUrl      string `json:"image_url"`
	VideoUrl      string `json:"video_url"`
	ImageLocalURL string `json:"image_local_url,omitempty"`
	VideoLocalURL string `json:"video_local_url,omitempty"`
	TimeStr       string `json:"time_str"`
	SendTime      int64  `json:"send_time"`
}

type GetChatMessagesResp struct {
	Messages     []*ChatMessageDTO `json:"messages"`
	NextCursorId int64             `json:"next_cursor_id"`
	PrevCursorId int64             `json:"prev_cursor_id"`
	HasMore      bool              `json:"has_more"`
	HasOlder     bool              `json:"has_older"`
	HasNewer     bool              `json:"has_newer"`
	AnchorId     int64             `json:"anchor_id"`
}

func HandleGetChatMessages(ctx context.Context, c *app.RequestContext) {
	displayName := strings.TrimSpace(string(c.Query("display_name")))
	direction := strings.TrimSpace(string(c.Query("direction")))
	date := strings.TrimSpace(string(c.Query("date")))

	msgType, err := parseInt32Query(c, "msg_type", 0)
	if err != nil {
		c.JSON(consts.StatusOK, BadResp(err.Error()))
		return
	}
	pageSize, err := parseInt32Query(c, "page_size", 20)
	if err != nil {
		c.JSON(consts.StatusOK, BadResp(err.Error()))
		return
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 1000 {
		pageSize = 1000
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

	if displayName == "" {
		c.JSON(consts.StatusOK, BadResp("display_name 或 (user_id + chat_room_id) 不能为空"))
		return
	}
	room, err := findChatRoom(displayName)
	if err != nil {
		c.JSON(consts.StatusOK, BadResp(err.Error()))
		return
	}
	if room.UserId == "" || room.ChatRoomId == "" {
		// 没找到，按空结果返回
		c.JSON(consts.StatusOK, &Resp{Data: &GetChatMessagesResp{Messages: []*ChatMessageDTO{}, NextCursorId: 0}})
		return
	}
	userID := room.UserId
	chatRoomID := room.ChatRoomId

	if date != "" && anchorID == 0 {
		anchorID, err = findFirstMessageIDByDate(userID, chatRoomID, msgType, date)
		if err != nil {
			c.JSON(consts.StatusOK, BadResp(err.Error()))
			return
		}
		if anchorID == 0 {
			c.JSON(consts.StatusOK, &Resp{Data: &GetChatMessagesResp{Messages: []*ChatMessageDTO{}, NextCursorId: 0}})
			return
		}
	}
	if anchorID > 0 && direction == "" {
		direction = "around"
	}

	msgs, hasOlder, hasNewer, err := queryChatMessages(userID, chatRoomID, msgType, cursorID, anchorID, pageSize, direction)
	if err != nil {
		hlog.Errorf("query chat_messages failed, uid=%s room=%s err=%v", userID, chatRoomID, err)
		c.JSON(consts.StatusOK, BadResp(err.Error()))
		return
	}

	respMsgs, prevCursor, nextCursor := buildChatMessageResp(msgs)
	c.JSON(consts.StatusOK, &Resp{Data: &GetChatMessagesResp{
		Messages:     respMsgs,
		NextCursorId: nextCursor,
		PrevCursorId: prevCursor,
		HasMore:      hasOlder,
		HasOlder:     hasOlder,
		HasNewer:     hasNewer,
		AnchorId:     anchorID,
	}})
}

// HandleGetChatDates returns every local calendar day containing a message in a chat room.
func HandleGetChatDates(ctx context.Context, c *app.RequestContext) {
	displayName := strings.TrimSpace(string(c.Query("display_name")))
	if displayName == "" {
		c.JSON(consts.StatusOK, BadResp("display_name 不能为空"))
		return
	}

	msgType, err := parseInt32Query(c, "msg_type", 0)
	if err != nil {
		c.JSON(consts.StatusOK, BadResp(err.Error()))
		return
	}

	room, err := findChatRoom(displayName)
	if err != nil {
		c.JSON(consts.StatusOK, BadResp(err.Error()))
		return
	}
	if room.UserId == "" || room.ChatRoomId == "" {
		c.JSON(consts.StatusOK, &Resp{Data: []string{}})
		return
	}

	type dateRow struct {
		DateKey string `gorm:"column:date_key"`
	}
	rows := make([]dateRow, 0)
	err = baseMessageQuery(room.UserId, room.ChatRoomId, msgType).
		Where("send_time > 0").
		Select("strftime('%Y-%m-%d', send_time, 'unixepoch', '+9 hours') AS date_key").
		Group("date_key").
		Order("date_key ASC").
		Scan(&rows).Error
	if err != nil {
		hlog.Errorf("query chat dates failed, uid=%s room=%s err=%v", room.UserId, room.ChatRoomId, err)
		c.JSON(consts.StatusOK, BadResp(err.Error()))
		return
	}

	dates := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.DateKey != "" {
			dates = append(dates, row.DateKey)
		}
	}
	c.JSON(consts.StatusOK, &Resp{Data: dates})
}

func HandleSearchChatMessages(ctx context.Context, c *app.RequestContext) {
	displayName := strings.TrimSpace(string(c.Query("display_name")))
	keyword := strings.TrimSpace(string(c.Query("keyword")))
	if displayName == "" {
		c.JSON(consts.StatusOK, BadResp("display_name 不能为空"))
		return
	}
	if keyword == "" {
		c.JSON(consts.StatusOK, BadResp("keyword 不能为空"))
		return
	}

	pageSize, err := parseInt32Query(c, "page_size", 20)
	if err != nil {
		c.JSON(consts.StatusOK, BadResp(err.Error()))
		return
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	cursorID, err := parseInt64Query(c, "cursor_id", 0)
	if err != nil {
		c.JSON(consts.StatusOK, BadResp(err.Error()))
		return
	}

	room, err := findChatRoom(displayName)
	if err != nil {
		c.JSON(consts.StatusOK, BadResp(err.Error()))
		return
	}
	if room.UserId == "" || room.ChatRoomId == "" {
		c.JSON(consts.StatusOK, &Resp{Data: &GetChatMessagesResp{Messages: []*ChatMessageDTO{}, NextCursorId: 0}})
		return
	}

	query := dal.ReadDB().Table(dal.ChatMessage{}.TableName()).
		Where("user_id = ? AND chat_room_id = ? AND content LIKE ?", room.UserId, room.ChatRoomId, "%"+keyword+"%")
	if cursorID > 0 {
		query = query.Where("id < ?", cursorID)
	}

	msgs := make([]dal.ChatMessage, 0, int(pageSize)+1)
	if err := query.Order("id desc").Limit(int(pageSize) + 1).Find(&msgs).Error; err != nil {
		c.JSON(consts.StatusOK, BadResp(err.Error()))
		return
	}
	hasMore := len(msgs) > int(pageSize)
	if hasMore {
		msgs = msgs[:pageSize]
	}

	respMsgs, _, nextCursor := buildChatMessageResp(msgs)
	c.JSON(consts.StatusOK, &Resp{Data: &GetChatMessagesResp{
		Messages:     respMsgs,
		NextCursorId: nextCursor,
		HasMore:      hasMore,
		HasOlder:     hasMore,
	}})
}

func findChatRoom(displayName string) (dal.ChatRoom, error) {
	var room dal.ChatRoom
	err := dal.ReadDB().Table(dal.ChatRoom{}.TableName()).
		Where("display_name = ?", displayName).
		Limit(1).
		Find(&room).Error
	if err != nil {
		hlog.Errorf("query chat_room failed, display_name=%s, err=%v", displayName, err)
		return room, err
	}
	return room, nil
}

func findFirstMessageIDByDate(userID string, chatRoomID string, msgType int32, date string) (int64, error) {
	start, err := time.ParseInLocation("2006-01-02", date, utils.JapanLocation())
	if err != nil {
		return 0, fmt.Errorf("date 参数非法，应为 yyyy-MM-dd: %v", err)
	}
	end := start.Add(24 * time.Hour)

	// 新旧本地数据库中 send_time 可能分别保存为秒或毫秒；部分旧记录还只
	// 保留了 time_str。日期跳转必须覆盖这三种格式，否则日历能显示日期，
	// 但 around 查询会找不到对应 anchor。
	startSeconds := start.Unix()
	endSeconds := end.Unix()
	startMillis := start.UnixNano() / int64(time.Millisecond)
	endMillis := end.UnixNano() / int64(time.Millisecond)
	query := baseMessageQuery(userID, chatRoomID, msgType).
		Where(`
			(send_time >= ? AND send_time < ?) OR
			(send_time >= ? AND send_time < ?) OR
			time_str LIKE ?`,
			startSeconds, endSeconds,
			startMillis, endMillis,
			date+"%",
		)
	var msg dal.ChatMessage
	if err := query.Order("send_time asc, id asc").Limit(1).Find(&msg).Error; err != nil {
		return 0, err
	}
	return msg.Id, nil
}

func queryChatMessages(userID string, chatRoomID string, msgType int32, cursorID int64, anchorID int64, pageSize int32, direction string) ([]dal.ChatMessage, bool, bool, error) {
	limit := int(pageSize) + 1
	query := baseMessageQuery(userID, chatRoomID, msgType)

	switch direction {
	case "newer":
		if cursorID <= 0 {
			return []dal.ChatMessage{}, false, false, nil
		}
		cursor, found, err := findChatMessageCursor(userID, chatRoomID, msgType, cursorID)
		if err != nil {
			return nil, false, false, err
		}
		if !found {
			return []dal.ChatMessage{}, false, false, nil
		}
		msgs := make([]dal.ChatMessage, 0, limit)
		if err := messagesAfter(query, cursor).Order("send_time asc, id asc").Limit(limit).Find(&msgs).Error; err != nil {
			return nil, false, false, err
		}
		hasNewer := len(msgs) > int(pageSize)
		if hasNewer {
			msgs = msgs[:pageSize]
		}
		hasOlder, err := hasMessageBefore(userID, chatRoomID, msgType, oldestMessageID(msgs))
		return msgs, hasOlder, hasNewer, err
	case "around":
		if anchorID <= 0 {
			return []dal.ChatMessage{}, false, false, nil
		}
		anchor, found, err := findChatMessageCursor(userID, chatRoomID, msgType, anchorID)
		if err != nil {
			return nil, false, false, err
		}
		if !found {
			return []dal.ChatMessage{}, false, false, nil
		}
		// 保证至少为 anchor 自身预留 1 条配额，前半段最多取 (pageSize - 1) / 2 条
		targetTotal := int(pageSize)
		if targetTotal < 1 {
			targetTotal = 1
		}
		halfBefore := (targetTotal - 1) / 2

		// 1. 取 anchor 前面的消息（按倒序查出再翻转）
		beforeMsgs := make([]dal.ChatMessage, 0, halfBefore)
		if halfBefore > 0 {
			if err := messagesBefore(query, anchor).Order("send_time desc, id desc").Limit(halfBefore).Find(&beforeMsgs).Error; err != nil {
				return nil, false, false, err
			}
			reverseChatMessages(beforeMsgs)
		}

		// 2. 取 anchor 及之后的消息（保证 neededAfter >= 1，必定包含 anchor 自己）
		neededAfter := targetTotal - len(beforeMsgs)
		if neededAfter < 1 {
			neededAfter = 1
		}
		atOrAfterMsgs := make([]dal.ChatMessage, 0, neededAfter+1)
		if err := messagesAtOrAfter(query, anchor).Order("send_time asc, id asc").Limit(neededAfter + 1).Find(&atOrAfterMsgs).Error; err != nil {
			return nil, false, false, err
		}
		hasNewer := len(atOrAfterMsgs) > neededAfter
		if hasNewer {
			atOrAfterMsgs = atOrAfterMsgs[:neededAfter]
		}

		// 3. 若向后不够（靠后位置），且向前有更多记录，向前多取补足至 targetTotal
		if len(beforeMsgs)+len(atOrAfterMsgs) < targetTotal {
			missing := targetTotal - (len(beforeMsgs) + len(atOrAfterMsgs))
			oldestAnchor := anchor
			if len(beforeMsgs) > 0 {
				oldestAnchor = beforeMsgs[0]
			}
			extraBefore := make([]dal.ChatMessage, 0, missing)
			if err := messagesBefore(query, oldestAnchor).Order("send_time desc, id desc").Limit(missing).Find(&extraBefore).Error; err != nil {
				return nil, false, false, err
			}
			if len(extraBefore) > 0 {
				reverseChatMessages(extraBefore)
				beforeMsgs = append(extraBefore, beforeMsgs...)
			}
		}

		msgs := append(beforeMsgs, atOrAfterMsgs...)
		hasOlder, err := hasMessageBefore(userID, chatRoomID, msgType, oldestMessageID(msgs))
		if err != nil {
			return nil, false, false, err
		}
		if !hasNewer {
			hasNewer, err = hasMessageAfter(userID, chatRoomID, msgType, newestMessageID(msgs))
			if err != nil {
				return nil, false, false, err
			}
		}
		return msgs, hasOlder, hasNewer, nil
	default:
		if cursorID > 0 {
			cursor, found, err := findChatMessageCursor(userID, chatRoomID, msgType, cursorID)
			if err != nil {
				return nil, false, false, err
			}
			if !found {
				return []dal.ChatMessage{}, false, false, nil
			}
			query = messagesBefore(query, cursor)
		}
		msgs := make([]dal.ChatMessage, 0, limit)
		if err := query.Order("send_time desc, id desc").Limit(limit).Find(&msgs).Error; err != nil {
			return nil, false, false, err
		}
		hasOlder := len(msgs) > int(pageSize)
		if hasOlder {
			msgs = msgs[:pageSize]
		}
		hasNewer, err := hasMessageAfter(userID, chatRoomID, msgType, newestMessageID(msgs))
		return msgs, hasOlder, hasNewer, err
	}
}

func baseMessageQuery(userID string, chatRoomID string, msgType int32) *gorm.DB {
	query := dal.ReadDB().Table(dal.ChatMessage{}.TableName()).
		Where("user_id = ? AND chat_room_id = ?", userID, chatRoomID)
	if msgType != 0 {
		query = query.Where("msg_type = ?", msgType)
	}
	return query
}

func hasMessageBefore(userID string, chatRoomID string, msgType int32, id int64) (bool, error) {
	if id <= 0 {
		return false, nil
	}
	anchor, found, err := findChatMessageCursor(userID, chatRoomID, msgType, id)
	if err != nil || !found {
		return false, err
	}
	var msg dal.ChatMessage
	err = messagesBefore(baseMessageQuery(userID, chatRoomID, msgType), anchor).
		Order("send_time desc, id desc").
		Limit(1).
		Find(&msg).Error
	return msg.Id > 0, err
}

func hasMessageAfter(userID string, chatRoomID string, msgType int32, id int64) (bool, error) {
	if id <= 0 {
		return false, nil
	}
	anchor, found, err := findChatMessageCursor(userID, chatRoomID, msgType, id)
	if err != nil || !found {
		return false, err
	}
	var msg dal.ChatMessage
	err = messagesAfter(baseMessageQuery(userID, chatRoomID, msgType), anchor).
		Order("send_time asc, id asc").
		Limit(1).
		Find(&msg).Error
	return msg.Id > 0, err
}

func findChatMessageCursor(userID string, chatRoomID string, msgType int32, id int64) (dal.ChatMessage, bool, error) {
	var msg dal.ChatMessage
	err := baseMessageQuery(userID, chatRoomID, msgType).
		Where("id = ?", id).
		Limit(1).
		Find(&msg).Error
	return msg, msg.Id > 0, err
}

func messagesBefore(query *gorm.DB, anchor dal.ChatMessage) *gorm.DB {
	return query.Where("send_time < ? OR (send_time = ? AND id < ?)", anchor.SendTime, anchor.SendTime, anchor.Id)
}

func messagesAfter(query *gorm.DB, anchor dal.ChatMessage) *gorm.DB {
	return query.Where("send_time > ? OR (send_time = ? AND id > ?)", anchor.SendTime, anchor.SendTime, anchor.Id)
}

func messagesAtOrAfter(query *gorm.DB, anchor dal.ChatMessage) *gorm.DB {
	return query.Where("send_time > ? OR (send_time = ? AND id >= ?)", anchor.SendTime, anchor.SendTime, anchor.Id)
}

func buildChatMessageResp(msgs []dal.ChatMessage) ([]*ChatMessageDTO, int64, int64) {
	respMsgs := make([]*ChatMessageDTO, 0, len(msgs))
	for i := range msgs {
		respMsgs = append(respMsgs, chatMessageDTO(msgs[i]))
	}
	return respMsgs, oldestMessageID(msgs), newestMessageID(msgs)
}

func chatMessageDTO(m dal.ChatMessage) *ChatMessageDTO {
	return &ChatMessageDTO{
		Id:            m.Id,
		UserId:        m.UserId,
		DisplayName:   m.DisplayName,
		ChatRoomId:    m.ChatRoomId,
		ChatMessageId: m.ChatMessageId,
		MsgType:       m.MsgType,
		Content:       m.Content,
		ImageUrl:      m.ImageUrl,
		VideoUrl:      m.VideoUrl,
		ImageLocalURL: localMediaURL(m.ChatMessageId, m.ImagePath, m.ImageUrl, ".jpeg"),
		VideoLocalURL: localMediaURL(m.ChatMessageId, m.VideoPath, m.VideoUrl, ".mp4"),
		TimeStr:       m.TimeStr,
		SendTime:      m.SendTime,
	}
}

func oldestMessageID(msgs []dal.ChatMessage) int64 {
	if len(msgs) == 0 {
		return 0
	}
	oldest := msgs[0]
	for _, msg := range msgs[1:] {
		if msg.SendTime < oldest.SendTime || (msg.SendTime == oldest.SendTime && msg.Id < oldest.Id) {
			oldest = msg
		}
	}
	return oldest.Id
}

func newestMessageID(msgs []dal.ChatMessage) int64 {
	if len(msgs) == 0 {
		return 0
	}
	newest := msgs[0]
	for _, msg := range msgs[1:] {
		if msg.SendTime > newest.SendTime || (msg.SendTime == newest.SendTime && msg.Id > newest.Id) {
			newest = msg
		}
	}
	return newest.Id
}

func reverseChatMessages(msgs []dal.ChatMessage) {
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
}

// HandleGetChatMedia 根据 chat_message_id 查本地文件并返回。
// 路由建议：GET /media/:file，其中 :file 形如 {chat_message_id}.mp4 / {chat_message_id}.jpeg
func HandleGetChatMedia(ctx context.Context, c *app.RequestContext) {
	file := strings.TrimSpace(c.Param("file"))
	if file == "" {
		c.SetStatusCode(consts.StatusBadRequest)
		_, _ = c.Write([]byte("missing file"))
		return
	}
	msgID, ext := splitMessageFile(file)
	if msgID == "" {
		c.SetStatusCode(consts.StatusBadRequest)
		_, _ = c.Write([]byte("invalid file"))
		return
	}
	msg, err := getChatMessageByID(msgID)
	if err != nil {
		c.SetStatusCode(consts.StatusInternalServerError)
		_, _ = c.Write([]byte(err.Error()))
		return
	}
	if msg.ChatMessageId == "" {
		c.SetStatusCode(consts.StatusNotFound)
		_, _ = c.Write([]byte("not found"))
		return
	}

	mediaPath := pickMediaPath(msg, ext)
	if !isLocalMediaPath(mediaPath) {
		c.SetStatusCode(consts.StatusNotFound)
		_, _ = c.Write([]byte("media not found"))
		return
	}
	info, err := os.Stat(mediaPath)
	if err != nil || !info.Mode().IsRegular() {
		c.SetStatusCode(consts.StatusNotFound)
		_, _ = c.Write([]byte("media not found"))
		return
	}

	// Hertz uses a file stream here and keeps byte-range requests for video previews.
	c.Response.Header.Set("Cache-Control", "private, max-age=86400")
	c.File(mediaPath)
}

func parseInt32Query(c *app.RequestContext, key string, def int32) (int32, error) {
	v := strings.TrimSpace(string(c.Query(key)))
	if v == "" {
		return def, nil
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s 参数非法: %v", key, err)
	}
	return int32(n), nil
}

func parseInt64Query(c *app.RequestContext, key string, def int64) (int64, error) {
	v := strings.TrimSpace(string(c.Query(key)))
	if v == "" {
		return def, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s 参数非法: %v", key, err)
	}
	return n, nil
}

func buildMediaURL(c *app.RequestContext, msgID, ext string) string {
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	scheme := "http"
	if v := c.Request.Header.Peek("X-Forwarded-Proto"); len(v) > 0 {
		scheme = string(v)
	} else if s := c.URI().Scheme(); len(s) > 0 {
		scheme = string(s)
	}
	host := string(c.Host())
	return fmt.Sprintf("%s://%s/media/%s%s", scheme, host, msgID, ext)
}

func localMediaURL(messageID, localPath, remoteURL, defaultExt string) string {
	if strings.TrimSpace(messageID) == "" || strings.TrimSpace(localPath) == "" {
		return ""
	}
	ext := filepath.Ext(localPath)
	if ext == "" {
		if parsed, err := url.Parse(remoteURL); err == nil {
			ext = filepath.Ext(parsed.Path)
		}
	}
	if ext == "" {
		ext = defaultExt
	}
	return "/media/" + url.PathEscape(messageID) + ext
}

func isLocalMediaPath(mediaPath string) bool {
	if strings.TrimSpace(mediaPath) == "" || strings.TrimSpace(config.GetMediaPath()) == "" {
		return false
	}
	root, err := filepath.Abs(config.GetMediaPath())
	if err != nil {
		return false
	}
	path, err := filepath.Abs(mediaPath)
	if err != nil {
		return false
	}
	if resolvedRoot, err := filepath.EvalSymlinks(root); err == nil {
		root = resolvedRoot
	}
	if resolvedPath, err := filepath.EvalSymlinks(path); err == nil {
		path = resolvedPath
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel)
}

func splitMessageFile(file string) (msgID string, ext string) {
	file = strings.TrimPrefix(file, "/")
	if file == "" {
		return "", ""
	}
	// 只允许一段，避免路径穿越
	if strings.Contains(file, "/") || strings.Contains(file, "\\") {
		return "", ""
	}
	msgID = file
	if i := strings.LastIndex(file, "."); i > 0 {
		msgID = file[:i]
		ext = file[i:]
	}
	return msgID, ext
}

func getChatMessageByID(chatMessageID string) (*dal.ChatMessage, error) {
	var msg dal.ChatMessage
	err := dal.ReadDB().Table(dal.ChatMessage{}.TableName()).
		Where("chat_message_id = ?", chatMessageID).
		Limit(1).
		Find(&msg).Error
	if err != nil {
		hlog.Errorf("query chat_message failed, id=%s err=%v", chatMessageID, err)
		return nil, err
	}
	return &msg, nil
}

func pickMediaPath(msg *dal.ChatMessage, ext string) string {
	ext = strings.ToLower(ext)
	// 有 ext 时按 ext 选，没 ext 时优先视频再图片
	if ext != "" {
		if strings.HasPrefix(ext, ".mp4") {
			return msg.VideoPath
		}
		if strings.HasPrefix(ext, ".jpg") || strings.HasPrefix(ext, ".jpeg") || strings.HasPrefix(ext, ".png") || strings.HasPrefix(ext, ".webp") {
			return msg.ImagePath
		}
	}
	if msg.VideoPath != "" {
		return msg.VideoPath
	}
	return msg.ImagePath
}

type SendMessageRequest struct {
	ChatRoomId string `json:"chat_room_id"`
	Content    string `json:"content"`
}

type SendMessageResponse struct {
	ChatMessageId string `json:"chat_message_id"`
}

// HandleSendChatMessage 发送聊天消息
func HandleSendChatMessage(ctx context.Context, c *app.RequestContext) {
	if !config.Conf.SendChatEnabled {
		c.JSON(consts.StatusOK, BadResp("发送消息功能已关闭，请在 config.yaml 中设置 send_chat: true 开启"))
		return
	}
	var req SendMessageRequest
	if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
		c.JSON(consts.StatusOK, BadResp("请求格式错误: "+err.Error()))
		return
	}
	req.ChatRoomId = strings.TrimSpace(req.ChatRoomId)
	req.Content = strings.TrimSpace(req.Content)
	if req.ChatRoomId == "" || req.Content == "" {
		c.JSON(consts.StatusOK, BadResp("chat_room_id 和 content 不能为空"))
		return
	}

	// 通过 chat_room_id 查找主播（Talent）的 ID 作为发送目标
	room, err := dal.GetChatRoomByChatRoomId(req.ChatRoomId)
	if err != nil {
		hlog.Errorf("HandleSendChatMessage: GetChatRoomByChatRoomId failed: %v", err)
		c.JSON(consts.StatusOK, BadResp("查找聊天室失败: "+err.Error()))
		return
	}
	hlog.Infof("HandleSendChatMessage: talent_user_id=%s chat_room_id=%s", room.UserId, req.ChatRoomId)
	if room == nil || room.UserId == "" {
		c.JSON(consts.StatusOK, BadResp("聊天室不存在或 user_id 为空"))
		return
	}

	chatMessageID, err := rep_api.SendChatMessageWithID(room.UserId, req.ChatRoomId, req.Content)
	if err != nil {
		hlog.Errorf("HandleSendChatMessage: SendChatMessage failed: %v", err)
		c.JSON(consts.StatusOK, BadResp("发送消息失败: "+err.Error()))
		return
	}
	hlog.Infof("HandleSendChatMessage: success")
	c.JSON(consts.StatusOK, &Resp{Data: &SendMessageResponse{ChatMessageId: chatMessageID}})
}
