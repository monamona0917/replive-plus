package service

import (
	"slices"
	"testing"
)

func TestBuildFfmpegRecordArgsSkipsReconnectForRtmp(t *testing.T) {
	args := buildFfmpegRecordArgs("rtmp://example.com/live?txSecret=abc&txTime=def", "out.mp4")

	if slices.Contains(args, "-reconnect") {
		t.Fatalf("rtmp args should not include reconnect options: %v", args)
	}
	if !slices.Contains(args, "-rw_timeout") {
		t.Fatalf("rtmp args should keep rw_timeout: %v", args)
	}
}

func TestBuildFfmpegRecordArgsUsesReconnectForHTTP(t *testing.T) {
	args := buildFfmpegRecordArgs("https://example.com/live.m3u8", "out.mp4")

	if !slices.Contains(args, "-reconnect") {
		t.Fatalf("http args should include reconnect options: %v", args)
	}
}
