package service

import (
	"os"
	"path/filepath"
	"replive/config"
	"replive/dal"
	"replive/model"
	"replive/rep_api"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"gorm.io/gorm"
)

const profileMediaMaxPages = 50

func syncChatRoomProfileMedia(room *model.ChatRoom, now time.Time) MediaSyncSummary {
	if room == nil || room.UserProfile == nil {
		return MediaSyncSummary{}
	}
	profile := room.UserProfile
	owner := firstNonEmpty(profile.GetDisplayName(), profile.GetUniqueId(), profile.GetUserId(), room.GetUserId())
	urls := map[string]string{
		"chat_room_avatar": profile.GetAvatarUrl(),
	}
	return downloadProfileURLSet(owner, urls, now)
}

// syncCurrentUserProfile stores the current user's display information and
// archives the avatar so the local web UI can keep working without Replive.
func syncCurrentUserProfile() error {
	user, err := rep_api.GetUserPrivate()
	if err != nil {
		return err
	}
	now := time.Now()
	if err := saveUserPrivate(user, now); err != nil {
		return err
	}
	if user == nil || strings.TrimSpace(user.GetProfileImageUrl()) == "" {
		return nil
	}
	path, _, downloadErr := downloadProfileMediaPath(
		userPrivateOwner(user),
		user.GetProfileImageUrl(),
		now,
		"user_private_profile",
	)
	if downloadErr != nil {
		hlog.Warnf("download current user avatar failed: %v", downloadErr)
		return nil
	}
	return dal.UpdateUserPrivateProfileImagePath(user.GetUserId(), path)
}

func syncOshiProfiles() error {
	now := time.Now()
	syncedCount := 0
	oshiMediaSummary := MediaSyncSummary{}
	followingMediaSummary := MediaSyncSummary{}
	followingCount := 0
	pageToken := ""
	defer func() {
		logMediaSyncSummary("Oshi profile media sync", oshiMediaSummary)
		logMediaSyncSummary("Following profile media sync", followingMediaSummary)
	}()

	for page := 0; page < profileMediaMaxPages; page++ {
		resp, err := rep_api.ListMyOshis(200, pageToken)
		if err != nil {
			return err
		}
		items := resp.GetOshis()
		hlog.Infof(
			"sync oshi profiles page: %d, page_count: %d, my_oshis_count: %d, next_token: %t",
			page+1,
			len(items),
			resp.GetMyOshisCount(),
			strings.TrimSpace(resp.GetNextPageToken()) != "",
		)
		if err := saveOshis(items, now); err != nil {
			return err
		}
		for _, item := range items {
			mergeMediaSummary(&oshiMediaSummary, downloadProfileURLSet(oshiOwner(item), oshiMediaURLs(item), now))
		}
		syncedCount += len(items)

		nextToken := strings.TrimSpace(resp.GetNextPageToken())
		if nextToken == "" || nextToken == pageToken {
			break
		}
		pageToken = nextToken
	}

	followingCount, followingMediaSummary, err := syncFollowings(now)
	if err != nil {
		return err
	}

	hlog.Infof("sync oshi profiles done, oshi_count: %d, following_count: %d", syncedCount, followingCount)
	return nil
}

func syncFollowings(now time.Time) (int, MediaSyncSummary, error) {
	syncedCount := 0
	mediaSummary := MediaSyncSummary{}
	pageToken := ""

	for page := 0; page < profileMediaMaxPages; page++ {
		resp, err := rep_api.ListFollowings(20, pageToken)
		if err != nil {
			return syncedCount, mediaSummary, err
		}
		items := resp.GetFollowTargets()
		hlog.Infof(
			"sync followings page: %d, page_count: %d, next_token: %t",
			page+1,
			len(items),
			strings.TrimSpace(resp.GetNextPageToken()) != "",
		)
		if err := saveFollowings(items, now); err != nil {
			return syncedCount, mediaSummary, err
		}
		for _, item := range items {
			mergeMediaSummary(&mediaSummary, downloadProfileURLSet(followTargetOwner(item), followTargetMediaURLs(item), now))
		}
		syncedCount += len(items)

		nextToken := strings.TrimSpace(resp.GetNextPageToken())
		if nextToken == "" || nextToken == pageToken {
			break
		}
		pageToken = nextToken
	}

	return syncedCount, mediaSummary, nil
}

func saveOshis(items []*model.ListMyOshisOshi, now time.Time) error {
	return dal.WithWriteDB(func(db *gorm.DB) error {
		for _, item := range items {
			dbOshi := buildDBOshi(item, now)
			if dbOshi == nil || strings.TrimSpace(dbOshi.OshiId) == "" {
				continue
			}
			var existing dal.Oshi
			if err := db.Table(dal.Oshi{}.TableName()).
				Where("oshi_id = ?", dbOshi.OshiId).
				Limit(1).
				Find(&existing).Error; err != nil {
				return err
			}
			if existing.Id > 0 {
				dbOshi.Id = existing.Id
				if err := db.Save(dbOshi).Error; err != nil {
					return err
				}
				continue
			}
			if err := db.Create(dbOshi).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func saveFollowings(items []*model.FollowTarget, now time.Time) error {
	return dal.WithWriteDB(func(db *gorm.DB) error {
		for _, item := range items {
			dbFollowing := buildDBFollowing(item, now)
			if dbFollowing == nil || strings.TrimSpace(dbFollowing.TargetKey) == "" {
				continue
			}
			var existing dal.Following
			if err := db.Table(dal.Following{}.TableName()).
				Where("target_key = ?", dbFollowing.TargetKey).
				Limit(1).
				Find(&existing).Error; err != nil {
				return err
			}
			if existing.Id > 0 {
				dbFollowing.Id = existing.Id
				if err := db.Save(dbFollowing).Error; err != nil {
					return err
				}
				continue
			}
			if err := db.Create(dbFollowing).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func saveUserPrivate(user *model.UserPrivate, now time.Time) error {
	dbUser := buildDBUserPrivate(user, now)
	if dbUser == nil || strings.TrimSpace(dbUser.UserId) == "" {
		return nil
	}
	return dal.WithWriteDB(func(db *gorm.DB) error {
		var existing dal.UserPrivate
		if err := db.Table(dal.UserPrivate{}.TableName()).
			Where("user_id = ?", dbUser.UserId).
			Limit(1).
			Find(&existing).Error; err != nil {
			return err
		}
		if existing.Id > 0 {
			dbUser.Id = existing.Id
			if dbUser.ProfileImagePath == "" {
				dbUser.ProfileImagePath = existing.ProfileImagePath
			}
			return db.Save(dbUser).Error
		}
		return db.Create(dbUser).Error
	})
}

func buildDBOshi(item *model.ListMyOshisOshi, now time.Time) *dal.Oshi {
	if item == nil {
		return nil
	}
	user := item.GetUser()
	dbOshi := &dal.Oshi{
		OshiId:                    item.GetOshiId(),
		Name:                      item.GetName(),
		ProfileImageUrl:           item.GetProfileImageUrl(),
		MembershipImageUrl:        item.GetMembershipImageUrl(),
		UserId:                    user.GetUserId(),
		UniqueId:                  user.GetUniqueId(),
		DisplayName:               user.GetDisplayName(),
		UserProfileImageUrl:       user.GetProfileImageUrl(),
		ProfileBackgroundImageUrl: user.GetProfileBackgroundImageUrl(),
		SmProfileImageUrl:         user.GetSmProfileImageUrl(),
		UserOshiId:                item.GetOshiId(),
		SyncedAt:                  now.Unix(),
	}
	if dbOshi.UserOshiId == "" {
		dbOshi.UserOshiId = dbOshi.OshiId
	}
	return dbOshi
}

func buildDBFollowing(item *model.FollowTarget, now time.Time) *dal.Following {
	if item == nil {
		return nil
	}
	user := item.GetUser()
	oshi := item.GetOshi()
	dbFollowing := &dal.Following{
		TargetType: int64(item.GetType()),
		SyncedAt:   now.Unix(),
	}
	if user != nil {
		dbFollowing.UserId = user.GetUserId()
		dbFollowing.UniqueId = user.GetUniqueId()
		dbFollowing.DisplayName = user.GetDisplayName()
		dbFollowing.ProfileImageUrl = user.GetProfileImageUrl()
		dbFollowing.SmProfileImageUrl = user.GetSmProfileImageUrl()
		dbFollowing.ProfileBackgroundImageUrl = user.GetProfileBackgroundImageUrl()
		if dbFollowing.UserId != "" {
			dbFollowing.TargetKey = "user:" + dbFollowing.UserId
		}
	}
	if oshi != nil {
		dbFollowing.OshiId = oshi.GetOshiId()
		dbFollowing.OshiName = oshi.GetName()
		dbFollowing.OshiProfileImageUrl = oshi.GetProfileImageUrl()
		dbFollowing.OshiMembershipImageUrl = oshi.GetMembershipImageUrl()
		if dbFollowing.DisplayName == "" {
			dbFollowing.DisplayName = oshi.GetName()
		}
		if dbFollowing.TargetKey == "" && dbFollowing.OshiId != "" {
			dbFollowing.TargetKey = "oshi:" + dbFollowing.OshiId
		}
	}
	if dbFollowing.TargetKey == "" && dbFollowing.UniqueId != "" {
		dbFollowing.TargetKey = "user_unique:" + dbFollowing.UniqueId
	}
	return dbFollowing
}

func buildDBUserPrivate(user *model.UserPrivate, now time.Time) *dal.UserPrivate {
	if user == nil {
		return nil
	}
	return &dal.UserPrivate{
		UserId:                    user.GetUserId(),
		UniqueId:                  user.GetUniqueId(),
		DisplayName:               user.GetDisplayName(),
		ProfileImageUrl:           user.GetProfileImageUrl(),
		SmProfileImageUrl:         user.GetSmProfileImageUrl(),
		ProfileBackgroundImageUrl: user.GetProfileBackgroundImageUrl(),
		SyncedAt:                  now.Unix(),
	}
}

func downloadProfileMediaPath(owner, rawURL string, now time.Time, kind string) (string, MediaResult, error) {
	path, result, err := DownloadProfileMediaWithResult(
		rawURL,
		now,
		filepath.Join(config.GetMediaPath(), "profile"),
		owner,
		kind,
	)
	if err != nil {
		return "", MediaFailed, err
	}
	storedPath, err := toStoredProfileMediaPath(path)
	if err != nil {
		return "", MediaFailed, err
	}
	return storedPath, result, nil
}

func toStoredProfileMediaPath(path string) (string, error) {
	root, err := filepath.Abs(config.GetMediaPath())
	if err != nil {
		return "", err
	}
	fullPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, fullPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return "", os.ErrInvalid
	}
	return filepath.ToSlash(rel), nil
}

func resolveStoredProfileMediaPath(storedPath string) string {
	storedPath = strings.TrimSpace(storedPath)
	if storedPath == "" {
		return ""
	}
	if filepath.IsAbs(storedPath) {
		return filepath.Clean(storedPath)
	}
	return filepath.Join(config.GetMediaPath(), filepath.FromSlash(storedPath))
}

func findArchivedProfileMediaPath(owner, rawURL string, now time.Time) string {
	if strings.TrimSpace(owner) == "" || strings.TrimSpace(rawURL) == "" {
		return ""
	}
	filename, err := getProfileMediaFileName(rawURL)
	if err != nil {
		return ""
	}
	ownerDir := filepath.Join(config.GetMediaPath(), "profile", sanitizeFileName(owner))
	year, month := getProfileMediaYearMonth(rawURL, now)
	candidate := filepath.Join(ownerDir, year, month, filename)
	if mediaFileExists(candidate) {
		stored, err := toStoredProfileMediaPath(candidate)
		if err == nil {
			return stored
		}
	}
	var found string
	_ = filepath.WalkDir(ownerDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry == nil {
			return nil
		}
		if !entry.IsDir() && strings.EqualFold(entry.Name(), filename) {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if found == "" {
		return ""
	}
	stored, err := toStoredProfileMediaPath(found)
	if err != nil {
		return ""
	}
	return stored
}

// BackfillProfileMediaPaths connects profile files downloaded by older builds
// with the corresponding local database rows. It never downloads anything.
func BackfillProfileMediaPaths() error {
	rooms, err := dal.GetChatRooms()
	if err != nil {
		return err
	}
	for _, room := range rooms {
		if room == nil || strings.TrimSpace(room.AvatarUrl) == "" {
			continue
		}
		if mediaFileExists(resolveStoredProfileMediaPath(room.AvatarPath)) {
			continue
		}
		path := findArchivedProfileMediaPath(room.DisplayName, room.AvatarUrl, time.Now())
		if path != "" {
			if err := dal.UpdateChatRoomAvatarPath(room.ChatRoomId, path); err != nil {
				return err
			}
		}
	}

	primeRooms, err := dal.GetPrimeChatRooms()
	if err != nil {
		return err
	}
	for _, room := range primeRooms {
		if room == nil || strings.TrimSpace(room.TalentAvatarUrl) == "" {
			continue
		}
		if mediaFileExists(resolveStoredProfileMediaPath(room.TalentAvatarPath)) {
			continue
		}
		path := findArchivedProfileMediaPath(room.TalentDisplayName, room.TalentAvatarUrl, time.Now())
		if path != "" {
			if err := dal.UpdatePrimeChatRoomTalentAvatarPath(room.ChatRoomId, path); err != nil {
				return err
			}
		}
	}

	user, err := dal.GetUserPrivate()
	if err != nil {
		return err
	}
	if user != nil && strings.TrimSpace(user.ProfileImageUrl) != "" && !mediaFileExists(resolveStoredProfileMediaPath(user.ProfileImagePath)) {
		path := findArchivedProfileMediaPath(user.DisplayName, user.ProfileImageUrl, time.Now())
		if path != "" {
			if err := dal.UpdateUserPrivateProfileImagePath(user.UserId, path); err != nil {
				return err
			}
		}
	}
	return nil
}

func oshiOwner(item *model.ListMyOshisOshi) string {
	if item == nil {
		return "unknown"
	}
	user := item.GetUser()
	return firstNonEmpty(user.GetDisplayName(), item.GetName(), user.GetUniqueId(), item.GetOshiId(), user.GetUserId(), "unknown")
}

func userPrivateOwner(user *model.UserPrivate) string {
	if user == nil {
		return "unknown"
	}
	return firstNonEmpty(user.GetDisplayName(), user.GetUniqueId(), user.GetUserId(), "unknown")
}

func followTargetOwner(item *model.FollowTarget) string {
	if item == nil {
		return "unknown"
	}
	user := item.GetUser()
	oshi := item.GetOshi()
	return firstNonEmpty(user.GetDisplayName(), oshi.GetName(), user.GetUniqueId(), user.GetUserId(), oshi.GetOshiId(), "unknown")
}

func oshiMediaURLs(item *model.ListMyOshisOshi) map[string]string {
	urls := make(map[string]string)
	if item == nil {
		return urls
	}
	user := item.GetUser()
	urls["oshi_profile"] = item.GetProfileImageUrl()
	urls["membership"] = item.GetMembershipImageUrl()
	urls["profile"] = user.GetProfileImageUrl()
	urls["sm_profile"] = user.GetSmProfileImageUrl()
	urls["profile_background"] = user.GetProfileBackgroundImageUrl()
	return urls
}

func followTargetMediaURLs(item *model.FollowTarget) map[string]string {
	urls := make(map[string]string)
	if item == nil {
		return urls
	}
	user := item.GetUser()
	if user != nil {
		urls["following_profile"] = user.GetProfileImageUrl()
		urls["following_sm_profile"] = user.GetSmProfileImageUrl()
		urls["following_profile_background"] = user.GetProfileBackgroundImageUrl()
	}
	oshi := item.GetOshi()
	if oshi != nil {
		user := oshi.GetUser()
		urls["following_oshi_profile"] = oshi.GetProfileImageUrl()
		urls["following_oshi_membership"] = oshi.GetMembershipImageUrl()
		urls["following_oshi_user_profile"] = user.GetProfileImageUrl()
		urls["following_oshi_user_sm_profile"] = user.GetSmProfileImageUrl()
		urls["following_oshi_user_profile_background"] = user.GetProfileBackgroundImageUrl()
	}
	return urls
}

func userPrivateMediaURLs(user *model.UserPrivate) map[string]string {
	urls := make(map[string]string)
	if user == nil {
		return urls
	}
	urls["user_private_profile"] = user.GetProfileImageUrl()
	urls["user_private_sm_profile"] = user.GetSmProfileImageUrl()
	urls["user_private_profile_background"] = user.GetProfileBackgroundImageUrl()
	return urls
}

func downloadProfileURLSet(owner string, urls map[string]string, now time.Time) MediaSyncSummary {
	mediaSummary := MediaSyncSummary{}
	owner = firstNonEmpty(owner, "unknown")
	prefix := filepath.Join(config.GetMediaPath(), "profile")
	seen := make(map[string]struct{}, len(urls))
	for kind, rawURL := range urls {
		rawURL = strings.TrimSpace(rawURL)
		if rawURL == "" {
			continue
		}
		if _, ok := seen[rawURL]; ok {
			continue
		}
		seen[rawURL] = struct{}{}
		if _, result, err := DownloadProfileMediaWithResult(rawURL, now, prefix, owner, kind); err != nil {
			mediaSummary.Add(MediaFailed)
			hlog.Warnf("download profile media failed, owner=%s kind=%s err=%v", owner, kind, err)
		} else {
			mediaSummary.Add(result)
		}
	}
	return mediaSummary
}

func mergeMediaSummary(target *MediaSyncSummary, source MediaSyncSummary) {
	if target == nil {
		return
	}
	target.Downloaded += source.Downloaded
	target.Skipped += source.Skipped
	target.Failed += source.Failed
}

func logMediaSyncSummary(label string, summary MediaSyncSummary) {
	if summary.Downloaded == 0 && summary.Failed == 0 {
		hlog.Infof("%s: downloaded=0 skipped=%d failed=0", label, summary.Skipped)
		return
	}
	hlog.Infof("%s: downloaded=%d skipped=%d failed=%d", label, summary.Downloaded, summary.Skipped, summary.Failed)
}

func timestampSeconds(ts *model.Timestamp) int64 {
	if ts == nil {
		return 0
	}
	return ts.GetSeconds()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
