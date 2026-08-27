package login

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"replive/config"
	"replive/model"
	"replive/rep_api"
	"runtime"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"
)

const (
	googleClientID        = "1046086565638-ndihf1c3r8hjli7l1robniulso85j5k5.apps.googleusercontent.com"
	googleCallbackScheme  = "com.googleusercontent.apps.1046086565638-ndihf1c3r8hjli7l1robniulso85j5k5"
	twitterCallbackScheme = "replive-user-auth"
	redirectURI           = googleCallbackScheme + ":/oauth2callback"
	defaultPort           = 53682
)

type Options struct {
	Proxy      string
	Port       int
	NoRegister bool
	Verbose    bool
	GuestToken string
}

type minimalConfig struct {
	Proxy struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"proxy"`
}

type googleTokenResponse struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

type guestTokenResponse struct {
	UserID     string
	GuestToken string
}

type debugLogger struct {
	enabled bool
}

func (l debugLogger) Printf(format string, args ...any) {
	if l.enabled {
		fmt.Printf("[debug] "+format+"\n", args...)
	}
}

func RunGoogleLogin(configPath string, options Options) error {
	if runtime.GOOS == "darwin" {
		message := "macOS 版本暂不支持自动接管 Google 登录回调，请先使用 Windows 或 Linux。"
		_ = openUnsupportedPage("Google 登录暂不支持", message)
		return errors.New(message)
	}

	port := options.Port
	if port == 0 {
		port = defaultPort
	}
	log := debugLogger{enabled: options.Verbose}
	client, err := newHTTPClient(configPath, options.Proxy, log)
	if err != nil {
		return err
	}

	callbackCh := make(chan string, 1)
	server, err := startCallbackServer(port, callbackCh, log)
	if err != nil {
		return err
	}
	defer server.Close()

	listenURL := fmt.Sprintf("http://127.0.0.1:%d/callback", port)
	if !options.NoRegister {
		if err := prepareCallbackHandoff(listenURL, googleCallbackScheme); err != nil {
			return err
		}
	}

	verifier, err := randomURLString(64)
	if err != nil {
		return err
	}
	state, err := randomURLString(32)
	if err != nil {
		return err
	}
	nonce, err := randomURLString(32)
	if err != nil {
		return err
	}
	authURL := buildAuthURL(state, nonce, codeChallenge(verifier))

	fmt.Println("Opening Google login in browser...")
	if err := openBrowser(authURL); err != nil {
		fmt.Println("Open this URL manually:")
		fmt.Println(authURL)
	}

	var callbackURL string
	select {
	case callbackURL = <-callbackCh:
		log.Printf("received callback: %s", redactURL(callbackURL))
	case <-time.After(5 * time.Minute):
		return errors.New("timed out waiting for Google callback")
	}

	code, err := parseAuthCode(callbackURL, state)
	if err != nil {
		return err
	}
	googleToken, err := exchangeGoogleCode(client, code, verifier, log)
	if err != nil {
		return err
	}
	if googleToken.IDToken == "" || googleToken.AccessToken == "" {
		return errors.New("Google token response did not include id_token/access_token")
	}
	logJWTClaims("Google id_token", googleToken.IDToken, log)

	guestToken := options.GuestToken
	if guestToken == "" {
		guestToken = os.Getenv("REPLIVE_GUEST_TOKEN")
	}
	if guestToken == "" {
		guest, err := signupAsGuest(client, log)
		if err != nil {
			return err
		}
		guestToken = guest.GuestToken
	}

	repliveToken, err := loginReplive(client, googleToken.IDToken, googleToken.AccessToken, guestToken, log)
	if err != nil {
		return err
	}
	if repliveToken.GetRefreshToken() == "" {
		if repliveToken.GetNeedSignup() {
			return errors.New("账号未注册，可能是你用的推特登录")
		}
		return errors.New("Replive login did not return refresh_token")
	}
	if err := config.UpdateRefreshToken(repliveToken.GetRefreshToken()); err != nil {
		return err
	}

	fmt.Printf("Login succeeded. %s refresh_token has been updated.\n", configPath)
	if expire := repliveToken.GetAccessTokenExpireTime(); expire != nil && expire.GetSeconds() > 0 {
		fmt.Println("Access token expires at:", time.Unix(expire.GetSeconds(), 0).Format("2006-01-02 15:04:05"))
	}
	return nil
}

func ForwardCallback(listenURL, callback string) error {
	if listenURL == "" {
		listenURL = fmt.Sprintf("http://127.0.0.1:%d/callback", defaultPort)
	}
	resp, err := http.Post(listenURL, "text/plain", strings.NewReader(callback))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("callback bridge returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func newHTTPClient(configPath, proxyFlag string, log debugLogger) (*http.Client, error) {
	proxyRaw := proxyFlag
	if proxyRaw == "" {
		data, err := os.ReadFile(configPath)
		if err == nil {
			var cfg minimalConfig
			if yaml.Unmarshal(data, &cfg) == nil && cfg.Proxy.Host != "" && cfg.Proxy.Port != 0 {
				proxyRaw = "http://" + cfg.Proxy.Host + ":" + strconv.Itoa(cfg.Proxy.Port)
			}
		}
	}
	transport := &http.Transport{}
	if proxyRaw != "" {
		log.Printf("using proxy %s", proxyRaw)
		proxyURL, err := url.Parse(proxyRaw)
		if err != nil {
			return nil, err
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	return &http.Client{Timeout: 60 * time.Second, Transport: transport}, nil
}

func startCallbackServer(port int, callbackCh chan<- string, log debugLogger) (*http.Server, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 8192))
		callback := strings.TrimSpace(string(body))
		if callback == "" {
			callback = r.URL.Query().Get("url")
		}
		if callback == "" {
			http.Error(w, "missing callback", http.StatusBadRequest)
			return
		}
		select {
		case callbackCh <- callback:
		default:
		}
		_, _ = w.Write([]byte("Replive login received. You can close this window."))
	})
	server := &http.Server{Addr: "127.0.0.1:" + strconv.Itoa(port), Handler: mux}
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return nil, err
	}
	log.Printf("callback bridge listening on http://127.0.0.1:%d/callback", port)
	go func() { _ = server.Serve(listener) }()
	return server, nil
}

func registerWindowsProtocol(listenURL, scheme string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, _ = filepath.Abs(exe)
	baseKey := `HKCU\Software\Classes\` + scheme
	cmd := fmt.Sprintf(`"%s" -callback "%%1" -listen "%s"`, exe, listenURL)
	commands := [][]string{
		{"add", baseKey, "/ve", "/d", "URL:" + scheme, "/f"},
		{"add", baseKey, "/v", "URL Protocol", "/d", "", "/f"},
		{"add", baseKey + `\shell\open\command`, "/ve", "/d", cmd, "/f"},
	}
	for _, args := range commands {
		out, err := exec.Command("reg", args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("reg %v failed: %v: %s", args, err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

func prepareCallbackHandoff(listenURL, scheme string) error {
	switch runtime.GOOS {
	case "windows":
		return registerWindowsProtocol(listenURL, scheme)
	case "linux":
		return registerLinuxProtocol(listenURL, scheme)
	case "darwin":
		return registerMacOSProtocol()
	default:
		return nil
	}
}

func registerLinuxProtocol(listenURL, scheme string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return err
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	desktopDir := filepath.Join(homeDir, ".local", "share", "applications")
	if err := os.MkdirAll(desktopDir, 0755); err != nil {
		return err
	}
	desktopFileName := "replive-oauth-" + strings.ReplaceAll(scheme, ".", "-") + ".desktop"
	desktopPath := filepath.Join(desktopDir, desktopFileName)
	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Replive OAuth Callback
Exec=%q -callback %%u -listen %q
Terminal=false
NoDisplay=true
MimeType=x-scheme-handler/%s;
`, exe, listenURL, scheme)
	if err := os.WriteFile(desktopPath, []byte(content), 0644); err != nil {
		return err
	}
	if _, err := exec.LookPath("xdg-mime"); err != nil {
		return fmt.Errorf("linux callback handler已写入 %s，但系统缺少 xdg-mime，请手动关联 x-scheme-handler/%s", desktopPath, scheme)
	}
	out, err := exec.Command("xdg-mime", "default", desktopFileName, "x-scheme-handler/"+scheme).CombinedOutput()
	if err != nil {
		return fmt.Errorf("xdg-mime failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func registerMacOSProtocol() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return err
	}
	if _, ok := findMacOSAppBundle(exe); !ok {
		return errors.New("macOS 暂不支持自动接管自定义登录回调；当前是裸二进制，无法捕获浏览器回跳 URL")
	}
	return nil
}

func findMacOSAppBundle(exePath string) (string, bool) {
	dir := filepath.Dir(exePath)
	for {
		if strings.HasSuffix(dir, ".app") {
			return dir, true
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
		dir = next
	}
	return "", false
}

func randomURLString(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func codeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func buildAuthURL(state, nonce, challenge string) string {
	baseURL := strings.TrimSpace(config.Conf.LoginPageURL)
	if baseURL == "" {
		baseURL = defaultGoogleAuthBaseURL
	}
	q := url.Values{}
	q.Set("client_id", googleClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", "openid email profile")
	q.Set("state", state)
	q.Set("nonce", nonce)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("include_granted_scopes", "true")
	q.Set("gpsdk", "gid-7.1.0")
	q.Set("gidenv", "ios")
	q.Set("device_os", "Windows")
	if parsed, err := url.Parse(baseURL); err == nil {
		existing := parsed.Query()
		for key, values := range q {
			for _, value := range values {
				existing.Set(key, value)
			}
		}
		parsed.RawQuery = existing.Encode()
		return parsed.String()
	}
	return baseURL + "?" + q.Encode()
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

func openUnsupportedPage(title, message string) error {
	path := filepath.Join(os.TempDir(), "replive_unsupported_google_login.html")
	body := fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>%s</title>
  <style>
    body {
      margin: 0;
      min-height: 100vh;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 24px;
      font-family: "PingFang SC", "Microsoft YaHei", sans-serif;
      background: linear-gradient(135deg, #f7f2e8 0%%, #e7eef7 100%%);
      color: #1f2937;
    }
    .card {
      width: min(620px, 100%%);
      background: rgba(255,255,255,0.94);
      border-radius: 24px;
      padding: 28px;
      box-shadow: 0 24px 80px rgba(15, 23, 42, 0.16);
    }
    h1 {
      margin: 0 0 14px;
      font-size: 28px;
    }
    p {
      margin: 0;
      color: #4b5563;
      line-height: 1.75;
    }
  </style>
</head>
<body>
  <div class="card">
    <h1>%s</h1>
    <p>%s</p>
  </div>
</body>
</html>`, htmlEscape(title), htmlEscape(title), htmlEscape(message))
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		return err
	}
	return openBrowser((&url.URL{Scheme: "file", Path: path}).String())
}

func htmlEscape(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(s)
}

func parseAuthCode(callbackURL, wantState string) (string, error) {
	u, err := url.Parse(callbackURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	if got := q.Get("state"); got != wantState {
		return "", errors.New("state mismatch")
	}
	if errCode := q.Get("error"); errCode != "" {
		return "", fmt.Errorf("Google returned error: %s %s", errCode, q.Get("error_description"))
	}
	code := q.Get("code")
	if code == "" {
		return "", errors.New("callback did not include code")
	}
	return code, nil
}

func exchangeGoogleCode(client *http.Client, code, verifier string, log debugLogger) (*googleTokenResponse, error) {
	form := url.Values{}
	form.Set("client_id", googleClientID)
	form.Set("code", code)
	form.Set("code_verifier", verifier)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", redirectURI)
	form.Set("gidenv", "ios")
	form.Set("gpsdk", "gid-7.1.0")
	form.Set("device_os", "Windows")
	req, err := http.NewRequest("POST", "https://oauth2.googleapis.com/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("User-Agent", "Replive/5601 CFNetwork/1406.0.4 Darwin/22.4.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := readMaybeGzip(resp)
	if err != nil {
		return nil, err
	}
	log.Printf("Google token response status=%s body_len=%d", resp.Status, len(body))
	var token googleTokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK || token.Error != "" {
		return nil, fmt.Errorf("Google token exchange failed: %s %s", token.Error, token.ErrorDesc)
	}
	return &token, nil
}

func loginReplive(client *http.Client, idToken, accessToken, guestToken string, log debugLogger) (*model.UserAuthBySNSResponse, error) {
	req := &model.UserAuthBySNSRequest{
		IdProvider:  model.IdProvider_ID_PROVIDER_GOOGLE,
		IdToken:     idToken,
		AccessToken: accessToken,
	}
	opts := rep_api.RequestOptions{
		SkipAuthorization: true,
		ExtraHeaders: map[string]string{
			"User-Agent": "v4.7.3 iPad11,3 iPadOS 16.4",
		},
	}
	if guestToken != "" {
		opts.ExtraHeaders["X-Replive-Guest-Token"] = guestToken
	}
	resp, err := rep_api.PostWithClient(client, "user.v1.UserService/UserAuthBySNS", req, opts)
	if err != nil {
		return nil, err
	}
	authResp := new(model.UserAuthBySNSResponse)
	if err := proto.Unmarshal(resp, authResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Replive auth response: %v", err)
	}
	log.Printf("Replive auth parsed: access_token_present=%v refresh_token_present=%v need_signup=%v", authResp.GetAccessToken() != "", authResp.GetRefreshToken() != "", authResp.GetNeedSignup())
	logJWTClaims("Replive access_token", authResp.GetAccessToken(), log)
	return authResp, nil
}

func signupAsGuest(client *http.Client, log debugLogger) (*guestTokenResponse, error) {
	body := make([]byte, 0, 32)
	body = appendBytesField(body, 1, []byte("JP"))
	body = appendBytesField(body, 2, []byte("en"))
	req, err := http.NewRequest("POST", rep_api.RepLiveHost+"user.v1.UserService/SignupAsGuest", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/proto")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Accept-Charset", "UTF-8")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "v4.7.3 iPad11,3 iPadOS 16.4")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := readMaybeGzip(resp)
	if err != nil {
		return nil, err
	}
	log.Printf("SignupAsGuest response status=%s body_len=%d", resp.Status, len(respBody))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SignupAsGuest failed: %s", resp.Status)
	}
	return parseGuestTokenResponse(respBody)
}

func readMaybeGzip(resp *http.Response) ([]byte, error) {
	var reader io.Reader = resp.Body
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		reader = gz
	}
	return io.ReadAll(reader)
}

func appendBytesField(dst []byte, field int, value []byte) []byte {
	dst = appendUvarint(dst, uint64(field<<3|2))
	dst = appendUvarint(dst, uint64(len(value)))
	return append(dst, value...)
}

func appendUvarint(dst []byte, x uint64) []byte {
	for x >= 0x80 {
		dst = append(dst, byte(x)|0x80)
		x >>= 7
	}
	return append(dst, byte(x))
}

func readUvarint(data []byte, i *int) (uint64, error) {
	var x uint64
	var s uint
	for ; *i < len(data); *i++ {
		b := data[*i]
		if b < 0x80 {
			if s >= 64 {
				return 0, errors.New("varint overflow")
			}
			*i++
			return x | uint64(b)<<s, nil
		}
		x |= uint64(b&0x7f) << s
		s += 7
	}
	return 0, io.ErrUnexpectedEOF
}

func parseGuestTokenResponse(data []byte) (*guestTokenResponse, error) {
	out := &guestTokenResponse{}
	for i := 0; i < len(data); {
		key, err := readUvarint(data, &i)
		if err != nil {
			return nil, err
		}
		field := int(key >> 3)
		wire := int(key & 7)
		if wire == 2 {
			l, err := readUvarint(data, &i)
			if err != nil {
				return nil, err
			}
			end := i + int(l)
			if end > len(data) {
				return nil, io.ErrUnexpectedEOF
			}
			value := string(data[i:end])
			i = end
			if field == 1 {
				out.UserID = value
			} else if field == 2 {
				out.GuestToken = value
			}
			continue
		}
		if wire == 0 {
			if _, err := readUvarint(data, &i); err != nil {
				return nil, err
			}
			continue
		}
		if wire == 1 {
			i += 8
			continue
		}
		if wire == 5 {
			i += 4
			continue
		}
		return nil, fmt.Errorf("unsupported protobuf wire type %d", wire)
	}
	if out.GuestToken == "" {
		return nil, errors.New("SignupAsGuest response did not include guest_token")
	}
	return out, nil
}

func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 18 {
		return s[:min(4, len(s))] + "..."
	}
	return s[:10] + "..." + s[len(s)-8:]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return redactText(raw)
	}
	q := u.Query()
	for _, key := range []string{"code", "state", "id_token", "access_token", "refresh_token", "oauth_token", "oauth_verifier"} {
		if q.Has(key) {
			q.Set(key, maskSecret(q.Get(key)))
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func redactText(s string) string {
	reJWT := regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)
	s = reJWT.ReplaceAllStringFunc(s, maskSecret)
	reBearer := regexp.MustCompile(`(?i)Bearer\s+[^\s,}]+`)
	s = reBearer.ReplaceAllStringFunc(s, func(v string) string { return "Bearer " + maskSecret(strings.TrimSpace(v[7:])) })
	if len(s) > 1200 {
		return s[:1200] + "..."
	}
	return s
}

func logJWTClaims(label, token string, log debugLogger) {
	if !log.enabled || token == "" || strings.Count(token, ".") != 2 {
		return
	}
	parts := strings.Split(token, ".")
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		log.Printf("%s JWT payload decode failed: %v", label, err)
		return
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		log.Printf("%s JWT JSON decode failed: %v", label, err)
		return
	}
	log.Printf("%s claims: aud=%v iss=%v sub=%v email=%v exp=%v iat=%v", label, claims["aud"], claims["iss"], claims["sub"], claims["email"], claims["exp"], claims["iat"])
}
