package config

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"replive/utils"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"gopkg.in/yaml.v3"
)

const (
	LoginProviderGoogle  = "google"
	LoginProviderTwitter = "twitter"
)

type Config struct {
	Proxy struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"proxy"`
	LoginProvider  string `yaml:"login_provider"`
	LoginPageURL   string `yaml:"login_page_url"`
	RefreshToken   string `yaml:"refresh_token"`
	FfmpegPath     string `yaml:"ffmpeg_path"`
	MediaPathWin   string `yaml:"media_path_win"`
	MediaPathLinux string `yaml:"media_path_linux,omitempty"`
	Email          struct {
		SmtpHost string `yaml:"smtp_host"`
		Sender   string `yaml:"sender"`
		AuthCode string `yaml:"auth_code"`
		Receiver string `yaml:"receiver"`
	} `yaml:"email"`
	// send_chat: false 关闭发送消息功能, true 开启
	SendChatEnabled bool `yaml:"send_chat"`
}

var (
	Conf       Config
	mediaPath  string
	configPath string
)

func DefaultConfig() Config {
	return Config{}
}

func EnsureConfig(path string) error {
	if path == "" {
		path = "config.yaml"
	}
	configPath = path
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("读取配置文件失败: %v", err)
	}
	Conf = DefaultConfig()
	return saveConfig(path, Conf)
}

func LoadConfig(path string) error {
	if path == "" {
		path = "config.yaml"
	}
	configPath = path
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %v", err)
	}
	if err := yaml.Unmarshal(data, &Conf); err != nil {
		return fmt.Errorf("解析YAML配置失败: %v", err)
	}
	if utils.IsWindows() {
		mediaPath = Conf.MediaPathWin
	} else {
		mediaPath = Conf.MediaPathLinux
	}
	if mediaPath == "" {
		mediaPath = "./media"
	}
	hlog.Infof("load config done, path: %s", mediaPath)
	return nil
}

// LoadConfigIfExists loads configuration for local-only consumers without
// creating a new config file. A missing config uses the default media folder.
func LoadConfigIfExists(path string) error {
	if path == "" {
		path = "config.yaml"
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			Conf = DefaultConfig()
			configPath = path
			mediaPath = "./media"
			return nil
		}
		return fmt.Errorf("读取配置文件失败: %v", err)
	}
	return LoadConfig(path)
}

func GetMediaPath() string {
	return mediaPath
}

func GetLivePath() string {
	return GetMediaPath() + "/live"
}

func GetLiveMonthPath(t time.Time) string {
	return filepath.Join(GetLivePath(), t.Format("2006"), t.Format("01"))
}

func ConfigProxyURL() *url.URL {
	proxyHost := strings.TrimSpace(Conf.Proxy.Host)
	if proxyHost == "" || proxyHost == "0" || Conf.Proxy.Port <= 0 {
		return nil
	}
	return &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", proxyHost, Conf.Proxy.Port),
	}
}

func ProxyFromEnvironmentOrConfig(req *http.Request) (*url.URL, error) {
	if proxyURL, err := http.ProxyFromEnvironment(req); err != nil || proxyURL != nil {
		return proxyURL, err
	}
	return ConfigProxyURL(), nil
}

func IsGoogleLoginProvider() bool {
	return strings.EqualFold(strings.TrimSpace(Conf.LoginProvider), LoginProviderGoogle)
}

func IsTwitterLoginProvider() bool {
	return strings.EqualFold(strings.TrimSpace(Conf.LoginProvider), LoginProviderTwitter)
}

func HasRefreshToken() bool {
	return strings.TrimSpace(Conf.RefreshToken) != ""
}

func NeedsLoginSetup() bool {
	return strings.TrimSpace(Conf.LoginProvider) == "" || strings.TrimSpace(Conf.LoginPageURL) == ""
}

func NeedsInitialLogin() bool {
	return IsGoogleLoginProvider() && strings.TrimSpace(Conf.RefreshToken) == ""
}

func UpdateLoginSettings(loginProvider, loginPageURL string) error {
	Conf.LoginProvider = strings.TrimSpace(loginProvider)
	Conf.LoginPageURL = strings.TrimSpace(loginPageURL)
	return saveCurrentConfig()
}

func UpdateRefreshToken(refreshToken string) error {
	Conf.RefreshToken = refreshToken
	return saveCurrentConfig()
}

func ArchiveAndClearRefreshToken(reason string) (string, error) {
	oldToken := strings.TrimSpace(Conf.RefreshToken)
	if oldToken == "" {
		return "", UpdateRefreshToken("")
	}
	if configPath == "" {
		configPath = "config.yaml"
	}
	archivePath := filepath.Join(filepath.Dir(configPath), "refresh_token.old")
	entry := fmt.Sprintf(
		"archived_at: %s\nlogin_provider: %s\nreason: %s\nrefresh_token: %s\n---\n",
		time.Now().Format(time.RFC3339),
		strings.TrimSpace(Conf.LoginProvider),
		strings.TrimSpace(reason),
		oldToken,
	)
	file, err := os.OpenFile(archivePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return "", err
	}
	if _, err := file.WriteString(entry); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	Conf.RefreshToken = ""
	return archivePath, saveCurrentConfig()
}

func saveCurrentConfig() error {
	if configPath == "" {
		configPath = "config.yaml"
	}
	return saveConfig(configPath, Conf)
}

func saveConfig(path string, cfg Config) error {
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}
	return nil
}
