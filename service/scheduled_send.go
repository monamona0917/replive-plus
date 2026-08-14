package service

import (
	"fmt"
	"replive/config"
	"replive/rep_api"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
)

const scheduledSendLocation = "Asia/Tokyo"

func startScheduledChatSender() {
	cfg := config.Conf.ScheduledChatMessage
	if !cfg.Enabled && !cfg.Card.Enabled {
		hlog.Infof("scheduled chat sender disabled")
		return
	}
	go runScheduledChatSender()
}

func runScheduledChatSender() {
	loc, err := time.LoadLocation(scheduledSendLocation)
	if err != nil {
		hlog.Errorf("load %s location failed: %v", scheduledSendLocation, err)
		return
	}
	for {
		next := nextSunday2300(time.Now(), loc)
		wait := time.Until(next)
		hlog.Infof("scheduled chat sender next run at %s", next.Format(time.RFC3339))
		timer := time.NewTimer(wait)
		<-timer.C
		if err := sendConfiguredScheduledMessages(); err != nil {
			hlog.Errorf("scheduled chat sender failed: %v", err)
		}
	}
}

func nextSunday2300(now time.Time, loc *time.Location) time.Time {
	localNow := now.In(loc)
	daysUntilSunday := (int(time.Sunday) - int(localNow.Weekday()) + 7) % 7
	next := time.Date(localNow.Year(), localNow.Month(), localNow.Day()+daysUntilSunday, 23, 0, 0, 0, loc)
	if !next.After(localNow) {
		next = next.AddDate(0, 0, 7)
	}
	return next
}

func sendConfiguredScheduledMessages() error {
	cfg := config.Conf.ScheduledChatMessage
	if cfg.Enabled {
		userID, chatRoomID, err := resolveScheduledChatRoom(cfg.UserID, cfg.ChatRoomID, cfg.DisplayName)
		if err != nil {
			return err
		}
		if _, err := rep_api.SendChatMessage(userID, chatRoomID, cfg.Content); err != nil {
			return err
		}
		hlog.Infof("scheduled chat message sent, display_name: %s, user_id: %s, chat_room_id: %s", cfg.DisplayName, userID, chatRoomID)
	}

	if cfg.Card.Enabled {
		userID, err := resolveScheduledCardUserID(cfg.Card.UserID, cfg.Card.DisplayName)
		if err != nil {
			return err
		}
		if _, err := rep_api.CreateCard(userID, cfg.Card.LiveID, cfg.Card.Content, cfg.Card.CoinAmount); err != nil {
			return err
		}
		hlog.Infof("scheduled card sent, display_name: %s, user_id: %s, coin_amount: %d", cfg.Card.DisplayName, userID, cfg.Card.CoinAmount)
	}
	return nil
}

func resolveScheduledChatRoom(userID, chatRoomID, displayName string) (string, string, error) {
	userID = strings.TrimSpace(userID)
	chatRoomID = strings.TrimSpace(chatRoomID)
	if userID != "" && chatRoomID != "" {
		return userID, chatRoomID, nil
	}
	if strings.TrimSpace(displayName) == "" {
		return "", "", fmt.Errorf("scheduled chat target requires user_id/chat_room_id or display_name")
	}
	room, err := findChatRoomByDisplayName(displayName)
	if err != nil {
		return "", "", err
	}
	return room.UserId, room.ChatRoomId, nil
}

func resolveScheduledCardUserID(userID, displayName string) (string, error) {
	userID = strings.TrimSpace(userID)
	if userID != "" {
		return userID, nil
	}
	room, err := findChatRoomByDisplayName(displayName)
	if err != nil {
		return "", err
	}
	return room.UserId, nil
}

func findChatRoomByDisplayName(displayName string) (*modelChatRoom, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return nil, fmt.Errorf("display_name is required")
	}
	rooms, err := rep_api.GetChatRooms()
	if err != nil {
		return nil, err
	}
	for _, room := range rooms {
		if room == nil || room.UserProfile == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(room.UserProfile.DisplayName), displayName) {
			return &modelChatRoom{UserId: room.UserId, ChatRoomId: room.ChatRoomId}, nil
		}
	}
	return nil, fmt.Errorf("chat room display_name not found: %s", displayName)
}

type modelChatRoom struct {
	UserId     string
	ChatRoomId string
}
