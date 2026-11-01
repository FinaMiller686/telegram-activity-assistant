package activity

import (
	"testing"
	"time"
)

func TestBackfilledExpiredActivitiesDoNotNotify(t *testing.T) {
	now := time.Date(2026, 10, 30, 22, 0, 0, 0, Singapore)
	if cardStillRelevant(Details{Date: "2026-10-29"}, now) {
		t.Fatal("past event would send a new card")
	}
	ended := now.Add(-time.Hour)
	if cardStillRelevant(Details{Date: "2026-10-30", End: &ended}, now) {
		t.Fatal("ended event would notify")
	}
	ongoingEnd := now.Add(time.Hour)
	if !cardStillRelevant(Details{Date: "2026-10-29", End: &ongoingEnd}, now) {
		t.Fatal("ongoing multi-day event was skipped")
	}
	if !cardStillRelevant(Details{Date: "2026-10-31"}, now) {
		t.Fatal("future event was skipped")
	}
	if !cardStillRelevant(Details{}, now) {
		t.Fatal("undated important notice was skipped")
	}
}
