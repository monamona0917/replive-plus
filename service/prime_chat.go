package service

import (
	"fmt"
	"path/filepath"
	"replive/config"
	"replive/dal"
	"replive/rep_api"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
)

const (
	primeChatFollowingPageSize = 20
	primeChatFollowingMaxPages = 50
)

type PrimeChatStartupSummary struct {
	Rooms           int
	BackgroundMedia MediaSyncSummary
}

// syncPrimeChatAtStartup performs the single read-only Prime Chat sync for a
// backend process. It is deliberately not registered as a periodic worker.
func syncPrimeChatAtStartup() error {
	_, err := syncPrimeChatAtStartupWithSummary()
	return err
}

func syncPrimeChatAtStartupWithSummary() (PrimeChatStartupSummary, error) {
	summary := PrimeChatStartupSummary{}
	now := time.Now()
	rooms, listErr := rep_api.ListPrimeChatRooms()
	if listErr != nil {
		hlog.Warnf("ListPrimeChatRooms failed; using followed-user discovery: %v", listErr)
	}

	// A Prime room list may contain only rooms already joined in the app. Scan
	// followed users as a startup-only supplement so an enabled Prime Chat is
	// not missed merely because it has no prior local message history.
	followedRooms, followingErr := findPrimeChatRoomsForFollowedUsers()
	if followingErr != nil {
		if listErr != nil || len(rooms) == 0 {
			return summary, fmt.Errorf("discover Prime Chat rooms from followed users: %w", followingErr)
		}
		hlog.Warnf("followed-user Prime Chat discovery failed; using listed rooms only: %v", followingErr)
	} else {
		rooms = mergePrimeChatRooms(rooms, followedRooms)
	}

	if len(rooms) == 0 {
		hlog.Infof("Prime Chat startup room sync found no available rooms")
	}

	summary.Rooms = len(rooms)
	backgroundSummary, err := processPrimeChatRooms(rooms, now)
	if err != nil {
		return summary, err
	}
	summary.BackgroundMedia = backgroundSummary
	if err := syncPrimeChatMessages(); err != nil {
		return summary, err
	}
	return summary, nil
}

// findPrimeChatRoomsForFollowedUsers discovers enabled Prime Chat rooms from
// the current follow list. It is called only once during backend startup.
func findPrimeChatRoomsForFollowedUsers() ([]*rep_api.PrimeChatRoom, error) {
	type candidate struct {
		userID string
		label  string
	}

	candidates := make([]candidate, 0)
	seenUsers := make(map[string]struct{})
	pageToken := ""

	for page := 0; page < primeChatFollowingMaxPages; page++ {
		resp, err := rep_api.ListFollowings(primeChatFollowingPageSize, pageToken)
		if err != nil {
			return nil, fmt.Errorf("list followed users for Prime fallback: %w", err)
		}

		for _, target := range resp.GetFollowTargets() {
			if target == nil {
				continue
			}
			user := target.GetUser()
			if user == nil && target.GetOshi() != nil {
				user = target.GetOshi().GetUser()
			}
			userID := strings.TrimSpace(user.GetUserId())
			if userID == "" {
				continue
			}
			if _, exists := seenUsers[userID]; exists {
				continue
			}
			seenUsers[userID] = struct{}{}
			candidates = append(candidates, candidate{
				userID: userID,
				label:  firstNonEmpty(user.GetDisplayName(), user.GetUniqueId(), userID),
			})
		}

		nextPageToken := strings.TrimSpace(resp.GetNextPageToken())
		if nextPageToken == "" || nextPageToken == pageToken {
			break
		}
		pageToken = nextPageToken
	}

	rooms := make([]*rep_api.PrimeChatRoom, 0, len(candidates))
	seenRooms := make(map[string]struct{})
	for _, candidate := range candidates {
		primeRoom, err := rep_api.GetPrimeChatRoom(candidate.userID)
		if err != nil {
			hlog.Debugf("GetPrimeChatRoom fallback skipped %s: %v", candidate.label, err)
			continue
		}
		if !isUsablePrimeChatRoom(primeRoom) {
			continue
		}
		if _, exists := seenRooms[primeRoom.ChatRoomId]; exists {
			continue
		}
		seenRooms[primeRoom.ChatRoomId] = struct{}{}
		rooms = append(rooms, primeRoom)
	}

	hlog.Infof("Prime Chat followed-user discovery done: candidates=%d rooms=%d", len(candidates), len(rooms))
	return rooms, nil
}

func mergePrimeChatRooms(groups ...[]*rep_api.PrimeChatRoom) []*rep_api.PrimeChatRoom {
	rooms := make([]*rep_api.PrimeChatRoom, 0)
	seen := make(map[string]struct{})
	for _, group := range groups {
		for _, room := range group {
			if !isUsablePrimeChatRoom(room) {
				continue
			}
			if _, exists := seen[room.ChatRoomId]; exists {
				continue
			}
			seen[room.ChatRoomId] = struct{}{}
			rooms = append(rooms, room)
		}
	}
	return rooms
}

func isUsablePrimeChatRoom(room *rep_api.PrimeChatRoom) bool {
	return room != nil && strings.TrimSpace(room.ChatRoomId) != "" && strings.TrimSpace(room.TalentUserId) != ""
}

func processPrimeChatRooms(rooms []*rep_api.PrimeChatRoom, now time.Time) (mediaSummary MediaSyncSummary, err error) {
	dbRooms := make([]*dal.PrimeChatRoom, 0, len(rooms))
	defer func() {
		logMediaSyncSummary("Prime Chat background media sync", mediaSummary)
	}()

	for _, room := range rooms {
		if !isUsablePrimeChatRoom(room) {
			continue
		}

		dbRooms = append(dbRooms, &dal.PrimeChatRoom{
			ChatRoomId:                room.ChatRoomId,
			TalentUserId:              room.TalentUserId,
			TalentUniqueId:            room.TalentUniqueId,
			TalentDisplayName:         room.TalentDisplayName,
			TalentAvatarUrl:           room.TalentAvatarUrl,
			MemberUserId:              room.MemberUserId,
			MemberBackgroundImageUrl:  room.MemberBackgroundImageUrl,
			TalentLastCheckTimeMillis: room.TalentLastCheckTimeMillis,
			MemberLastCheckTimeMillis: room.MemberLastCheckTimeMillis,
			SyncedAt:                  now.Unix(),
		})

		url := strings.TrimSpace(room.MemberBackgroundImageUrl)
		if url == "" {
			continue
		}
		owner := firstNonEmpty(room.TalentDisplayName, room.TalentUniqueId, room.TalentUserId, "unknown")
		prefix := filepath.Join(config.GetMediaPath(), "profile")
		if _, result, err := DownloadProfileMediaWithResult(url, now, prefix, owner, "prime_chat_background"); err != nil {
			mediaSummary.Add(MediaFailed)
			hlog.Warnf("download Prime Chat background for %s: %v", owner, err)
		} else {
			mediaSummary.Add(result)
		}
	}

	if err := dal.SavePrimeChatRooms(dbRooms); err != nil {
		return mediaSummary, fmt.Errorf("save Prime Chat rooms: %w", err)
	}
	hlog.Infof("Prime Chat startup room sync done: rooms=%d backgrounds_downloaded=%d backgrounds_skipped=%d backgrounds_failed=%d", len(dbRooms), mediaSummary.Downloaded, mediaSummary.Skipped, mediaSummary.Failed)
	return mediaSummary, nil
}

// syncPrimeChatMessages walks every page once at startup. The server orders
// this stream newest-to-oldest, so all pages must be visited to guarantee that
// the local database contains the full history and any reaction updates.
func syncPrimeChatMessages() error {
	rooms, err := dal.GetPrimeChatRooms()
	if err != nil {
		return fmt.Errorf("get saved Prime Chat rooms: %w", err)
	}
	if len(rooms) == 0 {
		hlog.Infof("Prime Chat startup message sync skipped: no joined rooms")
		return nil
	}

	cursor := ""
	seenCursors := make(map[string]struct{})
	for pageNo := 1; ; pageNo++ {
		page, err := rep_api.ListPrimeChatMessagesOfJoinedChatRooms(cursor)
		if err != nil {
			return fmt.Errorf("list Prime Chat messages page %d: %w", pageNo, err)
		}
		if page == nil {
			return fmt.Errorf("list Prime Chat messages page %d returned no response", pageNo)
		}

		messages, unmatched := buildPrimeChatMessages(page, rooms)
		if unmatched > 0 {
			hlog.Warnf("Prime Chat page %d contains %d messages that could not be matched to a saved room", pageNo, unmatched)
		}
		if len(messages) > 0 {
			changed, err := dal.SavePrimeChatMessages(messages)
			if err != nil {
				return fmt.Errorf("save Prime Chat messages page %d: %w", pageNo, err)
			}
			hlog.Infof("Prime Chat startup message sync page=%d saved_or_updated=%d", pageNo, changed)
		}

		if !page.HasNextPage {
			break
		}
		nextCursor := strings.TrimSpace(page.NextPageCursorMessageId)
		if nextCursor == "" {
			return fmt.Errorf("Prime Chat page %d has_next_page without a cursor", pageNo)
		}
		if nextCursor == cursor {
			return fmt.Errorf("Prime Chat page %d repeated its cursor", pageNo)
		}
		if _, exists := seenCursors[nextCursor]; exists {
			return fmt.Errorf("Prime Chat page %d repeated an earlier cursor", pageNo)
		}
		seenCursors[nextCursor] = struct{}{}
		cursor = nextCursor
	}
	return nil
}

func buildPrimeChatMessages(page *rep_api.PrimeChatMessagesPage, rooms []*dal.PrimeChatRoom) ([]*dal.PrimeChatMessage, int) {
	byID := make(map[string]*dal.PrimeChatMessage, len(page.Messages)+len(page.AllReactedMessages))
	orderedIDs := make([]string, 0, len(page.Messages)+len(page.AllReactedMessages))
	unmatched := 0

	for _, message := range page.Messages {
		dbMessage := buildPrimeChatMessage(message, rooms)
		if dbMessage == nil {
			if message != nil && strings.TrimSpace(message.MessageId) != "" {
				unmatched++
			}
			continue
		}
		if _, exists := byID[dbMessage.MessageId]; !exists {
			orderedIDs = append(orderedIDs, dbMessage.MessageId)
		}
		// The normal message list is the canonical record and can clear an old
		// reaction when the server no longer returns one.
		byID[dbMessage.MessageId] = dbMessage
	}

	for _, message := range page.AllReactedMessages {
		dbMessage := buildPrimeChatMessage(message, rooms)
		if dbMessage == nil {
			if message != nil && strings.TrimSpace(message.MessageId) != "" {
				unmatched++
			}
			continue
		}

		if current, exists := byID[dbMessage.MessageId]; exists {
			mergePrimeReactionMessage(current, dbMessage)
			continue
		}

		// This list can contain an older message outside the current normal
		// page. Mark it so DAL preserves any already saved full payload.
		dbMessage.ReactionOnly = true
		byID[dbMessage.MessageId] = dbMessage
		orderedIDs = append(orderedIDs, dbMessage.MessageId)
	}

	messages := make([]*dal.PrimeChatMessage, 0, len(orderedIDs))
	for _, messageID := range orderedIDs {
		if message := byID[messageID]; message != nil {
			messages = append(messages, message)
		}
	}
	return messages, unmatched
}

// mergePrimeReactionMessage enriches a canonical message with the separate
// reaction feed without replacing its body fields with an incomplete record.
func mergePrimeReactionMessage(target, reaction *dal.PrimeChatMessage) {
	if target == nil || reaction == nil {
		return
	}
	if target.ChatRoomId == "" {
		target.ChatRoomId = reaction.ChatRoomId
	}
	if target.ChatRoomOwnerUserId == "" {
		target.ChatRoomOwnerUserId = reaction.ChatRoomOwnerUserId
	}
	if target.MemberUserId == "" {
		target.MemberUserId = reaction.MemberUserId
	}
	if target.Sender == "" || target.Sender == rep_api.PrimeChatSenderUnknown {
		target.Sender = reaction.Sender
	}
	if target.BodyType == "" || target.BodyType == rep_api.PrimeChatBodyUnknown {
		target.BodyType = reaction.BodyType
	}
	if target.TextContent == "" {
		target.TextContent = reaction.TextContent
	}
	if target.ImageUrl == "" {
		target.ImageUrl = reaction.ImageUrl
	}
	if target.VideoUrl == "" {
		target.VideoUrl = reaction.VideoUrl
	}
	if target.VideoThumbnailUrl == "" {
		target.VideoThumbnailUrl = reaction.VideoThumbnailUrl
	}
	if target.CoinAmount == 0 {
		target.CoinAmount = reaction.CoinAmount
	}
	if target.CreateUnixTimeMillis == 0 {
		target.CreateUnixTimeMillis = reaction.CreateUnixTimeMillis
	}
	if reaction.ReactionEmoji != "" {
		target.ReactionEmoji = reaction.ReactionEmoji
	}
	target.IsDeleted = target.IsDeleted || reaction.IsDeleted
}

func buildPrimeChatMessage(message *rep_api.PrimeChatMessage, rooms []*dal.PrimeChatRoom) *dal.PrimeChatMessage {
	if message == nil || strings.TrimSpace(message.MessageId) == "" {
		return nil
	}
	room := matchPrimeRoom(message, rooms)
	if room == nil {
		return nil
	}
	return &dal.PrimeChatMessage{
		MessageId:            message.MessageId,
		ChatRoomId:           room.ChatRoomId,
		ChatRoomOwnerUserId:  room.TalentUserId,
		MemberUserId:         firstNonEmpty(message.MemberUserId, room.MemberUserId),
		Sender:               message.Sender,
		BodyType:             message.BodyType,
		TextContent:          message.Content,
		ImageUrl:             message.ImageUrl,
		VideoUrl:             message.VideoUrl,
		VideoThumbnailUrl:    message.VideoThumbnailUrl,
		CoinAmount:           message.CoinAmount,
		ReactionEmoji:        message.ReactionEmoji,
		IsDeleted:            message.IsDeleted,
		CreateUnixTimeMillis: message.CreateUnixTimeMillis,
	}
}

func matchPrimeRoom(message *rep_api.PrimeChatMessage, rooms []*dal.PrimeChatRoom) *dal.PrimeChatRoom {
	for _, room := range rooms {
		if room == nil {
			continue
		}
		if message.UserId != "" && room.TalentUserId == message.UserId &&
			(message.MemberUserId == "" || room.MemberUserId == message.MemberUserId) {
			return room
		}
		if message.MemberUserId != "" && room.MemberUserId == message.MemberUserId &&
			(message.UserId == "" || room.TalentUserId == message.UserId) {
			return room
		}
	}
	return nil
}
