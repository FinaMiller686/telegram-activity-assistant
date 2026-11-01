package telegram

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/gotd/td/tg"
)

type exportLink struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}
type textExportRecord struct {
	ID            int             `json:"id"`
	Date          time.Time       `json:"date"`
	Edited        *time.Time      `json:"edited,omitempty"`
	Sender        string          `json:"sender"`
	Text          string          `json:"text"`
	Kind          string          `json:"kind"`
	Media         string          `json:"media,omitempty"`
	SourceURL     string          `json:"sourceUrl,omitempty"`
	ReplyTo       int             `json:"replyToMessageId,omitempty"`
	Links         []exportLink    `json:"links"`
	Entities      json.RawMessage `json:"entities,omitempty"`
	ServiceAction json.RawMessage `json:"serviceAction,omitempty"`
}

func exportSource(chat model.Chat, id int) string {
	if chat.Username != "" {
		return fmt.Sprintf("https://t.me/%s/%d", chat.Username, id)
	}
	if chat.ChatType == "supergroup" {
		return fmt.Sprintf("https://t.me/c/%d/%d", chat.TelegramChatID, id)
	}
	return ""
}
func exportSenderKey(peer tg.PeerClass) string {
	switch p := peer.(type) {
	case *tg.PeerUser:
		return fmt.Sprintf("user:%d", p.UserID)
	case *tg.PeerChat:
		return fmt.Sprintf("group:%d", p.ChatID)
	case *tg.PeerChannel:
		return fmt.Sprintf("channel:%d", p.ChannelID)
	}
	return ""
}
func exportSenderNames(users []tg.UserClass, chats []tg.ChatClass) map[string]string {
	result := map[string]string{}
	for _, raw := range users {
		if u, ok := raw.(*tg.User); ok {
			name := strings.TrimSpace(u.FirstName + " " + u.LastName)
			if name == "" {
				name = u.Username
			}
			result[fmt.Sprintf("user:%d", u.ID)] = name
		}
	}
	for _, raw := range chats {
		switch c := raw.(type) {
		case *tg.Chat:
			result[fmt.Sprintf("group:%d", c.ID)] = c.Title
		case *tg.Channel:
			result[fmt.Sprintf("channel:%d", c.ID)] = c.Title
		}
	}
	return result
}
func exportRecord(chat model.Chat, raw tg.MessageClass, names map[string]string) (textExportRecord, bool) {
	loc, _ := time.LoadLocation("Asia/Singapore")
	row := textExportRecord{ID: raw.GetID(), SourceURL: exportSource(chat, raw.GetID()), Links: []exportLink{}}
	switch m := raw.(type) {
	case *tg.Message:
		if m.Noforwards {
			row.Date = time.Unix(int64(m.Date), 0).In(loc)
			row.Kind = "protected"
			row.Text = "[该消息受内容保存限制，未导出正文]"
			return row, true
		}
		row.Date = time.Unix(int64(m.Date), 0).In(loc)
		row.Text = m.Message
		row.Kind = "message"
		row.ReplyTo = replyToID(m)
		row.Entities, _ = json.Marshal(m.Entities)
		if m.EditDate > 0 {
			d := time.Unix(int64(m.EditDate), 0).In(loc)
			row.Edited = &d
		}
		from := messageSenderPeer(m)
		row.Sender = names[exportSenderKey(from)]
		if row.Sender == "" {
			row.Sender = exportSenderKey(from)
		}
		if m.Media != nil {
			row.Media = m.Media.TypeName()
		}
		if poll, ok := m.Media.(*tg.MessageMediaPoll); ok {
			row.Text += "\n[投票] " + poll.Poll.Question.Text
			for _, a := range poll.Poll.Answers {
				row.Text += "\n• " + a.Text.Text
			}
		}
		row.Links = exportMessageLinks(m)
	case *tg.MessageService:
		row.Date = time.Unix(int64(m.Date), 0).In(loc)
		row.Kind = "service"
		row.Sender = names[exportSenderKey(m.FromID)]
		if m.Action != nil {
			row.Text = "[系统消息] " + m.Action.TypeName()
			row.ServiceAction, _ = json.Marshal(m.Action)
		}
	default:
		return row, false
	}
	return row, true
}

var exportURLPattern = regexp.MustCompile(`https?://[^\s<>"\\]+`)

func exportMessageLinks(m *tg.Message) []exportLink {
	result := []exportLink{}
	seen := map[string]bool{}
	add := func(label, raw string) {
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || seen[raw] {
			return
		}
		seen[raw] = true
		result = append(result, exportLink{Text: label, URL: raw})
	}
	textAt := func(offset, length int) string {
		units := utf16.Encode([]rune(m.Message))
		if offset < 0 || length < 0 || offset+length > len(units) {
			return ""
		}
		return string(utf16.Decode(units[offset : offset+length]))
	}
	for _, e := range m.Entities {
		switch v := e.(type) {
		case *tg.MessageEntityTextURL:
			add(textAt(v.Offset, v.Length), v.URL)
		case *tg.MessageEntityURL:
			u := textAt(v.Offset, v.Length)
			add(u, u)
		}
	}
	for _, u := range exportURLPattern.FindAllString(m.Message, -1) {
		u = strings.TrimRight(u, ".,;!")
		add(u, u)
	}
	if markup, ok := m.ReplyMarkup.(*tg.ReplyInlineMarkup); ok {
		for _, r := range markup.Rows {
			for _, button := range r.Buttons {
				if b, ok := button.(*tg.KeyboardButtonURL); ok {
					add(b.Text, b.URL)
				}
			}
		}
	}
	return result
}
