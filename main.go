package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"replive/config"
	"replive/dal"
	"replive/handler"
	"replive/login"
	"replive/rep_api"
	"replive/service"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type appOptions struct {
	ConfigPath string
}

func main() {
	os.Exit(runMain())
}

func runMain() (exitCode int) {
	callback := flag.String("callback", "", "internal callback URL from browser")
	listenURL := flag.String("listen", "", "internal listener URL")
	flag.Parse()

	pauseOnPanic := *callback == ""
	defer func() {
		if recovered := recover(); recovered != nil {
			logPanic(recovered)
			if pauseOnPanic {
				waitBeforeExit()
			}
			exitCode = 1
		}
	}()

	configPath := "config.yaml"

	if *callback != "" {
		if err := login.ForwardCallback(*listenURL, *callback); err != nil {
			panic(err)
		}
		return 0
	}

	f, err := os.Create(fmt.Sprintf("replive_%v.log", time.Now().Format("200601021504")))
	if err != nil {
		panic(err)
	}
	defer f.Close()

	hlog.SetOutput(io.MultiWriter(os.Stdout, f))
	hlog.SetLevel(hlog.LevelInfo)

	authFailureCh := make(chan error, 1)
	rep_api.SetAuthFailureHandler(func(err error) {
		select {
		case authFailureCh <- err:
		default:
		}
	})
	defer rep_api.SetAuthFailureHandler(nil)

	onlineMode := Init(appOptions{
		ConfigPath: configPath,
	})

	h := server.Default(server.WithHostPorts("127.0.0.1:8888"))

	if onlineMode {
		startupSummary, err := service.Init()
		if err != nil {
			rep_api.SetOnline(false)
			logOfflineMode(err)
		} else {
			service.LogStartupSummary(startupSummary)
			service.StartBackgroundWorkers()
		}
	}

	registerRoutes(h)
	go h.Spin()

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case err := <-authFailureCh:
			rep_api.SetOnline(false)
			logOfflineMode(err)
		case <-ticker.C:
			if rand.IntN(100) < 10 {
				hlog.Infof("正常运行中(:3_ヽ)_")
			}
		}
	}

}

func logPanic(recovered any) {
	stack := debug.Stack()
	message := fmt.Sprintf("panic: %v\n%s", recovered, stack)
	fmt.Fprintln(os.Stderr, message)
	hlog.Errorf("%s", message)
}

func waitBeforeExit() {
	if runtime.GOOS != "windows" {
		return
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "程序遇到错误已退出，panic 信息已写入 replive_*.log。按 Enter 关闭窗口...")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}

func Init(options appOptions) bool {
	if err := config.EnsureConfig(options.ConfigPath); err != nil {
		panic(err)
	}
	if err := config.LoadConfig(options.ConfigPath); err != nil {
		panic(err)
	}
	if err := ensureLoginSetup(options); err != nil {
		panic(err)
	}
	if err := ensureLoginReady(options); err != nil {
		panic(err)
	}
	if err := dal.InitDB(); err != nil {
		panic(err)
	}
	if err := service.BackfillProfileMediaPaths(); err != nil {
		hlog.Errorf("backfill local profile media paths failed: %v", err)
	}
	if err := initRepAPI(options); err != nil {
		rep_api.SetOnline(false)
		logOfflineMode(err)
		return false
	}
	if err := service.BackfillChatMediaPaths(); err != nil {
		hlog.Errorf("backfill local Fandom media paths failed: %v", err)
	}
	return true
}

func ensureLoginSetup(options appOptions) error {
	if !config.NeedsLoginSetup() {
		return nil
	}
	hlog.Infof("login config missing, opening setup page")
	if err := login.RunSetupWizard(options.ConfigPath); err != nil {
		return err
	}
	return config.LoadConfig(options.ConfigPath)
}

func ensureLoginReady(options appOptions) error {
	switch {
	case config.IsGoogleLoginProvider():
		if !config.NeedsInitialLogin() {
			return nil
		}
		hlog.Infof("refresh_token missing, starting Google login")
		return runGoogleLogin(options)
	case config.IsTwitterLoginProvider():
		if config.HasRefreshToken() {
			return nil
		}
		hlog.Infof("refresh_token missing, starting Twitter login")
		return runTwitterLogin(options)
	default:
		return fmt.Errorf("未识别的 login_provider: %s", config.Conf.LoginProvider)
	}
}

func initRepAPI(options appOptions) error {
	_ = options
	return rep_api.InitHttp()
}

func logOfflineMode(err error) {
	if rep_api.IsUnauthorizedError(err) {
		hlog.Errorf("无法连接 Replive 服务器：认证失效，refresh_token 可能已过期或无效。")
	} else {
		hlog.Errorf("无法连接 Replive 服务器，可能原因：本地网络异常、代理配置错误或服务器不可用。")
	}
	hlog.Infof("将以离线模式运行，仅浏览本地聊天记录。")
}

func runGoogleLogin(options appOptions) error {
	if err := login.RunGoogleLogin(options.ConfigPath, login.Options{}); err != nil {
		return err
	}
	return config.LoadConfig(options.ConfigPath)
}

func runTwitterLogin(options appOptions) error {
	if err := login.RunTwitterLogin(options.ConfigPath, login.Options{}); err != nil {
		return err
	}
	return config.LoadConfig(options.ConfigPath)
}

func registerRoutes(h *server.Hertz) {
	h.GET("/ping", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(consts.StatusOK, utils.H{"message": "pong"})
	})

	// Fandom 本地媒体访问：按消息 ID 映射，前端会在本地文件不可用时回退远程 URL。
	h.GET("/media/:file", handler.HandleGetChatMedia)
	h.GET("/profile-media", handler.HandleGetProfileMedia)

	chatGroup := h.Group("/api/chat")
	{
		chatGroup.GET("/rooms", handler.HandleGetChatRooms)
		chatGroup.GET("/messages", handler.HandleGetChatMessages)
		chatGroup.GET("/dates", handler.HandleGetChatDates)
		chatGroup.GET("/search", handler.HandleSearchChatMessages)
		chatGroup.POST("/send", handler.HandleSendChatMessage)
	}
	primeGroup := h.Group("/api/prime")
	{
		primeGroup.GET("/rooms", handler.HandleGetPrimeChatRooms)
		primeGroup.GET("/messages", handler.HandleGetPrimeChatMessages)
		primeGroup.GET("/dates", handler.HandleGetPrimeChatDates)
		primeGroup.GET("/search", handler.HandleSearchPrimeChatMessages)
	}
	h.GET("/api/user/me", handler.HandleGetCurrentUser)

}
