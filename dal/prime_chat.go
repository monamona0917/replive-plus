package dal

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

func GetPrimeChatRooms() ([]*PrimeChatRoom, error) {
	rooms := make([]*PrimeChatRoom, 0)
	err := ReadDB().Table(PrimeChatRoom{}.TableName()).
		Select(`prime_chat_rooms.*,
			COALESCE((
				SELECT create_unix_time_ms
				FROM prime_chat_messages
				WHERE prime_chat_messages.chat_room_id = prime_chat_rooms.chat_room_id
				ORDER BY create_unix_time_ms DESC, id DESC
				LIMIT 1
			), 0) AS last_message_time,
			COALESCE((
				SELECT text_content
				FROM prime_chat_messages
				WHERE prime_chat_messages.chat_room_id = prime_chat_rooms.chat_room_id
				ORDER BY create_unix_time_ms DESC, id DESC
				LIMIT 1
			), '') AS last_message_content,
			COALESCE((
				SELECT body_type
				FROM prime_chat_messages
				WHERE prime_chat_messages.chat_room_id = prime_chat_rooms.chat_room_id
				ORDER BY create_unix_time_ms DESC, id DESC
				LIMIT 1
			), '') AS last_message_type`).
		Order("last_message_time DESC, talent_display_name ASC, id ASC").
		Find(&rooms).Error
	return rooms, err
}

func GetPrimeChatRoomByChatRoomID(chatRoomID string) (*PrimeChatRoom, error) {
	var room PrimeChatRoom
	err := ReadDB().Table(PrimeChatRoom{}.TableName()).
		Where("chat_room_id = ?", chatRoomID).
		Limit(1).
		Find(&room).Error
	if err != nil {
		return nil, err
	}
	if room.Id == 0 {
		return nil, nil
	}
	return &room, nil
}

func GetPrimeChatRoomByParticipants(talentUserID, memberUserID string) (*PrimeChatRoom, error) {
	var room PrimeChatRoom
	query := ReadDB().Table(PrimeChatRoom{}.TableName()).
		Where("talent_user_id = ?", talentUserID)
	if memberUserID != "" {
		query = query.Where("member_user_id = ?", memberUserID)
	}
	err := query.Order("id desc").Limit(1).Find(&room).Error
	if err != nil {
		return nil, err
	}
	if room.Id == 0 {
		return nil, nil
	}
	return &room, nil
}

func SavePrimeChatRooms(rooms []*PrimeChatRoom) error {
	return WithWriteDB(func(db *gorm.DB) error {
		return db.Transaction(func(tx *gorm.DB) error {
			for _, room := range rooms {
				if room == nil || room.ChatRoomId == "" || room.TalentUserId == "" {
					continue
				}

				var existing PrimeChatRoom
				err := tx.Table(PrimeChatRoom{}.TableName()).
					// Earlier builds mistakenly persisted talent_user_id as
					// chat_room_id. Match either key so startup sync migrates that
					// row in place instead of colliding with the participant index.
					Where("chat_room_id = ? OR (talent_user_id = ? AND member_user_id = ?)", room.ChatRoomId, room.TalentUserId, room.MemberUserId).
					Limit(1).
					First(&existing).Error
				switch {
				case errors.Is(err, gorm.ErrRecordNotFound):
					if err := tx.Create(room).Error; err != nil {
						return fmt.Errorf("create prime chat room %s: %w", room.ChatRoomId, err)
					}
				case err != nil:
					return fmt.Errorf("query prime chat room %s: %w", room.ChatRoomId, err)
				default:
					if existing.ChatRoomId != "" && existing.ChatRoomId != room.ChatRoomId {
						if err := tx.Table(PrimeChatMessage{}.TableName()).
							Where("chat_room_id = ?", existing.ChatRoomId).
							Update("chat_room_id", room.ChatRoomId).Error; err != nil {
							return fmt.Errorf("migrate prime chat messages from %s to %s: %w", existing.ChatRoomId, room.ChatRoomId, err)
						}
					}
					room.Id = existing.Id
					if err := tx.Save(room).Error; err != nil {
						return fmt.Errorf("update prime chat room %s: %w", room.ChatRoomId, err)
					}
				}
			}
			return nil
		})
	})
}

// SavePrimeChatMessages updates existing rows as well as inserting new ones.
// A reaction can be added to an old message after the message was first saved.
func SavePrimeChatMessages(messages []*PrimeChatMessage) (int, error) {
	changed := 0
	err := WithWriteDB(func(db *gorm.DB) error {
		for _, message := range messages {
			if message == nil || message.MessageId == "" || message.ChatRoomId == "" {
				continue
			}

			var existing PrimeChatMessage
			err := db.Table(PrimeChatMessage{}.TableName()).
				Where("message_id = ?", message.MessageId).
				Limit(1).
				First(&existing).Error
			switch {
			case errors.Is(err, gorm.ErrRecordNotFound):
				if err := db.Create(message).Error; err != nil {
					return fmt.Errorf("create prime chat message %s: %w", message.MessageId, err)
				}
				changed++
			case err != nil:
				return fmt.Errorf("query prime chat message %s: %w", message.MessageId, err)
			default:
				toSave := message
				if message.ReactionOnly {
					toSave = mergePrimeReactionOnlyMessage(&existing, message)
				}
				toSave.Id = existing.Id
				if err := db.Save(toSave).Error; err != nil {
					return fmt.Errorf("update prime chat message %s: %w", message.MessageId, err)
				}
				changed++
			}
		}
		return nil
	})
	return changed, err
}

// mergePrimeReactionOnlyMessage keeps an already persisted message intact when
// the API's all-reacted list supplies only a reaction update for an older row.
func mergePrimeReactionOnlyMessage(existing, incoming *PrimeChatMessage) *PrimeChatMessage {
	merged := *incoming
	merged.ReactionOnly = false

	if merged.ChatRoomId == "" {
		merged.ChatRoomId = existing.ChatRoomId
	}
	if merged.ChatRoomOwnerUserId == "" {
		merged.ChatRoomOwnerUserId = existing.ChatRoomOwnerUserId
	}
	if merged.MemberUserId == "" {
		merged.MemberUserId = existing.MemberUserId
	}
	if merged.Sender == "" || merged.Sender == "unknown" {
		merged.Sender = existing.Sender
	}
	if merged.BodyType == "" || merged.BodyType == "unknown" {
		merged.BodyType = existing.BodyType
	}
	if merged.TextContent == "" {
		merged.TextContent = existing.TextContent
	}
	if merged.ImageUrl == "" {
		merged.ImageUrl = existing.ImageUrl
	}
	if merged.ImageThumbnailUrl == "" {
		merged.ImageThumbnailUrl = existing.ImageThumbnailUrl
	}
	if merged.VideoUrl == "" {
		merged.VideoUrl = existing.VideoUrl
	}
	if merged.VideoThumbnailUrl == "" {
		merged.VideoThumbnailUrl = existing.VideoThumbnailUrl
	}
	if merged.VideoDurationSeconds == 0 {
		merged.VideoDurationSeconds = existing.VideoDurationSeconds
	}
	if merged.CoinAmount == 0 {
		merged.CoinAmount = existing.CoinAmount
	}
	if merged.ReactionEmoji == "" {
		merged.ReactionEmoji = existing.ReactionEmoji
	}
	merged.IsDeleted = merged.IsDeleted || existing.IsDeleted
	if merged.CreateUnixTimeMillis == 0 {
		merged.CreateUnixTimeMillis = existing.CreateUnixTimeMillis
	}

	return &merged
}

func HasPrimeChatMessages() (bool, error) {
	var count int64
	err := ReadDB().Table(PrimeChatMessage{}.TableName()).Limit(1).Count(&count).Error
	return count > 0, err
}

func ExistingPrimeChatMessageIDs(messageIDs []string) (map[string]struct{}, error) {
	existing := make(map[string]struct{})
	if len(messageIDs) == 0 {
		return existing, nil
	}

	type row struct {
		MessageId string `gorm:"column:message_id"`
	}
	rows := make([]row, 0, len(messageIDs))
	err := ReadDB().Table(PrimeChatMessage{}.TableName()).
		Select("message_id").
		Where("message_id IN ?", messageIDs).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, item := range rows {
		if item.MessageId != "" {
			existing[item.MessageId] = struct{}{}
		}
	}
	return existing, nil
}
