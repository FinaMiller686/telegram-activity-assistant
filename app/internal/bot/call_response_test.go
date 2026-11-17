package bot

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCallClassifiesMalformedReceiptsWithoutExposingResponse(t *testing.T) {
	for _, fixture := range []struct {
		name string
		body string
		want string
	}{
		{"accepted", `{"ok":true,"result":{"message_id":42}}`, ""},
		{"wrong result type", `{"ok":true,"result":{"message_id":"private-response"}}`, "bot_response_invalid"},
		{"missing result", `{"ok":true}`, "bot_response_invalid"},
		{"truncated envelope", `{"ok":true,"result":`, "bot_response_invalid"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(fixture.body))
			}))
			defer server.Close()
			var receipt struct {
				MessageID int64 `json:"message_id"`
			}
			err := newService(server.URL).Call(t.Context(), "fixture-token", "sendMessage", map[string]any{"text": "fixture"}, &receipt)
			if fixture.want == "" {
				if err != nil || receipt.MessageID != 42 {
					t.Fatalf("valid receipt rejected: %v", err)
				}
			} else if err == nil || err.Error() != fixture.want {
				t.Fatalf("got %v, want %s", err, fixture.want)
			}
		})
	}
}
