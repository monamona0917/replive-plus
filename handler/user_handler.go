package handler

import (
	"context"
	"replive/config"
	"replive/dal"
	"replive/rep_api"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type UserProfileDTO struct {
	UserId          string `json:"user_id"`
	UniqueId        string `json:"unique_id"`
	DisplayName     string `json:"display_name"`
	AvatarUrl       string `json:"avatar_url"`
	AvatarLocalURL  string `json:"avatar_local_url,omitempty"`
	SendChatEnabled bool   `json:"send_chat"`
	OfflineMode     bool   `json:"offline_mode"`
}

// HandleGetCurrentUser 返回当前登录用户的信息
func HandleGetCurrentUser(ctx context.Context, c *app.RequestContext) {
	user, err := dal.GetUserPrivate()
	if err != nil {
		hlog.Errorf("HandleGetCurrentUser: local profile query failed: %v", err)
		c.JSON(consts.StatusOK, BadResp("读取本地用户信息失败: "+err.Error()))
		return
	}
	if user == nil {
		c.JSON(consts.StatusOK, &Resp{Data: &UserProfileDTO{
			SendChatEnabled: config.Conf.SendChatEnabled && rep_api.IsOnline(),
			OfflineMode:     !rep_api.IsOnline(),
		}})
		return
	}
	hlog.Infof("HandleGetCurrentUser: user_id=%s display_name=%s", user.UserId, user.DisplayName)
	c.JSON(consts.StatusOK, &Resp{Data: &UserProfileDTO{
		UserId:          user.UserId,
		UniqueId:        user.UniqueId,
		DisplayName:     user.DisplayName,
		AvatarUrl:       user.ProfileImageUrl,
		AvatarLocalURL:  localProfileMediaURL(user.ProfileImagePath),
		SendChatEnabled: config.Conf.SendChatEnabled && rep_api.IsOnline(),
		OfflineMode:     !rep_api.IsOnline(),
	}})
}
