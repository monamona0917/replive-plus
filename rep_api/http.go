package rep_api

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"replive/config"
	"replive/model"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	RepLiveHost = "https://api.replive.com/"
)

const maxConsecutiveRefreshUnauthorized = 30

type RequestOptions struct {
	SkipAuthorization bool
	ExtraHeaders      map[string]string
}

var (
	client       *http.Client
	accessToken  *model.RefreshAccessTokenResponse
	mutex        sync.Mutex
	refreshToken string

	authFailureMu                  sync.Mutex
	authFailureHandler             func(error)
	consecutiveRefreshUnauthorized int
	refreshUnauthorizedNotified    bool
)

var ErrUnauthorized = errors.New("unauthorized")

func SetAuthFailureHandler(handler func(error)) {
	authFailureMu.Lock()
	authFailureHandler = handler
	authFailureMu.Unlock()
}

func IsUnauthorizedError(err error) bool {
	return err != nil && (errors.Is(err, ErrUnauthorized) || strings.Contains(err.Error(), "status code: 401"))
}

func InitHttp() error {
	refreshToken = config.Conf.RefreshToken
	client = &http.Client{
		Timeout: 240 * time.Second,
	}
	if proxyURL := buildProxyURL(); proxyURL != nil {
		client.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
		hlog.Infof("use proxy: %s", proxyURL.Host)
	}
	accessToken = &model.RefreshAccessTokenResponse{
		AccessToken: "",
		AccessTokenExpireTime: &model.Timestamp{
			Seconds: 0,
		},
	}

	if _, err := getToken(); err != nil {
		return fmt.Errorf("get token err: %v", err)
	}
	return nil
}

func buildProxyURL() *url.URL {
	return config.ConfigProxyURL()
}

func getToken() (string, error) {
	mutex.Lock()
	defer mutex.Unlock()
	if accessToken != nil && accessToken.AccessTokenExpireTime != nil && accessToken.AccessTokenExpireTime.GetSeconds()-180 > time.Now().Unix() {
		return accessToken.GetAccessToken(), nil
	}
	req := &model.RefreshAccessTokenRequest{
		RefreshToken: refreshToken,
	}
	resp, err := Post("user.v1.UserService/RefreshAccessToken", req)
	if err != nil {
		recordRefreshTokenFailure(err)
		return "", fmt.Errorf("get token err: %v", err)
	}
	resetRefreshTokenFailures()
	tokenResp := new(model.RefreshAccessTokenResponse)
	if err := proto.Unmarshal(resp, tokenResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %v", err)
	}
	accessToken = tokenResp
	hlog.Infof("refresh access token, expire time: %s", time.Unix(accessToken.AccessTokenExpireTime.GetSeconds(), 0).Format("2006-01-02 15:04:05"))
	return accessToken.GetAccessToken(), nil
}

func recordRefreshTokenFailure(err error) {
	if !IsUnauthorizedError(err) {
		return
	}

	var handler func(error)
	var fatalErr error
	authFailureMu.Lock()
	consecutiveRefreshUnauthorized++
	if consecutiveRefreshUnauthorized >= maxConsecutiveRefreshUnauthorized && !refreshUnauthorizedNotified {
		refreshUnauthorizedNotified = true
		refreshToken = ""
		accessToken = &model.RefreshAccessTokenResponse{AccessToken: "", AccessTokenExpireTime: &model.Timestamp{Seconds: 0}}
		archivePath, archiveErr := config.ArchiveAndClearRefreshToken("repeated refresh 401")
		if archiveErr != nil {
			hlog.Errorf("failed to archive and clear refresh token after repeated 401: %v", archiveErr)
		} else {
			hlog.Errorf("archived invalid refresh token to %s", archivePath)
		}
		fatalErr = fmt.Errorf("refresh token unauthorized %d consecutive times; old refresh_token has been archived to %s and config refresh_token has been cleared, restart to login again", consecutiveRefreshUnauthorized, archivePath)
		handler = authFailureHandler
	}
	authFailureMu.Unlock()

	if fatalErr != nil {
		hlog.Errorf("%v", fatalErr)
		if handler != nil {
			handler(fatalErr)
		}
	}
}

func resetRefreshTokenFailures() {
	authFailureMu.Lock()
	consecutiveRefreshUnauthorized = 0
	refreshUnauthorizedNotified = false
	authFailureMu.Unlock()
}

func expireAccessToken() {
	mutex.Lock()
	accessToken = &model.RefreshAccessTokenResponse{AccessToken: "", AccessTokenExpireTime: &model.Timestamp{Seconds: 0}}
	mutex.Unlock()
}

func setHeaders(req *http.Request, opts RequestOptions) error {
	req.Header.Set("Host", RepLiveHost)
	req.Header.Set("Content-Type", "application/proto")
	req.Header.Set("accept-encoding", "gzip")
	req.Header.Set("accept-charset", "UTF-8")
	req.Header.Set("accept", "application/json")
	req.Header.Set("user-agent", "v3.1.1 23116PN5BC Android 12")
	if !opts.SkipAuthorization && !strings.Contains(req.URL.Path, "user.v1.UserService/RefreshAccessToken") {
		token, err := getToken()
		if err != nil {
			return fmt.Errorf("failed to get token: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		//req.Header.Set("x-rep_api-guest-token", token)
	}
	for key, value := range opts.ExtraHeaders {
		req.Header.Set(key, value)
	}
	return nil
}

func Get(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to do request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to do request, status code: %v, message: %v", resp.StatusCode, resp.Status)
	}
	if resp.StatusCode == http.StatusFound {
		location, err := resp.Location()
		if err != nil {
			return nil, fmt.Errorf("failed to get location: %v", err)
		}
		return Get(location.String())
	}
	if resp.Header.Get("Content-Encoding") == "gzip" {
		buf, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %v", err)
		}
		respBuf, err := io.ReadAll(buf)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %v", err)
		}
		return respBuf, nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}
	return body, nil
}

func GetReplive(uri string, params protoreflect.ProtoMessage) ([]byte, error) {
	buf, err := proto.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal proto: %v", err)
	}
	base64Str := base64.StdEncoding.EncodeToString(buf)
	url := fmt.Sprintf(RepLiveHost+uri+"?encoding=proto&base64=1&message=%v", base64Str)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	if err := setHeaders(req, RequestOptions{}); err != nil {
		return nil, fmt.Errorf("failed to set headers: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to do request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		if resp.StatusCode == http.StatusUnauthorized {
			expireAccessToken()
			return nil, fmt.Errorf("%w: failed to do request, status code: %v, message: %v", ErrUnauthorized, resp.StatusCode, resp.Status)
		}
		return nil, fmt.Errorf("failed to do request, status code: %v, message: %v", resp.StatusCode, resp.Status)
	}
	gzipReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}
	respBuf, err := io.ReadAll(gzipReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}
	return respBuf, nil
}

// GetRepliveRaw 发送 GET 请求，使用已 marshaled 的 protobuf 字节（无需 ProtoMessage 类型）。
// 用于手动编码 protobuf 的场景（如未定义 .proto 的临时 API）。
func GetRepliveRaw(uri string, rawData []byte) ([]byte, error) {
	base64Str := base64.StdEncoding.EncodeToString(rawData)
	url := fmt.Sprintf(RepLiveHost+uri+"?encoding=proto&base64=1&message=%v", base64Str)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	if err := setHeaders(req, RequestOptions{}); err != nil {
		return nil, fmt.Errorf("failed to set headers: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to do request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		if resp.StatusCode == http.StatusUnauthorized {
			expireAccessToken()
			return nil, fmt.Errorf("%w: failed to do request, status code: %v, message: %v", ErrUnauthorized, resp.StatusCode, resp.Status)
		}
		return nil, fmt.Errorf("failed to do request, status code: %v, message: %v", resp.StatusCode, resp.Status)
	}
	gzipReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}
	respBuf, err := io.ReadAll(gzipReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}
	return respBuf, nil
}

func Post(url string, body protoreflect.ProtoMessage) ([]byte, error) {
	return PostWithClient(client, url, body, RequestOptions{})
}

// PostReplive 发送 POST 请求，将 protobuf 以 base64 编码放在查询参数中（与 GetReplive 格式一致但使用 POST 方法）。
func PostReplive(uri string, params protoreflect.ProtoMessage) ([]byte, error) {
	buf, err := proto.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal proto: %v", err)
	}
	base64Str := base64.StdEncoding.EncodeToString(buf)
	url := fmt.Sprintf(RepLiveHost+uri+"?encoding=proto&base64=1&message=%v", base64Str)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	if err := setHeaders(req, RequestOptions{}); err != nil {
		return nil, fmt.Errorf("failed to set headers: %v", err)
	}
	// POST 无 body 时移除 Content-Type（空 body 不应设 Content-Type）
	req.Header.Del("Content-Type")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to do request: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应体（即使非 200，也用于调试）
	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		respBody = []byte(fmt.Sprintf("read body failed: %v", readErr))
	}

	if resp.StatusCode != 200 {
		if resp.StatusCode == http.StatusUnauthorized {
			expireAccessToken()
		}
		return nil, fmt.Errorf("failed to do request, status code: %v, message: %v, body: %s", resp.StatusCode, resp.Status, string(respBody))
	}

	gzipReader, err := gzip.NewReader(bytes.NewReader(respBody))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v, raw: %s", err, string(respBody))
	}
	respBuf, err := io.ReadAll(gzipReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}
	return respBuf, nil
}

// PostRawWithClient 直接发送 protobuf 二进制（无需有对应的 pb.go 类型）。
// 用于一次性/临时脚本场景。
func PostRaw(url string, body []byte) ([]byte, error) {
	return PostRawWithClient(client, url, body, RequestOptions{})
}

func PostRawWithClient(httpClient *http.Client, url string, body []byte, opts RequestOptions) ([]byte, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("http client is nil")
	}
	reader := bytes.NewReader(body)
	// 与 PostWithClient 一致：这里的 url 期望是 "user.v1.UserService/XXX"。
	req, err := http.NewRequest("POST", RepLiveHost+url, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	if err := setHeaders(req, opts); err != nil {
		return nil, fmt.Errorf("failed to set headers: %v", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		if resp.StatusCode == http.StatusUnauthorized {
			if !opts.SkipAuthorization && !strings.Contains(req.URL.Path, "user.v1.UserService/RefreshAccessToken") {
				expireAccessToken()
			}
			return nil, fmt.Errorf("%w: failed to send request, status code: %v, message: %v", ErrUnauthorized, resp.StatusCode, resp.Status)
		}
		return nil, fmt.Errorf("failed to send request, status code: %v, message: %v", resp.StatusCode, resp.Status)
	}

	gzipReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}
	respBuf, err := io.ReadAll(gzipReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}
	return respBuf, nil
}

func PostWithClient(httpClient *http.Client, url string, body protoreflect.ProtoMessage, opts RequestOptions) ([]byte, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("http client is nil")
	}
	buf, err := proto.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal body: %v", err)
	}
	reader := bytes.NewReader(buf)
	req, err := http.NewRequest("POST", RepLiveHost+url, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	err = setHeaders(req, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to set headers: %v", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		if resp.StatusCode == http.StatusUnauthorized {
			if !opts.SkipAuthorization && !strings.Contains(req.URL.Path, "user.v1.UserService/RefreshAccessToken") {
				expireAccessToken()
			}
			return nil, fmt.Errorf("%w: failed to send request, status code: %v, message: %v", ErrUnauthorized, resp.StatusCode, resp.Status)
		}
		return nil, fmt.Errorf("failed to send request, status code: %v, message: %v", resp.StatusCode, resp.Status)
	}
	gzipReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}
	respBuf, err := io.ReadAll(gzipReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}
	return respBuf, nil
}
