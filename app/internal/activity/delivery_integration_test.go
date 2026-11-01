package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type receiptBot struct {
	mu       sync.Mutex
	texts    []string
	failText string
	failure  string
}

func (f *receiptBot) Call(_ context.Context, _, method string, payload any, out any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if method != "sendMessage" {
		return fmt.Errorf("unexpected_method")
	}
	text := payload.(map[string]any)["text"].(string)
	if f.failText != "" && strings.Contains(text, f.failText) {
		return fmt.Errorf("%s", f.failure)
	}
	f.texts = append(f.texts, text)
	if out != nil {
		return json.Unmarshal([]byte(fmt.Sprintf(`{"message_id":%d}`, len(f.texts)+100)), out)
	}
	return nil
}

func deliveryFixture(t *testing.T) (*Service, *receiptBot, int64) {
	t.Helper()
	st := testStore(t)
	s := New(st)
	if err := s.Forget(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Forget(context.Background()) })
	var chat int64
	if err := st.Pool.QueryRow(context.Background(), `insert into chats(telegram_chat_id,title,enabled) values(99110,'Synthetic Delivery',true) on conflict(telegram_chat_id) do update set enabled=true returning id`).Scan(&chat); err != nil {
		t.Fatal(err)
	}
	fake := &receiptBot{}
	s.bot = fake
	return s, fake, chat
}

func sendFixtureCard(t *testing.T, s *Service, id int64) Event {
	t.Helper()
	events, err := s.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range events {
		if v.ID == id {
			if err = s.sendOnce(context.Background(), fmt.Sprintf("card:%d:%d", v.ID, v.Revision), Card(v), Keyboard(v), 7001, "fixture-token"); err != nil {
				t.Fatal(err)
			}
			return v
		}
	}
	t.Fatal("fixture event missing")
	return Event{}
}

func TestCardSuppressesInvisibleRevisionButSendsChangesAndReversions(t *testing.T) {
	s, fake, chat := deliveryFixture(t)
	ctx := context.Background()
	d := Details{NameEN: "Synthetic Seminar", Summary: "Original content", Date: "2099-01-01"}
	id, err := s.Save(ctx, chat, 1, "https://t.me/example/1", &d)
	if err != nil {
		t.Fatal(err)
	}
	first := sendFixtureCard(t, s, id)
	// Emulate receipts created before this version, which have no Telegram ID.
	if _, err = s.Store.Pool.Exec(ctx, `update activity_deliveries set bot_message_id=null`); err != nil {
		t.Fatal(err)
	}
	d.Eligibility = "Students"
	if _, err = s.Save(ctx, chat, 1, "https://t.me/example/1", &d); err != nil {
		t.Fatal(err)
	}
	second := sendFixtureCard(t, s, id)
	if second.Revision <= first.Revision || len(fake.texts) != 1 {
		t.Fatal("hidden-field revision resent identical text")
	}
	var reason string
	if err = s.Store.Pool.QueryRow(ctx, `select skip_reason from activity_deliveries where key=$1`, fmt.Sprintf("card:%d:%d", id, second.Revision)).Scan(&reason); err != nil || reason != "unchanged" {
		t.Fatal("skip receipt missing")
	}
	d.Summary = "Changed content"
	if _, err = s.Save(ctx, chat, 1, "https://t.me/example/1", &d); err != nil {
		t.Fatal(err)
	}
	sendFixtureCard(t, s, id)
	d.Summary = "Original content"
	if _, err = s.Save(ctx, chat, 1, "https://t.me/example/1", &d); err != nil {
		t.Fatal(err)
	}
	current := sendFixtureCard(t, s, id)
	if len(fake.texts) != 3 {
		t.Fatal("a real change or reversion was lost")
	}
	restarted := New(s.Store)
	restarted.bot = fake
	sendFixtureCard(t, restarted, id)
	if len(fake.texts) != 3 {
		t.Fatal("restart resent the current revision")
	}
	// A stale concurrent worker cannot re-send the old snapshot.
	if err = s.sendOnce(ctx, fmt.Sprintf("card:%d:%d", id, current.Revision-1), "Stale snapshot", nil, 7001, "fixture-token"); err != nil {
		t.Fatal(err)
	}
	if len(fake.texts) != 3 {
		t.Fatal("stale snapshot sent")
	}
}

func TestConcurrentDeliveryKeepsSingleReceipt(t *testing.T) {
	s, fake, _ := deliveryFixture(t)
	var group sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			other := New(s.Store)
			other.bot = fake
			errs <- other.sendOnce(context.Background(), "probe:concurrent", "Concurrent fixture", nil, 7001, "fixture-token")
		}()
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(fake.texts) != 1 {
		t.Fatal("concurrent workers sent duplicates")
	}
	var messageID int64
	if err := s.Store.Pool.QueryRow(context.Background(), `select bot_message_id from activity_deliveries where key='probe:concurrent' and sent_at is not null`).Scan(&messageID); err != nil || messageID <= 0 {
		t.Fatal("Telegram acknowledgement missing")
	}
}

func TestSendFailureDoesNotBlockOtherCardsAndRetriesAfterRestart(t *testing.T) {
	s, fake, chat := deliveryFixture(t)
	ctx := context.Background()
	settings, err := s.Store.Settings.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oldSettings := settings
	settings.BotEnabled = true
	settings.BotToken = "fixture-token"
	if _, err = s.Store.Settings.Save(ctx, settings); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = s.Store.Settings.Save(ctx, oldSettings) })
	cfg, err := s.Config(ctx)
	if err != nil {
		t.Fatal(err)
	}
	oldCfg := cfg
	cfg.OwnerID = 7001
	if err = s.SaveConfig(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.SaveConfig(ctx, oldCfg) })
	if _, err = s.Store.Pool.Exec(ctx, `insert into telegram_accounts(telegram_user_id,status) values(7001,'authorized')`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = s.Store.Pool.Exec(ctx, `delete from telegram_accounts where telegram_user_id=7001`) })
	for index, name := range []string{"Blocked fixture", "Healthy fixture"} {
		if _, err = s.Save(ctx, chat, index+1, "", &Details{NameEN: name, Date: "2099-01-01"}); err != nil {
			t.Fatal(err)
		}
	}
	fake.failText = "Blocked fixture"
	fake.failure = "bot_api_status_400"
	if err = s.Deliver(ctx); err == nil {
		t.Fatal("failure concealed")
	}
	if len(fake.texts) != 1 || !strings.Contains(fake.texts[0], "Healthy fixture") {
		t.Fatal("failed card blocked a different card")
	}
	report, err := s.DeliveryReport(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if report.PendingDeliveries != 1 || report.RetryingDeliveries != 1 {
		t.Fatal("pending failure not reported")
	}
	fake.failText = ""
	restarted := New(s.Store)
	restarted.bot = fake
	if err = restarted.Deliver(ctx); err != nil {
		t.Fatal(err)
	}
	if len(fake.texts) != 1 {
		t.Fatal("backoff ignored")
	}
	if _, err = s.Store.Pool.Exec(ctx, `update activity_deliveries set retry_at=now()-interval '1 minute' where sent_at is null`); err != nil {
		t.Fatal(err)
	}
	if err = restarted.Deliver(ctx); err != nil {
		t.Fatal(err)
	}
	if len(fake.texts) != 2 || !strings.Contains(fake.texts[1], "Blocked fixture") {
		t.Fatal("failed card not recovered")
	}
	report, err = s.DeliveryReport(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if report.PendingDeliveries != 0 || report.RetryingDeliveries != 0 {
		t.Fatal("receipts not reconciled")
	}
}

func TestUnconfirmedSendAndSnapshotRemainVisible(t *testing.T) {
	s, fake, _ := deliveryFixture(t)
	ctx := context.Background()
	fake.failText = "Stored snapshot"
	fake.failure = "bot_network_failed"
	if err := s.sendOnce(ctx, "probe:snapshot", "Stored snapshot", nil, 7001, "fixture-token"); err == nil {
		t.Fatal("network failure concealed")
	}
	report, err := s.DeliveryReport(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if report.UnconfirmedDeliveries != 1 || report.Recent[0].State != "unconfirmed" {
		t.Fatal("ambiguous send marked successful")
	}
	if _, err = s.Store.Pool.Exec(ctx, `update activity_deliveries set retry_at=$1`, time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	fake.failText = ""
	if err = s.sendOnce(ctx, "probe:snapshot", "Changed during retry", nil, 7001, "fixture-token"); err != nil {
		t.Fatal(err)
	}
	if len(fake.texts) != 1 || fake.texts[0] != "Stored snapshot" {
		t.Fatal("retry replaced persisted snapshot")
	}
}

func TestReversionSupersedesAnOlderFailedCard(t *testing.T) {
	s, fake, chat := deliveryFixture(t)
	ctx := context.Background()
	d := Details{NameEN: "Synthetic Reversion", Summary: "Original", Date: "2099-01-01"}
	id, err := s.Save(ctx, chat, 1, "", &d)
	if err != nil {
		t.Fatal(err)
	}
	sendFixtureCard(t, s, id)
	d.Summary = "Failed revision"
	if _, err = s.Save(ctx, chat, 1, "", &d); err != nil {
		t.Fatal(err)
	}
	fake.failText, fake.failure = "Failed revision", "bot_api_status_400"
	if err = s.sendOnce(ctx, fmt.Sprintf("card:%d:2", id), Card(Event{Details: d}), nil, 7001, "fixture-token"); err == nil {
		t.Fatal("failure concealed")
	}
	d.Summary = "Original"
	if _, err = s.Save(ctx, chat, 1, "", &d); err != nil {
		t.Fatal(err)
	}
	sendFixtureCard(t, s, id)
	report, err := s.DeliveryReport(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.texts) != 1 || report.PendingDeliveries != 0 || report.RetryingDeliveries != 0 {
		t.Fatal("superseded failure remained queued or original card was resent")
	}
}

func TestDuplicateSuppressionIsScopedToOneActivity(t *testing.T) {
	s, fake, chat := deliveryFixture(t)
	ctx := context.Background()
	d := Details{NameEN: "Shared title", Date: "2099-01-01", Organizer: "First organizer"}
	id, err := s.Save(ctx, chat, 1, "", &d)
	if err != nil {
		t.Fatal(err)
	}
	sendFixtureCard(t, s, id)
	// Distinct records can have identical visible text. Never suppress globally.
	d.Organizer = "Second organizer"
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var other int64
	if err = s.Store.Pool.QueryRow(ctx, `insert into activities(data) values($1) returning id`, raw).Scan(&other); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Store.Pool.Exec(ctx, `insert into activity_sources(chat_id,telegram_message_id,activity_id) values($1,2,$2)`, chat, other); err != nil {
		t.Fatal(err)
	}
	sendFixtureCard(t, s, other)
	if len(fake.texts) != 2 {
		t.Fatal("different activity hidden by global text matching")
	}
}
