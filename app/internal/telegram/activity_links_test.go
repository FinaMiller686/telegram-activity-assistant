package telegram

import (
	"encoding/json"
	"github.com/fr0der1c/tgtldr/app/internal/activity"
	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/gotd/td/tg"
	"testing"
)

func TestGotdEntityAndButtonLinkExtraction(t *testing.T) {
	msg := &tg.Message{Message: "😀 Register", Entities: []tg.MessageEntityClass{&tg.MessageEntityTextURL{Offset: 3, Length: 8, URL: "https://example.org/register"}}}
	raw, e := json.Marshal(msg)
	if e != nil {
		t.Fatal(e)
	}
	links := activity.Links(msg.Message, string(raw))
	if len(links) != 1 || links[0] != "https://example.org/register" {
		t.Fatalf("actual gotd entity: %v", links)
	}
	msg = &tg.Message{Message: "Campus event", ReplyMarkup: &tg.ReplyInlineMarkup{Rows: []tg.KeyboardButtonRow{{Buttons: []tg.KeyboardButtonClass{&tg.KeyboardButtonURL{Text: "Register", URL: "https://bit.ly/example"}}}}}}
	raw, e = json.Marshal(msg)
	if e != nil {
		t.Fatal(e)
	}
	links = activity.Links(msg.Message, string(raw))
	if len(links) != 1 || links[0] != "https://bit.ly/example" {
		t.Fatalf("actual gotd button: %v", links)
	}
}
func TestBackfillDisabledChatIsRejected(t *testing.T) {
	s := &Service{}
	_, e := s.StartHistoryBackfill(model.Chat{Enabled: false}, "2026-10-01", "2026-10-02")
	if e == nil {
		t.Fatal("unselected chat was allowed")
	}
}

func TestAIIntakeIncludesTextWithoutRuleKeywords(t *testing.T) {
	text := "Summer Placement\nApplications open tomorrow."
	if collectActivityText(text, false) {
		t.Fatal("fixture unexpectedly matches rules")
	}
	if !collectActivityText(text, true) {
		t.Fatal("AI mode discarded useful text before model evaluation")
	}
	if collectActivityText(" \n ", true) {
		t.Fatal("empty text sent for AI processing")
	}
}
