package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestUnencryptedStorageBlocksNetworkAndCollection(t *testing.T) {
	t.Setenv("TGTLDR_STORAGE_PROTECTED", "")
	router := (&Router{origin: "http://localhost:3000", timeout: time.Second}).withMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("unsafe action reached handler") }))
	for _, path := range []string{"/api/telegram/auth/start", "/api/telegram/chats/sync", "/api/telegram/accounts/1/sync", "/api/bot/target-chat/resolve", "/api/chats/1", "/api/history-backfills"} {
		req := httptest.NewRequest("POST", path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		out := httptest.NewRecorder()
		router.ServeHTTP(out, req)
		if out.Code != 403 || !strings.Contains(out.Body.String(), "storage_encryption_required") {
			t.Fatalf("%s: %d %s", path, out.Code, out.Body.String())
		}
	}
}
func TestOriginMustMatchConfiguredPort(t *testing.T) {
	if allowedOrigin("http://localhost:3000", "http://localhost:13000") != "" {
		t.Fatal("accepted an unrelated local origin")
	}
	if allowedOrigin("http://127.0.0.1:13000", "http://localhost:13000") == "" {
		t.Fatal("local alias rejected")
	}
}
