package activity

import "testing"

func TestSameRoomAloneDoesNotMergeEvents(t *testing.T) {
	a := Details{NameEN: "Research Seminar", Date: "2026-11-03", Location: "Building A", Room: "101", Organizer: "Research Club"}
	b := a
	b.NameEN = "Career Workshop"
	if SameEvent(a, b) {
		t.Fatal("different events in the same room were merged")
	}
	b = a
	b.Organizer = "Career Club"
	if SameEvent(a, b) {
		t.Fatal("different known organizers were ignored")
	}
	b = a
	b.Room = "102"
	if SameEvent(a, b) {
		t.Fatal("different rooms were ignored")
	}
	if !SameEvent(a, a) {
		t.Fatal("identical repost did not match")
	}
}
