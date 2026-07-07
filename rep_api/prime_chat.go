package rep_api

import (
	"fmt"
	"strings"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"google.golang.org/protobuf/encoding/protowire"
)

// PrimeChatRoom Prime Chat 房间信息
type PrimeChatRoom struct {
	ChatRoomId               string
	TalentUserId             string
	TalentUniqueId           string
	TalentDisplayName        string
	TalentAvatarUrl          string
	MemberUserId             string
	MemberBackgroundImageUrl string
	MemberProfileImageUrl    string
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

	DumpRawResponse("ListPrimeChatRooms", respBuf)

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

func parsePrimeChatRoom(b []byte, room *PrimeChatRoom) {
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			break
		}
		b = b[n:]

		switch {
		case num == 1 && typ == protowire.BytesType:
			// chat_room_id
			v, n2 := protowire.ConsumeBytes(b)
			if n2 < 0 {
				return
			}
			room.ChatRoomId = string(v)
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

		default:
			n2 := protowire.ConsumeFieldValue(num, typ, b)
			if n2 < 0 {
				return
			}
			b = b[n2:]
		}
	}
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

// DumpRawResponse 输出响应中所有 protobuf 字段，用于诊断
func DumpRawResponse(tag string, respBuf []byte) {
	if len(respBuf) == 0 {
		hlog.Infof("%s: empty response", tag)
		return
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("=== %s raw response (%d bytes) ===", tag, len(respBuf)))
	b := respBuf
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			break
		}
		b = b[n:]
		if typ == protowire.BytesType {
			v, n2 := protowire.ConsumeBytes(b)
			if n2 < 0 {
				break
			}
			b = b[n2:]
			if num == 1 && len(v) > 0 && v[0] != 0 {
				subLines := dumpSubFields(v, "  ")
				lines = append(lines, fmt.Sprintf("  field %d (BytesType): nested {", num))
				lines = append(lines, subLines...)
				lines = append(lines, "  }")
			} else {
				lines = append(lines, fmt.Sprintf("  field %d (BytesType): len=%d value=%q", num, len(v), truncateString(string(v), 100)))
			}
		} else if typ == protowire.VarintType {
			v, n2 := protowire.ConsumeVarint(b)
			if n2 < 0 {
				break
			}
			b = b[n2:]
			lines = append(lines, fmt.Sprintf("  field %d (VarintType): %d", num, v))
		} else if typ == protowire.Fixed32Type {
			v, n2 := protowire.ConsumeFixed32(b)
			if n2 < 0 {
				break
			}
			b = b[n2:]
			lines = append(lines, fmt.Sprintf("  field %d (Fixed32Type): %d", num, v))
		} else if typ == protowire.Fixed64Type {
			v, n2 := protowire.ConsumeFixed64(b)
			if n2 < 0 {
				break
			}
			b = b[n2:]
			lines = append(lines, fmt.Sprintf("  field %d (Fixed64Type): %d", num, v))
		} else {
			n2 := protowire.ConsumeFieldValue(num, typ, b)
			if n2 < 0 {
				break
			}
			b = b[n2:]
			lines = append(lines, fmt.Sprintf("  field %d (typ=%d): (skipped)", num, typ))
		}
	}
	lines = append(lines, "=== end ===")
	hlog.Infof(strings.Join(lines, "\n"))
}

func dumpSubFields(b []byte, indent string) []string {
	var lines []string
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			break
		}
		b = b[n:]
		if typ == protowire.BytesType {
			v, n2 := protowire.ConsumeBytes(b)
			if n2 < 0 {
				break
			}
			b = b[n2:]
			lines = append(lines, fmt.Sprintf("%sfield %d (BytesType): len=%d value=%q", indent, num, len(v), truncateString(string(v), 100)))
		} else if typ == protowire.VarintType {
			v, n2 := protowire.ConsumeVarint(b)
			if n2 < 0 {
				break
			}
			b = b[n2:]
			lines = append(lines, fmt.Sprintf("%sfield %d (VarintType): %d", indent, num, v))
		} else if typ == protowire.Fixed32Type {
			v, n2 := protowire.ConsumeFixed32(b)
			if n2 < 0 {
				break
			}
			b = b[n2:]
			lines = append(lines, fmt.Sprintf("%sfield %d (Fixed32Type): %d", indent, num, v))
		} else {
			n2 := protowire.ConsumeFieldValue(num, typ, b)
			if n2 < 0 {
				break
			}
			b = b[n2:]
			lines = append(lines, fmt.Sprintf("%sfield %d (typ=%d): (skipped)", indent, num, typ))
		}
	}
	if len(lines) == 0 {
		lines = append(lines, fmt.Sprintf("%s(empty or primitive)", indent))
	}
	return lines
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
