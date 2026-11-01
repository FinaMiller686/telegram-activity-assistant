package activity

import (
	"context"
	"encoding/json"
	"github.com/fr0der1c/tgtldr/app/internal/openai"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestModelDatesUseSourceAndPublicationTime(t *testing.T) {
	published := time.Date(2026, 10, 23, 9, 0, 0, 0, Singapore)
	text := "Research Seminar tomorrow, 6pm to 8pm. Register by today at noon."
	d := Details{}
	applyModelTimes(&d, modelTimes{Date: "2026-10-24", Start: "2026-10-24T18:00:00+08:00", End: "2026-10-24T20:00:00+08:00", DeadlineDate: "2026-10-23", Deadline: "2026-10-23T12:00:00+08:00", DateEvidence: "tomorrow", TimeEvidence: "6pm to 8pm", DeadlineEvidence: "today at noon"}, text, published)
	if d.Date != "2026-10-24" || d.Start == nil || d.Start.Hour() != 18 || d.End == nil || d.End.Hour() != 20 || d.Deadline == nil || d.Deadline.Day() != 23 {
		t.Fatalf("dates were not extracted: %+v", d)
	}
	wrong := Details{}
	applyModelTimes(&wrong, modelTimes{Date: "2026-10-31", DateEvidence: "tomorrow"}, text, published)
	if wrong.Date != "" {
		t.Fatal("relative date used processing time instead of publication time")
	}
}

func TestModelYearlessAndDateOnlyDeadline(t *testing.T) {
	published := time.Date(2026, 10, 30, 9, 0, 0, 0, Singapore)
	text := "Workshop on 6th November, 6pm to 8pm. Register by 2nd November."
	d := Details{}
	applyModelTimes(&d, modelTimes{Date: "2026-11-06", Start: "2026-11-06T18:00:00+08:00", End: "2026-11-06T20:00:00+08:00", DeadlineDate: "2026-11-02", DateEvidence: "6th November", TimeEvidence: "6pm to 8pm", DeadlineEvidence: "2nd November"}, text, published)
	if d.Date != "2026-11-06" || d.Start == nil || d.DeadlineDate != "2026-11-02" || d.Deadline != nil || len(d.Warnings) == 0 {
		t.Fatalf("yearless/date-only extraction: %+v", d)
	}
	if !strings.Contains(Card(Event{Details: d}), "原文未写年份") {
		t.Fatal("private Bot card hid the inferred-year warning")
	}
	published = time.Date(2026, 12, 30, 9, 0, 0, 0, Singapore)
	d = Details{}
	applyModelTimes(&d, modelTimes{Date: "2027-01-02", DateEvidence: "2 January"}, "Workshop on 2 January", published)
	if d.Date != "2027-01-02" {
		t.Fatal("cross-year January date lost")
	}
}

func TestModelMissingFactsRemainMissing(t *testing.T) {
	published := time.Date(2026, 10, 30, 9, 0, 0, 0, Singapore)
	d := Details{}
	applyModelTimes(&d, modelTimes{Date: "2099-01-01", Start: "2099-01-01T10:00:00+08:00", DeadlineDate: "2099-01-01", DateEvidence: "1 January 2099", TimeEvidence: "10am", DeadlineEvidence: "1 January 2099"}, "Research Seminar. Details to follow.", published)
	if d.Date != "" || d.Start != nil || d.DeadlineDate != "" || d.Deadline != nil {
		t.Fatal("invented source facts accepted")
	}
	d = Details{}
	applyModelTimes(&d, modelTimes{Date: "2026-11-01", DateEvidence: "6pm", Start: "2026-11-01T18:00:00+08:00", TimeEvidence: "6pm"}, "The meetup starts at 6pm", published)
	if d.Date != "" || d.Start != nil || d.TimeText != "6pm" {
		t.Fatal("time-only notice gained an invented date")
	}
}

func TestModelOvernightHasNoGuessedDuration(t *testing.T) {
	published := time.Date(2026, 10, 29, 9, 0, 0, 0, Singapore)
	text := "Hackathon 30 October 2026, 11pm to 1am."
	d := Details{}
	applyModelTimes(&d, modelTimes{Date: "2026-10-30", Start: "2026-10-30T23:00:00+08:00", End: "2026-10-31T01:00:00+08:00", DateEvidence: "30 October 2026", TimeEvidence: "11pm to 1am"}, text, published)
	if d.Start == nil || d.End == nil || d.End.Sub(*d.Start) != 2*time.Hour {
		t.Fatal("overnight event was not preserved")
	}
	d = Details{}
	applyModelTimes(&d, modelTimes{Date: "2026-10-30", Start: "2026-10-30T23:00:00+08:00", DateEvidence: "30 October 2026", TimeEvidence: "11pm"}, text, published)
	if d.End != nil {
		t.Fatal("missing end time gained a default duration")
	}
}

func TestEnrichSendsOriginalTextAndAcceptsGroundedTimes(t *testing.T) {
	published := time.Date(2026, 10, 23, 9, 0, 0, 0, Singapore)
	original := "Research Seminar tomorrow, 6pm to 8pm. Register by today at noon."
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		var input map[string]any
		if err := json.Unmarshal([]byte(request.Messages[1].Content), &input); err != nil {
			t.Error(err)
		}
		if input["message"] != original || input["publishedAtSingapore"] != published.Format(time.RFC3339) || input["timezone"] != "Asia/Singapore" || input["known"] != nil {
			t.Error("model did not receive the original text and its publication anchor")
		}
		content := `{"nameEn":"Research Seminar","nameZh":"研究讲座","summary":"明晚参加讲座，今日中午前报名","category":"学术讲座","date":"2026-10-24","start":"2026-10-24T18:00:00+08:00","end":"2026-10-24T20:00:00+08:00","deadlineDate":"2026-10-23","deadline":"2026-10-23T12:00:00+08:00","dateEvidence":"tomorrow","timeEvidence":"6pm to 8pm","deadlineEvidence":"today at noon"}`
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
	}))
	defer server.Close()
	d, err := Enrich(context.Background(), openai.New(openai.Config{BaseURL: server.URL, Model: "fixture"}), original, Details{}, published)
	if err != nil || d == nil || d.Date != "2026-10-24" || d.Start == nil || d.End == nil || d.Deadline == nil {
		t.Fatalf("temporal model output lost: %+v %v", d, err)
	}
}
