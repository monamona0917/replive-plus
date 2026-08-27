package rep_api

import (
	"fmt"
	"time"

	"replive/model"

	"google.golang.org/protobuf/encoding/protowire"
)

const (
	chatRoomDayCountField            = protowire.Number(6)
	chatRoomTalentLastCheckTimeField = protowire.Number(101)
)

// ChatRoomTiming is the Fandom room timing and subscription state returned by ListChatRooms.
// The generated ChatRoom model predates these fields, so protobuf retains them
// as unknown fields. The Android 4.7.7 client confirms tag 6 (day_count) and tag 101.
type ChatRoomTiming struct {
	DayCount               int64
	HasDayCount            bool
	TalentLastCheckTime    time.Time
	HasTalentLastCheckTime bool
}

// ParseChatRoomTiming reads the service-provided talent room timestamp from a
// ListChatRooms response without changing the generated protobuf descriptor.
func ParseChatRoomTiming(room *model.ChatRoom) (ChatRoomTiming, error) {
	if room == nil {
		return ChatRoomTiming{}, fmt.Errorf("chat room is nil")
	}

	var timing ChatRoomTiming
	unknown := room.ProtoReflect().GetUnknown()
	for len(unknown) > 0 {
		field, wireType, n := protowire.ConsumeTag(unknown)
		if n < 0 {
			return timing, fmt.Errorf("decode chat room timing tag: %v", protowire.ParseError(n))
		}
		unknown = unknown[n:]

		switch {
		case field == chatRoomDayCountField && wireType == protowire.VarintType:
			value, n := protowire.ConsumeVarint(unknown)
			if n < 0 {
				return timing, fmt.Errorf("decode day_count: %v", protowire.ParseError(n))
			}
			timing.DayCount = int64(value)
			timing.HasDayCount = true
			unknown = unknown[n:]

		case field == chatRoomTalentLastCheckTimeField && wireType == protowire.BytesType:
			value, n := protowire.ConsumeBytes(unknown)
			if n < 0 {
				return timing, fmt.Errorf("decode talent_last_check_time: %v", protowire.ParseError(n))
			}
			parsed, present, err := parseChatRoomProtoInstant(value)
			if err != nil {
				return timing, fmt.Errorf("decode talent_last_check_time: %w", err)
			}
			timing.TalentLastCheckTime = parsed
			timing.HasTalentLastCheckTime = present
			unknown = unknown[n:]

		default:
			n := protowire.ConsumeFieldValue(field, wireType, unknown)
			if n < 0 {
				return timing, fmt.Errorf("skip chat room field %d: %v", field, protowire.ParseError(n))
			}
			unknown = unknown[n:]
		}
	}
	return timing, nil
}

func parseChatRoomProtoInstant(raw []byte) (time.Time, bool, error) {
	var seconds int64
	var nanos int64
	present := false

	for len(raw) > 0 {
		field, wireType, n := protowire.ConsumeTag(raw)
		if n < 0 {
			return time.Time{}, false, protowire.ParseError(n)
		}
		raw = raw[n:]

		switch {
		case field == 1 && wireType == protowire.VarintType:
			value, n := protowire.ConsumeVarint(raw)
			if n < 0 {
				return time.Time{}, false, protowire.ParseError(n)
			}
			seconds = int64(value)
			present = true
			raw = raw[n:]
		case field == 2 && wireType == protowire.VarintType:
			value, n := protowire.ConsumeVarint(raw)
			if n < 0 {
				return time.Time{}, false, protowire.ParseError(n)
			}
			nanos = int64(value)
			present = true
			raw = raw[n:]
		default:
			n := protowire.ConsumeFieldValue(field, wireType, raw)
			if n < 0 {
				return time.Time{}, false, protowire.ParseError(n)
			}
			raw = raw[n:]
		}
	}
	if !present {
		return time.Time{}, false, nil
	}
	return time.Unix(seconds, nanos), true, nil
}
