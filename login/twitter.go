package login

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"replive/config"
	"replive/model"
	"replive/rep_api"
	"runtime"
	"time"

	"google.golang.org/protobuf/proto"
)

func RunTwitterLogin(configPath string, options Options) error {
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
		if err := prepareCallbackHandoff(listenURL, twitterCallbackScheme); err != nil {
			if runtime.GOOS != "darwin" {
				return err
			}
			fmt.Printf("macOS cannot auto-register Twitter callback for this binary: %v\n", err)
			fmt.Printf("If the browser cannot open %s://, paste the full callback URL into: %s?url=<urlencoded_callback_url>\n", twitterCallbackScheme, listenURL)
		}
	}

	codeVerifier, err := randomURLString(32)
	if err != nil {
		return err
	}

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

	loginURL, err := getTwitterSNSLoginURL(client, codeChallenge(codeVerifier), guestToken, log)
	if err != nil {
		return err
	}

	fmt.Println("Opening Twitter/X login in browser...")
	if err := openBrowser(loginURL); err != nil {
		fmt.Println("Open this URL manually:")
		fmt.Println(loginURL)
	}

	var callbackURL string
	select {
	case callbackURL = <-callbackCh:
		log.Printf("received Twitter callback: %s", redactURL(callbackURL))
	case <-time.After(5 * time.Minute):
		return errors.New("timed out waiting for Twitter callback")
	}

	oauthToken, oauthVerifier, err := parseTwitterCallback(callbackURL)
	if err != nil {
		return err
	}

	repliveToken, err := loginRepliveTwitter(client, oauthToken, oauthVerifier, codeVerifier, guestToken, log)
	if err != nil {
		return err
	}
	if repliveToken.GetRefreshToken() == "" {
		if repliveToken.GetNeedSignup() {
			return errors.New("账号未注册，Twitter 登录返回 need_signup")
		}
		return errors.New("Replive Twitter login did not return refresh_token")
	}
	if err := config.UpdateRefreshToken(repliveToken.GetRefreshToken()); err != nil {
		return err
	}

	fmt.Printf("Twitter login succeeded. %s refresh_token has been updated.\n", configPath)
	if expire := repliveToken.GetAccessTokenExpireTime(); expire != nil && expire.GetSeconds() > 0 {
		fmt.Println("Access token expires at:", time.Unix(expire.GetSeconds(), 0).Format("2006-01-02 15:04:05"))
	}
	return nil
}

func getTwitterSNSLoginURL(client *http.Client, codeChallenge, guestToken string, log debugLogger) (string, error) {
	req := &model.GetSNSLoginURLRequest{
		IdProvider:    model.IdProvider_ID_PROVIDER_TWITTER,
		State:         "",
		CodeChallenge: codeChallenge,
	}
	body, err := marshalTwitterSNSLoginURLRequest(req)
	if err != nil {
		return "", err
	}
	resp, err := rep_api.PostRawWithClient(client, "user.v1.UserService/GetSNSLoginURL", body, snsRequestOptions(guestToken))
	if err != nil {
		return "", err
	}
	loginResp := new(model.GetSNSLoginURLResponse)
	if err := proto.Unmarshal(resp, loginResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal Replive SNS login URL response: %v", err)
	}
	if loginResp.GetLoginUrl() == "" {
		return "", errors.New("Replive SNS login URL response did not include login_url")
	}
	log.Printf("Twitter login URL received")
	return loginResp.GetLoginUrl(), nil
}

func marshalTwitterSNSLoginURLRequest(req *model.GetSNSLoginURLRequest) ([]byte, error) {
	body, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Replive SNS login URL request: %v", err)
	}
	// 抓包里 Twitter 登录 URL 请求会额外带 field 4=true；当前 proto 没有命名该字段。
	body = appendUvarint(body, uint64(4<<3|0))
	body = appendUvarint(body, 1)
	return body, nil
}

func parseTwitterCallback(callbackURL string) (string, string, error) {
	u, err := url.Parse(callbackURL)
	if err != nil {
		return "", "", err
	}
	if u.Scheme != twitterCallbackScheme || u.Host != "user-auth" {
		return "", "", fmt.Errorf("unexpected Twitter callback URL: %s", redactURL(callbackURL))
	}
	q := u.Query()
	oauthToken := q.Get("oauth_token")
	oauthVerifier := q.Get("oauth_verifier")
	if oauthToken == "" || oauthVerifier == "" {
		return "", "", errors.New("Twitter callback did not include oauth_token/oauth_verifier")
	}
	return oauthToken, oauthVerifier, nil
}

func loginRepliveTwitter(client *http.Client, oauthToken, oauthVerifier, codeVerifier, guestToken string, log debugLogger) (*model.UserAuthBySNSResponse, error) {
	req := &model.UserAuthBySNSRequest{
		IdProvider:    model.IdProvider_ID_PROVIDER_TWITTER,
		OauthToken:    oauthToken,
		OauthVerifier: oauthVerifier,
		CodeVerifier:  codeVerifier,
	}
	resp, err := rep_api.PostWithClient(client, "user.v1.UserService/UserAuthBySNS", req, snsRequestOptions(guestToken))
	if err != nil {
		return nil, err
	}
	authResp := new(model.UserAuthBySNSResponse)
	if err := proto.Unmarshal(resp, authResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Replive auth response: %v", err)
	}
	log.Printf("Replive Twitter auth parsed: access_token_present=%v refresh_token_present=%v need_signup=%v", authResp.GetAccessToken() != "", authResp.GetRefreshToken() != "", authResp.GetNeedSignup())
	logJWTClaims("Replive access_token", authResp.GetAccessToken(), log)
	return authResp, nil
}

func snsRequestOptions(guestToken string) rep_api.RequestOptions {
	opts := rep_api.RequestOptions{
		SkipAuthorization: true,
		ExtraHeaders: map[string]string{
			"User-Agent": "v4.7.3 iPad11,3 iPadOS 16.4",
		},
	}
	if guestToken != "" {
		opts.ExtraHeaders["X-Replive-Guest-Token"] = guestToken
	}
	return opts
}
