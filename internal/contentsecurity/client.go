package contentsecurity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	defaultAPIBaseURL = "https://api.weixin.qq.com"
	defaultScene      = 3
)

type Suggest string

const (
	SuggestPass    Suggest = "pass"
	SuggestReview  Suggest = "review"
	SuggestRisky   Suggest = "risky"
	SuggestUnknown Suggest = "unknown"
)

type TextResult struct {
	Suggest Suggest
	Label   int
	TraceID string
}

type ImageSubmission struct {
	TraceID string
	URL     string
}

type Client interface {
	CheckText(ctx context.Context, openID, content string) (TextResult, error)
	SubmitImages(ctx context.Context, openID string, imageURLs []string) ([]ImageSubmission, error)
}

type Config struct {
	AppID      string
	AppSecret  string
	APIBaseURL string
	HTTPClient *http.Client
	Scene      int
	Now        func() time.Time
}

type WeChatClient struct {
	appID      string
	appSecret  string
	apiBaseURL string
	httpClient *http.Client
	scene      int
	now        func() time.Time

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

func NewWeChatClient(config Config) (*WeChatClient, error) {
	appID := strings.TrimSpace(config.AppID)
	appSecret := strings.TrimSpace(config.AppSecret)
	if appID == "" || appSecret == "" {
		return nil, fmt.Errorf("wechat content security requires app id and app secret")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(config.APIBaseURL), "/")
	if baseURL == "" {
		baseURL = defaultAPIBaseURL
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 8 * time.Second}
	}
	scene := config.Scene
	if scene <= 0 {
		scene = defaultScene
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &WeChatClient{
		appID: appID, appSecret: appSecret, apiBaseURL: baseURL,
		httpClient: httpClient, scene: scene, now: now,
	}, nil
}

func (c *WeChatClient) CheckText(ctx context.Context, openID, content string) (TextResult, error) {
	payload := map[string]any{
		"content": content,
		"version": 2,
		"scene":   c.scene,
		"openid":  openID,
	}
	var response struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		TraceID string `json:"trace_id"`
		Result  struct {
			Suggest string `json:"suggest"`
			Label   int    `json:"label"`
		} `json:"result"`
	}
	if err := c.callAPI(ctx, "/wxa/msg_sec_check", payload, &response); err != nil {
		return TextResult{}, err
	}
	if response.ErrCode == 87014 {
		return TextResult{Suggest: SuggestRisky, TraceID: response.TraceID}, nil
	}
	if response.ErrCode != 0 {
		return TextResult{}, fmt.Errorf("wechat msg_sec_check: %d %s", response.ErrCode, response.ErrMsg)
	}
	return TextResult{
		Suggest: normalizeSuggest(response.Result.Suggest),
		Label:   response.Result.Label,
		TraceID: response.TraceID,
	}, nil
}

func (c *WeChatClient) SubmitImages(ctx context.Context, openID string, imageURLs []string) ([]ImageSubmission, error) {
	submissions := make([]ImageSubmission, 0, len(imageURLs))
	for _, imageURL := range imageURLs {
		payload := map[string]any{
			"media_url":  imageURL,
			"media_type": 2,
			"version":    2,
			"scene":      c.scene,
			"openid":     openID,
		}
		var response struct {
			ErrCode int    `json:"errcode"`
			ErrMsg  string `json:"errmsg"`
			TraceID string `json:"trace_id"`
		}
		if err := c.callAPI(ctx, "/wxa/media_check_async", payload, &response); err != nil {
			return nil, err
		}
		if response.ErrCode != 0 {
			return nil, fmt.Errorf("wechat media_check_async: %d %s", response.ErrCode, response.ErrMsg)
		}
		if strings.TrimSpace(response.TraceID) == "" {
			return nil, fmt.Errorf("wechat media_check_async returned an empty trace id")
		}
		submissions = append(submissions, ImageSubmission{TraceID: response.TraceID, URL: imageURL})
	}
	return submissions, nil
}

func (c *WeChatClient) callAPI(ctx context.Context, path string, payload, output any) error {
	for attempt := 0; attempt < 2; attempt++ {
		token, err := c.getAccessToken(ctx)
		if err != nil {
			return err
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		endpoint := c.apiBaseURL + path + "?access_token=" + url.QueryEscape(token)
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := c.httpClient.Do(request)
		if err != nil {
			return err
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		response.Body.Close()
		if readErr != nil {
			return readErr
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return fmt.Errorf("wechat content security returned HTTP %d", response.StatusCode)
		}
		var status struct {
			ErrCode int `json:"errcode"`
		}
		if err := json.Unmarshal(body, &status); err != nil {
			return fmt.Errorf("decode wechat content security response: %w", err)
		}
		if attempt == 0 && (status.ErrCode == 40001 || status.ErrCode == 42001 || status.ErrCode == 40014) {
			c.invalidateAccessToken(token)
			continue
		}
		if err := json.Unmarshal(body, output); err != nil {
			return fmt.Errorf("decode wechat content security payload: %w", err)
		}
		return nil
	}
	return fmt.Errorf("wechat content security access token was rejected")
}

func (c *WeChatClient) getAccessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if c.accessToken != "" && now.Before(c.tokenExpiry.Add(-time.Minute)) {
		return c.accessToken, nil
	}
	endpoint := c.apiBaseURL + "/cgi-bin/token?" + url.Values{
		"grant_type": {"client_credential"},
		"appid":      {c.appID},
		"secret":     {c.appSecret},
	}.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("wechat access token returned HTTP %d", response.StatusCode)
	}
	var payload struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode wechat access token: %w", err)
	}
	if payload.AccessToken == "" {
		return "", fmt.Errorf("wechat access token failed: %d %s", payload.ErrCode, payload.ErrMsg)
	}
	if payload.ExpiresIn <= 0 {
		payload.ExpiresIn = 7200
	}
	c.accessToken = payload.AccessToken
	c.tokenExpiry = now.Add(time.Duration(payload.ExpiresIn) * time.Second)
	return c.accessToken, nil
}

func (c *WeChatClient) invalidateAccessToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.accessToken == token {
		c.accessToken = ""
		c.tokenExpiry = time.Time{}
	}
}

func normalizeSuggest(value string) Suggest {
	switch Suggest(strings.ToLower(strings.TrimSpace(value))) {
	case SuggestPass:
		return SuggestPass
	case SuggestReview:
		return SuggestReview
	case SuggestRisky:
		return SuggestRisky
	default:
		return SuggestUnknown
	}
}
