package activity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/config"
	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/store"
)

// The test URL must point to a disposable database; no Telegram credentials are used.
func testStore(t *testing.T) *store.Store {
	t.Helper()
	u := os.Getenv("ACTIVITY_TEST_DATABASE_URL")
	if u == "" {
		t.Skip("ACTIVITY_TEST_DATABASE_URL not set")
	}
	st, e := store.Open(context.Background(), config.Config{DatabaseURL: u, MasterKey: make([]byte, 32)})
	if e != nil {
		t.Fatal(e)
	}
	if e = store.RunMigrations(context.Background(), st); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(st.Close)
	return st
}
func TestDatabaseLifecycle(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	s := New(st)
	if e := s.Forget(ctx); e != nil {
		t.Fatal(e)
	}
	var a, b int64
	if e := st.Pool.QueryRow(ctx, `insert into chats(telegram_chat_id,title,enabled) values(91001,'Synthetic A',true) on conflict(telegram_chat_id) do update set enabled=true returning id`).Scan(&a); e != nil {
		t.Fatal(e)
	}
	if e := st.Pool.QueryRow(ctx, `insert into chats(telegram_chat_id,title,enabled) values(91002,'Synthetic B',true) on conflict(telegram_chat_id) do update set enabled=true returning id`).Scan(&b); e != nil {
		t.Fatal(e)
	}
	detail := Details{NameEN: "Research Seminar", Date: "2026-10-29", Location: "Hall", Room: "101"}
	ids := make(chan int64, 2)
	var wg sync.WaitGroup
	for _, chat := range []int64{a, b} {
		wg.Add(1)
		go func(chat int64) {
			defer wg.Done()
			id, e := s.Save(ctx, chat, 1, "https://t.me/demo/1", &detail)
			if e != nil {
				t.Error(e)
			}
			ids <- id
		}(chat)
	}
	wg.Wait()
	close(ids)
	id := <-ids
	if other := <-ids; id != other {
		t.Fatal("concurrent repost created duplicates")
	}
	if e := s.SetStatus(ctx, id, "going"); e != nil {
		t.Fatal(e)
	}
	var cb Callback
	cb.Data = "act:" + itoa(id) + ":registered"
	cb.From.ID = 42
	cb.Message.Chat.ID = 42
	cb.Message.Chat.Type = "private"
	for i := 0; i < 2; i++ {
		if _, e := s.ApplyCallback(ctx, cb, 42); e != nil {
			t.Fatal(e)
		}
	}
	items, e := New(st).List(ctx)
	if e != nil || len(items) != 1 || items[0].Status != "registered" || len(items[0].Sources) != 2 {
		t.Fatalf("%+v %v", items, e)
	}
	detail.Location = "New Hall"
	_, e = s.Save(ctx, a, 1, "https://t.me/demo/1", &detail)
	if e != nil {
		t.Fatal(e)
	}
	items, _ = s.List(ctx)
	if items[0].Status != "registered" || items[0].Location != "New Hall" {
		t.Fatal("update lost intent")
	}
	// Reconnect to the database to verify that state survives a new process connection.
	reopened, e := store.Open(ctx, config.Config{DatabaseURL: os.Getenv("ACTIVITY_TEST_DATABASE_URL"), MasterKey: make([]byte, 32)})
	if e != nil {
		t.Fatal(e)
	}
	items, e = New(reopened).List(ctx)
	reopened.Close()
	if e != nil || items[0].Status != "registered" {
		t.Fatal("restart lost status")
	}
	if _, e = s.Save(ctx, a, 1, "", nil); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Save(ctx, b, 1, "", nil); e != nil {
		t.Fatal(e)
	}
	items, _ = s.List(ctx)
	if !items[0].Withdrawn {
		t.Fatal("withdrawal missing")
	}
	if e = s.Forget(ctx); e != nil {
		t.Fatal(e)
	}
	items, e = s.List(ctx)
	if e != nil || len(items) != 0 {
		t.Fatal("history not deleted")
	}
}
func TestWorkerPrivacyAndIdempotency(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	s := New(st)
	if e := s.Forget(ctx); e != nil {
		t.Fatal(e)
	}
	var chat int64
	if e := st.Pool.QueryRow(ctx, `insert into chats(telegram_chat_id,title,enabled) values(92001,'Synthetic Worker',true) on conflict(telegram_chat_id) do update set enabled=true returning id`).Scan(&chat); e != nil {
		t.Fatal(e)
	}
	text := "Career Workshop\nDate: tomorrow\nTime: 7pm-9pm\nVenue: Demo Hall\nRegister: https://forms.gle/synthetic"
	m := model.Message{ChatID: chat, TelegramMessageID: 1, TextContent: text, MessageTime: time.Now(), RawJSON: `{"Entities":[],"FromID":{"UserID":123},"ReplyMarkup":null}`, TelegramSenderID: 123, SenderName: "DO_NOT_STORE"}
	if e := st.Messages.Upsert(ctx, m); e != nil {
		t.Fatal(e)
	}
	var name, raw string
	if e := st.Pool.QueryRow(ctx, `select sender_name,raw_json::text from messages where chat_id=$1`, chat).Scan(&name, &raw); e != nil {
		t.Fatal(e)
	}
	if name != "" || stringsContains(raw, "DO_NOT_STORE") || stringsContains(raw, "FromID") {
		t.Fatal("identity retained")
	}
	if e := s.Tick(ctx); e != nil {
		t.Fatal(e)
	}
	if e := s.Tick(ctx); e != nil {
		t.Fatal(e)
	}
	items, e := s.List(ctx)
	if e != nil || len(items) != 1 || items[0].Revision != 1 {
		t.Fatalf("%+v %v", items, e)
	}
	m.TextContent = text + "\nRoom: 204"
	if e := st.Messages.Upsert(ctx, m); e != nil {
		t.Fatal(e)
	}
	if e := s.Tick(ctx); e != nil {
		t.Fatal(e)
	}
	items, _ = s.List(ctx)
	if len(items) != 1 || items[0].Room != "204" {
		t.Fatal("edit not updated")
	}
	if _, e := st.Pool.Exec(ctx, `update chats set enabled=false where id=$1`, chat); e != nil {
		t.Fatal(e)
	}
	m.TelegramMessageID = 2
	if e := st.Messages.Upsert(ctx, m); e != nil {
		t.Fatal(e)
	}
	var count int
	_ = st.Pool.QueryRow(ctx, `select count(*) from messages where chat_id=$1`, chat).Scan(&count)
	if count != 1 {
		t.Fatal("disabled group collected")
	}
	if e := s.Forget(ctx); e != nil {
		t.Fatal(e)
	}
}
func stringsContains(a, b string) bool {
	for i := 0; i+len(b) <= len(a); i++ {
		if a[i:i+len(b)] == b {
			return true
		}
	}
	return false
}

func TestAIForSelectedChatsAndEditedMessages(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	s := New(st)
	if e := s.Forget(ctx); e != nil {
		t.Fatal(e)
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		var payload struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if e := json.NewDecoder(r.Body).Decode(&payload); e != nil {
			t.Error(e)
			w.WriteHeader(400)
			return
		}
		if len(payload.Messages) != 2 {
			t.Error("unexpected request shape")
		}
		input := payload.Messages[len(payload.Messages)-1].Content
		if strings.Contains(input, "DO_NOT_SEND") {
			t.Error("unselected chat or full Telegram payload sent")
		}
		content := `{"nameEn":"Summer Placement","nameZh":"暑期实习","summary":"开放申请","category":"职业发展与实习"}`
		if strings.Contains(input, "thanks!") {
			content = `{"ignore":true}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
	}))
	defer server.Close()
	settings, e := st.Settings.Get(ctx)
	if e != nil {
		t.Fatal(e)
	}
	old := settings
	settings.OpenAIBaseURL = server.URL
	settings.OpenAIModel = "test"
	if _, e = st.Settings.Save(ctx, settings); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _, _ = st.Settings.Save(context.Background(), old) })
	cfg, e := s.Config(ctx)
	if e != nil {
		t.Fatal(e)
	}
	cfg.AIEnabled = true
	if e = s.SaveConfig(ctx, cfg); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { cfg.AIEnabled = false; _ = s.SaveConfig(context.Background(), cfg) })
	var chat int64
	for _, fixture := range []struct {
		id      int
		enabled bool
		kind    string
	}{{93001, true, "group"}, {93002, false, "group"}, {93003, true, "private"}} {
		var id int64
		if e = st.Pool.QueryRow(ctx, `insert into chats(telegram_chat_id,title,enabled,chat_type) values($1,'Synthetic AI',$2,$3) on conflict(telegram_chat_id) do update set enabled=excluded.enabled,chat_type=excluded.chat_type returning id`, fixture.id, fixture.enabled, fixture.kind).Scan(&id); e != nil {
			t.Fatal(e)
		}
		text := "Summer Placement\nApplications open tomorrow."
		if fixture.id != 93001 {
			text += " DO_NOT_SEND"
		} else {
			chat = id
		}
		if e = st.Messages.Upsert(ctx, model.Message{ChatID: id, TelegramMessageID: 42, TextContent: text, MessageTime: time.Now(), RawJSON: `{"private_metadata":"DO_NOT_SEND"}`}); e != nil {
			t.Fatal(e)
		}
	}
	if Candidate("Summer Placement\nApplications open tomorrow.") {
		t.Fatal("fixture must bypass the rule candidate filter")
	}
	if e = s.Tick(ctx); e != nil {
		t.Fatal(e)
	}
	if requests.Load() != 1 {
		t.Fatalf("selected message not processed: %d", requests.Load())
	}
	if e = s.Tick(ctx); e != nil {
		t.Fatal(e)
	}
	if requests.Load() != 1 {
		t.Fatal("unchanged message processed twice")
	}
	m := model.Message{ChatID: chat, TelegramMessageID: 42, TextContent: "Summer Placement\nApplications open tomorrow.\nRoom: 204", MessageTime: time.Now(), RawJSON: `{}`}
	if e = st.Messages.Upsert(ctx, m); e != nil {
		t.Fatal(e)
	}
	if e = s.Tick(ctx); e != nil {
		t.Fatal(e)
	}
	if requests.Load() != 2 {
		t.Fatal("edited message not processed")
	}
	if e = st.Messages.Upsert(ctx, model.Message{ChatID: chat, TelegramMessageID: 43, TextContent: "thanks!", MessageTime: time.Now(), RawJSON: `{}`}); e != nil {
		t.Fatal(e)
	}
	if e = s.Tick(ctx); e != nil {
		t.Fatal(e)
	}
	if requests.Load() != 3 {
		t.Fatal("AI did not evaluate non-event text")
	}
	var count int
	if e = st.Pool.QueryRow(ctx, `select count(*) from activities`).Scan(&count); e != nil {
		t.Fatal(e)
	}
	if count != 1 {
		t.Fatalf("ignored text or edit created extra activities: %d", count)
	}
	cfg.AIEnabled = false
	if e = s.SaveConfig(ctx, cfg); e != nil {
		t.Fatal(e)
	}
	m.TextContent += " updated"
	if e = st.Messages.Upsert(ctx, m); e != nil {
		t.Fatal(e)
	}
	if e = s.Tick(ctx); e != nil {
		t.Fatal(e)
	}
	if requests.Load() != 3 {
		t.Fatal("AI master switch was ignored")
	}
	if e = s.Forget(ctx); e != nil {
		t.Fatal(e)
	}
}

type fakeBot struct {
	messages  []map[string]any
	documents []map[string]any
}

func (f *fakeBot) Call(_ context.Context, _, method string, payload any, out any) error {
	if method == "sendMessage" {
		f.messages = append(f.messages, payload.(map[string]any))
		if out != nil {
			data, _ := json.Marshal(map[string]any{"message_id": len(f.messages)})
			return json.Unmarshal(data, out)
		}
	}
	if method == "sendDocument" {
		f.documents = append(f.documents, payload.(map[string]any))
	}
	return nil
}
func TestBotDeliveryAndMissingLink(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	s := New(st)
	if e := s.Forget(ctx); e != nil {
		t.Fatal(e)
	}
	fake := &fakeBot{}
	s.bot = fake
	settings, e := st.Settings.Get(ctx)
	if e != nil {
		t.Fatal(e)
	}
	old := settings
	settings.BotEnabled = true
	settings.BotToken = "synthetic-token"
	if _, e = st.Settings.Save(ctx, settings); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _, _ = st.Settings.Save(context.Background(), old) })
	_, e = st.Pool.Exec(ctx, `insert into telegram_accounts(telegram_user_id,status) values(707,'authorized')`)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		_, _ = st.Pool.Exec(context.Background(), `delete from telegram_accounts where telegram_user_id=707`)
	})
	cfg, e := s.Config(ctx)
	if e != nil {
		t.Fatal(e)
	}
	oldCfg := cfg
	cfg.OwnerID = 707
	if e = s.SaveConfig(ctx, cfg); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = s.SaveConfig(context.Background(), oldCfg) })
	var chat int64
	if e = st.Pool.QueryRow(ctx, `insert into chats(telegram_chat_id,title,enabled) values(94001,'Synthetic Bot',true) on conflict(telegram_chat_id) do update set enabled=true returning id`).Scan(&chat); e != nil {
		t.Fatal(e)
	}
	deadline := time.Now().Add(2 * time.Hour)
	details := Details{NameEN: "Demo Event", Summary: "示例活动", Date: time.Now().In(Singapore).Format("2006-01-02"), Deadline: &deadline}
	id, e := s.Save(ctx, chat, 1, "https://t.me/demo/1", &details)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = st.Pool.Exec(ctx, `insert into activity_deliveries(key,content) values('digest:2000-01-01',$1)`, strings.Repeat("测试条目\n", 1000)); e != nil {
		t.Fatal(e)
	}
	if e = s.Deliver(ctx); e != nil {
		t.Fatal(e)
	}
	sent := len(fake.messages)
	if sent < 4 {
		t.Fatalf("expected activity, reminder, and split digest, got %d", sent)
	}
	if e = s.Deliver(ctx); e != nil {
		t.Fatal(e)
	}
	if len(fake.messages) != sent {
		t.Fatal("duplicate card or reminder")
	}
	var cb Callback
	cb.From.ID = 707
	cb.Message.Chat.ID = 707
	cb.Message.Chat.Type = "private"
	cb.Data = "act:" + itoa(id) + ":going"
	reply, e := s.ApplyCallback(ctx, cb, 707)
	if e != nil || !strings.Contains(reply, "未找到有效报名链接") || !strings.Contains(reply, "https://t.me/demo/1") {
		t.Fatalf("missing link reply %q %v", reply, e)
	}
	cb.From.ID = 708
	if _, e = s.ApplyCallback(ctx, cb, 707); e == nil {
		t.Fatal("unauthorized callback accepted")
	}
	items, e := s.List(ctx)
	if e != nil || items[0].Status != "going" {
		t.Fatal("unauthorized callback changed status")
	}
	if e = s.Forget(ctx); e != nil {
		t.Fatal(e)
	}
}
func TestSourceFingerprintSurvivesRetention(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	s := New(st)
	if e := s.Forget(ctx); e != nil {
		t.Fatal(e)
	}
	var chat int64
	if e := st.Pool.QueryRow(ctx, `insert into chats(telegram_chat_id,title,enabled) values(95001,'Synthetic Retention',true) on conflict(telegram_chat_id) do update set enabled=true returning id`).Scan(&chat); e != nil {
		t.Fatal(e)
	}
	m := model.Message{ChatID: chat, TelegramMessageID: 123, TextContent: "Campus Event\nDate: tomorrow", MessageTime: time.Now(), RawJSON: `{}`}
	if e := st.Messages.Upsert(ctx, m); e != nil {
		t.Fatal(e)
	}
	if e := s.Tick(ctx); e != nil {
		t.Fatal(e)
	}
	before, e := s.List(ctx)
	if e != nil || len(before) != 1 {
		t.Fatal(e)
	}
	if _, e = st.Pool.Exec(ctx, `delete from messages where chat_id=$1`, chat); e != nil {
		t.Fatal(e)
	}
	if e = st.Messages.Upsert(ctx, m); e != nil {
		t.Fatal(e)
	}
	if e = s.Tick(ctx); e != nil {
		t.Fatal(e)
	}
	after, e := s.List(ctx)
	if e != nil || len(after) != 1 || after[0].Revision != before[0].Revision {
		t.Fatal("unchanged retained source was reprocessed")
	}
	if e = s.Forget(ctx); e != nil {
		t.Fatal(e)
	}
}

func TestOldBackfillCannotOverwriteNewerEdit(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	s := New(st)
	if e := s.Forget(ctx); e != nil {
		t.Fatal(e)
	}
	var chat int64
	if e := st.Pool.QueryRow(ctx, `insert into chats(telegram_chat_id,title,enabled) values(96001,'Synthetic Edits',true) on conflict(telegram_chat_id) do update set enabled=true returning id`).Scan(&chat); e != nil {
		t.Fatal(e)
	}
	m := model.Message{ChatID: chat, TelegramMessageID: 3, TextContent: "Event updated", MessageTime: time.Now(), RawJSON: `{"EditDate":200}`}
	if e := st.Messages.Upsert(ctx, m); e != nil {
		t.Fatal(e)
	}
	m.TextContent = "Event old"
	m.RawJSON = `{"EditDate":100}`
	if e := st.Messages.Upsert(ctx, m); e != nil {
		t.Fatal(e)
	}
	var text string
	if e := st.Pool.QueryRow(ctx, `select text_content from messages where chat_id=$1 and telegram_message_id=3`, chat).Scan(&text); e != nil || text != "Event updated" {
		t.Fatalf("stale backfill replaced edit: %q %v", text, e)
	}
	if e := s.Forget(ctx); e != nil {
		t.Fatal(e)
	}
}

func TestParallelAIJobsKeepTheirOwnMessage(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	service := New(st)
	if err := service.Forget(ctx); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(400)
			return
		}
		name := "Beta Campus Event"
		if strings.Contains(payload.Messages[1].Content, "Alpha Campus Event") {
			name = "Alpha Campus Event"
		}
		started <- struct{}{}
		<-release
		content, _ := json.Marshal(map[string]string{"nameEn": name, "nameZh": name, "summary": "校园活动", "category": "校园活动"})
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(content)}}}})
	}))
	defer server.Close()
	settings, err := st.Settings.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	old := settings
	settings.OpenAIBaseURL = server.URL
	settings.OpenAIModel = "fixture"
	settings.SummaryParallelism = 2
	if _, err = st.Settings.Save(ctx, settings); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = st.Settings.Save(context.Background(), old) })
	cfg, err := service.Config(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cfg.AIEnabled = true
	if err = service.SaveConfig(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cfg.AIEnabled = false
		_ = service.SaveConfig(context.Background(), cfg)
		_ = service.Forget(context.Background())
	})
	for index, name := range []string{"Alpha Campus Event", "Beta Campus Event"} {
		var chat int64
		if err = st.Pool.QueryRow(ctx, `insert into chats(telegram_chat_id,title,enabled) values($1,'Synthetic Parallel',true) on conflict(telegram_chat_id) do update set enabled=true returning id`, 98500+index).Scan(&chat); err != nil {
			t.Fatal(err)
		}
		if err = st.Messages.Upsert(ctx, model.Message{ChatID: chat, TelegramMessageID: 1, TextContent: name + "\nDate: tomorrow", MessageTime: time.Now(), RawJSON: `{}`}); err != nil {
			t.Fatal(err)
		}
	}
	result := make(chan error, 1)
	go func() { result <- service.Tick(ctx) }()
	for range 2 {
		select {
		case <-started:
		case <-time.After(3 * time.Second):
			close(release)
			<-result
			t.Fatal("configured parallel model calls did not start")
		}
	}
	close(release)
	if err = <-result; err != nil {
		t.Fatal(err)
	}
	var count int
	if err = st.Pool.QueryRow(ctx, `select count(distinct data->>'nameEn') from activities`).Scan(&count); err != nil || count != 2 {
		t.Fatal("parallel jobs mixed up message contents")
	}
}
