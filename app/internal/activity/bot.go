package activity

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Callback struct {
	ID   string `json:"id"`
	Data string `json:"data"`
	From struct {
		ID int64 `json:"id"`
	} `json:"from"`
	Message struct {
		Chat struct {
			ID   int64  `json:"id"`
			Type string `json:"type"`
		} `json:"chat"`
	} `json:"message"`
}

func AuthorizedCallback(c Callback, owner int64) bool {
	return owner > 0 && c.From.ID == owner && c.Message.Chat.ID == owner && c.Message.Chat.Type == "private"
}
func (s *Service) ApplyCallback(ctx context.Context, c Callback, owner int64) (string, error) {
	if !AuthorizedCallback(c, owner) {
		return "", fmt.Errorf("forbidden")
	}
	parts := strings.Split(c.Data, ":")
	if len(parts) != 3 || parts[0] != "act" || !ValidStatus(parts[2]) {
		return "", fmt.Errorf("invalid_callback")
	}
	id, e := strconv.ParseInt(parts[1], 10, 64)
	if e != nil || id <= 0 {
		return "", fmt.Errorf("invalid_callback")
	}
	if e = s.SetStatus(ctx, id, parts[2]); e != nil {
		return "", e
	}
	items, e := s.List(ctx)
	if e != nil {
		return "", e
	}
	for _, v := range items {
		if v.ID == id {
			reply := "已保存。"
			if parts[2] == "going" {
				if ValidURL(v.RegistrationURL) {
					reply = "已加入待参加清单。报名链接：\n" + v.RegistrationURL
				} else {
					reply = "已加入待参加清单，但未找到有效报名链接。请查看原始消息。"
				}
				linked := false
				for _, source := range v.Sources {
					if source.Link != "" && !source.Deleted {
						reply += "\n原始消息：" + source.Link
						linked = true
						break
					}
				}
				if !linked && len(v.Sources) > 0 {
					reply += "\n来源群组：" + v.Sources[0].Title + "（原消息链接不可用）"
				}
			}
			if parts[2] == "registered" {
				reply = "已标记为已报名。"
			}
			if len(v.Conflicts) > 0 {
				reply += "\n这两场活动存在时间冲突。"
			}
			return reply, nil
		}
	}
	return "", fmt.Errorf("activity_not_found")
}
func Keyboard(v Event) map[string]any {
	buttons := [][]map[string]string{{{"text": "我要参加", "callback_data": fmt.Sprintf("act:%d:going", v.ID)}, {"text": "稍后决定", "callback_data": fmt.Sprintf("act:%d:later", v.ID)}}, {{"text": "不感兴趣", "callback_data": fmt.Sprintf("act:%d:ignored", v.ID)}, {"text": "已完成报名", "callback_data": fmt.Sprintf("act:%d:registered", v.ID)}}}
	if calendarTimesComplete(v.Details) {
		buttons = append(buttons, []map[string]string{{"text": "添加到日历", "callback_data": fmt.Sprintf("cal:%d", v.ID)}})
	}
	for _, source := range v.Sources {
		if !source.Deleted && ValidURL(source.Link) {
			buttons = append(buttons, []map[string]string{{"text": "查看原始消息", "url": source.Link}})
			break
		}
	}
	return map[string]any{"inline_keyboard": buttons}
}
func trimMessage(s string) string {
	r := []rune(s)
	if len(r) > 3400 {
		return string(r[:3400]) + "\n…请在管理界面查看完整信息"
	}
	return s
}
func (s *Service) ownerReady(ctx context.Context, owner int64) bool {
	var ready bool
	_ = s.Store.Pool.QueryRow(ctx, `select exists(select 1 from telegram_accounts where telegram_user_id=$1 and status='authorized')`, owner).Scan(&ready)
	return owner > 0 && ready
}
func (s *Service) PollBot(ctx context.Context) error {
	transport := s.bot
	for {
		if ctx.Err() != nil {
			return nil
		}
		cfg, e := s.Config(ctx)
		if e != nil {
			return e
		}
		settings, e := s.Store.Settings.Get(ctx)
		if e != nil {
			return e
		}
		if !settings.BotEnabled || settings.BotToken == "" || !s.ownerReady(ctx, cfg.OwnerID) {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(10 * time.Second):
				continue
			}
		}
		var offset int64
		if e = s.Store.Pool.QueryRow(ctx, `select bot_offset from activity_config where id=1`).Scan(&offset); e != nil {
			return e
		}
		var updates []struct {
			ID       int64     `json:"update_id"`
			Callback *Callback `json:"callback_query"`
			Message  *struct {
				Text string `json:"text"`
				From struct {
					ID int64 `json:"id"`
				} `json:"from"`
				Chat struct {
					ID   int64  `json:"id"`
					Type string `json:"type"`
				} `json:"chat"`
			} `json:"message"`
		}
		e = transport.Call(ctx, settings.BotToken, "getUpdates", map[string]any{"offset": offset, "timeout": 10, "allowed_updates": []string{"callback_query", "message"}}, &updates)
		if e != nil {
			s.Record(ctx, "bot", "poll_failed_check_token_or_webhook")
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(15 * time.Second):
				continue
			}
		}
		for _, u := range updates {
			if u.Callback != nil {
				if strings.HasPrefix(u.Callback.Data, "cal:") {
					s.handleCalendarCallback(ctx, *u.Callback, cfg.OwnerID, settings.BotToken)
				} else {
					reply, e := s.ApplyCallback(ctx, *u.Callback, cfg.OwnerID)
					notice := "仅限本人操作"
					if e == nil {
						notice = "已保存"
						_ = transport.Call(ctx, settings.BotToken, "sendMessage", map[string]any{"chat_id": cfg.OwnerID, "text": trimMessage(reply)}, nil)
					}
					_ = transport.Call(ctx, settings.BotToken, "answerCallbackQuery", map[string]any{"callback_query_id": u.Callback.ID, "text": notice}, nil)
				}
			}
			if m := u.Message; m != nil && m.Chat.Type == "private" && m.Chat.ID == cfg.OwnerID && m.From.ID == cfg.OwnerID {
				if !s.handleCalendarCommand(ctx, m.Text, cfg.OwnerID, settings.BotToken) {
					reply := "使用 /week 查看未来 7 天，/going 查看待参加和已报名活动，/calendar 导出日历文件或打开 Google / Outlook。"
					if m.Text == "/week" || m.Text == "/going" {
						items, err := s.List(ctx)
						if err == nil {
							reply = ""
							now := time.Now().In(Singapore)
							for _, v := range items {
								match := v.Date != "" && v.Date >= now.Format("2006-01-02") && v.Date <= now.AddDate(0, 0, 7).Format("2006-01-02")
								if m.Text == "/going" {
									match = v.Status == "going" || v.Status == "registered"
								}
								if match && !v.Withdrawn && v.Status != "ignored" {
									reply += Card(v) + "\n\n"
								}
							}
							if reply == "" {
								reply = "暂无符合条件的活动。"
							}
						}
					}
					for _, part := range splitMessage(reply) {
						_ = transport.Call(ctx, settings.BotToken, "sendMessage", map[string]any{"chat_id": cfg.OwnerID, "text": part}, nil)
					}
				}
			}
			if _, e = s.Store.Pool.Exec(ctx, `update activity_config set bot_offset=$1 where id=1`, u.ID+1); e != nil {
				return e
			}
		}
		s.Record(ctx, "bot", "ok")
	}
}
func Digest(items []Event, now time.Time, hours int) string {
	today := now.In(Singapore).Format("2006-01-02")
	text := "今日活动摘要（" + today + "，新加坡时间）\n"
	groups := []struct {
		title string
		match func(Event) bool
	}{{"今日新增", func(v Event) bool { return v.Created.In(Singapore).Format("2006-01-02") == today }}, {"未来 7 天", func(v Event) bool {
		return v.Date >= today && v.Date <= now.In(Singapore).AddDate(0, 0, 7).Format("2006-01-02")
	}}, {"即将截止报名", func(v Event) bool {
		return reminderNeeded(v.Details, now, hours)
	}}, {"其他重要通知", func(v Event) bool {
		return v.Category == "其他重要通知" && v.Created.In(Singapore).Format("2006-01-02") == today
	}}}
	for _, g := range groups {
		text += "\n" + g.title + "\n"
		count := 0
		for _, v := range items {
			if !v.Withdrawn && v.Status != "ignored" && g.match(v) {
				text += fmt.Sprintf("• #%d %s %s｜%s｜%s\n", v.ID, shown(v.NameZH), v.NameEN, shown(v.Date), shown(v.Location))
				if v.RegistrationURL != "" {
					text += v.RegistrationURL + "\n"
				}
				count++
			}
		}
		if count == 0 {
			text += "暂无\n"
		}
	}
	return text
}
func (s *Service) Deliver(ctx context.Context) error {
	cfg, e := s.Config(ctx)
	if e != nil {
		return e
	}
	items, e := s.List(ctx)
	if e != nil {
		return e
	}
	now := time.Now().In(Singapore)
	digestKey := "digest:" + now.Format("2006-01-02")
	digest := Digest(items, now, cfg.ReminderHours)
	if digestDue(now, cfg) {
		_, e = s.Store.Pool.Exec(ctx, `insert into activity_deliveries(key,content) values($1,$2) on conflict do nothing`, digestKey, digest)
		if e != nil {
			return e
		}
	}
	settings, e := s.Store.Settings.Get(ctx)
	if e != nil {
		return e
	}
	if !settings.BotEnabled || settings.BotToken == "" || !s.ownerReady(ctx, cfg.OwnerID) {
		return nil
	}
	var failures []error
	for _, v := range items {
		if v.Status == "ignored" || v.Withdrawn || !cardStillRelevant(v.Details, now) {
			if _, e = s.Store.Pool.Exec(ctx, `update activity_deliveries set skipped_at=now(),skip_reason='inactive' where (key like $1 or key like $2) and sent_at is null and skipped_at is null`, fmt.Sprintf("card:%d:%%", v.ID), fmt.Sprintf("deadline:%d:%%", v.ID)); e != nil {
				return e
			}
			continue
		}
		if e = s.sendOnce(ctx, fmt.Sprintf("card:%d:%d", v.ID, v.Revision), Card(v), Keyboard(v), cfg.OwnerID, settings.BotToken); e != nil {
			failures = append(failures, e)
		}
		deadlineKey := ""
		if v.Status != "registered" && v.Status != "attended" && reminderNeeded(v.Details, now, cfg.ReminderHours) {
			date := v.DeadlineDate
			if v.Deadline != nil {
				date = v.Deadline.Format(time.RFC3339)
			}
			deadlineKey = fmt.Sprintf("deadline:%d:%s", v.ID, date)
		}
		if _, e = s.Store.Pool.Exec(ctx, `update activity_deliveries set skipped_at=now(),skip_reason='superseded' where key like $1 and key<>$2 and sent_at is null and skipped_at is null`, fmt.Sprintf("deadline:%d:%%", v.ID), deadlineKey); e != nil {
			return e
		}
		if deadlineKey != "" {
			if e = s.sendOnce(ctx, deadlineKey, "报名截止日期临近，请核对具体时间\n"+Card(v), Keyboard(v), cfg.OwnerID, settings.BotToken); e != nil {
				failures = append(failures, e)
			}
		}
	}
	// Send the stored snapshot, also after a restart. Never replace digest history on retry.
	rows, e := s.Store.Pool.Query(ctx, `select key,content from activity_deliveries where key like 'digest:%' and sent_at is null order by key limit 7`)
	if e != nil {
		return e
	}
	type delivery struct{ key, text string }
	pending := []delivery{}
	for rows.Next() {
		var d delivery
		if e = rows.Scan(&d.key, &d.text); e != nil {
			rows.Close()
			return e
		}
		pending = append(pending, d)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, d := range pending {
		complete := true
		for index, part := range splitMessage(d.text) {
			partKey := fmt.Sprintf("digestpart:%s:%d", d.key, index)
			if e = s.sendOnce(ctx, partKey, part, nil, cfg.OwnerID, settings.BotToken); e != nil {
				failures = append(failures, e)
				complete = false
				break
			}
			var sent bool
			if e = s.Store.Pool.QueryRow(ctx, `select sent_at is not null from activity_deliveries where key=$1`, partKey).Scan(&sent); e != nil {
				return e
			}
			if !sent {
				complete = false
				break
			}
		}
		if complete {
			if _, e = s.Store.Pool.Exec(ctx, `update activity_deliveries set sent_at=now() where key=$1`, d.key); e != nil {
				return e
			}
		}
	}
	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	var retrying bool
	if e = s.Store.Pool.QueryRow(ctx, `select exists(select 1 from activity_deliveries where sent_at is null and skipped_at is null and last_error_code<>'')`).Scan(&retrying); e != nil {
		return e
	}
	code := "ok"
	if retrying {
		code = "bot_delivery_retrying"
	}
	s.Record(ctx, "delivery", code)
	return nil
}

func digestDue(now time.Time, cfg Config) bool {
	now = now.In(Singapore)
	scheduled, err := time.ParseInLocation("2006-01-02 15:04", now.Format("2006-01-02")+" "+cfg.DigestTime, Singapore)
	return err == nil && !now.Before(scheduled) && (cfg.CollectSince == nil || !cfg.CollectSince.After(scheduled))
}

func cardStillRelevant(d Details, now time.Time) bool {
	if d.End != nil {
		return d.End.After(now)
	}
	return d.Date == "" || d.Date >= now.In(Singapore).Format("2006-01-02")
}

func splitMessage(text string) []string {
	const limit = 3000
	runes := []rune(text)
	parts := []string{}
	for len(runes) > limit {
		cut := limit
		for i := limit - 1; i >= limit/2; i-- {
			if runes[i] == '\n' {
				cut = i + 1
				break
			}
		}
		parts = append(parts, string(runes[:cut]))
		runes = runes[cut:]
	}
	if len(runes) > 0 {
		parts = append(parts, string(runes))
	}
	return parts
}

func reminderNeeded(d Details, now time.Time, hours int) bool {
	if d.Deadline != nil {
		return d.Deadline.After(now) && !d.Deadline.After(now.Add(time.Duration(hours)*time.Hour))
	}
	if d.DeadlineDate == "" {
		return false
	}
	day, err := time.ParseInLocation("2006-01-02", d.DeadlineDate, Singapore)
	if err != nil {
		return false
	}
	// A date without a time is not treated as a known midnight deadline.
	return !now.Before(day.Add(-time.Duration(hours)*time.Hour)) && now.Before(day.AddDate(0, 0, 1))
}
