package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/bot"
	"github.com/fr0der1c/tgtldr/app/internal/store"
	"github.com/jackc/pgx/v5"
)

type botTransport interface {
	Call(context.Context, string, string, any, any) error
}
type Service struct {
	Store *store.Store
	bot   botTransport
}

func New(st *store.Store) *Service { return &Service{Store: st, bot: bot.New()} }
func (s *Service) Config(ctx context.Context) (Config, error) {
	var c Config
	e := s.Store.Pool.QueryRow(ctx, `select ai_enabled,digest_time,reminder_hours,retention_days,owner_id,collect_since from activity_config where id=1`).Scan(&c.AIEnabled, &c.DigestTime, &c.ReminderHours, &c.RetentionDays, &c.OwnerID, &c.CollectSince)
	return c, e
}
func (s *Service) SaveConfig(ctx context.Context, c Config) error {
	if _, e := time.Parse("15:04", c.DigestTime); e != nil || c.ReminderHours < 1 || c.ReminderHours > 168 || c.RetentionDays < 1 || c.RetentionDays > 365 || c.OwnerID < 0 {
		return fmt.Errorf("invalid_config")
	}
	_, e := s.Store.Pool.Exec(ctx, `update activity_config set ai_enabled=$1,digest_time=$2,reminder_hours=$3,retention_days=$4,owner_id=$5 where id=1`, c.AIEnabled, c.DigestTime, c.ReminderHours, c.RetentionDays, c.OwnerID)
	return e
}
func (s *Service) List(ctx context.Context) ([]Event, error) {
	rows, e := s.Store.Pool.Query(ctx, `select id,data,status,revision,created_at,updated_at from activities order by coalesce(data->>'date','9999'),id`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	events := []Event{}
	for rows.Next() {
		var v Event
		var raw []byte
		if e = rows.Scan(&v.ID, &raw, &v.Status, &v.Revision, &v.Created, &v.Updated); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(raw, &v.Details); e != nil {
			return nil, e
		}
		events = append(events, v)
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	rows.Close()
	sources, e := s.Store.Pool.Query(ctx, `select a.activity_id,a.chat_id,a.telegram_message_id,c.title,a.link,a.deleted from activity_sources a join chats c on c.id=a.chat_id order by a.chat_id,a.telegram_message_id`)
	if e != nil {
		return nil, e
	}
	defer sources.Close()
	index := map[int64]int{}
	for i := range events {
		index[events[i].ID] = i
		events[i].Withdrawn = true
		events[i].Sources = []Source{}
		events[i].Conflicts = []int64{}
	}
	for sources.Next() {
		var id int64
		var v Source
		if e = sources.Scan(&id, &v.ChatID, &v.MessageID, &v.Title, &v.Link, &v.Deleted); e != nil {
			return nil, e
		}
		if i, ok := index[id]; ok {
			events[i].Sources = append(events[i].Sources, v)
			if !v.Deleted {
				events[i].Withdrawn = false
			}
		}
	}
	if e = sources.Err(); e != nil {
		return nil, e
	}
	for i := range events {
		for j := i + 1; j < len(events); j++ {
			if !events[i].Withdrawn && !events[j].Withdrawn && Overlap(events[i].Details, events[j].Details) {
				events[i].Conflicts = append(events[i].Conflicts, events[j].ID)
				events[j].Conflicts = append(events[j].Conflicts, events[i].ID)
			}
		}
	}
	return events, nil
}
func (s *Service) SetStatus(ctx context.Context, id int64, status string) error {
	if !ValidStatus(status) {
		return fmt.Errorf("invalid_status")
	}
	tag, e := s.Store.Pool.Exec(ctx, `update activities set status=$2,updated_at=now() where id=$1`, id, status)
	if e == nil && tag.RowsAffected() == 0 {
		return fmt.Errorf("activity_not_found")
	}
	return e
}
func (s *Service) Save(ctx context.Context, chatID int64, messageID int, link string, details *Details, fingerprint ...string) (int64, error) {
	tx, e := s.Store.Pool.Begin(ctx)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback(ctx)
	// One short transaction serializes identity matching, including cross-group reposts.
	if _, e = tx.Exec(ctx, `select pg_advisory_xact_lock(451010)`); e != nil {
		return 0, e
	}
	if len(fingerprint) > 0 {
		var current bool
		err := tx.QueryRow(ctx, `select md5(text_content||raw_json::text)=$3 from messages where chat_id=$1 and telegram_message_id=$2 for update`, chatID, messageID, fingerprint[0]).Scan(&current)
		if err == pgx.ErrNoRows || err == nil && !current {
			return 0, nil
		}
		if err != nil {
			return 0, err
		}
	}
	var id int64
	e = tx.QueryRow(ctx, `select activity_id from activity_sources where chat_id=$1 and telegram_message_id=$2`, chatID, messageID).Scan(&id)
	if e != nil && e != pgx.ErrNoRows {
		return 0, e
	}
	if details == nil {
		_, e = tx.Exec(ctx, `update activity_sources set deleted=true where chat_id=$1 and telegram_message_id=$2`, chatID, messageID)
		if e != nil {
			return 0, e
		}
		return 0, tx.Commit(ctx)
	}
	sameSource := id != 0
	if id == 0 {
		rows, e := tx.Query(ctx, `select id,data from activities where data->>'date'=$1 or ($2<>'' and ($3 or $1='' or coalesce(data->>'date','')='') and data->>'registrationUrl'=$2) order by id`, details.Date, details.RegistrationURL, details.UpdateNotice)
		if e != nil {
			return 0, e
		}
		for rows.Next() {
			var candidateID int64
			var raw []byte
			var previous Details
			if e = rows.Scan(&candidateID, &raw); e != nil {
				rows.Close()
				return 0, e
			}
			if e = json.Unmarshal(raw, &previous); e != nil {
				rows.Close()
				return 0, e
			}
			if SameEvent(previous, *details) {
				id = candidateID
				break
			}
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return 0, e
		}
	}
	if id != 0 && !sameSource {
		var oldRaw []byte
		var previous Details
		if e = tx.QueryRow(ctx, `select data from activities where id=$1`, id).Scan(&oldRaw); e != nil {
			return 0, e
		}
		if e = json.Unmarshal(oldRaw, &previous); e != nil {
			return 0, e
		}
		merged := mergeDetails(previous, *details)
		details = &merged
	}
	raw, e := json.Marshal(details)
	if e != nil {
		return 0, e
	}
	if id == 0 {
		e = tx.QueryRow(ctx, `insert into activities(data) values($1) returning id`, raw).Scan(&id)
	} else {
		_, e = tx.Exec(ctx, `update activities set data=$2,revision=revision+case when data<>$2::jsonb then 1 else 0 end,updated_at=case when data<>$2::jsonb then now() else updated_at end where id=$1`, id, raw)
	}
	if e != nil {
		return 0, e
	}
	sourceHash := ""
	if len(fingerprint) > 0 {
		sourceHash = fingerprint[0]
	}
	_, e = tx.Exec(ctx, `insert into activity_sources(chat_id,telegram_message_id,activity_id,link,fingerprint) values($1,$2,$3,$4,$5) on conflict(chat_id,telegram_message_id) do update set activity_id=excluded.activity_id,link=excluded.link,fingerprint=excluded.fingerprint,deleted=false`, chatID, messageID, id, link, sourceHash)
	if e != nil {
		return 0, e
	}
	return id, tx.Commit(ctx)
}

// Reposts often omit fields from the original announcement. Preserve those fields
// with an explicit warning; an edit of the same source instead replaces its data.
func mergeDetails(old, current Details) Details {
	preserved := false
	fill := func(dst *string, value string) {
		if *dst == "" && value != "" {
			*dst = value
			preserved = true
		}
	}
	fill(&current.NameZH, old.NameZH)
	fill(&current.Organizer, old.Organizer)
	fill(&current.Date, old.Date)
	fill(&current.DateText, old.DateText)
	fill(&current.TimeText, old.TimeText)
	fill(&current.DeadlineText, old.DeadlineText)
	fill(&current.Location, old.Location)
	fill(&current.Floor, old.Floor)
	fill(&current.Room, old.Room)
	fill(&current.RegistrationURL, old.RegistrationURL)
	fill(&current.DeadlineDate, old.DeadlineDate)
	fill(&current.Eligibility, old.Eligibility)
	fill(&current.DressCode, old.DressCode)
	if current.Date == old.Date && current.Start == nil && old.Start != nil {
		current.Start = old.Start
		preserved = true
	}
	if current.Date == old.Date && current.End == nil && old.End != nil {
		current.End = old.End
		preserved = true
	}
	if current.Deadline == nil && old.Deadline != nil {
		current.Deadline = old.Deadline
		preserved = true
	}
	if current.RegistrationRequired == nil && old.RegistrationRequired != nil {
		current.RegistrationRequired = old.RegistrationRequired
		preserved = true
	}
	if current.Summary == "收到一则活动或重要通知，请查看时间、地点与报名信息，并核对原始消息。" && old.Summary != "" {
		current.Summary = old.Summary
		preserved = true
	}
	if preserved {
		current.Warnings = append(current.Warnings, "部分信息来自先前通知，请核对最新原文")
	}
	return current
}
func (s *Service) Forget(ctx context.Context) error {
	tx, e := s.Store.Pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `select pg_advisory_xact_lock(451010)`); e != nil {
		return e
	}
	_, e = tx.Exec(ctx, `update chats set enabled=false,summary_enabled=false; delete from activity_sync; truncate activities,activity_sources,activity_jobs,activity_deliveries restart identity cascade; delete from messages; delete from summaries; delete from catch_ups; delete from daily_digests; delete from telegram_entities; delete from media_assets; delete from activity_runtime;`)
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// ResetFromNow clears collected records while retaining accounts, credentials,
// monitoring choices, and settings. Sequences remain monotonic so old Bot
// buttons cannot accidentally address a newly created activity.
func (s *Service) ResetFromNow(ctx context.Context) (time.Time, error) {
	tx, err := s.Store.Pool.Begin(ctx)
	if err != nil {
		return time.Time{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock(451010)`); err != nil {
		return time.Time{}, err
	}
	var since time.Time
	if err = tx.QueryRow(ctx, `update activity_config set collect_since=clock_timestamp() where id=1 returning collect_since`).Scan(&since); err != nil {
		return time.Time{}, err
	}
	_, err = tx.Exec(ctx, `truncate activities,activity_sources,activity_jobs,activity_deliveries continue identity cascade;
delete from messages; delete from summaries; delete from catch_ups; delete from daily_digests;
delete from activity_runtime; delete from activity_sync;`)
	if err != nil {
		return time.Time{}, err
	}
	_, err = tx.Exec(ctx, `insert into activity_sync(chat_id,synced_at) select id,$1 from chats where enabled`, since)
	if err != nil {
		return time.Time{}, err
	}
	return since, tx.Commit(ctx)
}
func (s *Service) Record(ctx context.Context, name, code string) {
	_, _ = s.Store.Pool.Exec(ctx, `insert into activity_runtime(name,code) values($1,$2) on conflict(name) do update set code=excluded.code,updated_at=now()`, name, code)
}
