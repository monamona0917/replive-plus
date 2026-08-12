package main

import (
	"bytes"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os/exec"
	"path"
	"runtime"
	"strings"
)

//go:embed dist
var distEmbedFS embed.FS

func main() {
	listenAddr := flag.String("listen", "127.0.0.1:5173", "listen address")
	backendURL := flag.String("backend", "http://127.0.0.1:8888", "backend base URL")
	noOpen := flag.Bool("no-open", false, "do not open browser after start")
	flag.Parse()

	distFS, err := fs.Sub(distEmbedFS, "dist")
	if err != nil {
		log.Fatalf("加载前端资源失败: %v", err)
	}

	backend, err := url.Parse(*backendURL)
	if err != nil {
		log.Fatalf("后端地址不正确: %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(backend)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/", proxy.ServeHTTP)
	mux.HandleFunc("/media/", proxy.ServeHTTP)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		serveFrontend(w, r, distFS)
	})

	pageURL := "http://" + *listenAddr + "/"
	if !*noOpen {
		go func() {
			if err := openBrowser(pageURL); err != nil {
				log.Printf("打开浏览器失败，请手动访问 %s: %v", pageURL, err)
			}
		}()
	}

	fmt.Printf("Replive+ Web 前端已启动: %s\n", pageURL)
	fmt.Printf("后端 API 代理: %s\n", backend.String())
	if err := http.ListenAndServe(*listenAddr, mux); err != nil {
		log.Fatal(err)
	}
}

func serveFrontend(w http.ResponseWriter, r *http.Request, distFS fs.FS) {
	filePath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if filePath == "." || filePath == "" {
		filePath = "index.html"
	}

	data, err := fs.ReadFile(distFS, filePath)
	if err != nil {
		filePath = "index.html"
		data, err = fs.ReadFile(distFS, filePath)
		if err != nil {
			http.Error(w, "前端资源不存在，请重新打包 replive_web.exe", http.StatusInternalServerError)
			return
		}
	}

	stat, err := fs.Stat(distFS, filePath)
	if err != nil {
		http.Error(w, "读取前端资源失败", http.StatusInternalServerError)
		return
	}

	if strings.HasPrefix(filePath, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	http.ServeContent(w, r, filePath, stat.ModTime(), bytes.NewReader(data))
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
