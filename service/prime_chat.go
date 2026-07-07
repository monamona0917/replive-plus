package service

import (
	"fmt"
	"path/filepath"
	"replive/config"
	"replive/dal"
	"replive/rep_api"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// syncPrimeChatRooms 调用 GetPrimeChatRoom 获取背景图
func syncPrimeChatRooms() error {
	now := time.Now()

	// 方式1: 尝试 ListPrimeChatRooms（先发送带 max_page_size 的请求）
	rooms, listErr := rep_api.ListPrimeChatRooms()
	if listErr == nil && len(rooms) > 0 {
		return processPrimeChatRooms(rooms, now)
	}
	if listErr != nil {
		hlog.Debugf("syncPrimeChatRooms: ListPrimeChatRooms failed: %v", listErr)
	}

	// 方式2: 逐房间调用 GetPrimeChatRoom
	return syncPrimeChatRoomsPerRoom(now)
}

// syncPrimeChatRoomsPerRoom 逐一调用 GetPrimeChatRoom
func syncPrimeChatRoomsPerRoom(now time.Time) error {
	chatRooms, err := dal.GetChatRooms()
	if err != nil {
		return fmt.Errorf("get chat rooms failed: %v", err)
	}
	if len(chatRooms) == 0 {
		return nil
	}

	hlog.Infof("syncPrimeChatRooms: checking %d rooms via GetPrimeChatRoom", len(chatRooms))
	downloaded := 0
	for _, room := range chatRooms {
		if room == nil || strings.TrimSpace(room.UserId) == "" {
			continue
		}

		primeRoom, err := rep_api.GetPrimeChatRoom(room.UserId)
		if err != nil {
			hlog.Debugf("syncPrimeChatRooms: GetPrimeChatRoom failed for %s: %v", room.DisplayName, err)
			continue
		}
		if primeRoom == nil {
			continue
		}

		url := strings.TrimSpace(primeRoom.MemberBackgroundImageUrl)
		if url == "" {
			hlog.Debugf("syncPrimeChatRooms: no background for %s", room.DisplayName)
			continue
		}

		owner := firstNonEmpty(room.DisplayName, room.UniqueId, room.UserId, "unknown")
		prefix := filepath.Join(config.GetMediaPath(), "profile")
		hlog.Infof("syncPrimeChatRooms: downloading background for %s from %s", owner, url)
		if _, err := DownloadProfileMedia(url, now, prefix, owner, "prime_chat_background"); err != nil {
			hlog.Warnf("syncPrimeChatRooms: download failed for %s: %v", owner, err)
		} else {
			downloaded++
		}
	}

	hlog.Infof("syncPrimeChatRooms per-room done, checked: %d, downloaded: %d", len(chatRooms), downloaded)
	return nil
}

// processPrimeChatRooms 处理 ListPrimeChatRooms 返回的结果
func processPrimeChatRooms(rooms []*rep_api.PrimeChatRoom, now time.Time) error {
	downloaded := 0
	for _, room := range rooms {
		if room == nil {
			continue
		}
		url := strings.TrimSpace(room.MemberBackgroundImageUrl)
		if url == "" {
			continue
		}
		owner := firstNonEmpty(room.TalentDisplayName, room.TalentUserId, "unknown")
		prefix := filepath.Join(config.GetMediaPath(), "profile")
		hlog.Infof("syncPrimeChatRooms: downloading background for %s", owner)
		if _, err := DownloadProfileMedia(url, now, prefix, owner, "prime_chat_background"); err != nil {
			hlog.Warnf("syncPrimeChatRooms: download failed for %s: %v", owner, err)
		} else {
			downloaded++
		}
	}
	hlog.Infof("syncPrimeChatRooms done, rooms: %d, downloaded: %d", len(rooms), downloaded)
	return nil
}
