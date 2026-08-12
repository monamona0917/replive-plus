package handler

import (
	"testing"

	"replive/dal"
)

func TestBuildChatMessageRespUsesChronologicalCursors(t *testing.T) {
	messages := []dal.ChatMessage{
		{Id: 80, SendTime: 100},
		{Id: 2, SendTime: 300},
		{Id: 100, SendTime: 200},
		{Id: 1, SendTime: 300},
	}

	_, prevCursor, nextCursor := buildChatMessageResp(messages)
	if prevCursor != 80 {
		t.Fatalf("prevCursor = %d, want chronological oldest ID 80", prevCursor)
	}
	if nextCursor != 2 {
		t.Fatalf("nextCursor = %d, want chronological newest ID 2", nextCursor)
	}
}
