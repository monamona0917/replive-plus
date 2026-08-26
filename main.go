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
	repliveUtils "replive/utils"
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
	repliveUtils.UseJapanLocalTime()

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

	Init(appOptions{
		ConfigPath: configPath,
	})

	h := server.Default(server.WithHostPorts("127.0.0.1:8888"))

	startupSummary := service.Init()
	service.LogStartupSummary(startupSummary)
	service.StartBackgroundWorkers()

	registerRoutes(h)
	go h.Spin()

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case err := <-authFailureCh:
			panic(fmt.Errorf("认证已失效：%v。已清空本地 refresh_token，请重新打开程序完成登录", err))
		case <-ticker.C:
			if rand.IntN(100) < 2 {
				hlog.Infof("listening...")
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

func Init(options appOptions) {
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
	if err := initRepAPI(options); err != nil {
		panic(err)
	}
	if err := dal.InitDB(); err != nil {
		panic(err)
	}
	if err := service.BackfillChatMediaPaths(); err != nil {
		hlog.Errorf("backfill local Fandom media paths failed: %v", err)
	}
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
	if err := rep_api.InitHttp(); err != nil {
		if config.IsGoogleLoginProvider() {
			hlog.Warnf("rep_api init failed, retrying Google login: %v", err)
			if loginErr := runGoogleLogin(options); loginErr != nil {
				return fmt.Errorf("rep_api init failed: %v; google login failed: %v", err, loginErr)
			}
			return rep_api.InitHttp()
		}
		if config.IsTwitterLoginProvider() {
			if !rep_api.IsUnauthorizedError(err) {
				return err
			}
			hlog.Warnf("rep_api init failed, retrying Twitter login: %v", err)
			if loginErr := runTwitterLogin(options); loginErr != nil {
				return fmt.Errorf("rep_api init failed: %v; twitter login failed: %v", err, loginErr)
			}
			return rep_api.InitHttp()
		}
		return err
	}
	return nil
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
