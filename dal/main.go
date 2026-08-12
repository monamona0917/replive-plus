package dal

type ChatRoom struct {
	Id                 int64  `json:"id"`
	UserId             string `json:"user_id"`
	UniqueId           string `json:"unique_id"`
	DisplayName        string `json:"display_name"`
	ChatRoomId         string `json:"chat_room_id"`
	AvatarUrl          string `json:"avatar_url"`
	LastMessageTime    int64  `json:"last_message_time" gorm:"column:last_message_time;->;-:migration"`
	LastMessageContent string `json:"last_message_content" gorm:"column:last_message_content;->;-:migration"`
	LastMessageType    int32  `json:"last_message_type" gorm:"column:last_message_type;->;-:migration"`
}

func (c ChatRoom) TableName() string {
	return "chat_rooms"
}

type Oshi struct {
	Id                        int64  `json:"id"`
	OshiId                    string `json:"oshi_id"`
	Name                      string `json:"name"`
	Type                      int64  `json:"type"`
	ProfileImageUrl           string `json:"profile_image_url"`
	MembershipImageUrl        string `json:"membership_image_url"`
	ProfileSharelinkUrl       string `json:"profile_sharelink_url"`
	IsFollowing               bool   `json:"is_following"`
	UserId                    string `json:"user_id"`
	UniqueId                  string `json:"unique_id"`
	DisplayName               string `json:"display_name"`
	UserProfileImageUrl       string `json:"user_profile_image_url"`
	ProfileBackgroundImageUrl string `json:"profile_background_image_url"`
	SmProfileImageUrl         string `json:"sm_profile_image_url"`
	Biography                 string `json:"biography"`
	OnelinerMessage           string `json:"oneliner_message"`
	FandomName                string `json:"fandom_name"`
	UserOshiId                string `json:"user_oshi_id"`
	IsVerified                bool   `json:"is_verified"`
	IsFollowed                bool   `json:"is_followed"`
	IsUserFollowing           bool   `json:"is_user_following"`
	IsRetired                 bool   `json:"is_retired"`
	IsSuspended               bool   `json:"is_suspended"`
	IsBlocking                bool   `json:"is_blocking"`
	IsBlocked                 bool   `json:"is_blocked"`
	IsSubscribing             bool   `json:"is_subscribing"`
	HasMembership             bool   `json:"has_membership"`
	CanStream                 bool   `json:"can_stream"`
	HasOshiCalendar           bool   `json:"has_oshi_calendar"`
	SubscribeStartTime        int64  `json:"subscribe_start_time"`
	SubscribeDayCount         int64  `json:"subscribe_day_count"`
	CreateTime                int64  `json:"create_time"`
	UpdateTime                int64  `json:"update_time"`
	SyncedAt                  int64  `json:"synced_at"`
}

func (c Oshi) TableName() string {
	return "oshis"
}

type UserPrivate struct {
	Id                                     int64  `json:"id"`
	UserId                                 string `json:"user_id"`
	UniqueId                               string `json:"unique_id"`
	DisplayName                            string `json:"display_name"`
	ProfileImageUrl                        string `json:"profile_image_url"`
	SmProfileImageUrl                      string `json:"sm_profile_image_url"`
	ProfileBackgroundImageUrl              string `json:"profile_background_image_url"`
	Biography                              string `json:"biography"`
	OnelinerMessage                        string `json:"oneliner_message"`
	FandomName                             string `json:"fandom_name"`
	OshiId                                 string `json:"oshi_id"`
	InstallLinkUrl                         string `json:"install_link_url"`
	LivePreparationSharelinkUrl            string `json:"live_preparation_sharelink_url"`
	CardboxSharelinkUrl                    string `json:"cardbox_sharelink_url"`
	ProfileSharelinkUrl                    string `json:"profile_sharelink_url"`
	TalentPageSharelinkUrl                 string `json:"talent_page_sharelink_url"`
	FollowerCount                          int64  `json:"follower_count"`
	FollowingCount                         int64  `json:"following_count"`
	IsVerified                             bool   `json:"is_verified"`
	CanStream                              bool   `json:"can_stream"`
	HasMembership                          bool   `json:"has_membership"`
	HasPrimeChat                           bool   `json:"has_prime_chat"`
	HasOshiCalendar                        bool   `json:"has_oshi_calendar"`
	IsPremiumSubscriber                    bool   `json:"is_premium_subscriber"`
	IsSmsLogin                             bool   `json:"is_sms_login"`
	IsUniqueIdUpdatable                    bool   `json:"is_unique_id_updatable"`
	IsUniqueIdUpdatableManyTimes           bool   `json:"is_unique_id_updatable_many_times"`
	IsDisplayNameUpdatable                 bool   `json:"is_display_name_updatable"`
	IsDisplayNameUpdatableManyTimes        bool   `json:"is_display_name_updatable_many_times"`
	CanPurchaseItems                       bool   `json:"can_purchase_items"`
	IsRegisteredPaymentPersonalInformation bool   `json:"is_registered_payment_personal_information"`
	IsRegisteredBankAccount                bool   `json:"is_registered_bank_account"`
	UniqueIdUpdatableTime                  int64  `json:"unique_id_updatable_time"`
	DisplayNameUpdatableTime               int64  `json:"display_name_updatable_time"`
	PermanentRemoveTime                    int64  `json:"permanent_remove_time"`
	SyncedAt                               int64  `json:"synced_at"`
}

func (c UserPrivate) TableName() string {
	return "user_private"
}

type Following struct {
	Id                        int64  `json:"id"`
	TargetType                int64  `json:"target_type"`
	TargetKey                 string `json:"target_key"`
	UserId                    string `json:"user_id"`
	UniqueId                  string `json:"unique_id"`
	DisplayName               string `json:"display_name"`
	ProfileImageUrl           string `json:"profile_image_url"`
	SmProfileImageUrl         string `json:"sm_profile_image_url"`
	ProfileBackgroundImageUrl string `json:"profile_background_image_url"`
	OshiId                    string `json:"oshi_id"`
	OshiName                  string `json:"oshi_name"`
	OshiProfileImageUrl       string `json:"oshi_profile_image_url"`
	OshiMembershipImageUrl    string `json:"oshi_membership_image_url"`
	SyncedAt                  int64  `json:"synced_at"`
}

func (c Following) TableName() string {
	return "followings"
}

func createTable() error {
	var err error
	err = db.Exec(`CREATE TABLE IF NOT EXISTS chat_rooms (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    unique_id TEXT NOT NULL,
    display_name TEXT NOT NULL,
    chat_room_id TEXT NOT NULL,
    avatar_url TEXT NOT NULL
)`).Error
	if err != nil {
		return err
	}
	err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_chat_rooms_display_name ON chat_rooms(display_name);`).Error
	if err != nil {
		return err
	}
	err = db.Exec(`CREATE TABLE IF NOT EXISTS live_streams (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    display_name TEXT NOT NULL,
    title TEXT NOT NULL,
    webrtc_url TEXT NOT NULL,
    rtmp_url TEXT NOT NULL,
    start_time INTEGER NOT NULL,
    end_time INTEGER NOT NULL
)`).Error
	if err != nil {
		return err
	}
	err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_live_streams_display_name ON live_streams(display_name);`).Error
	if err != nil {
		return err
	}
	err = db.Exec(`CREATE TABLE IF NOT EXISTS oshis (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    oshi_id TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    type INTEGER NOT NULL DEFAULT 0,
    profile_image_url TEXT NOT NULL DEFAULT '',
    membership_image_url TEXT NOT NULL DEFAULT '',
    profile_sharelink_url TEXT NOT NULL DEFAULT '',
    is_following INTEGER NOT NULL DEFAULT 0,
    user_id TEXT NOT NULL DEFAULT '',
    unique_id TEXT NOT NULL DEFAULT '',
    display_name TEXT NOT NULL DEFAULT '',
    user_profile_image_url TEXT NOT NULL DEFAULT '',
    profile_background_image_url TEXT NOT NULL DEFAULT '',
    sm_profile_image_url TEXT NOT NULL DEFAULT '',
    biography TEXT NOT NULL DEFAULT '',
    oneliner_message TEXT NOT NULL DEFAULT '',
    fandom_name TEXT NOT NULL DEFAULT '',
    user_oshi_id TEXT NOT NULL DEFAULT '',
    is_verified INTEGER NOT NULL DEFAULT 0,
    is_followed INTEGER NOT NULL DEFAULT 0,
    is_user_following INTEGER NOT NULL DEFAULT 0,
    is_retired INTEGER NOT NULL DEFAULT 0,
    is_suspended INTEGER NOT NULL DEFAULT 0,
    is_blocking INTEGER NOT NULL DEFAULT 0,
    is_blocked INTEGER NOT NULL DEFAULT 0,
    is_subscribing INTEGER NOT NULL DEFAULT 0,
    has_membership INTEGER NOT NULL DEFAULT 0,
    can_stream INTEGER NOT NULL DEFAULT 0,
    has_oshi_calendar INTEGER NOT NULL DEFAULT 0,
    subscribe_start_time INTEGER NOT NULL DEFAULT 0,
    subscribe_day_count INTEGER NOT NULL DEFAULT 0,
    create_time INTEGER NOT NULL DEFAULT 0,
    update_time INTEGER NOT NULL DEFAULT 0,
    synced_at INTEGER NOT NULL DEFAULT 0
)`).Error
	if err != nil {
		return err
	}
	err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_oshis_oshi_id ON oshis(oshi_id);`).Error
	if err != nil {
		return err
	}
	err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_oshis_name ON oshis(name);`).Error
	if err != nil {
		return err
	}
	err = db.Exec(`CREATE TABLE IF NOT EXISTS user_private (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    unique_id TEXT NOT NULL DEFAULT '',
    display_name TEXT NOT NULL DEFAULT '',
    profile_image_url TEXT NOT NULL DEFAULT '',
    sm_profile_image_url TEXT NOT NULL DEFAULT '',
    profile_background_image_url TEXT NOT NULL DEFAULT '',
    biography TEXT NOT NULL DEFAULT '',
    oneliner_message TEXT NOT NULL DEFAULT '',
    fandom_name TEXT NOT NULL DEFAULT '',
    oshi_id TEXT NOT NULL DEFAULT '',
    install_link_url TEXT NOT NULL DEFAULT '',
    live_preparation_sharelink_url TEXT NOT NULL DEFAULT '',
    cardbox_sharelink_url TEXT NOT NULL DEFAULT '',
    profile_sharelink_url TEXT NOT NULL DEFAULT '',
    talent_page_sharelink_url TEXT NOT NULL DEFAULT '',
    follower_count INTEGER NOT NULL DEFAULT 0,
    following_count INTEGER NOT NULL DEFAULT 0,
    is_verified INTEGER NOT NULL DEFAULT 0,
    can_stream INTEGER NOT NULL DEFAULT 0,
    has_membership INTEGER NOT NULL DEFAULT 0,
    has_prime_chat INTEGER NOT NULL DEFAULT 0,
    has_oshi_calendar INTEGER NOT NULL DEFAULT 0,
    is_premium_subscriber INTEGER NOT NULL DEFAULT 0,
    is_sms_login INTEGER NOT NULL DEFAULT 0,
    is_unique_id_updatable INTEGER NOT NULL DEFAULT 0,
    is_unique_id_updatable_many_times INTEGER NOT NULL DEFAULT 0,
    is_display_name_updatable INTEGER NOT NULL DEFAULT 0,
    is_display_name_updatable_many_times INTEGER NOT NULL DEFAULT 0,
    can_purchase_items INTEGER NOT NULL DEFAULT 0,
    is_registered_payment_personal_information INTEGER NOT NULL DEFAULT 0,
    is_registered_bank_account INTEGER NOT NULL DEFAULT 0,
    unique_id_updatable_time INTEGER NOT NULL DEFAULT 0,
    display_name_updatable_time INTEGER NOT NULL DEFAULT 0,
    permanent_remove_time INTEGER NOT NULL DEFAULT 0,
    synced_at INTEGER NOT NULL DEFAULT 0
)`).Error
	if err != nil {
		return err
	}
	err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_user_private_user_id ON user_private(user_id);`).Error
	if err != nil {
		return err
	}
	err = db.Exec(`CREATE TABLE IF NOT EXISTS followings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_type INTEGER NOT NULL DEFAULT 0,
    target_key TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
    unique_id TEXT NOT NULL DEFAULT '',
    display_name TEXT NOT NULL DEFAULT '',
    profile_image_url TEXT NOT NULL DEFAULT '',
    sm_profile_image_url TEXT NOT NULL DEFAULT '',
    profile_background_image_url TEXT NOT NULL DEFAULT '',
    oshi_id TEXT NOT NULL DEFAULT '',
    oshi_name TEXT NOT NULL DEFAULT '',
    oshi_profile_image_url TEXT NOT NULL DEFAULT '',
    oshi_membership_image_url TEXT NOT NULL DEFAULT '',
    synced_at INTEGER NOT NULL DEFAULT 0
)`).Error
	if err != nil {
		return err
	}
	err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_followings_target_key ON followings(target_key);`).Error
	if err != nil {
		return err
	}
	err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_followings_display_name ON followings(display_name);`).Error
	if err != nil {
		return err
	}
	err = db.Exec(`CREATE TABLE IF NOT EXISTS chat_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    display_name TEXT NOT NULL,
    chat_room_id TEXT NOT NULL,
    chat_message_id TEXT NOT NULL,
    msg_type INTEGER NOT NULL,
    content TEXT DEFAULT '',
    image_url TEXT DEFAULT '',
    video_url TEXT DEFAULT '',
    video_path TEXT DEFAULT '',
    image_path TEXT DEFAULT '',
    send_time INTEGER NOT NULL,
    Time_str TEXT NOT NULL DEFAULT ''
)`).Error
	if err != nil {
		return err
	}
	err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_chat_messages_user_id ON chat_messages(user_id);`).Error
	if err != nil {
		return err
	}
	err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_chat_messages_chat_message_id ON chat_messages(chat_message_id);`).Error
	if err != nil {
		return err
	}
	err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_chat_messages_room_send_time ON chat_messages(user_id, chat_room_id, send_time);`).Error
	if err != nil {
		return err
	}
	err = db.Exec(`CREATE TABLE IF NOT EXISTS prime_chat_rooms (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chat_room_id TEXT NOT NULL,
    talent_user_id TEXT NOT NULL,
    talent_unique_id TEXT NOT NULL DEFAULT '',
    talent_display_name TEXT NOT NULL DEFAULT '',
    talent_avatar_url TEXT NOT NULL DEFAULT '',
    member_user_id TEXT NOT NULL DEFAULT '',
    member_background_image_url TEXT NOT NULL DEFAULT '',
    synced_at INTEGER NOT NULL DEFAULT 0
)`).Error
	if err != nil {
		return err
	}
	err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_prime_chat_rooms_room_id ON prime_chat_rooms(chat_room_id);`).Error
	if err != nil {
		return err
	}
	err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_prime_chat_rooms_talent_member ON prime_chat_rooms(talent_user_id, member_user_id);`).Error
	if err != nil {
		return err
	}
	err = db.Exec(`CREATE TABLE IF NOT EXISTS prime_chat_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id TEXT NOT NULL,
    chat_room_id TEXT NOT NULL,
    chat_room_owner_user_id TEXT NOT NULL DEFAULT '',
    member_user_id TEXT NOT NULL DEFAULT '',
    sender TEXT NOT NULL DEFAULT '',
    body_type TEXT NOT NULL DEFAULT '',
    text_content TEXT NOT NULL DEFAULT '',
    image_url TEXT NOT NULL DEFAULT '',
    image_thumbnail_url TEXT NOT NULL DEFAULT '',
    video_url TEXT NOT NULL DEFAULT '',
    video_thumbnail_url TEXT NOT NULL DEFAULT '',
    video_duration_seconds INTEGER NOT NULL DEFAULT 0,
    coin_amount INTEGER NOT NULL DEFAULT 0,
    reaction_emoji TEXT NOT NULL DEFAULT '',
    is_deleted INTEGER NOT NULL DEFAULT 0,
    is_read INTEGER NOT NULL DEFAULT 0,
    create_unix_time_ms INTEGER NOT NULL DEFAULT 0
)`).Error
	if err != nil {
		return err
	}
	err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_prime_chat_messages_message_id ON prime_chat_messages(message_id);`).Error
	if err != nil {
		return err
	}
	err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_prime_chat_messages_room_time ON prime_chat_messages(chat_room_id, create_unix_time_ms, id);`).Error
	if err != nil {
		return err
	}
	return nil
}

type ChatMessage struct {
	Id            int64  `json:"id"`
	UserId        string `json:"user_id"`
	DisplayName   string `json:"display_name"`
	ChatRoomId    string `json:"chat_room_id"`
	ChatMessageId string `json:"chat_message_id"`
	MsgType       int32  `json:"msg_type"`
	Content       string `json:"content"`
	ImageUrl      string `json:"image_url"`
	VideoUrl      string `json:"video_url"`
	VideoPath     string `json:"video_path"`
	ImagePath     string `json:"image_path"`
	TimeStr       string `json:"time_str"`
	SendTime      int64  `json:"send_time"`
}

func (c ChatMessage) TableName() string {
	return "chat_messages"
}

// PrimeChatRoom is deliberately separate from ChatRoom. Prime Chat is a
// different Replive product and must never be mixed into the Fandom tables.
type PrimeChatRoom struct {
	Id                       int64  `json:"id"`
	ChatRoomId               string `json:"chat_room_id"`
	TalentUserId             string `json:"talent_user_id"`
	TalentUniqueId           string `json:"talent_unique_id"`
	TalentDisplayName        string `json:"talent_display_name"`
	TalentAvatarUrl          string `json:"talent_avatar_url"`
	MemberUserId             string `json:"member_user_id"`
	MemberBackgroundImageUrl string `json:"member_background_image_url"`
	SyncedAt                 int64  `json:"synced_at"`
	LastMessageTime          int64  `json:"last_message_time" gorm:"column:last_message_time;->;-:migration"`
	LastMessageContent       string `json:"last_message_content" gorm:"column:last_message_content;->;-:migration"`
	LastMessageType          string `json:"last_message_type" gorm:"column:last_message_type;->;-:migration"`
}

func (PrimeChatRoom) TableName() string {
	return "prime_chat_rooms"
}

type PrimeChatMessage struct {
	Id                   int64  `json:"id"`
	MessageId            string `json:"message_id"`
	ChatRoomId           string `json:"chat_room_id"`
	ChatRoomOwnerUserId  string `json:"chat_room_owner_user_id"`
	MemberUserId         string `json:"member_user_id"`
	Sender               string `json:"sender"`
	BodyType             string `json:"body_type"`
	TextContent          string `json:"text_content"`
	ImageUrl             string `json:"image_url"`
	ImageThumbnailUrl    string `json:"image_thumbnail_url"`
	VideoUrl             string `json:"video_url"`
	VideoThumbnailUrl    string `json:"video_thumbnail_url"`
	VideoDurationSeconds int64  `json:"video_duration_seconds"`
	CoinAmount           int64  `json:"coin_amount"`
	ReactionEmoji        string `json:"reaction_emoji"`
	IsDeleted            bool   `json:"is_deleted"`
	IsRead               bool   `json:"is_read"`
	CreateUnixTimeMillis int64  `gorm:"column:create_unix_time_ms" json:"create_unix_time_ms"`
	// ReactionOnly is metadata for the current startup sync only. It is not a
	// database column and tells DAL to retain an existing full message payload.
	ReactionOnly bool `gorm:"-" json:"-"`
}

func (PrimeChatMessage) TableName() string {
	return "prime_chat_messages"
}

type LiveStream struct {
	Id          int64  `json:"id"`
	UserId      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	Title       string `json:"title"`
	WebrtcUrl   string `json:"webrtc_url"`
	RtmpUrl     string `json:"rtmp_url"`
	StartTime   int64  `json:"start_time"`
	EndTime     int64  `json:"end_time"`
}

func (c LiveStream) TableName() string {
	return "live_streams"
}
