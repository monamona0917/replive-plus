package service

import (
	"testing"
	"time"
)

func TestChatRoomTimingLogValueUsesNanosecondsOnlyForLogState(t *testing.T) {
	value := time.Unix(1787245796, 123456789)

	if got := chatRoomTimingLogValue(value, true); got != value.UnixNano() {
		t.Fatalf("chatRoomTimingLogValue() = %d, want %d", got, value.UnixNano())
	}
	if got := chatRoomTimingLogValue(value, false); got != 0 {
		t.Fatalf("chatRoomTimingLogValue() for absent value = %d, want 0", got)
	}
	if got := chatRoomTimingUnix(value, true); got != value.Unix() {
		t.Fatalf("chatRoomTimingUnix() = %d, want %d", got, value.Unix())
	}
}

func TestShouldLogChatRoomTimingTracksFirstAndRepeatedValues(t *testing.T) {
	resetChatRoomTimingLogStateForTest(t)

	if !shouldLogChatRoomTiming("room-1", 100) {
		t.Fatal("first timing value should be logged")
	}
	if shouldLogChatRoomTiming("room-1", 100) {
		t.Fatal("unchanged timing value should not be logged")
	}
}

func TestShouldLogChatRoomTimingDetectsNanosecondChanges(t *testing.T) {
	resetChatRoomTimingLogStateForTest(t)

	first := chatRoomTimingLogValue(time.Unix(1787245796, 100), true)
	second := chatRoomTimingLogValue(time.Unix(1787245796, 200), true)
	if first == second {
		t.Fatal("test values must differ at nanosecond precision")
	}
	if !shouldLogChatRoomTiming("room-1", first) {
		t.Fatal("first timing value should be logged")
	}
	if !shouldLogChatRoomTiming("room-1", second) {
		t.Fatal("same-second nanosecond change should be logged")
	}
	if shouldLogChatRoomTiming("room-1", second) {
		t.Fatal("repeated nanosecond value should not be logged")
	}
}

func TestShouldLogChatRoomTimingKeepsRoomsIndependent(t *testing.T) {
	resetChatRoomTimingLogStateForTest(t)

	if !shouldLogChatRoomTiming("room-1", 100) {
		t.Fatal("first room-1 value should be logged")
	}
	if !shouldLogChatRoomTiming("room-2", 100) {
		t.Fatal("first room-2 value should be logged")
	}
	if shouldLogChatRoomTiming("room-1", 100) {
		t.Fatal("unchanged room-1 value should not be logged")
	}
	if shouldLogChatRoomTiming("room-2", 100) {
		t.Fatal("unchanged room-2 value should not be logged")
	}
	if !shouldLogChatRoomTiming("room-2", 200) {
		t.Fatal("changed room-2 value should be logged")
	}
	if shouldLogChatRoomTiming("room-1", 100) {
		t.Fatal("room-2 change should not affect room-1")
	}
}

func TestShouldLogChatRoomTimingDeduplicatesUnsetValues(t *testing.T) {
	resetChatRoomTimingLogStateForTest(t)

	if !shouldLogChatRoomTiming("room-1", 0) {
		t.Fatal("first unset timing value should be logged")
	}
	if shouldLogChatRoomTiming("room-1", 0) {
		t.Fatal("repeated unset timing value should not be logged")
	}
}

func resetChatRoomTimingLogStateForTest(t *testing.T) {
	t.Helper()

	chatRoomTimingLogState.Lock()
	chatRoomTimingLogState.values = make(map[string]int64)
	chatRoomTimingLogState.Unlock()
	t.Cleanup(func() {
		chatRoomTimingLogState.Lock()
		chatRoomTimingLogState.values = nil
		chatRoomTimingLogState.Unlock()
	})
}
