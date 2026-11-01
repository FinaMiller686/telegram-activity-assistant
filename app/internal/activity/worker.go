package activity

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/config"
	"github.com/fr0der1c/tgtldr/app/internal/openai"
	"golang.org/x/sync/errgroup"
)

func LocalEndpoint(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1" || u.Hostname() == "host.docker.internal")
}
func (s *Service) Tick(ctx context.Context) error {
	cfg, e := s.Config(ctx)
	if e != nil {
		return e
	}
	rows, e := s.Store.Pool.Query(ctx, `select m.id,m.chat_id,m.telegram_message_id,m.text_content,m.raw_json::text,m.message_time,c.username,c.telegram_chat_id,c.chat_type,md5(m.text_content||m.raw_json::text),coalesce(j.attempts,0) from messages m join chats c on c.id=m.chat_id left join activity_jobs j on j.message_id=m.id where c.enabled and c.chat_type in ('group','supergroup','channel') and ((j.message_id is null and not exists(select 1 from activity_sources a where a.chat_id=m.chat_id and a.telegram_message_id=m.telegram_message_id and not a.deleted and a.fingerprint=md5(m.text_content||m.raw_json::text))) or j.fingerprint<>md5(m.text_content||m.raw_json::text) or (j.error_code<>'' and j.attempts<5 and j.next_attempt<=now())) order by m.message_time,m.id limit 10`)
	if e != nil {
		return e
	}
	type job struct {
		id, chatID, tgID            int64
		msg                         int
		text, raw, user, kind, hash string
		published                   time.Time
		attempts                    int
	}
	jobs := []job{}
	for rows.Next() {
		var j job
		if e = rows.Scan(&j.id, &j.chatID, &j.msg, &j.text, &j.raw, &j.published, &j.user, &j.tgID, &j.kind, &j.hash, &j.attempts); e != nil {
			rows.Close()
			return e
		}
		jobs = append(jobs, j)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	settings, e := s.Store.Settings.Get(ctx)
	if e != nil {
		return e
	}
	group, processingCtx := errgroup.WithContext(ctx)
	parallelism := settings.SummaryParallelism
	if parallelism < 1 {
		parallelism = 1
	}
	if parallelism > 4 {
		parallelism = 4
	}
	group.SetLimit(parallelism)
	for _, j := range jobs {
		group.Go(func() error {
			ctx := processingCtx
			var e error
			d := RuleExtract(j.text, j.raw, j.published)
			code := ""
			if cfg.AIEnabled && strings.TrimSpace(j.text) != "" {
				// Recheck monitoring and content immediately before sending queued text.
				var current bool
				e = s.Store.Pool.QueryRow(ctx, `select exists(select 1 from messages m join chats c on c.id=m.chat_id where m.id=$1 and c.enabled and c.chat_type in ('group','supergroup','channel') and md5(m.text_content||m.raw_json::text)=$2)`, j.id, j.hash).Scan(&current)
				if e != nil {
					return e
				}
				if !current {
					return nil
				}
				base := extractFields(j.text, j.raw, j.published)
				client := openai.New(openai.Config{BaseURL: settings.OpenAIBaseURL, APIKey: settings.OpenAIAPIKey, Model: settings.OpenAIModel, Timeout: 90 * time.Second})
				result, err := Enrich(ctx, client, j.text, *base, j.published)
				if err != nil {
					code = "model_request_failed"
				} else {
					d = result
					s.Record(ctx, "ai", "ok")
				}
			}
			if code == "" {
				if _, e = s.Save(ctx, j.chatID, j.msg, SourceLink(j.user, j.tgID, j.msg, j.kind), d, j.hash); e != nil {
					return e
				}
			}
			_, e = s.Store.Pool.Exec(ctx, `insert into activity_jobs(message_id,fingerprint,attempts,next_attempt,error_code) select id,$2,1,now()+interval '2 minutes',$3 from messages where id=$1 and md5(text_content||raw_json::text)=$2 on conflict(message_id) do update set attempts=case when activity_jobs.fingerprint<>excluded.fingerprint then 1 else activity_jobs.attempts+1 end,fingerprint=excluded.fingerprint,error_code=excluded.error_code,next_attempt=now()+interval '2 minutes'*least(activity_jobs.attempts+1,30),updated_at=now()`, j.id, j.hash, code)
			if e != nil {
				return e
			}
			if code != "" {
				s.Record(ctx, "extraction", code)
			}
			return nil
		})
	}
	if e = group.Wait(); e != nil {
		return e
	}
	// Raw text is short-lived. Activity metadata and source links remain manageable separately.
	_, e = s.Store.Pool.Exec(ctx, `delete from messages where message_time < now()-make_interval(days=>$1)`, cfg.RetentionDays)
	if e != nil {
		return e
	}
	var failed bool
	if e = s.Store.Pool.QueryRow(ctx, `select exists(select 1 from activity_jobs where error_code<>'')`).Scan(&failed); e != nil {
		return e
	}
	if !failed {
		s.Record(ctx, "extraction", "ok")
	}
	return nil
}
func (s *Service) Run(ctx context.Context) error {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		if !config.StorageProtected() {
			s.Record(ctx, "worker", "storage_encryption_required")
		} else if e := s.Tick(ctx); e != nil {
			s.Record(ctx, "worker", "database_or_processing_error")
		} else {
			s.Record(ctx, "worker", "ok")
		}
		if e := s.deliverIfProtected(ctx); e != nil {
			s.Record(ctx, "delivery", "bot_delivery_failed")
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
func displayTime(t *time.Time) string {
	if t == nil {
		return Unknown
	}
	return t.In(Singapore).Format("2006-01-02 15:04")
}
func shown(s string) string {
	if strings.TrimSpace(s) == "" {
		return Unknown
	}
	return s
}
func Card(v Event) string {
	name := v.NameZH
	if name == "" {
		name = v.NameEN
	}
	text := shortField(shown(name), 150)
	if v.NameZH != "" && v.NameEN != "" {
		text += "\n" + shortField(v.NameEN, 150)
	}
	if v.Start != nil {
		text += "\n时间：" + displayTime(v.Start)
		if v.End != nil {
			text += " 至 " + displayTime(v.End)
		}
		text += "（新加坡时间）"
	} else {
		dateText := v.Date
		if !hasActivityInfo(dateText) {
			dateText = v.DateText
		}
		if hasActivityInfo(dateText) {
			text += "\n日期：" + shortField(dateText, 150)
		}
		if hasActivityInfo(v.TimeText) {
			text += "\n时间：" + shortField(v.TimeText, 240)
		}
	}
	location := []string{}
	for _, part := range []string{v.Location, v.Floor, v.Room} {
		if hasActivityInfo(part) {
			location = append(location, part)
		}
	}
	if len(location) > 0 {
		text += "\n地点：" + shortField(strings.Join(location, " "), 240)
	}
	if hasActivityInfo(v.Summary) {
		text += "\n" + shortField(v.Summary, 450)
	}
	if v.Deadline != nil || hasActivityInfo(v.DeadlineDate) || hasActivityInfo(v.DeadlineText) {
		text += "\n报名截止：" + deadlineDisplay(v.Details)
	}

	if len(v.Conflicts) > 0 {
		text += "\n这两场活动存在时间冲突。"
	}
	for _, warning := range v.Warnings {
		if strings.HasPrefix(warning, "原文未写年份") {
			text += "\n" + warning
		}
	}
	if v.Withdrawn {
		text += "\n来源已删除或撤回，请核对。"
	}
	return text
}

func hasActivityInfo(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "未知", "unknown", "原文未注明", "未提供", "n/a", "null", "tba", "tbc", "待定":
		return false
	}
	return true
}

func shortField(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit]) + "…"
}

func deadlineDisplay(d Details) string {
	if d.Deadline != nil {
		return displayTime(d.Deadline)
	}
	if d.DeadlineDate != "" {
		return d.DeadlineDate
	}
	if d.DeadlineText != "" {
		return shortField(d.DeadlineText, 200)
	}
	return "原文未注明"
}

func (s *Service) deliverIfProtected(ctx context.Context) error {
	if !config.StorageProtected() {
		return nil
	}
	return s.Deliver(ctx)
}
