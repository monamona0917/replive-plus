package rep_api

import (
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
)

func TestParsePrimeChatRoomCreatesLocalParticipantKey(t *testing.T) {
	profile := make([]byte, 0)
	profile = appendPrimeBytesField(profile, 1, "talent-user")
	profile = appendPrimeBytesField(profile, 2, "talent_unique")
	profile = appendPrimeBytesField(profile, 3, "Talent Display")
	profile = appendPrimeBytesField(profile, 4, "https://example.invalid/avatar.jpg")

	payload := make([]byte, 0)
	payload = appendPrimeBytesField(payload, 1, "talent-user")
	payload = appendPrimeBytesField(payload, 2, "member-user")
	payload = appendPrimeBytesField(payload, 3, string(profile))
	payload = appendPrimeBytesField(payload, 8, "https://example.invalid/background.jpg")

	room := &PrimeChatRoom{}
	parsePrimeChatRoom(payload, room)

	if room.TalentUserId != "talent-user" {
		t.Fatalf("TalentUserId = %q, want talent-user", room.TalentUserId)
	}
	if room.MemberUserId != "member-user" {
		t.Fatalf("MemberUserId = %q, want member-user", room.MemberUserId)
	}
	if room.ChatRoomId != "prime:talent-user:member-user" {
		t.Fatalf("ChatRoomId = %q, want stable local Prime room key", room.ChatRoomId)
	}
	if room.MemberBackgroundImageUrl != "https://example.invalid/background.jpg" {
		t.Fatalf("MemberBackgroundImageUrl = %q", room.MemberBackgroundImageUrl)
	}
}

func TestParsePrimeChatMessageReadsReactionEmoji(t *testing.T) {
	timestamp := make([]byte, 0)
	timestamp = appendPrimeVarintField(timestamp, 1, 1_700_000_000)
	timestamp = appendPrimeVarintField(timestamp, 2, 500_000_000)

	payload := make([]byte, 0)
	payload = appendPrimeBytesField(payload, 1, "talent-user")
	payload = appendPrimeBytesField(payload, 2, "member-user")
	payload = appendPrimeBytesField(payload, 3, "message-1")
	payload = appendPrimeVarintField(payload, 4, 1)
	payload = appendPrimeVarintField(payload, 6, 1)
	payload = appendPrimeBytesField(payload, 7, "welcome")
	payload = appendPrimeBytesField(payload, 13, "\U0001F44D")
	payload = appendPrimeBytesField(payload, 100, string(timestamp))

	message := parsePrimeChatMessage(payload)

	if message.Sender != PrimeChatSenderTalent {
		t.Fatalf("Sender = %q, want %q", message.Sender, PrimeChatSenderTalent)
	}
	if message.BodyType != PrimeChatBodyText {
		t.Fatalf("BodyType = %q, want %q", message.BodyType, PrimeChatBodyText)
	}
	if message.ReactionEmoji != "\U0001F44D" {
		t.Fatalf("ReactionEmoji = %q, want thumbs-up", message.ReactionEmoji)
	}
	wantTime := time.Unix(1_700_000_000, 500_000_000).UnixMilli()
	if message.CreateUnixTimeMillis != wantTime {
		t.Fatalf("CreateUnixTimeMillis = %d, want %d", message.CreateUnixTimeMillis, wantTime)
	}
}

func appendPrimeBytesField(buffer []byte, field protowire.Number, value string) []byte {
	buffer = protowire.AppendTag(buffer, field, protowire.BytesType)
	return protowire.AppendString(buffer, value)
}

func appendPrimeVarintField(buffer []byte, field protowire.Number, value uint64) []byte {
	buffer = protowire.AppendTag(buffer, field, protowire.VarintType)
	return protowire.AppendVarint(buffer, value)
}
