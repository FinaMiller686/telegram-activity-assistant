package api

import (
	"github.com/fr0der1c/tgtldr/app/internal/model"
	"testing"
)

func TestModelEndpointCredentialScope(t *testing.T) {
	current := model.AppSettings{OpenAIBaseURL: "https://original.example/v1", OpenAIAPIKey: "original-key"}
	for _, raw := range []string{"https://different.example/v1", "http://external.example/v1", "https://name:password@example.org/v1", "https://example.org/v1?key=unsafe"} {
		if _, err := prepareModelEndpoint(model.AppSettings{OpenAIBaseURL: raw}, current); err == nil {
			t.Fatalf("accepted unsafe endpoint or credential reuse: %q", raw)
		}
	}
	got, err := prepareModelEndpoint(model.AppSettings{OpenAIBaseURL: "https://original.example/v1/"}, current)
	if err != nil || got.OpenAIAPIKey != "original-key" {
		t.Fatalf("same provider: %v", err)
	}
	got, err = prepareModelEndpoint(model.AppSettings{OpenAIBaseURL: "https://different.example/v1", OpenAIAPIKey: "new-key"}, current)
	if err != nil || got.OpenAIAPIKey != "new-key" {
		t.Fatalf("explicit new credential: %v", err)
	}
}
