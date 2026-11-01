package activity

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/bot"
)

func (s *Service) SendCalendarCallback(ctx context.Context, c Callback, owner int64, token string) error {
	if !AuthorizedCallback(c, owner) || !s.calendarOwnerReady(ctx, owner) {
		return fmt.Errorf("forbidden")
	}
	parts := strings.Split(c.Data, ":")
	if len(parts) != 2 || parts[0] != "cal" {
		return fmt.Errorf("invalid_callback")
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || id <= 0 {
		return fmt.Errorf("invalid_callback")
	}
	items, err := s.List(ctx)
	if err != nil {
		return err
	}
	for _, v := range items {
		if v.ID != id {
			continue
		}
		if v.Withdrawn {
			return fmt.Errorf("calendar_activity_unavailable")
		}
		file, err := CalendarFile(v, time.Now())
		if err != nil {
			return err
		}
		caption := "日历文件使用新加坡时间（UTC+8），可导入 Apple 日历、Google Calendar 或 Outlook。"
		payload := map[string]any{"chat_id": owner, "document": bot.Upload{Filename: fmt.Sprintf("activity-%d.ics", v.ID), ContentType: "text/calendar; charset=utf-8", Data: file}, "disable_content_type_detection": true}
		if keyboard := calendarLinkKeyboard(v); keyboard != nil {
			payload["reply_markup"] = keyboard
			caption += "也可点下方按钮，核对后保存。"
		}
		payload["caption"] = caption
		return s.bot.Call(ctx, token, "sendDocument", payload, nil)
	}
	return fmt.Errorf("calendar_activity_unavailable")
}

func (s *Service) handleCalendarCallback(ctx context.Context, c Callback, owner int64, token string) {
	notice := "正在生成日历文件…"
	if !AuthorizedCallback(c, owner) {
		notice = "仅限本人操作"
	}
	_ = s.bot.Call(ctx, token, "answerCallbackQuery", map[string]any{"callback_query_id": c.ID, "text": notice}, nil)
	if !AuthorizedCallback(c, owner) {
		return
	}
	if err := s.SendCalendarCallback(ctx, c, owner, token); err != nil {
		text := "日历文件发送失败，请稍后重试。"
		switch err.Error() {
		case "calendar_time_incomplete":
			text = "缺少开始或结束时间，请在网页活动卡补齐后导出。"
		case "calendar_activity_unavailable", "invalid_callback":
			text = "这条活动已不可用，请使用 /calendar 查看最新活动。"
		}
		_ = s.bot.Call(ctx, token, "sendMessage", map[string]any{"chat_id": owner, "text": text}, nil)
	}
}

func CalendarMenu(items []Event, now time.Time) (string, map[string]any) {
	rows := [][]map[string]string{}
	for _, v := range items {
		if v.Withdrawn || v.Status == "ignored" || !calendarTimesComplete(v.Details) || !cardStillRelevant(v.Details, now) {
			continue
		}
		name := v.NameZH
		if name == "" {
			name = v.NameEN
		}
		label := v.Start.In(Singapore).Format("01-02 15:04") + " " + name
		rows = append(rows, []map[string]string{{"text": shortField(label, 55), "callback_data": fmt.Sprintf("cal:%d", v.ID)}})
		if len(rows) == 20 {
			break
		}
	}
	if len(rows) == 0 {
		return "暂无可导出的活动，需要完整的开始和结束时间。", nil
	}
	return "选择活动，导出日历文件或打开 Google / Outlook 日历页面。", map[string]any{"inline_keyboard": rows}
}

func (s *Service) SendCalendarMenu(ctx context.Context, owner int64, token string) error {
	if !s.calendarOwnerReady(ctx, owner) {
		return fmt.Errorf("forbidden")
	}
	items, err := s.List(ctx)
	if err != nil {
		return err
	}
	text, keyboard := CalendarMenu(items, time.Now())
	payload := map[string]any{"chat_id": owner, "text": text}
	if keyboard != nil {
		payload["reply_markup"] = keyboard
	}
	return s.bot.Call(ctx, token, "sendMessage", payload, nil)
}

func (s *Service) calendarOwnerReady(ctx context.Context, owner int64) bool {
	cfg, err := s.Config(ctx)
	return err == nil && owner > 0 && cfg.OwnerID == owner && s.ownerReady(ctx, owner)
}

func (s *Service) handleCalendarCommand(ctx context.Context, text string, owner int64, token string) bool {
	parts := strings.Fields(text)
	if len(parts) == 0 || strings.Split(parts[0], "@")[0] != "/calendar" {
		return false
	}
	if err := s.SendCalendarMenu(ctx, owner, token); err != nil {
		s.Record(ctx, "bot", "calendar_delivery_failed")
	}
	return true
}
