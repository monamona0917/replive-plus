package service

import (
	"reflect"
	"testing"

	"replive/dal"
)

func TestShuffleChatRoomsKeepsSameRooms(t *testing.T) {
	rooms := []*dal.ChatRoom{
		{ChatRoomId: "room-1"},
		{ChatRoomId: "room-2"},
		{ChatRoomId: "room-3"},
		{ChatRoomId: "room-4"},
	}
	before := chatRoomIDs(rooms)

	shuffleChatRooms(rooms)

	after := chatRoomIDs(rooms)
	if !sameStringSet(before, after) {
		t.Fatalf("shuffleChatRooms changed room set, before=%v after=%v", before, after)
	}
}

func chatRoomIDs(rooms []*dal.ChatRoom) []string {
	ids := make([]string, 0, len(rooms))
	for _, room := range rooms {
		ids = append(ids, room.ChatRoomId)
	}
	return ids
}

func sameStringSet(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	am := make(map[string]int, len(a))
	for _, value := range a {
		am[value]++
	}
	bm := make(map[string]int, len(b))
	for _, value := range b {
		bm[value]++
	}
	return reflect.DeepEqual(am, bm)
}
