package activity

import (
	"net/url"
	"strings"
	"testing"
)

func TestCalendarDraftLinks(t *testing.T) {
	v := calendarFixture()
	v.NameZH = "研究 & 招生 + #1"
	v.Warnings = []string{"请核对年份"}
	links := CalendarLinks(v)
	if len(links) != 3 {
		t.Fatal("missing provider")
	}
	for provider, raw := range links {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		q := u.Query()
		if q.Get("location") != "Hall A 2 101" {
			t.Fatal("location changed")
		}
		if provider == "google" {
			if u.Host != "calendar.google.com" || q.Get("text") != v.NameZH || q.Get("dates") != "20261029T153000Z/20261029T170000Z" || q.Get("ctz") != "Asia/Singapore" {
				t.Fatal("wrong Google draft")
			}
			if !strings.Contains(q.Get("details"), "请核对年份") {
				t.Fatal("warning lost")
			}
		} else {
			want := "outlook.live.com"
			if provider == "office365" {
				want = "outlook.office.com"
			}
			if u.Host != want || q.Get("subject") != v.NameZH || q.Get("startdt") != "2026-10-29T15:30:00Z" || q.Get("enddt") != "2026-10-29T17:00:00Z" {
				t.Fatal("wrong Outlook draft")
			}
			if !strings.Contains(q.Get("body"), "请核对年份") {
				t.Fatal("warning lost")
			}
		}
	}
	v.Summary = strings.Repeat("<img src=bad>很长的简介", 2000)
	for provider, raw := range CalendarLinks(v) {
		if len(raw) > 2000 {
			t.Fatal("oversized URL")
		}
		u, _ := url.Parse(raw)
		if provider != "google" && strings.Contains(u.Query().Get("body"), "<img") {
			t.Fatal("active markup in draft")
		}
	}
	v.NameZH = strings.Repeat("长", 3000)
	if len(CalendarLinks(v)) != 0 {
		t.Fatal("changed important fields to fit URL")
	}
	v = calendarFixture()
	v.End = nil
	if len(CalendarLinks(v)) != 0 || calendarLinkKeyboard(v) != nil {
		t.Fatal("guessed missing time")
	}
}
