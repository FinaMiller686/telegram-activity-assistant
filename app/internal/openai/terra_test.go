package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTerraRequestCompatibility(t *testing.T) {
	for _, effort := range []string{"", "none"} {
		t.Run("effort_"+effort, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				want := effort
				if want == "" {
					want = "low"
				}
				if payload["reasoning_effort"] != want {
					t.Errorf("effort = %v", payload["reasoning_effort"])
				}
				if effort != "none" && payload["temperature"] != nil {
					t.Error("reasoning request contains temperature")
				}
				if payload["max_completion_tokens"] != float64(4096) || payload["max_tokens"] != nil {
					t.Error("incorrect token limit")
				}
				_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"OK"}}]}`))
			}))
			defer server.Close()
			c := New(Config{BaseURL: server.URL, Model: "gpt-6.1-terra"})
			if _, err := c.Chat(context.Background(), ChatRequest{Temperature: 0.2, MaxOutput: 4096, ReasoningEffort: effort}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
