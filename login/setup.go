package login

import (
	"fmt"
	"html/template"
	"net"
	"net/http"
	"replive/config"
	"strings"
)

const (
	defaultGoogleAuthBaseURL = "https://accounts.google.com/o/oauth2/v2/auth"
	defaultTwitterLoginURL   = "https://x.com/"
)

type setupResult struct {
	Provider string
	PageURL  string
}

type setupPageData struct {
	Provider     string
	LoginPageURL string
	ErrorMessage string
}

const setupPageHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Replive 首次配置</title>
  <style>
    :root {
      color-scheme: light;
      --bg: linear-gradient(135deg, #f7f2e8 0%, #e7eef7 100%);
      --card: rgba(255, 255, 255, 0.92);
      --text: #1f2937;
      --muted: #6b7280;
      --line: #d1d9e6;
      --accent: #0f766e;
      --accent-2: #0b5ed7;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      font-family: "PingFang SC", "Microsoft YaHei", sans-serif;
      background: var(--bg);
      color: var(--text);
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 24px;
    }
    .card {
      width: min(680px, 100%);
      background: var(--card);
      border: 1px solid rgba(255,255,255,0.5);
      border-radius: 24px;
      box-shadow: 0 24px 80px rgba(15, 23, 42, 0.16);
      padding: 28px;
      backdrop-filter: blur(14px);
    }
    h1 {
      margin: 0 0 12px;
      font-size: 28px;
    }
    p {
      margin: 0 0 14px;
      color: var(--muted);
      line-height: 1.65;
    }
    .section {
      margin-top: 22px;
    }
    .label {
      display: block;
      margin-bottom: 10px;
      font-weight: 700;
    }
    .choices {
      display: grid;
      gap: 12px;
    }
    .choice {
      display: flex;
      gap: 10px;
      align-items: flex-start;
      padding: 14px 16px;
      border: 1px solid var(--line);
      border-radius: 16px;
      background: rgba(255,255,255,0.75);
    }
    .choice strong {
      display: block;
      margin-bottom: 4px;
    }
    input[type="url"] {
      width: 100%;
      padding: 14px 16px;
      font-size: 15px;
      border-radius: 14px;
      border: 1px solid var(--line);
      outline: none;
    }
    input[type="url"]:focus {
      border-color: var(--accent-2);
      box-shadow: 0 0 0 4px rgba(11, 94, 215, 0.12);
    }
    .hint {
      margin-top: 10px;
      font-size: 13px;
    }
    .error {
      margin-top: 18px;
      padding: 12px 14px;
      border-radius: 14px;
      background: #fef2f2;
      color: #b91c1c;
      border: 1px solid #fecaca;
    }
    button {
      margin-top: 24px;
      width: 100%;
      padding: 15px 18px;
      border: 0;
      border-radius: 16px;
      background: linear-gradient(135deg, var(--accent) 0%, var(--accent-2) 100%);
      color: white;
      font-size: 16px;
      font-weight: 700;
      cursor: pointer;
    }
    .footer {
      margin-top: 16px;
      font-size: 13px;
    }
  </style>
  <script>
    function applyDefaultURL() {
      const google = document.getElementById('provider_google');
      const input = document.getElementById('login_page_url');
      if (!input) return;
      if (google && google.checked && !input.dataset.touched) {
        input.value = '__GOOGLE_LOGIN_URL__';
        return;
      }
      if (!google.checked && !input.dataset.touched) {
        input.value = '__TWITTER_LOGIN_URL__';
      }
    }
    function markTouched() {
      const input = document.getElementById('login_page_url');
      if (input) input.dataset.touched = '1';
    }
    window.addEventListener('DOMContentLoaded', function() {
      const input = document.getElementById('login_page_url');
      if (input && input.value) input.dataset.touched = '1';
      applyDefaultURL();
    });
  </script>
</head>
<body>
  <form class="card" method="post" action="/save">
    <h1>Replive 首次配置</h1>
    <p>这个程序会把登录方式和登录页地址写入本地配置文件。后续直接双击 exe 即可，不需要再手动输命令。</p>
    <p>如果你现在只打算先接通 Google，保持默认值即可。</p>

    <div class="section">
      <span class="label">1. 选择登录方式</span>
      <div class="choices">
        <label class="choice">
          <input id="provider_google" type="radio" name="login_provider" value="google" {{if eq .Provider "google"}}checked{{end}} onclick="applyDefaultURL()">
          <span>
            <strong>Google 登录</strong>
            <span>当前已接入自动登录。首次配置完成后，会继续自动拉起 Google 授权页。</span>
          </span>
        </label>
        <label class="choice">
          <input type="radio" name="login_provider" value="twitter" {{if eq .Provider "twitter"}}checked{{end}} onclick="applyDefaultURL()">
          <span>
            <strong>Twitter 登录</strong>
            <span>保存配置后，会继续自动拉起 Twitter/X 授权页。</span>
          </span>
        </label>
      </div>
    </div>

    <div class="section">
      <label class="label" for="login_page_url">2. 登录页地址</label>
      <input id="login_page_url" name="login_page_url" type="url" value="{{.LoginPageURL}}" placeholder="https://accounts.google.com/o/oauth2/v2/auth" oninput="markTouched()">
      <p class="hint">Google 默认使用官方 OAuth 授权页；Twitter 会通过 Replive 接口生成实际授权地址，这里保存默认入口即可。</p>
    </div>

    {{if .ErrorMessage}}
    <div class="error">{{.ErrorMessage}}</div>
    {{end}}

    <button type="submit">保存并继续</button>
    <p class="footer">保存后程序会继续初始化。如果还没有 refresh_token，下一步会自动打开对应授权页。</p>
  </form>
</body>
</html>`

var setupPageTmpl = template.Must(template.New("setup").Parse(strings.NewReplacer(
	"__GOOGLE_LOGIN_URL__", defaultGoogleAuthBaseURL,
	"__TWITTER_LOGIN_URL__", defaultTwitterLoginURL,
).Replace(setupPageHTML)))

var setupDoneTmpl = template.Must(template.New("done").Parse(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>配置已保存</title>
  <style>
    body {
      margin: 0;
      min-height: 100vh;
      display: flex;
      align-items: center;
      justify-content: center;
      font-family: "PingFang SC", "Microsoft YaHei", sans-serif;
      background: linear-gradient(135deg, #f7f2e8 0%, #e7eef7 100%);
      color: #1f2937;
      padding: 24px;
    }
    .card {
      width: min(560px, 100%);
      background: rgba(255,255,255,0.92);
      border-radius: 24px;
      padding: 28px;
      box-shadow: 0 24px 80px rgba(15, 23, 42, 0.16);
    }
    h1 { margin-top: 0; }
    p { line-height: 1.7; color: #4b5563; }
  </style>
</head>
<body>
  <div class="card">
    <h1>配置已保存</h1>
    <p>当前登录方式：{{.Provider}}</p>
    <p>当前登录页地址：{{.PageURL}}</p>
    <p>这个页面现在可以关闭了，程序会继续执行后续初始化。</p>
  </div>
</body>
</html>`))

func RunSetupWizard(configPath string) error {
	_ = configPath
	resultCh := make(chan setupResult, 1)
	errCh := make(chan error, 1)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close()

	addr := listener.Addr().String()

	mux := http.NewServeMux()
	renderPage := func(w http.ResponseWriter, data setupPageData) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = setupPageTmpl.Execute(w, data)
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, setupPageData{
			Provider:     normalizedProvider(),
			LoginPageURL: strings.TrimSpace(config.Conf.LoginPageURL),
		})
	})

	mux.HandleFunc("/save", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			renderPage(w, setupPageData{ErrorMessage: "表单解析失败，请重试。"})
			return
		}

		provider := strings.TrimSpace(r.FormValue("login_provider"))
		pageURL := strings.TrimSpace(r.FormValue("login_page_url"))
		if provider == "" {
			renderPage(w, setupPageData{
				Provider:     provider,
				LoginPageURL: pageURL,
				ErrorMessage: "请选择登录方式。",
			})
			return
		}
		if pageURL == "" {
			pageURL = defaultPageURLForProvider(provider)
		}
		if !isSupportedProvider(provider) {
			renderPage(w, setupPageData{
				Provider:     provider,
				LoginPageURL: pageURL,
				ErrorMessage: "暂不支持该登录方式。",
			})
			return
		}
		if _, err := http.NewRequest(http.MethodGet, pageURL, nil); err != nil {
			renderPage(w, setupPageData{
				Provider:     provider,
				LoginPageURL: pageURL,
				ErrorMessage: "登录页地址格式不正确。",
			})
			return
		}
		if err := config.UpdateLoginSettings(provider, pageURL); err != nil {
			select {
			case errCh <- err:
			default:
			}
			renderPage(w, setupPageData{
				Provider:     provider,
				LoginPageURL: pageURL,
				ErrorMessage: "保存配置失败，请检查文件权限。",
			})
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = setupDoneTmpl.Execute(w, setupResult{Provider: provider, PageURL: pageURL})
		select {
		case resultCh <- setupResult{Provider: provider, PageURL: pageURL}:
		default:
		}
	})

	server := &http.Server{Addr: addr, Handler: mux}
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && serveErr != http.ErrServerClosed {
			select {
			case errCh <- serveErr:
			default:
			}
		}
	}()
	defer server.Close()

	setupURL := fmt.Sprintf("http://%s/", addr)
	if err := openBrowser(setupURL); err != nil {
		return fmt.Errorf("打开初始化页面失败: %v", err)
	}

	select {
	case err := <-errCh:
		return err
	case <-resultCh:
		return nil
	}
}

func normalizedProvider() string {
	if config.IsTwitterLoginProvider() {
		return config.LoginProviderTwitter
	}
	return config.LoginProviderGoogle
}

func defaultPageURLForProvider(provider string) string {
	if strings.EqualFold(strings.TrimSpace(provider), config.LoginProviderTwitter) {
		return defaultTwitterLoginURL
	}
	return defaultGoogleAuthBaseURL
}

func isSupportedProvider(provider string) bool {
	provider = strings.TrimSpace(strings.ToLower(provider))
	return provider == config.LoginProviderGoogle || provider == config.LoginProviderTwitter
}
