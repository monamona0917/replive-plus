package service

import (
	"os"
	"path/filepath"
	"replive/dal"
	"replive/model"
	"strings"
	"testing"
)

func TestBuildMediaEmailInfosWithSmallAttachment(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "image.jpeg")
	if err := os.WriteFile(filePath, []byte("small-file"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	msg := &dal.ChatMessage{
		DisplayName: "tester",
		MsgType:     int32(model.ChatMessageType_Image),
		ImageUrl:    "https://example.com/image.jpeg",
		ImagePath:   filePath,
	}

	infos := buildMediaEmailInfos(msg, "ctx")
	if len(infos) != 2 {
		t.Fatalf("len(infos) = %d, want 2", len(infos))
	}
	if infos[0].FilePath != "" {
		t.Fatalf("summary email should not contain attachment, got %q", infos[0].FilePath)
	}
	if infos[1].FilePath != filePath {
		t.Fatalf("attachment email FilePath = %q, want %q", infos[1].FilePath, filePath)
	}
	if !strings.Contains(infos[0].Title, "[URL]") {
		t.Fatalf("summary email title = %q", infos[0].Title)
	}
	if !strings.Contains(infos[1].Title, "[附件]") {
		t.Fatalf("attachment email title = %q", infos[1].Title)
	}
	if !strings.Contains(infos[0].Content, "原始URL: https://example.com/image.jpeg") {
		t.Fatalf("summary email content = %q", infos[0].Content)
	}
}

func TestBuildMediaEmailInfosAlwaysReturnsAttachmentEmail(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "video.mp4")
	if err := os.WriteFile(filePath, []byte("video"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	msg := &dal.ChatMessage{
		DisplayName: "tester",
		MsgType:     int32(model.ChatMessageType_Video),
		VideoUrl:    "https://example.com/video.mp4",
		VideoPath:   filePath,
	}

	infos := buildMediaEmailInfos(msg, "ctx")
	if len(infos) != 2 {
		t.Fatalf("len(infos) = %d, want 2", len(infos))
	}
	if infos[1].FilePath != filePath {
		t.Fatalf("attachment email FilePath = %q, want %q", infos[1].FilePath, filePath)
	}
	if !strings.Contains(infos[0].Content, "原始URL: https://example.com/video.mp4") {
		t.Fatalf("summary email content = %q", infos[0].Content)
	}
}
