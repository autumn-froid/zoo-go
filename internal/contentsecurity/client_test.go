package contentsecurity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestWeChatClientChecksTextAndSubmitsImages(t *testing.T) {
	var tokenCalls atomic.Int32
	var imageCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/token":
			tokenCalls.Add(1)
			if r.URL.Query().Get("appid") != "wx-test" || r.URL.Query().Get("secret") != "secret-test" {
				t.Errorf("unexpected token query: %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "access-token", "expires_in": 7200,
			})
		case "/wxa/msg_sec_check":
			if r.URL.Query().Get("access_token") != "access-token" {
				t.Errorf("unexpected access token: %s", r.URL.RawQuery)
			}
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode text payload: %v", err)
			}
			if payload["content"] == "" || payload["openid"] != "openid-test" || payload["scene"] != float64(3) {
				t.Errorf("unexpected text payload: %+v", payload)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errcode":  0,
				"result":   map[string]any{"suggest": "review", "label": 20001},
				"trace_id": "text-trace",
			})
		case "/wxa/media_check_async":
			index := imageCalls.Add(1)
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode image payload: %v", err)
			}
			if !strings.HasPrefix(fmt.Sprint(payload["media_url"]), "https://api.example.com/uploads/") {
				t.Errorf("unexpected media url: %v", payload["media_url"])
			}
			if payload["media_type"] != float64(2) || payload["version"] != float64(2) || payload["openid"] != "openid-test" {
				t.Errorf("unexpected image payload: %+v", payload)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errcode": 0, "trace_id": fmt.Sprintf("image-trace-%d", index),
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewWeChatClient(Config{
		AppID: "wx-test", AppSecret: "secret-test", APIBaseURL: server.URL,
		Now: func() time.Time { return time.Unix(1_800_000_000, 0) },
	})
	if err != nil {
		t.Fatalf("NewWeChatClient() error = %v", err)
	}
	text, err := client.CheckText(context.Background(), "openid-test", "今天猫猫很乖")
	if err != nil {
		t.Fatalf("CheckText() error = %v", err)
	}
	if text.Suggest != SuggestReview || text.Label != 20001 || text.TraceID != "text-trace" {
		t.Fatalf("CheckText() = %+v", text)
	}
	if _, err := client.CheckText(context.Background(), "openid-test", "再次检查"); err != nil {
		t.Fatalf("second CheckText() error = %v", err)
	}
	if tokenCalls.Load() != 1 {
		t.Fatalf("token calls = %d, want 1", tokenCalls.Load())
	}
	images, err := client.SubmitImages(context.Background(), "openid-test", []string{
		"https://api.example.com/uploads/one.jpg?exp=1&sig=one",
		"https://api.example.com/uploads/two.jpg?exp=2&sig=two",
	})
	if err != nil {
		t.Fatalf("SubmitImages() error = %v", err)
	}
	if len(images) != 2 || images[0].TraceID != "image-trace-1" || images[1].TraceID != "image-trace-2" {
		t.Fatalf("SubmitImages() = %+v", images)
	}
}

func TestWeChatClientRejectsMissingCredentials(t *testing.T) {
	if _, err := NewWeChatClient(Config{}); err == nil {
		t.Fatal("NewWeChatClient() accepted empty credentials")
	}
}
