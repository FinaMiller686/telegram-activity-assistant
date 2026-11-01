package activity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/openai"
)

func anchor() time.Time { return time.Date(2026, 10, 29, 23, 30, 0, 0, Singapore) }
func TestDates(t *testing.T) {
	for _, tc := range []struct{ in, want string }{{"Thursday, 29 October 2026", "2026-10-29"}, {"Oct 30, 2026", "2026-10-30"}, {"29th Oct 2026", "2026-10-29"}, {"today", "2026-10-29"}, {"tomorrow", "2026-10-30"}, {"next Monday", "2026-11-02"}, {"2026-11-02", "2026-11-02"}} {
		t.Run(tc.in, func(t *testing.T) {
			d, e := ParseDate(tc.in, anchor())
			if e != nil || d.Format("2006-01-02") != tc.want {
				t.Fatalf("%v %v", d, e)
			}
		})
	}
	for _, bad := range []string{"31 February 2026", "29 October", "soon", ""} {
		if d, e := ParseDate(bad, anchor()); e == nil || d != nil {
			t.Fatalf("guessed %q", bad)
		}
	}
}
func TestRelativeDateUsesMessageTimezone(t *testing.T) {
	d, _ := ParseDate("today", time.Date(2026, 10, 29, 18, 0, 0, 0, time.UTC))
	if d.Format("2006-01-02") != "2026-10-30" {
		t.Fatal(d)
	}
}
func TestClocks(t *testing.T) {
	for _, tc := range []struct{ in, start, end string }{{"7-9pm", "19:00", "21:00"}, {"19:30 to 21:00", "19:30", "21:00"}, {"11pm–1am", "23:00", "01:00"}, {"10am-12pm", "10:00", "12:00"}} {
		t.Run(tc.in, func(t *testing.T) {
			a, b := ParseTimes(tc.in, anchor())
			if a == nil || b == nil || a.Format("15:04") != tc.start || b.Format("15:04") != tc.end || !b.After(*a) {
				t.Fatalf("%v %v", a, b)
			}
		})
	}
	a, b := ParseTimes("2026-10-29", anchor())
	if a != nil || b != nil {
		t.Fatal("ISO date mistaken for time")
	}
	a, _ = ParseTimes("29 October 2026", anchor())
	if a != nil {
		t.Fatal("date mistaken for time")
	}
}
func TestExtractionMissingAndDeadline(t *testing.T) {
	d := RuleExtract("Research Seminar\nThursday, 29 October 2026, 7pm-9pm\nVenue: Demo Hall\nRegistration deadline: 28 October 2026, 5pm", "{}", anchor())
	if d == nil || d.Date != "2026-10-29" || d.Deadline == nil || d.Deadline.Day() != 28 || d.RegistrationURL != "" || d.Location != "Demo Hall" {
		t.Fatalf("%+v", d)
	}
	if RuleExtract("lol 😂", "{}", anchor()) != nil {
		t.Fatal("chatter")
	}
	d = RuleExtract("Career Event\nDate: tomorrow\nTime: 11pm-1am", "{}", anchor())
	if d.End.Day() != 31 {
		t.Fatalf("overnight %v", d.End)
	}
}

func TestRuleExtractionLabeledNotice(t *testing.T) {
	text := "📣 Event: Quantitative Finance Workshop\n📅 Date: Thursday, 29 October 2026\n🕒 Time: 7pm–9pm\n📍 Venue: Nanyang Auditorium, Room 101\n🏫 Organised by: Example Club\n⏳ Registration deadline: 28 October 2026, 5pm\n🔗 Register here: https://forms.gle/example"
	d := RuleExtract(text, "{}", anchor())
	if d == nil || d.NameEN != "Quantitative Finance Workshop" || d.Date != "2026-10-29" || d.Start == nil || d.Start.Hour() != 19 || d.End == nil || d.End.Hour() != 21 || d.Location != "Nanyang Auditorium, Room 101" || d.Organizer != "Example Club" || d.Deadline == nil || d.RegistrationURL != "https://forms.gle/example" || d.RegistrationRequired == nil || !*d.RegistrationRequired {
		t.Fatalf("failed to extract labeled notice: %+v", d)
	}
	if !strings.Contains(d.Summary, "Quantitative Finance Workshop") || !strings.Contains(d.Summary, "2026-10-29") {
		t.Fatalf("summary does not include extracted facts: %q", d.Summary)
	}
}

func TestRuleExtractionDoesNotGuess(t *testing.T) {
	d := RuleExtract("Career announcement\nRegistration details will be announced later.\nDate: 29 October\n📍 Location: Hall A", "{}", anchor())
	if d == nil || d.Date != "" || d.RegistrationRequired != nil || d.RegistrationURL != "" || d.Location != "Hall A" {
		t.Fatalf("unexpected guesses: %+v", d)
	}
	if !strings.Contains(strings.Join(d.Warnings, " "), "29 October") {
		t.Fatal("yearless original date was not preserved for review")
	}
	d = RuleExtract("📅 Date: tomorrow\n📍 Venue: Hall B\n🎓 Event: Career Talk\nNo registration required", "{}", anchor())
	if d == nil || d.NameEN != "Career Talk" || d.Date != "2026-10-30" || d.RegistrationRequired == nil || *d.RegistrationRequired {
		t.Fatalf("bad explicit fields: %+v", d)
	}
}

func TestCardUsesOriginalNameWhenTranslationUnavailable(t *testing.T) {
	card := Card(Event{Details: Details{NameEN: "Research Seminar"}})
	if card != "Research Seminar" {
		t.Fatalf("missing original name: %q", card)
	}
}
func TestRegistrationLinks(t *testing.T) {
	tests := []struct{ text, raw, want string }{{"RSVP at https://example.org/apply", "{}", "https://example.org/apply"}, {"Campus workshop RSVP", "{\"ReplyMarkup\":{\"Rows\":[{\"Buttons\":[{\"Text\":\"Register\",\"URL\":\"https://bit.ly/demo\"}]}]}}", "https://bit.ly/demo"}, {"😀 Register now", "{\"Entities\":[{\"Offset\":3,\"Length\":8,\"URL\":\"https://example.org/form\"}]}", "https://example.org/form"}, {"See https://example.org/about", "{}", ""}, {"Sign up https://t.me/demo", "{}", ""}, {"Workshop", "{\"Url\":\"javascript:alert(1)\"}", ""}}
	for _, tc := range tests {
		links := Links(tc.text, tc.raw)
		got := ""
		if len(links) > 0 {
			got = links[0]
		}
		if got != tc.want {
			t.Fatalf("%q %v", tc.text, links)
		}
	}
}
func TestDedupeAndRooms(t *testing.T) {
	a := Details{NameEN: "Research Seminar", Date: "2026-10-29", Location: "Hall", Room: "101"}
	b := a
	b.NameEN = "FINAL CALL: Research Seminar"
	if !SameEvent(a, b) {
		t.Fatal("repost")
	}
	b.Room = "102"
	if SameEvent(a, b) {
		t.Fatal("different room")
	}
	b = a
	b.NameEN = "Finance Seminar"
	if SameEvent(a, b) {
		t.Fatal("same venue different event")
	}
	b = a
	b.Date = "2026-11-01"
	if SameEvent(a, b) {
		t.Fatal("different date")
	}
}
func TestConflicts(t *testing.T) {
	a, b := anchor(), anchor().Add(time.Hour)
	c, d := anchor().Add(30*time.Minute), anchor().Add(2*time.Hour)
	if !Overlap(Details{Start: &a, End: &b}, Details{Start: &c, End: &d}) {
		t.Fatal("expected conflict")
	}
	if Overlap(Details{Start: &a, End: &b}, Details{Start: &b, End: &d}) {
		t.Fatal("touching boundaries")
	}
	if Overlap(Details{Start: &a}, Details{Start: &c, End: &d}) {
		t.Fatal("guessed end")
	}
}
func TestBotIdentity(t *testing.T) {
	var c Callback
	c.From.ID = 42
	c.Message.Chat.ID = 42
	c.Message.Chat.Type = "private"
	if !AuthorizedCallback(c, 42) {
		t.Fatal("owner")
	}
	for _, owner := range []int64{0, 43, -42} {
		if AuthorizedCallback(c, owner) {
			t.Fatal("unauthorized")
		}
	}
	c.Message.Chat.Type = "group"
	if AuthorizedCallback(c, 42) {
		t.Fatal("group")
	}
}
func TestLocalEndpoint(t *testing.T) {
	for _, s := range []string{"http://127.0.0.1:11434/v1", "http://host.docker.internal:11434/v1"} {
		if !LocalEndpoint(s) {
			t.Fatal(s)
		}
	}
	for _, s := range []string{"https://localhost.attacker.test/v1", "https://api.example.org/v1", "file:///tmp/model"} {
		if LocalEndpoint(s) {
			t.Fatal(s)
		}
	}
}
func TestModelSchemaAndHallucinatedFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		_ = json.NewDecoder(r.Body).Decode(&request)
		if strings.Contains(stringMustJSON(request), "TelegramSenderID") {
			t.Fatal("identity leaked")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": `{"nameEn":"Invented","nameZh":"研究讲座","summary":"研究活动通知","location":"Invented Venue","registrationUrl":"https://invented.test","start":"2099-01-01T10:00:00Z","category":"学术讲座"}`}}}})
	}))
	defer srv.Close()
	base := *RuleExtract("Research Seminar\n29 October 2026\nVenue: Hall", "{}", anchor())
	got, e := Enrich(context.Background(), openai.New(openai.Config{BaseURL: srv.URL, Model: "fixture"}), "Research Seminar\n29 October 2026\nVenue: Hall", base, anchor())
	if e != nil || got.NameEN != "Research Seminar" || got.Location != "Hall" || got.RegistrationURL != "" || got.Start != nil {
		t.Fatalf("%+v %v", got, e)
	}
}
func stringMustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
func TestDigestIncludesTodayAndSelected(t *testing.T) {
	d := Digest([]Event{{ID: 1, Created: anchor(), Details: Details{NameEN: "Demo", Date: "2026-10-30"}, Sources: []Source{{Link: "https://t.me/demo/1"}}}}, anchor(), 24)
	if !strings.Contains(d, "Demo") || !strings.Contains(d, "未来 7 天") {
		t.Fatal(d)
	}
}

func TestAmbiguousLinksAndRoomSeparation(t *testing.T) {
	d := RuleExtract("Campus event\n29 October 2026\nRegister: https://forms.gle/one\nRSVP: https://forms.gle/two", "{}", anchor())
	if d.RegistrationURL != "" {
		t.Fatal("ambiguous registration link selected")
	}
	a := Details{NameEN: "Campus Event", Date: "2026-10-29", Room: "101", RegistrationURL: "https://forms.gle/one"}
	b := a
	b.Room = "102"
	if SameEvent(a, b) {
		t.Fatal("different rooms merged by matching URL")
	}
}
func TestDateOnlyDeadlineAndMultiDay(t *testing.T) {
	d := RuleExtract("Campus event\n29 October 2026 11pm to 31 October 2026 1am\nRegistration deadline: 28 October 2026", "{}", anchor())
	if d.Deadline != nil || d.DeadlineDate != "2026-10-28" {
		t.Fatal("deadline should be date-only")
	}
	if d.End == nil || d.End.Format("2006-01-02 15:04") != "2026-10-31 01:00" {
		t.Fatalf("multi-day end %v", d.End)
	}
	if !reminderNeeded(Details{DeadlineDate: "2026-10-30"}, anchor(), 24) {
		t.Fatal("missing date-only reminder")
	}
}
func TestRepostPreservesKnownFields(t *testing.T) {
	old := Details{RegistrationURL: "https://forms.gle/example", Location: "Hall"}
	v := mergeDetails(old, Details{NameEN: "Event"})
	if v.RegistrationURL != old.RegistrationURL || v.Location != "Hall" || len(v.Warnings) == 0 {
		t.Fatal("repost lost original information")
	}
}
func TestRescheduledEventNeedsUpdateEvidence(t *testing.T) {
	original := Details{NameEN: "Campus Event", Date: "2026-10-29", RegistrationURL: "https://forms.gle/demo"}
	updated := original
	updated.Date = "2026-11-01"
	updated.UpdateNotice = true
	if !SameEvent(original, updated) {
		t.Fatal("matching rescheduling notice was not linked")
	}
	updated.UpdateNotice = false
	if SameEvent(original, updated) {
		t.Fatal("different dates merged without update notice")
	}
	d := RuleExtract("UPDATED DETAILS\nCampus Event\n1 November 2026\nRegister: https://forms.gle/demo", "{}", anchor())
	if d.NameEN != "Campus Event" || !d.UpdateNotice {
		t.Fatalf("%+v", d)
	}
}
func TestSeparateLabeledTimes(t *testing.T) {
	d := RuleExtract("Campus event\nDate: 29 October 2026\nStart: 11pm\nEnd: 1am", "{}", anchor())
	if d.Start == nil || d.End == nil || d.End.Day() != 30 {
		t.Fatalf("%+v", d)
	}
}
func TestSplitDigestPreservesAllText(t *testing.T) {
	input := strings.Repeat("示例活动及真实链接 https://example.org/event\n", 500)
	parts := splitMessage(input)
	if len(parts) < 2 || strings.Join(parts, "") != input {
		t.Fatal("digest split lost content")
	}
	for _, part := range parts {
		if len([]rune(part)) > 3000 {
			t.Fatal("message too long")
		}
	}
}
func TestAmbiguousTimesRemainUnknown(t *testing.T) {
	for _, text := range []string{"11-1pm", "10:00"} {
		a, b := ParseTimes(text, anchor())
		if a != nil || b != nil {
			t.Fatalf("ambiguous time was guessed: %s", text)
		}
	}
}
