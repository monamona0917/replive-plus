package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"replive/config"
	"replive/dal"
	"replive/handler"
	"replive/service"
	"runtime"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

//go:embed dist
var distEmbedFS embed.FS

func main() {
	listenAddr := flag.String("listen", "127.0.0.1:5173", "listen address")
	backendURL := flag.String("backend", "http://127.0.0.1:8888", "backend base URL")
	dataDirFlag := flag.String("data-dir", "", "force offline mode with a directory containing sqlite.db, config.yaml and media")
	noOpen := flag.Bool("no-open", false, "do not open browser after start")
	flag.Parse()
	hlog.SetLevel(hlog.LevelInfo)

	distFS, err := fs.Sub(distEmbedFS, "dist")
	if err != nil {
		log.Fatalf("加载前端资源失败: %v", err)
	}

	if strings.TrimSpace(*dataDirFlag) == "" && backendReachable(*backendURL) {
		runBackendProxy(*listenAddr, *backendURL, distFS, *noOpen)
		return
	}

	dataDir, err := resolveDataDir(*dataDirFlag)
	if err != nil {
		log.Fatalf("数据目录不可用: %v", err)
	}
	if err := os.Chdir(dataDir); err != nil {
		log.Fatalf("切换到数据目录失败: %v", err)
	}

	if err := config.LoadConfigIfExists(filepath.Join(dataDir, "config.yaml")); err != nil {
		log.Fatalf("加载本地配置失败: %v", err)
	}
	if err := dal.InitDBAt(filepath.Join(dataDir, "sqlite.db")); err != nil {
		log.Fatalf("加载本地聊天数据库失败: %v", err)
	}
	defer dal.CloseDB()
	if err := service.BackfillProfileMediaPaths(); err != nil {
		log.Printf("修复本地头像索引失败: %v", err)
	}

	h := server.Default(
		server.WithHostPorts(*listenAddr),
		server.WithExitWaitTime(time.Second),
	)
	registerOfflineRoutes(h, distFS)

	pageURL := "http://" + *listenAddr + "/"
	serverDone := make(chan struct{})
	go func() {
		h.Spin()
		close(serverDone)
	}()
	if !waitForHertzReady(h, pageURL, serverDone, 5*time.Second) {
		select {
		case <-serverDone:
			return
		default:
			log.Printf("本地 Web 服务启动较慢，请稍后手动访问 %s", pageURL)
		}
	}
	if !*noOpen {
		if err := openBrowser(pageURL); err != nil {
			log.Printf("打开浏览器失败，请手动访问 %s: %v", pageURL, err)
		}
	}

	fmt.Printf("Replive+ Web 前端已启动: %s\n", pageURL)
	fmt.Printf("离线浏览数据目录: %s\n", dataDir)
	<-serverDone
}

func backendReachable(rawURL string) bool {
	backend, err := url.Parse(rawURL)
	if err != nil || backend.Scheme == "" || backend.Host == "" {
		return false
	}
	pingURL := backend.ResolveReference(&url.URL{Path: "/ping"})
	response, err := (&http.Client{Timeout: time.Second}).Get(pingURL.String())
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.StatusCode == http.StatusOK
}

func runBackendProxy(listenAddr, rawBackendURL string, distFS fs.FS, noOpen bool) {
	backend, err := url.Parse(rawBackendURL)
	if err != nil {
		log.Fatalf("后端地址不正确: %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(backend)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/", proxy.ServeHTTP)
	mux.HandleFunc("/media/", proxy.ServeHTTP)
	mux.HandleFunc("/profile-media", proxy.ServeHTTP)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		serveFrontendHTTP(w, r, distFS)
	})

	pageURL := "http://" + listenAddr + "/"
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("启动本地 Web 服务失败: %v", err)
	}
	httpServer := &http.Server{Handler: mux}
	serverDone := make(chan error, 1)
	serverStopped := make(chan struct{})
	go func() {
		serverDone <- httpServer.Serve(listener)
		close(serverStopped)
	}()
	if !waitForHTTPReady(pageURL, serverStopped, 5*time.Second) {
		select {
		case err := <-serverDone:
			if err != http.ErrServerClosed {
				log.Fatalf("本地 Web 服务启动失败: %v", err)
			}
			return
		default:
			log.Printf("本地 Web 服务启动较慢，请稍后手动访问 %s", pageURL)
		}
	}
	if !noOpen {
		if err := openBrowser(pageURL); err != nil {
			log.Printf("打开浏览器失败，请手动访问 %s: %v", pageURL, err)
		}
	}

	fmt.Printf("Replive+ Web 前端已启动: %s\n", pageURL)
	fmt.Printf("后端 API 代理: %s\n", backend.String())
	if err := <-serverDone; err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func waitForHertzReady(h *server.Hertz, rawURL string, done <-chan struct{}, timeout time.Duration) bool {
	if !waitForListener(h, done, timeout) {
		return false
	}
	return waitForHTTPReady(rawURL, done, timeout)
}

func waitForListener(h *server.Hertz, done <-chan struct{}, timeout time.Duration) bool {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if h.IsRunning() {
			return true
		}
		select {
		case <-done:
			return false
		case <-ticker.C:
		case <-deadline.C:
			return false
		}
	}
}

func waitForHTTPReady(rawURL string, done <-chan struct{}, timeout time.Duration) bool {
	client := &http.Client{Timeout: 200 * time.Millisecond}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		response, err := client.Get(rawURL)
		if err == nil {
			_ = response.Body.Close()
			return true
		}
		select {
		case <-done:
			return false
		case <-ticker.C:
		case <-deadline.C:
			return false
		}
	}
}

func resolveDataDir(raw string) (string, error) {
	if strings.TrimSpace(raw) != "" {
		dataDir, err := filepath.Abs(raw)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(dataDir)
		if err != nil {
			return "", err
		}
		if !info.IsDir() {
			return "", fmt.Errorf("%s 不是目录", dataDir)
		}
		if !pathExists(filepath.Join(dataDir, "sqlite.db")) {
			return "", fmt.Errorf("%s 中未找到 sqlite.db", dataDir)
		}
		return dataDir, nil
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if pathExists(filepath.Join(workingDir, "sqlite.db")) {
		return workingDir, nil
	}

	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		if pathExists(filepath.Join(exeDir, "sqlite.db")) {
			return exeDir, nil
		}
	}
	return "", fmt.Errorf("未找到 sqlite.db；请在数据目录中启动，或使用 --data-dir 指定目录")
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func registerOfflineRoutes(h *server.Hertz, distFS fs.FS) {
	h.GET("/ping", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(consts.StatusOK, utils.H{"message": "pong", "offline": true})
	})

	// These are the same local database handlers used by the full backend.
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

	frontendHandler := func(ctx context.Context, c *app.RequestContext) {
		serveFrontend(c, distFS)
	}
	h.GET("/", frontendHandler)
	h.GET("/*path", frontendHandler)
}

func serveFrontend(c *app.RequestContext, distFS fs.FS) {
	requestPath := c.Param("path")
	if requestPath == "" {
		requestPath = string(c.Path())
	}
	filePath := strings.TrimPrefix(path.Clean("/"+requestPath), "/")
	if filePath == "." || filePath == "" {
		filePath = "index.html"
	}

	data, err := fs.ReadFile(distFS, filePath)
	if err != nil {
		filePath = "index.html"
		data, err = fs.ReadFile(distFS, filePath)
		if err != nil {
			c.SetStatusCode(consts.StatusInternalServerError)
			c.SetContentType("text/plain; charset=utf-8")
			c.SetBodyString("前端资源不存在，请重新打包 replive-plus-web.exe")
			return
		}
	}

	contentType := mime.TypeByExtension(filepath.Ext(filePath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Response.Header.Set("Cache-Control", "no-cache")
	if strings.HasPrefix(filePath, "assets/") {
		c.Response.Header.Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	c.Data(consts.StatusOK, contentType, data)
}

func serveFrontendHTTP(w http.ResponseWriter, r *http.Request, distFS fs.FS) {
	filePath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if filePath == "." || filePath == "" {
		filePath = "index.html"
	}

	data, err := fs.ReadFile(distFS, filePath)
	if err != nil {
		filePath = "index.html"
		data, err = fs.ReadFile(distFS, filePath)
		if err != nil {
			http.Error(w, "前端资源不存在，请重新打包 replive-plus-web.exe", http.StatusInternalServerError)
			return
		}
	}

	contentType := mime.TypeByExtension(filepath.Ext(filePath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	if strings.HasPrefix(filePath, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	_, _ = w.Write(data)
}

func openBrowser(rawURL string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL).Start()
	case "darwin":
		return exec.Command("open", rawURL).Start()
	default:
		return exec.Command("xdg-open", rawURL).Start()
	}
}
