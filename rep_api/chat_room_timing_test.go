package rep_api

import (
	"testing"

	"replive/model"

	"google.golang.org/protobuf/encoding/protowire"
)

func TestParseChatRoomTimingWithDayCount(t *testing.T) {
	room := &model.ChatRoom{
		UserId:     "talent_123",
		ChatRoomId: "room_456",
	}

	// Construct unknown fields with tag 6 (day_count = 119) and tag 101 (talent_last_check_time)
	var raw []byte
	raw = protowire.AppendTag(raw, chatRoomDayCountField, protowire.VarintType)
	raw = protowire.AppendVarint(raw, 119)

	// Instant with seconds = 1724650000
	var instantRaw []byte
	instantRaw = protowire.AppendTag(instantRaw, 1, protowire.VarintType)
	instantRaw = protowire.AppendVarint(instantRaw, 1724650000)

	raw = protowire.AppendTag(raw, chatRoomTalentLastCheckTimeField, protowire.BytesType)
	raw = protowire.AppendBytes(raw, instantRaw)

	room.ProtoReflect().SetUnknown(raw)

	timing, err := ParseChatRoomTiming(room)
	if err != nil {
		t.Fatalf("ParseChatRoomTiming failed: %v", err)
	}

	if !timing.HasDayCount || timing.DayCount != 119 {
		t.Errorf("expected DayCount=119, got has=%v, val=%d", timing.HasDayCount, timing.DayCount)
	}

	if !timing.HasTalentLastCheckTime || timing.TalentLastCheckTime.Unix() != 1724650000 {
		t.Errorf("expected TalentLastCheckTime=1724650000, got has=%v, val=%v", timing.HasTalentLastCheckTime, timing.TalentLastCheckTime)
	}
}
