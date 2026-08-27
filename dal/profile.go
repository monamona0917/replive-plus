package dal

import "gorm.io/gorm"

func UpdatePrimeChatRoomTalentAvatarPath(chatRoomID, path string) error {
	return WithWriteDB(func(db *gorm.DB) error {
		return db.Model(&PrimeChatRoom{}).
			Where("chat_room_id = ?", chatRoomID).
			Update("talent_avatar_path", path).Error
	})
}
