package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTongjiUsesDocumentedTokenLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["max_tokens"] != float64(256) || payload["max_completion_tokens"] != nil {
			t.Error("wrong token parameter")
		}
		_, _ = w.Write([]byte(`{"model":"fixture","choices":[{"message":{"content":"OK"}}]}`))
	}))
	defer server.Close()
	client := New(Config{BaseURL: "https://agi.tongji.edu.cn/v1", Model: "fixture"})
	client.baseURL = server.URL
	if _, err := client.Chat(context.Background(), ChatRequest{UserPrompt: "fixture", MaxOutput: 256}); err != nil {
		t.Fatal(err)
	}
}

func TestNumericProviderCodeDoesNotHideMessage(t *testing.T) {
	err := decodeAPIError(403, []byte(`{"error":{"code":403,"message":"当前网络环境不支持"}}`))
	if err.Code != "403" || err.Message != "当前网络环境不支持" {
		t.Fatalf("bad provider error: %#v", err)
	}
}

func TestOnlyCampusClientUsesDirectHostRoute(t *testing.T) {
	campus := New(Config{BaseURL: "https://agi.tongji.edu.cn/v1"})
	transport, ok := campus.httpClient.Transport.(*http.Transport)
	if !ok || transport.Proxy != nil || transport.TLSClientConfig != nil && transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("campus requests must follow the host route while keeping certificate verification")
	}
	other := New(Config{BaseURL: "https://other.example/v1"})
	if other.httpClient.Transport != nil {
		t.Fatal("unrelated provider proxy behavior changed")
	}
}
