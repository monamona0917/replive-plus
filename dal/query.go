package dal

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
