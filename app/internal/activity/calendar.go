package activity

import (
	"fmt"
	"strings"
	"time"
)

func calendarEscape(value string) string {
	value = strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\r", "\n")
	value = strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' || r == 127 {
			return -1
		}
		return r
	}, value)
	return strings.NewReplacer("\\", "\\\\", "\n", "\\n", ",", "\\,", ";", "\\;").Replace(value)
}

func calendarFold(line string) string {
	var out strings.Builder
	bytes := 0
	for _, r := range line {
		text := string(r)
		if bytes+len(text) > 75 {
			out.WriteString("\r\n ")
			bytes = 1
		}
		out.WriteString(text)
		bytes += len(text)
	}
	return out.String()
}

// CalendarFile uses explicit Singapore event times. DTSTAMP remains UTC as
// required by iCalendar. It does not access any system calendar.
func CalendarFile(v Event, now time.Time) ([]byte, error) {
	if v.ID <= 0 {
		return nil, fmt.Errorf("calendar_activity_invalid")
	}
	if !calendarTimesComplete(v.Details) {
		return nil, fmt.Errorf("calendar_time_incomplete")
	}
	name := v.NameZH
	if name == "" {
		name = v.NameEN
	}
	if name == "" {
		name = "未命名活动"
	}
	description := []string{}
	if v.NameZH != "" && v.NameEN != "" {
		description = append(description, "英文名称："+v.NameEN)
	}
	if v.Summary != "" {
		description = append(description, v.Summary)
	}
	if ValidURL(v.RegistrationURL) {
		description = append(description, "报名链接："+v.RegistrationURL)
	}
	for _, source := range v.Sources {
		if !source.Deleted && ValidURL(source.Link) {
			description = append(description, "原始消息："+source.Link)
			break
		}
	}
	description = append(description, v.Warnings...)
	stamp := func(t time.Time) string { return t.UTC().Format("20060102T150405Z") }
	localStamp := func(t time.Time) string { return t.In(Singapore).Format("20060102T150405") }
	lines := []string{
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//TGTLDR//Activity Assistant//ZH", "CALSCALE:GREGORIAN",
		"X-WR-TIMEZONE:Asia/Singapore",
		"BEGIN:VTIMEZONE", "TZID:Asia/Singapore", "X-LIC-LOCATION:Asia/Singapore",
		"BEGIN:STANDARD", "DTSTART:19820101T000000", "TZOFFSETFROM:+0800", "TZOFFSETTO:+0800", "TZNAME:SGT", "END:STANDARD", "END:VTIMEZONE",
		"BEGIN:VEVENT", fmt.Sprintf("UID:activity-%d@tgtldr.local", v.ID), "DTSTAMP:" + stamp(now),
		"DTSTART;TZID=Asia/Singapore:" + localStamp(*v.Start), "DTEND;TZID=Asia/Singapore:" + localStamp(*v.End),
		"SUMMARY:" + calendarEscape(name), "DESCRIPTION:" + calendarEscape(strings.Join(description, "\n")),
	}
	location := strings.TrimSpace(strings.Join([]string{v.Location, v.Floor, v.Room}, " "))
	if location != "" {
		lines = append(lines, "LOCATION:"+calendarEscape(location))
	}
	lines = append(lines, "END:VEVENT", "END:VCALENDAR")
	for i, line := range lines {
		lines[i] = calendarFold(line)
	}
	return []byte(strings.Join(lines, "\r\n") + "\r\n"), nil
}

func calendarTimesComplete(d Details) bool {
	return d.Start != nil && d.End != nil && !d.Start.IsZero() && d.End.After(*d.Start)
}
