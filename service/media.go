package service

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"replive/rep_api"
	"strings"
	"time"
)

var fetchMedia = rep_api.Get

type MediaResult int

const (
	MediaDownloaded MediaResult = iota
	MediaSkipped
	MediaFailed
)

type MediaSyncSummary struct {
	Downloaded int
	Skipped    int
	Failed     int
}

func (s *MediaSyncSummary) Add(result MediaResult) {
	if s == nil {
		return
	}
	switch result {
	case MediaDownloaded:
		s.Downloaded++
	case MediaSkipped:
		s.Skipped++
	case MediaFailed:
		s.Failed++
	}
}

func getImgFileName(mediaUrl string, imgTime time.Time, msgId string) (name string, path string, err error) {
	var u *url.URL
	u, err = url.Parse(mediaUrl)
	if err != nil {
		err = fmt.Errorf("failed to parse url: %v", err)
		return
	}
	paths := strings.Split(u.Path, "/")
	name = fmt.Sprintf("%s-%s-%s", imgTime.Format("2006-01-02"), msgId, paths[len(paths)-1])
	if !strings.Contains(name, ".") {
		name = fmt.Sprintf("%s.jpeg", name)
	}
	path = fmt.Sprintf("%d/%d", imgTime.Year(), imgTime.Month())
	return
}

func DownloadImage(mediaUrl string, imgTime time.Time, pathPrefix string, msgId string) (string, error) {
	path, _, err := DownloadImageWithResult(mediaUrl, imgTime, pathPrefix, msgId)
	return path, err
}

func DownloadImageWithResult(mediaUrl string, imgTime time.Time, pathPrefix string, msgId string) (string, MediaResult, error) {
	name, path, err := getImgFileName(mediaUrl, imgTime, msgId)
	if err != nil {
		return "", MediaFailed, fmt.Errorf("failed to get img file name: %v", err)
	}
	path = fmt.Sprintf("%s/%s", pathPrefix, path)
	return downloadMediaWithResult(mediaUrl, path, name)
}

func DownloadVideo(mediaUrl string, videoTime time.Time, pathPrefix string, msgId string) (string, error) {
	path, _, err := DownloadVideoWithResult(mediaUrl, videoTime, pathPrefix, msgId)
	return path, err
}

func DownloadVideoWithResult(mediaUrl string, videoTime time.Time, pathPrefix string, msgId string) (string, MediaResult, error) {
	path := fmt.Sprintf("%d/%d", videoTime.Year(), videoTime.Month())
	path = fmt.Sprintf("%s/%s", pathPrefix, path)
	name := fmt.Sprintf("%s_%s.mp4", videoTime.Format("2006-01-02"), msgId)
	return downloadMediaWithResult(mediaUrl, path, name)
}

func DownloadProfileMedia(mediaUrl string, mediaTime time.Time, pathPrefix string, owner string, kind string) (string, error) {
	path, _, err := DownloadProfileMediaWithResult(mediaUrl, mediaTime, pathPrefix, owner, kind)
	return path, err
}

func DownloadProfileMediaWithResult(mediaUrl string, mediaTime time.Time, pathPrefix string, owner string, kind string) (string, MediaResult, error) {
	name, err := getProfileMediaFileName(mediaUrl)
	if err != nil {
		return "", MediaFailed, fmt.Errorf("failed to get profile media file name: %v", err)
	}
	year, month := getProfileMediaYearMonth(mediaUrl, mediaTime)
	path := filepath.Join(pathPrefix, sanitizeFileName(owner), year, month)
	return downloadMediaWithResult(mediaUrl, path, name)
}

func getProfileMediaFileName(mediaUrl string) (string, error) {
	u, err := url.Parse(mediaUrl)
	if err != nil {
		return "", fmt.Errorf("failed to parse url: %v", err)
	}
	paths := strings.Split(u.Path, "/")
	base := strings.TrimSpace(paths[len(paths)-1])
	if base == "" || !strings.Contains(base, ".") {
		base = "image.jpeg"
	}
	return sanitizeFileName(base), nil
}

func getProfileMediaYearMonth(mediaUrl string, fallback time.Time) (string, string) {
	u, err := url.Parse(mediaUrl)
	if err == nil {
		parts := strings.FieldsFunc(u.EscapedPath(), func(r rune) bool {
			return r == '/' || r == '-' || r == '_' || r == '.'
		})
		for i := 0; i+1 < len(parts); i++ {
			year := parts[i]
			month := parts[i+1]
			if len(year) == 4 && strings.HasPrefix(year, "20") && len(month) >= 1 && len(month) <= 2 {
				if monthNum := parseMonth(month); monthNum > 0 {
					return year, fmt.Sprintf("%02d", monthNum)
				}
			}
		}
	}
	return fallback.Format("2006"), fallback.Format("01")
}

func parseMonth(value string) int {
	if len(value) == 1 {
		value = "0" + value
	}
	switch value {
	case "01":
		return 1
	case "02":
		return 2
	case "03":
		return 3
	case "04":
		return 4
	case "05":
		return 5
	case "06":
		return 6
	case "07":
		return 7
	case "08":
		return 8
	case "09":
		return 9
	case "10":
		return 10
	case "11":
		return 11
	case "12":
		return 12
	default:
		return 0
	}
}

func sanitizeFileName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	replacer := strings.NewReplacer(
		"\\", "_",
		"/", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return replacer.Replace(s)
}

func downloadMedia(media, path, name string) (string, error) {
	filePath, _, err := downloadMediaWithResult(media, path, name)
	return filePath, err
}

func downloadMediaWithResult(media, path, name string) (string, MediaResult, error) {
	err := os.MkdirAll(path, 0755)
	if err != nil {
		return "", MediaFailed, fmt.Errorf("failed to create dir: %v", err)
	}
	filePath := filepath.Join(path, name)
	if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
		return filePath, MediaSkipped, nil
	} else if err != nil && !os.IsNotExist(err) {
		return "", MediaFailed, fmt.Errorf("failed to stat media file: %v", err)
	}

	body, err := fetchMedia(media)
	if err != nil {
		return "", MediaFailed, fmt.Errorf("failed to get media: %v", err)
	}
	err = os.WriteFile(filePath, body, 0644)
	if err != nil {
		return "", MediaFailed, fmt.Errorf("failed to write file: %v", err)
	}
	return filePath, MediaDownloaded, nil
}
