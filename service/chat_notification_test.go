package service

import "testing"

func TestFormatChatSenderNotificationIncludesMessageImageAndVideoCounts(t *testing.T) {
	got := formatChatSenderNotification("幸村恵理", ChatSenderSummary{
		NewMessages:      3,
		NewImageMessages: 2,
		NewVideoMessages: 1,
	})
	want := "幸村恵理 给你发了3条新消息！而且有 2 张新照片！1个新视频！⸜(*ˊᗜˋ*)⸝"
	if got != want {
		t.Fatalf("notification = %q, want %q", got, want)
	}
}

func TestFormatChatSenderNotificationKeepsMessageCountWithoutMedia(t *testing.T) {
	got := formatChatSenderNotification("幸村恵理", ChatSenderSummary{NewMessages: 1})
	want := "幸村恵理 给你发了1条新消息！⸜(*ˊᗜˋ*)⸝"
	if got != want {
		t.Fatalf("notification = %q, want %q", got, want)
	}
}
