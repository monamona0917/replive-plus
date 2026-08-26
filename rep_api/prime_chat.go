package rep_api

import (
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"google.golang.org/protobuf/encoding/protowire"
)

// PrimeChatRoom Prime Chat 房间信息
type PrimeChatRoom struct {
	ChatRoomId                string
	TalentUserId              string
	TalentUniqueId            string
	TalentDisplayName         string
	TalentAvatarUrl           string
	MemberUserId              string
	MemberBackgroundImageUrl  string
	MemberProfileImageUrl     string
	TalentLastCheckTimeMillis int64
	MemberLastCheckTimeMillis int64
}

const (
	PrimeChatSenderUnknown = "unknown"
	PrimeChatSenderTalent  = "talent"
	PrimeChatSenderMember  = "member"

	PrimeChatBodyUnknown = "unknown"
	PrimeChatBodyText    = "text"
	PrimeChatBodyImage   = "image"
	PrimeChatBodyVideo   = "video"
)

// PrimeChatMessage is the read-only subset returned by
// ListPrimeChatMessagesOfJoinedChatRooms. The API does not include a chat room
// ID in each message; UserId and MemberUserId identify the saved Prime room.
type PrimeChatMessage struct {
	UserId               string
	MemberUserId         string
	MessageId            string
	Sender               string
	BodyType             string
	Content              string
	ImageUrl             string
	VideoUrl             string
	VideoThumbnailUrl    string
	IsDeleted            bool
	CoinAmount           int64
	ReactionEmoji        string
	CreateUnixTimeMillis int64
}

type PrimeChatMessagesPage struct {
	Messages                []*PrimeChatMessage
	AllReactedMessages      []*PrimeChatMessage
	HasNextPage             bool
	NextPageCursorMessageId string
}

func ListPrimeChatRooms() ([]*PrimeChatRoom, error) {
	// ListPrimeChatRoomsRequest { max_page_size = 1: 100 }
	reqBuf := protowire.AppendVarint(
		protowire.AppendTag(nil, 1, protowire.VarintType),
		100,
	)
	respBuf, err := GetRepliveRaw("user.v1.ChatService/ListPrimeChatRooms", reqBuf)
	if err != nil {
		return nil, fmt.Errorf("ListPrimeChatRooms failed: %v", err)
	}

	if len(respBuf) == 0 {
		return nil, nil
	}

	// 解析响应：ListPrimeChatRoomsResponse { prime_chat_rooms = 1: repeated PrimeChatRoom { ... } }
	var rooms []*PrimeChatRoom
	b := respBuf
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			break
		}
		b = b[n:]
		if num == 1 && typ == protowire.BytesType {
			roomBytes, n2 := protowire.ConsumeBytes(b)
			if n2 < 0 {
				break
			}
			b = b[n2:]
			room := &PrimeChatRoom{}
			parsePrimeChatRoom(roomBytes, room)
			rooms = append(rooms, room)
		} else {
			n2 := protowire.ConsumeFieldValue(num, typ, b)
			if n2 < 0 {
				break
			}
			b = b[n2:]
		}
	}
	return rooms, nil
}

// GetPrimeChatRoom 获取指定Prime Chat 房间信息
func GetPrimeChatRoom(talentUserId string) (*PrimeChatRoom, error) {
	// 手动编码 GetPrimeChatRoomRequest { talent_user_id = 1: talentUserId }
	reqBuf := protowire.AppendString(
		protowire.AppendTag(nil, 1, protowire.BytesType),
		talentUserId,
	)

	respBuf, err := GetRepliveRaw("user.v1.ChatService/GetPrimeChatRoom", reqBuf)
	if err != nil {
		return nil, fmt.Errorf("GetPrimeChatRoom failed: %v", err)
	}

	DumpRawResponse("GetPrimeChatRoom", respBuf)

	// 手动解码响应
	room := &PrimeChatRoom{}
	b := respBuf
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			break
		}
		b = b[n:]
		if num == 1 && typ == protowire.BytesType {
			roomBytes, n2 := protowire.ConsumeBytes(b)
			if n2 < 0 {
				break
			}
			b = b[n2:]
			parsePrimeChatRoom(roomBytes, room)
		} else {
			n2 := protowire.ConsumeFieldValue(num, typ, b)
			if n2 < 0 {
				break
			}
			b = b[n2:]
		}
	}
	return room, nil
}

// ListPrimeChatMessagesOfJoinedChatRooms reads Prime messages for all joined
// rooms. It is intentionally a GET request and never changes chat or reaction
// state. The wire fields are verified against Replive 4.7.7's generated
// request adapter:
//
//	1 max_page_size
//	2 cursor_prime_chat_message_id
//	3 force_strong_read
//	4 order_by_create_time_desc
//	5 include_all_reacted_prime_chat_messages
func ListPrimeChatMessagesOfJoinedChatRooms(cursorMessageID string) (*PrimeChatMessagesPage, error) {
	reqBuf := make([]byte, 0, 64)
	reqBuf = protowire.AppendTag(reqBuf, 1, protowire.VarintType)
	reqBuf = protowire.AppendVarint(reqBuf, 100)
	if cursorMessageID != "" {
		reqBuf = protowire.AppendTag(reqBuf, 2, protowire.BytesType)
		reqBuf = protowire.AppendString(reqBuf, cursorMessageID)
	}
	// Keep force_strong_read at its zero value. This client only reads data and
	// must not opt into a stronger server-side read behavior unnecessarily.
	reqBuf = protowire.AppendTag(reqBuf, 4, protowire.VarintType)
	reqBuf = protowire.AppendVarint(reqBuf, 1)
	reqBuf = protowire.AppendTag(reqBuf, 5, protowire.VarintType)
	reqBuf = protowire.AppendVarint(reqBuf, 1)

	respBuf, err := GetRepliveRaw("user.v1.ChatService/ListPrimeChatMessagesOfJoinedChatRooms", reqBuf)
	if err != nil {
		return nil, fmt.Errorf("ListPrimeChatMessagesOfJoinedChatRooms failed: %w", err)
	}

	page := &PrimeChatMessagesPage{
		Messages:           make([]*PrimeChatMessage, 0),
		AllReactedMessages: make([]*PrimeChatMessage, 0),
	}
	b := respBuf
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return nil, fmt.Errorf("decode prime chat messages response tag: %v", protowire.ParseError(n))
		}
		b = b[n:]

		switch {
		case num == 1 && typ == protowire.BytesType:
			messageBuf, n2 := protowire.ConsumeBytes(b)
			if n2 < 0 {
				return nil, fmt.Errorf("decode prime chat message: %v", protowire.ParseError(n2))
			}
			b = b[n2:]
			page.Messages = append(page.Messages, parsePrimeChatMessage(messageBuf))
		case num == 2 && typ == protowire.VarintType:
			value, n2 := protowire.ConsumeVarint(b)
			if n2 < 0 {
				return nil, fmt.Errorf("decode prime chat has_next_page: %v", protowire.ParseError(n2))
			}
			b = b[n2:]
			page.HasNextPage = value != 0
		case num == 3 && typ == protowire.BytesType:
			value, n2 := protowire.ConsumeBytes(b)
			if n2 < 0 {
				return nil, fmt.Errorf("decode prime chat next cursor: %v", protowire.ParseError(n2))
			}
			b = b[n2:]
			page.NextPageCursorMessageId = string(value)
		case num == 4 && typ == protowire.BytesType:
			messageBuf, n2 := protowire.ConsumeBytes(b)
			if n2 < 0 {
				return nil, fmt.Errorf("decode reacted prime chat message: %v", protowire.ParseError(n2))
			}
			b = b[n2:]
			page.AllReactedMessages = append(page.AllReactedMessages, parsePrimeChatMessage(messageBuf))
		default:
			n2 := protowire.ConsumeFieldValue(num, typ, b)
			if n2 < 0 {
				return nil, fmt.Errorf("skip prime chat response field %d: %v", num, protowire.ParseError(n2))
			}
			b = b[n2:]
		}
	}

	return page, nil
}

// PrimeChatMessage wire fields from Replive 4.7.7's generated adapter:
//
//	1 user_id, 2 member_user_id, 3 prime_chat_message_id,
//	4 send_user_type, 6 type, 7 content, 8 image_url, 9 video_url,
//
// 10 video_thumbnail_jpeg_url, 11 is_deleted, 12 coin_amount,
// 13 reaction_emoji, 100 create_time.
func parsePrimeChatMessage(b []byte) *PrimeChatMessage {
	message := &PrimeChatMessage{
		Sender:   PrimeChatSenderUnknown,
		BodyType: PrimeChatBodyUnknown,
	}
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return message
		}
		b = b[n:]

		switch {
		case num == 1 && typ == protowire.BytesType:
			value, n2 := protowire.ConsumeBytes(b)
			if n2 < 0 {
				return message
			}
			message.UserId = string(value)
			b = b[n2:]
		case num == 2 && typ == protowire.BytesType:
			value, n2 := protowire.ConsumeBytes(b)
			if n2 < 0 {
				return message
			}
			message.MemberUserId = string(value)
			b = b[n2:]
		case num == 3 && typ == protowire.BytesType:
			value, n2 := protowire.ConsumeBytes(b)
			if n2 < 0 {
				return message
			}
			message.MessageId = string(value)
			b = b[n2:]
		case num == 4 && typ == protowire.VarintType:
			value, n2 := protowire.ConsumeVarint(b)
			if n2 < 0 {
				return message
			}
			message.Sender = primeChatSenderFromValue(value)
			b = b[n2:]
		case num == 6 && typ == protowire.VarintType:
			value, n2 := protowire.ConsumeVarint(b)
			if n2 < 0 {
				return message
			}
			message.BodyType = primeChatBodyTypeFromValue(value)
			b = b[n2:]
		case num == 7 && typ == protowire.BytesType:
			value, n2 := protowire.ConsumeBytes(b)
			if n2 < 0 {
				return message
			}
			message.Content = string(value)
			b = b[n2:]
		case num == 8 && typ == protowire.BytesType:
			value, n2 := protowire.ConsumeBytes(b)
			if n2 < 0 {
				return message
			}
			message.ImageUrl = string(value)
			b = b[n2:]
		case num == 9 && typ == protowire.BytesType:
			value, n2 := protowire.ConsumeBytes(b)
			if n2 < 0 {
				return message
			}
			message.VideoUrl = string(value)
			b = b[n2:]
		case num == 10 && typ == protowire.BytesType:
			value, n2 := protowire.ConsumeBytes(b)
			if n2 < 0 {
				return message
			}
			message.VideoThumbnailUrl = string(value)
			b = b[n2:]
		case num == 11 && typ == protowire.VarintType:
			value, n2 := protowire.ConsumeVarint(b)
			if n2 < 0 {
				return message
			}
			message.IsDeleted = value != 0
			b = b[n2:]
		case num == 12 && typ == protowire.VarintType:
			value, n2 := protowire.ConsumeVarint(b)
			if n2 < 0 {
				return message
			}
			message.CoinAmount = int64(value)
			b = b[n2:]
		case num == 13 && typ == protowire.BytesType:
			value, n2 := protowire.ConsumeBytes(b)
			if n2 < 0 {
				return message
			}
			message.ReactionEmoji = string(value)
			b = b[n2:]
		case num == 100:
			value, n2 := parsePrimeChatCreateTime(typ, b)
			if n2 < 0 {
				return message
			}
			message.CreateUnixTimeMillis = value
			b = b[n2:]
		default:
			n2 := protowire.ConsumeFieldValue(num, typ, b)
			if n2 < 0 {
				return message
			}
			b = b[n2:]
		}
	}
	return message
}

func primeChatSenderFromValue(value uint64) string {
	switch value {
	case 1:
		return PrimeChatSenderTalent
	case 2:
		return PrimeChatSenderMember
	default:
		return PrimeChatSenderUnknown
	}
}

func primeChatBodyTypeFromValue(value uint64) string {
	switch value {
	case 1:
		return PrimeChatBodyText
	case 2:
		return PrimeChatBodyImage
	case 3:
		return PrimeChatBodyVideo
	default:
		return PrimeChatBodyUnknown
	}
}

func parsePrimeChatCreateTime(typ protowire.Type, b []byte) (int64, int) {
	switch typ {
	case protowire.VarintType:
		value, n := protowire.ConsumeVarint(b)
		if n < 0 {
			return 0, n
		}
		return normalizePrimeUnixTime(int64(value)), n
	case protowire.Fixed64Type:
		value, n := protowire.ConsumeFixed64(b)
		if n < 0 {
			return 0, n
		}
		return normalizePrimeUnixTime(int64(value)), n
	case protowire.BytesType:
		value, n := protowire.ConsumeBytes(b)
		if n < 0 {
			return 0, n
		}
		return parsePrimeProtoTimestamp(value), n
	default:
		return 0, -1
	}
}

func parsePrimeProtoTimestamp(b []byte) int64 {
	var seconds int64
	var nanos int64
	hasValue := false
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return 0
		}
		b = b[n:]
		switch {
		case num == 1 && typ == protowire.VarintType:
			value, n2 := protowire.ConsumeVarint(b)
			if n2 < 0 {
				return 0
			}
			seconds = int64(value)
			hasValue = true
			b = b[n2:]
		case num == 2 && typ == protowire.VarintType:
			value, n2 := protowire.ConsumeVarint(b)
			if n2 < 0 {
				return 0
			}
			nanos = int64(value)
			hasValue = true
			b = b[n2:]
		default:
			n2 := protowire.ConsumeFieldValue(num, typ, b)
			if n2 < 0 {
				return 0
			}
			b = b[n2:]
		}
	}
	if !hasValue {
		return 0
	}
	return time.Unix(seconds, nanos).UnixMilli()
}

func normalizePrimeUnixTime(value int64) int64 {
	if value == 0 {
		return 0
	}
	if value > 10_000_000_000 || value < -10_000_000_000 {
		return value
	}
	return value * 1000
}

func parsePrimeChatRoom(b []byte, room *PrimeChatRoom) {
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			break
		}
		b = b[n:]

		switch {
		case num == 1 && typ == protowire.BytesType:
			// user_id: Prime Chat has no remote chat_room_id. This is the
			// talent / room-owner ID and forms part of our local room key.
			v, n2 := protowire.ConsumeBytes(b)
			if n2 < 0 {
				return
			}
			room.TalentUserId = string(v)
			b = b[n2:]

		case num == 2 && typ == protowire.BytesType:
			// member_user_id（当前登录用户）
			v, n2 := protowire.ConsumeBytes(b)
			if n2 < 0 {
				return
			}
			room.MemberUserId = string(v)
			b = b[n2:]

		case num == 3 && typ == protowire.BytesType:
			// talent 的用户信息（内嵌 UserProfile）
			v, n2 := protowire.ConsumeBytes(b)
			if n2 < 0 {
				return
			}
			b = b[n2:]
			parseUserProfile(v, room)

		case num == 4 && typ == protowire.BytesType:
			// member 的用户信息（当前用户，不需要解析）
			_, n2 := protowire.ConsumeBytes(b)
			if n2 < 0 {
				return
			}
			b = b[n2:]

		case num == 8 && typ == protowire.BytesType:
			// member_background_image_url（背景图！）
			v, n2 := protowire.ConsumeBytes(b)
			if n2 < 0 {
				return
			}
			room.MemberBackgroundImageUrl = string(v)
			b = b[n2:]

		case num == 100:
			value, n2 := parsePrimeChatCreateTime(typ, b)
			if n2 < 0 {
				return
			}
			room.TalentLastCheckTimeMillis = value
			b = b[n2:]

		case num == 101:
			value, n2 := parsePrimeChatCreateTime(typ, b)
			if n2 < 0 {
				return
			}
			room.MemberLastCheckTimeMillis = value
			b = b[n2:]
		default:
			n2 := protowire.ConsumeFieldValue(num, typ, b)
			if n2 < 0 {
				return
			}
			b = b[n2:]
		}
	}
	room.ChatRoomId = primeChatRoomID(room.TalentUserId, room.MemberUserId)
}

// primeChatRoomID creates a stable local identifier for a Prime Chat room.
// The platform returns the two participants but does not expose a chat_room_id.
func primeChatRoomID(talentUserID, memberUserID string) string {
	talentUserID = strings.TrimSpace(talentUserID)
	memberUserID = strings.TrimSpace(memberUserID)
	if talentUserID == "" {
		return ""
	}
	return "prime:" + talentUserID + ":" + memberUserID
}

// parseUserProfile 解析内嵌的 UserProfile（field 3 中的子消息）
// UserProfile { user_id=1, unique_id=2, display_name=3, avatar_url=4 }
func parseUserProfile(b []byte, room *PrimeChatRoom) {
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			break
		}
		b = b[n:]

		switch {
		case num == 1 && typ == protowire.BytesType:
			v, n2 := protowire.ConsumeBytes(b)
			if n2 < 0 {
				return
			}
			room.TalentUserId = string(v)
			b = b[n2:]
		case num == 2 && typ == protowire.BytesType:
			v, n2 := protowire.ConsumeBytes(b)
			if n2 < 0 {
				return
			}
			room.TalentUniqueId = string(v)
			b = b[n2:]
		case num == 3 && typ == protowire.BytesType:
			v, n2 := protowire.ConsumeBytes(b)
			if n2 < 0 {
				return
			}
			room.TalentDisplayName = string(v)
			b = b[n2:]
		case num == 4 && typ == protowire.BytesType:
			v, n2 := protowire.ConsumeBytes(b)
			if n2 < 0 {
				return
			}
			room.TalentAvatarUrl = string(v)
			b = b[n2:]
		default:
			n2 := protowire.ConsumeFieldValue(num, typ, b)
			if n2 < 0 {
				return
			}
			b = b[n2:]
		}
	}
}

// DumpRawResponse 输出响应概要，避免正常运行时打印完整 protobuf 内容。
func DumpRawResponse(tag string, respBuf []byte) {
	if len(respBuf) == 0 {
		hlog.Infof("%s: empty response", tag)
		return
	}
	hlog.Infof("%s response received: %d bytes", tag, len(respBuf))
}
