package activity

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/config"
	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/store"
)

func TestResetFromNowPreservesSettingsAndRejectsReplayedHistory(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	s := New(st)
	if err := s.Forget(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = st.Pool.Exec(context.Background(), `update activity_config set collect_since=null where id=1`)
	})
	before, err := s.Config(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var chat int64
	if err = st.Pool.QueryRow(ctx, `insert into chats(telegram_chat_id,title,enabled) values(98001,'Synthetic Reset',true) on conflict(telegram_chat_id) do update set enabled=true returning id`).Scan(&chat); err != nil {
		t.Fatal(err)
	}
	m := model.Message{ChatID: chat, TelegramMessageID: 1, TextContent: "Campus Event", MessageTime: time.Now().Add(-time.Hour), RawJSON: `{}`}
	if err = st.Messages.Upsert(ctx, m); err != nil {
		t.Fatal(err)
	}
	oldID, err := s.Save(ctx, chat, 1, "", &Details{NameEN: "Old Event", Date: "2026-10-30"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetStatus(ctx, oldID, "registered"); err != nil {
		t.Fatal(err)
	}
	if _, err = st.Pool.Exec(ctx, `insert into summaries(chat_id,summary_date) values($1,current_date)`, chat); err != nil {
		t.Fatal(err)
	}
	since, err := s.ResetFromNow(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var records int
	if err = st.Pool.QueryRow(ctx, `select (select count(*) from messages)+(select count(*) from activities)+(select count(*) from summaries)+(select count(*) from activity_deliveries)`).Scan(&records); err != nil {
		t.Fatal(err)
	}
	if records != 0 {
		t.Fatal("old records survived reset")
	}
	var enabled bool
	if err = st.Pool.QueryRow(ctx, `select enabled from chats where id=$1`, chat).Scan(&enabled); err != nil || !enabled {
		t.Fatal("monitoring choice lost")
	}
	after, err := s.Config(ctx)
	if err != nil || after.CollectSince == nil || !after.CollectSince.Equal(since) || after.AIEnabled != before.AIEnabled || after.OwnerID != before.OwnerID {
		t.Fatal("settings or cutoff not preserved")
	}
	// Simulate a replayed edit and a history backfill after reconnecting.
	m.RawJSON = `{"EditDate":9999999999}`
	if err = st.Messages.Upsert(ctx, m); err != nil {
		t.Fatal(err)
	}
	if err = st.Pool.QueryRow(ctx, `select count(*) from messages`).Scan(&records); err != nil || records != 0 {
		t.Fatal("old message returned after reset")
	}
	reopened, err := store.Open(ctx, config.Config{DatabaseURL: os.Getenv("ACTIVITY_TEST_DATABASE_URL"), MasterKey: make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	m.TelegramMessageID = 2
	m.MessageTime = since.Add(time.Second)
	if err = reopened.Messages.Upsert(ctx, m); err != nil {
		t.Fatal(err)
	}
	if err = st.Pool.QueryRow(ctx, `select count(*) from messages`).Scan(&records); err != nil || records != 1 {
		t.Fatal("new message was not stored after reconnect")
	}
	newID, err := s.Save(ctx, chat, 2, "", &Details{NameEN: "New Event", Date: "2026-10-31"})
	if err != nil || newID <= oldID {
		t.Fatal("activity IDs reused, allowing stale Bot buttons to target new events")
	}
	if err = s.SetStatus(ctx, oldID, "going"); err == nil {
		t.Fatal("old activity button remained valid")
	}
	if err = s.Forget(ctx); err != nil {
		t.Fatal(err)
	}
}
