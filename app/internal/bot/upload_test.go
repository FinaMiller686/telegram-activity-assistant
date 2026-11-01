package bot

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCallUploadsCalendarAsMultipart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Error("file was not uploaded as multipart")
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		file, header, err := r.FormFile("document")
		if err != nil {
			t.Error(err)
			return
		}
		defer file.Close()
		data, _ := io.ReadAll(file)
		if header.Filename != "activity-12.ics" || header.Header.Get("Content-Type") != "text/calendar; charset=utf-8" || !strings.Contains(string(data), "BEGIN:VCALENDAR") {
			t.Error("calendar attachment changed")
		}
		if r.FormValue("chat_id") != "42" || r.FormValue("disable_content_type_detection") != "true" {
			t.Error("wrong recipient or type handling")
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":99}}`))
	}))
	defer server.Close()
	s := newService(server.URL)
	err := s.Call(context.Background(), "fixture-token", "sendDocument", map[string]any{"chat_id": int64(42), "disable_content_type_detection": true, "document": Upload{Filename: "activity-12.ics", ContentType: "text/calendar; charset=utf-8", Data: []byte("BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n")}}, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestUploadErrorDoesNotExposeTokenAndDoesNotRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "http://127.0.0.1:1/fixture-token")
		w.WriteHeader(302)
		_, _ = w.Write([]byte(`{"ok":false}`))
	}))
	defer server.Close()
	err := newService(server.URL).Call(context.Background(), "fixture-token", "sendMessage", map[string]any{"chat_id": 42, "text": "fixture"}, nil)
	if err == nil || strings.Contains(err.Error(), "fixture-token") || strings.Contains(err.Error(), "127.0.0.1") {
		t.Fatal("unsafe delivery error")
	}
}
