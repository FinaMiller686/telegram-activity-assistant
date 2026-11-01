package activity

import (
	"html"
	"net/url"
	"strings"
)

// CalendarLinks creates draft links only. No request is made to a calendar provider.
func CalendarLinks(v Event) map[string]string {
	links := map[string]string{}
	if v.ID <= 0 || !calendarTimesComplete(v.Details) {
		return links
	}
	title := v.NameZH
	if title == "" {
		title = v.NameEN
	}
	if title == "" {
		title = "未命名活动"
	}
	location := []string{}
	for _, value := range []string{v.Location, v.Floor, v.Room} {
		if value != "" {
			location = append(location, value)
		}
	}
	source := ""
	for _, item := range v.Sources {
		if !item.Deleted && ValidURL(item.Link) {
			source = "原始消息：" + item.Link
			break
		}
	}
	bases := map[string]string{
		"google":    "https://calendar.google.com/calendar/render",
		"outlook":   "https://outlook.live.com/calendar/0/deeplink/compose",
		"office365": "https://outlook.office.com/calendar/0/deeplink/compose",
	}
	for provider, base := range bases {
		for _, limit := range []int{500, 200, 80, 0} {
			description := []string{}
			if limit > 0 && v.Summary != "" {
				runes := []rune(v.Summary)
				summary := v.Summary
				if len(runes) > limit {
					summary = string(runes[:limit]) + "…"
				}
				description = append(description, summary)
			}
			description = append(description, v.Warnings...)
			if source != "" {
				description = append(description, source)
			}
			body := strings.Join(description, "\n")
			q := url.Values{"location": {strings.TrimSpace(strings.Join(location, " "))}}
			if provider == "google" {
				q.Set("action", "TEMPLATE")
				q.Set("text", title)
				q.Set("dates", v.Start.UTC().Format("20060102T150405Z")+"/"+v.End.UTC().Format("20060102T150405Z"))
				q.Set("ctz", "Asia/Singapore")
				q.Set("details", body)
			} else {
				q.Set("path", "/calendar/action/compose")
				q.Set("rru", "addevent")
				q.Set("subject", title)
				q.Set("startdt", v.Start.UTC().Format("2006-01-02T15:04:05Z"))
				q.Set("enddt", v.End.UTC().Format("2006-01-02T15:04:05Z"))
				body = strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\r", "\n")
				q.Set("body", strings.ReplaceAll(html.EscapeString(body), "\n", "<br>"))
			}
			link := base + "?" + q.Encode()
			if len(link) <= 2000 {
				links[provider] = link
				break
			}
		}
	}
	return links
}

func calendarLinkKeyboard(v Event) map[string]any {
	links := CalendarLinks(v)
	rows := [][]map[string]string{}
	for _, item := range []struct{ key, label string }{{"google", "Google Calendar"}, {"office365", "Outlook（学校/工作）"}, {"outlook", "Outlook.com（个人）"}} {
		if link := links[item.key]; link != "" {
			rows = append(rows, []map[string]string{{"text": item.label, "url": link}})
		}
	}
	if len(rows) == 0 {
		return nil
	}
	return map[string]any{"inline_keyboard": rows}
}
