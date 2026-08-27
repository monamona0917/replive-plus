package dal

import "gorm.io/gorm"

func GetChatRooms() ([]*ChatRoom, error) {
	innerChatRooms := make([]*ChatRoom, 0)
	err := db.Table(ChatRoom{}.TableName()).
		Select(`chat_rooms.*,
			COALESCE((
				SELECT send_time
				FROM chat_messages
				WHERE chat_messages.user_id = chat_rooms.user_id
					AND chat_messages.chat_room_id = chat_rooms.chat_room_id
				ORDER BY send_time DESC, id DESC
				LIMIT 1
			), 0) AS last_message_time,
			COALESCE((
				SELECT content
				FROM chat_messages
				WHERE chat_messages.user_id = chat_rooms.user_id
					AND chat_messages.chat_room_id = chat_rooms.chat_room_id
				ORDER BY send_time DESC, id DESC
				LIMIT 1
			), '') AS last_message_content,
			COALESCE((
				SELECT msg_type
				FROM chat_messages
				WHERE chat_messages.user_id = chat_rooms.user_id
					AND chat_messages.chat_room_id = chat_rooms.chat_room_id
				ORDER BY send_time DESC, id DESC
				LIMIT 1
			), 0) AS last_message_type`).
		Order("last_message_time DESC, display_name ASC, id ASC").
		Find(&innerChatRooms).Error
	return innerChatRooms, err
}

func GetChatRoomByChatRoomId(chatRoomId string) (*ChatRoom, error) {
	var room ChatRoom
	err := db.Table(ChatRoom{}.TableName()).
		Where("chat_room_id = ?", chatRoomId).
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

func GetUserPrivate() (*UserPrivate, error) {
	var user UserPrivate
	err := ReadDB().Table(UserPrivate{}.TableName()).Order("id desc").Limit(1).Find(&user).Error
	if err != nil {
		return nil, err
	}
	if user.Id == 0 {
		return nil, nil
	}
	return &user, nil
}

func UpdateChatRoomAvatarPath(chatRoomID, path string) error {
	return WithWriteDB(func(db *gorm.DB) error {
		return db.Model(&ChatRoom{}).Where("chat_room_id = ?", chatRoomID).Update("avatar_path", path).Error
	})
}

func UpdateUserPrivateProfileImagePath(userID, path string) error {
	return WithWriteDB(func(db *gorm.DB) error {
		return db.Model(&UserPrivate{}).Where("user_id = ?", userID).Update("profile_image_path", path).Error
	})
}
