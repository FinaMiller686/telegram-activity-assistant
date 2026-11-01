package activity

import (
	"context"
	"strings"
	"testing"

	"github.com/fr0der1c/tgtldr/app/internal/bot"
)

func TestPrivateCalendarCallbackDoesNotChangeActivityState(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	s := New(st)
	if err := s.Forget(ctx); err != nil {
		t.Fatal(err)
	}
	fake := &fakeBot{}
	s.bot = fake
	cfg, err := s.Config(ctx)
	if err != nil {
		t.Fatal(err)
	}
	old := cfg
	cfg.OwnerID = 7003
	if err = s.SaveConfig(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.SaveConfig(context.Background(), old); _ = s.Forget(context.Background()) })
	if _, err = st.Pool.Exec(ctx, `insert into telegram_accounts(telegram_user_id,status) values(7003,'authorized')`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = st.Pool.Exec(context.Background(), `delete from telegram_accounts where telegram_user_id=7003`)
	})
	var chat int64
	if err = st.Pool.QueryRow(ctx, `insert into chats(telegram_chat_id,title,enabled) values(98700,'Synthetic Calendar',true) on conflict(telegram_chat_id) do update set enabled=true returning id`).Scan(&chat); err != nil {
		t.Fatal(err)
	}
	v := calendarFixture()
	id, err := s.Save(ctx, chat, 1, "https://t.me/example/123", &v.Details)
	if err != nil {
		t.Fatal(err)
	}
	var cb Callback
	cb.Data = "cal:" + itoa(id)
	cb.From.ID = 7003
	cb.Message.Chat.ID = 7003
	cb.Message.Chat.Type = "private"
	for range 2 {
		if err = s.SendCalendarCallback(ctx, cb, 7003, "fixture-token"); err != nil {
			t.Fatal(err)
		}
	}
	if len(fake.documents) != 2 {
		t.Fatal("calendar document was not sent")
	}
	for _, payload := range fake.documents {
		file, ok := payload["document"].(bot.Upload)
		if !ok || !strings.Contains(string(file.Data), "BEGIN:VCALENDAR") || !strings.HasSuffix(file.Filename, ".ics") || payload["chat_id"] != int64(7003) {
			t.Fatal("wrong private calendar file")
		}
		keyboard := stringMustJSON(payload["reply_markup"])
		for _, host := range []string{"calendar.google.com", "outlook.office.com", "outlook.live.com"} {
			if !strings.Contains(keyboard, host) {
				t.Fatal("missing calendar provider link")
			}
		}
	}
	var count int
	var status string
	if err = st.Pool.QueryRow(ctx, `select count(*) from activities`).Scan(&count); err != nil || count != 1 {
		t.Fatal("repeated calendar click duplicated activities")
	}
	if err = st.Pool.QueryRow(ctx, `select status from activities where id=$1`, id).Scan(&status); err != nil || status != "new" {
		t.Fatal("calendar export changed participation")
	}
	cb.From.ID = 7004
	if err = s.SendCalendarCallback(ctx, cb, 7003, "fixture-token"); err == nil {
		t.Fatal("another user exported calendar data")
	}
	cb.From.ID = 7003
	cb.Message.Chat.Type = "group"
	if err = s.SendCalendarCallback(ctx, cb, 7003, "fixture-token"); err == nil {
		t.Fatal("group callback exported calendar data")
	}
	cb.Message.Chat.Type = "private"
	v.End = nil
	if _, err = s.Save(ctx, chat, 1, "https://t.me/example/123", &v.Details); err != nil {
		t.Fatal(err)
	}
	if err = s.SendCalendarCallback(ctx, cb, 7003, "fixture-token"); err == nil {
		t.Fatal("incomplete event produced calendar data")
	}
	if len(fake.documents) != 2 {
		t.Fatal("blocked request sent a file")
	}
	// The menu is also private and filters incomplete or expired activities.
	if err = s.SendCalendarMenu(ctx, 7003, "fixture-token"); err != nil {
		t.Fatal(err)
	}
	if len(fake.messages) != 1 {
		t.Fatal("calendar command did not respond")
	}
	if strings.Contains(fake.messages[0]["text"].(string), "Mac") {
		t.Fatal("phone command forced desktop-only instructions")
	}
}
