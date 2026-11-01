package telegram

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	dialogsquery "github.com/gotd/td/telegram/query/dialogs"
	"github.com/gotd/td/tg"
)

func TestTextExportExcludesDirectPeers(t *testing.T) {
	for _, peer := range []tg.InputPeerClass{&tg.InputPeerUser{UserID: 42, AccessHash: 1}, &tg.InputPeerSelf{}} {
		if _, ok := dialogToChat(dialogsquery.Elem{Peer: peer}); ok {
			t.Fatal("private peer was eligible")
		}
	}
}

func TestTextExportPreservesHiddenLinksAndSafeHTML(t *testing.T) {
	out := t.TempDir()
	chat := model.Chat{Title: "Fixture Group", ChatType: "supergroup", TelegramChatID: 55, Username: "fixture"}
	msg := &tg.Message{ID: 7, Date: 1793271600, Message: "😀 Register <script>alert(1)</script>", Entities: []tg.MessageEntityClass{&tg.MessageEntityTextURL{Offset: 3, Length: 8, URL: "https://example.org/form(test)"}}}
	row, ok := exportRecord(chat, msg, nil)
	if !ok || len(row.Links) != 1 || row.Links[0].URL != "https://example.org/form(test)" || row.Date.Location().String() != "Asia/Singapore" {
		t.Fatalf("bad export record: %+v", row)
	}
	encoded, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(out, "supergroup-55.jsonl")
	if err = os.WriteFile(path, append(encoded, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	seen, err := repairExportJSONL(path)
	if err != nil || !seen[7] {
		t.Fatalf("cannot resume entity-containing message: %v", err)
	}
	if err = renderExportGroup(out, &TextExportGroup{Key: "supergroup-55", Title: chat.Title, Count: 1, Complete: true}); err != nil {
		t.Fatal(err)
	}
	html, err := os.ReadFile(filepath.Join(out, "supergroup-55.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(html), "<script>alert") || !strings.Contains(string(html), "&lt;script&gt;") {
		t.Fatal("unsafe rendered message")
	}
	if !strings.Contains(string(html), "https://t.me/fixture/7") {
		t.Fatal("missing source link")
	}
}

func TestTextExportRecoversInterruptedLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records.jsonl")
	if err := os.WriteFile(path, []byte("{\"id\":1}\n{\"id\":"), 0600); err != nil {
		t.Fatal(err)
	}
	ids, err := repairExportJSONL(path)
	if err != nil || !ids[1] || len(ids) != 1 {
		t.Fatalf("bad recovery: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "{\"id\":1}\n" {
		t.Fatal("partial JSON was not repaired")
	}
}
