package activity

import (
	"strings"
	"testing"
	"time"
)

func calendarFixture() Event {
	start := time.Date(2026, 10, 29, 23, 30, 0, 0, Singapore)
	end := time.Date(2026, 10, 30, 1, 0, 0, 0, Singapore)
	return Event{ID: 12, Details: Details{NameZH: "研究讲座", NameEN: "Research Seminar", Summary: "讲座介绍", Location: "Hall A", Floor: "2", Room: "101", RegistrationURL: "https://example.org/form", Start: &start, End: &end}, Sources: []Source{{Link: "https://t.me/example/123"}}}
}

func TestCalendarFileMatchesWebExportContract(t *testing.T) {
	v := calendarFixture()
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	file, err := CalendarFile(v, now)
	if err != nil {
		t.Fatal(err)
	}
	text := string(file)
	unfolded := strings.ReplaceAll(text, "\r\n ", "")
	for _, want := range []string{"UID:activity-12@tgtldr.local", "DTSTAMP:20261001T000000Z", "BEGIN:VTIMEZONE", "TZID:Asia/Singapore", "TZOFFSETTO:+0800", "DTSTART;TZID=Asia/Singapore:20261029T233000\r\nDTEND;TZID=Asia/Singapore:20261030T010000", "SUMMARY:研究讲座", "https://t.me/example/123", "https://example.org/form"} {
		if !strings.Contains(unfolded, want) {
			t.Errorf("missing %s", want)
		}
	}
	v.NameZH = strings.Repeat("讲座\r\nATTENDEE:other@example.org,;", 12)
	file, err = CalendarFile(v, now)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(file), "\r\nATTENDEE:") {
		t.Fatal("calendar property injection")
	}
	for _, line := range strings.Split(string(file), "\r\n") {
		if len(line) > 75 {
			t.Fatal("calendar line exceeds UTF-8 byte limit")
		}
	}
}

func TestCalendarConvertsStoredUTCToSingapore(t *testing.T) {
	v := calendarFixture()
	start, end := v.Start.UTC(), v.End.UTC()
	v.Start, v.End = &start, &end
	file, err := CalendarFile(v, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(file), "DTSTART;TZID=Asia/Singapore:20261029T233000") || !strings.Contains(string(file), "DTEND;TZID=Asia/Singapore:20261030T010000") {
		t.Fatal("stored UTC time was not converted to Singapore")
	}
}

func TestCalendarRequiresActualStartAndEnd(t *testing.T) {
	v := calendarFixture()
	v.End = nil
	if _, err := CalendarFile(v, time.Now()); err == nil {
		t.Fatal("invented an end time")
	}
	if strings.Contains(stringMustJSON(Keyboard(v)), "cal:12") {
		t.Fatal("incomplete activity has a calendar button")
	}
	v = calendarFixture()
	v.End = v.Start
	if _, err := CalendarFile(v, time.Now()); err == nil {
		t.Fatal("invalid interval accepted")
	}
	v = calendarFixture()
	if !strings.Contains(stringMustJSON(Keyboard(v)), "cal:12") {
		t.Fatal("complete activity has no calendar button")
	}
}

func TestCalendarMenuFiltersAndMissingFieldsStayHidden(t *testing.T) {
	now := time.Date(2026, 10, 28, 0, 0, 0, 0, Singapore)
	v := calendarFixture()
	missing := v
	missing.ID = 13
	missing.End = nil
	past := v
	past.ID = 14
	end := now.Add(-time.Hour)
	past.End = &end
	text, keyboard := CalendarMenu([]Event{v, missing, past}, now)
	if text == "" || keyboard == nil {
		t.Fatal("calendar picker missing")
	}
	encoded := stringMustJSON(keyboard)
	if !strings.Contains(encoded, "cal:12") || strings.Contains(encoded, "cal:13") || strings.Contains(encoded, "cal:14") {
		t.Fatal("invalid picker choices")
	}
	card := Card(Event{ID: 1, Details: Details{NameEN: "Only Known Title", Location: "未知", Summary: "可确认的简介"}})
	for _, unwanted := range []string{"未知", "原文未注明", "报名截止：", "地点：", "时间："} {
		if strings.Contains(card, unwanted) {
			t.Fatalf("missing fact shown in card: %s", unwanted)
		}
	}
}
