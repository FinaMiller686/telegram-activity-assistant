package activity

import (
	"testing"
	"time"
)

func TestDigestDoesNotRecreateScheduleBeforeFreshStart(t *testing.T) {
	since := time.Date(2026, 10, 30, 22, 17, 21, 0, Singapore)
	cfg := Config{DigestTime: "21:00", CollectSince: &since}
	if digestDue(since.Add(time.Minute), cfg) {
		t.Fatal("recreated a schedule before the reset")
	}
	if digestDue(time.Date(2026, 10, 31, 20, 0, 0, 0, Singapore), cfg) {
		t.Fatal("digest generated before its scheduled time")
	}
	if !digestDue(time.Date(2026, 10, 31, 21, 0, 0, 0, Singapore), cfg) {
		t.Fatal("next daily digest skipped")
	}
	cfg.CollectSince = nil
	if !digestDue(since, cfg) {
		t.Fatal("normal catch-up behavior changed without a reset")
	}
}
