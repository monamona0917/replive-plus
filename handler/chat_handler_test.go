package handler

import (
	"testing"
	"time"

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

func TestLocalDateKeyFromUnixValueUsesSystemTimeZone(t *testing.T) {
	originalLocation := time.Local
	time.Local = time.FixedZone("UTC+08", 8*60*60)
	defer func() { time.Local = originalLocation }()

	seconds := time.Date(2026, 1, 1, 16, 30, 0, 0, time.UTC).Unix()
	if got := localDateKeyFromUnixValue(seconds); got != "2026-01-02" {
		t.Fatalf("seconds date key = %q, want 2026-01-02", got)
	}

	millis := time.Date(2026, 1, 2, 15, 30, 0, 0, time.UTC).UnixMilli()
	if got := localDateKeyFromUnixValue(millis); got != "2026-01-02" {
		t.Fatalf("milliseconds date key = %q, want 2026-01-02", got)
	}
}
