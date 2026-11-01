package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestListModelsSendsNoPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "Bearer fixture-key" || r.ContentLength > 0 {
			t.Error("unexpected model discovery request")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"model-b"},{"id":"model-a"},{"id":"model-a"},{"id":""}]}`))
	}))
	defer server.Close()
	got, err := New(Config{BaseURL: server.URL + "/v1", APIKey: "fixture-key"}).ListModels(context.Background())
	if err != nil || !reflect.DeepEqual(got, []string{"model-a", "model-b"}) {
		t.Fatalf("%v %v", got, err)
	}
}

func TestListModelsDoesNotExposeProviderErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "echoed-fixture-key", 401) }))
	defer server.Close()
	_, err := New(Config{BaseURL: server.URL, APIKey: "fixture-key"}).ListModels(context.Background())
	if err == nil || strings.Contains(err.Error(), "fixture-key") {
		t.Fatalf("unsafe error: %v", err)
	}
}
