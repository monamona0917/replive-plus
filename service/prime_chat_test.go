package service

import (
	"errors"
	"fmt"
	"replive/dal"
	"replive/rep_api"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestQueryPrimeChatRoomsLimitsConcurrencyAndContinuesAfterErrors(t *testing.T) {
	candidates := make([]primeChatFollowedUser, 12)
	for i := range candidates {
		candidates[i] = primeChatFollowedUser{
			userID: fmt.Sprintf("user-%d", i),
			label:  fmt.Sprintf("user-%d", i),
		}
	}

	var active int32
	var maxActive int32
	var releaseOnce sync.Once
	release := make(chan struct{})
	getRoom := func(userID string) (*rep_api.PrimeChatRoom, error) {
		current := atomic.AddInt32(&active, 1)
		for {
			previous := atomic.LoadInt32(&maxActive)
			if current <= previous || atomic.CompareAndSwapInt32(&maxActive, previous, current) {
				break
			}
		}
		if current == primeChatRoomQueryConcurrency {
			releaseOnce.Do(func() { close(release) })
		}
		select {
		case <-release:
		case <-time.After(time.Second):
			atomic.AddInt32(&active, -1)
			return nil, errors.New("query barrier timed out")
		}
		atomic.AddInt32(&active, -1)
		if userID == "user-2" || userID == "user-7" {
			return nil, errors.New("prime chat unavailable")
		}
		return &rep_api.PrimeChatRoom{
			ChatRoomId:   "prime:" + userID,
			TalentUserId: userID,
		}, nil
	}

	rooms := queryPrimeChatRooms(candidates, getRoom)
	if len(rooms) != len(candidates)-2 {
		t.Fatalf("len(rooms) = %d, want %d", len(rooms), len(candidates)-2)
	}
	if maxActive > primeChatRoomQueryConcurrency {
		t.Fatalf("max concurrent queries = %d, want <= %d", maxActive, primeChatRoomQueryConcurrency)
	}
	if maxActive != primeChatRoomQueryConcurrency {
		t.Fatalf("max concurrent queries = %d, want %d", maxActive, primeChatRoomQueryConcurrency)
	}
}

func TestBuildPrimeChatMessagesMergesReactionIntoCanonicalMessage(t *testing.T) {
	rooms := []*dal.PrimeChatRoom{{
		ChatRoomId:   "prime:talent-user:member-user",
		TalentUserId: "talent-user",
		MemberUserId: "member-user",
	}}
	page := &rep_api.PrimeChatMessagesPage{
		Messages: []*rep_api.PrimeChatMessage{{
			UserId:               "talent-user",
			MemberUserId:         "member-user",
			MessageId:            "message-1",
			Sender:               rep_api.PrimeChatSenderMember,
			BodyType:             rep_api.PrimeChatBodyText,
			Content:              "member message",
			CreateUnixTimeMillis: 1_700_000_000_000,
		}},
		AllReactedMessages: []*rep_api.PrimeChatMessage{{
			UserId:        "talent-user",
			MemberUserId:  "member-user",
			MessageId:     "message-1",
			ReactionEmoji: "\U0001F44D",
		}},
	}

	messages, unmatched := buildPrimeChatMessages(page, rooms)
	if unmatched != 0 {
		t.Fatalf("unmatched = %d, want 0", unmatched)
	}
	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(messages))
	}

	message := messages[0]
	if message.ChatRoomId != "prime:talent-user:member-user" {
		t.Fatalf("ChatRoomId = %q", message.ChatRoomId)
	}
	if message.TextContent != "member message" {
		t.Fatalf("TextContent = %q", message.TextContent)
	}
	if message.ReactionEmoji != "\U0001F44D" {
		t.Fatalf("ReactionEmoji = %q, want thumbs-up", message.ReactionEmoji)
	}
	if message.ReactionOnly {
		t.Fatal("canonical message must not be marked ReactionOnly")
	}
}

func TestMergePrimeChatRoomsDeduplicatesParticipantKey(t *testing.T) {
	first := &rep_api.PrimeChatRoom{
		ChatRoomId:   "prime:talent-user:member-user",
		TalentUserId: "talent-user",
		MemberUserId: "member-user",
	}
	duplicate := &rep_api.PrimeChatRoom{
		ChatRoomId:   "prime:talent-user:member-user",
		TalentUserId: "talent-user",
		MemberUserId: "member-user",
	}

	rooms := mergePrimeChatRooms([]*rep_api.PrimeChatRoom{first}, []*rep_api.PrimeChatRoom{duplicate})
	if len(rooms) != 1 {
		t.Fatalf("len(rooms) = %d, want 1", len(rooms))
	}
	if rooms[0] != first {
		t.Fatal("mergePrimeChatRooms should retain the first canonical room")
	}
}
