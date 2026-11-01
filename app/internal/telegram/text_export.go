package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	dialogsquery "github.com/gotd/td/telegram/query/dialogs"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
)

type TextExportGroup struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	Kind        string `json:"kind"`
	Count       int    `json:"messageCount"`
	OffsetID    int    `json:"offsetId"`
	Complete    bool   `json:"complete"`
	Error       string `json:"error,omitempty"`
	Unavailable int    `json:"unavailableMessages"`
	Restricted int     `json:"protectedMessages"`
}
type TextExportManifest struct {
	Start    time.Time          `json:"startInclusive"`
	End      time.Time          `json:"endExclusive"`
	Timezone string             `json:"timezone"`
	Complete bool               `json:"complete"`
	Groups   []*TextExportGroup `json:"groups"`
}

// ExportText fetches history directly; it never queues AI jobs or changes monitoring.
// Only group/channel peers are eligible, including the archived dialog folder.
func (s *Service) ExportText(ctx context.Context, accountID int64, out string, start, end time.Time, progress func(int, int, int, string)) error {
	if !start.Before(end) {
		return fmt.Errorf("invalid_export_range")
	}
	if err := os.MkdirAll(out, 0700); err != nil {
		return err
	}
	manifest := TextExportManifest{Start: start, End: end, Timezone: "Asia/Singapore", Groups: []*TextExportGroup{}}
	manifestPath := filepath.Join(out, "manifest.json")
	if raw, err := os.ReadFile(manifestPath); err == nil {
		if err = json.Unmarshal(raw, &manifest); err != nil {
			return fmt.Errorf("invalid_export_manifest")
		}
		start, end = manifest.Start, manifest.End
	}
	save := func() error { return writeExportJSON(manifestPath, manifest) }
	client, _, err := s.newClient(accountID)
	if err != nil {
		return fmt.Errorf("export_client_configuration_failed")
	}
	return client.Run(ctx, func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil || !status.Authorized {
			return fmt.Errorf("export_account_not_authorized")
		}
		peers := map[string]model.Chat{}
		for _, folder := range []int{0, 1} {
			for {
				err = dialogsquery.NewQueryBuilder(client.API()).GetDialogs().FolderID(folder).BatchSize(100).ForEach(ctx, func(ctx context.Context, elem dialogsquery.Elem) error {
					chat, ok := dialogToChat(elem)
					if !ok {
						return nil
					}
					key := fmt.Sprintf("%s-%d", chat.ChatType, chat.TelegramChatID)
					peers[key] = chat
					return nil
				})
				if wait, ok := tgerr.AsFloodWait(err); ok {
					if progress != nil {
						progress(0, 0, 0, "rate_limited")
					}
					if e := exportWait(ctx, wait+time.Second); e != nil {
						return e
					}
					continue
				}
				if err != nil {
					return fmt.Errorf("dialog_listing_failed")
				}
				break
			}
		}
		existing := map[string]*TextExportGroup{}
		for _, g := range manifest.Groups {
			existing[g.Key] = g
		}
		keys := []string{}
		for key, chat := range peers {
			keys = append(keys, key)
			if existing[key] == nil {
				g := &TextExportGroup{Key: key, Title: chat.Title, Kind: chat.ChatType}
				manifest.Groups = append(manifest.Groups, g)
				existing[key] = g
			}
		}
		sort.Strings(keys)
		if err = save(); err != nil {
			return err
		}
		for i, key := range keys {
			g := existing[key]
			if g.Complete {
				if progress != nil {
					progress(i+1, len(keys), g.Count, "already_complete")
				}
				continue
			}
			g.Error = ""
			peer, err := inputPeerForChat(peers[key])
			if err != nil {
				return err
			}
			path := filepath.Join(out, key+".jsonl")
			seen, err := repairExportJSONL(path)
			if err != nil {
				return err
			}
			g.Count = len(seen)
			f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
			if err != nil {
				return err
			}
			for !g.Complete {
				result, e := client.API().MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{Peer: peer, OffsetID: g.OffsetID, OffsetDate: int(end.Unix()), Limit: 100})
				if wait, ok := tgerr.AsFloodWait(e); ok {
					if progress != nil {
						progress(i+1, len(keys), g.Count, "rate_limited")
					}
					if err = exportWait(ctx, wait+time.Second); err != nil {
						f.Close()
						return err
					}
					continue
				}
				if e != nil {
					g.Error = exportErrorCode(e)
					break
				}
				modified, ok := result.AsModified()
				if !ok {
					g.Error = "unexpected_history_response"
					break
				}
				messages := modified.GetMessages()
				if len(messages) == 0 {
					g.Complete = true
					break
				}
				senderNames := exportSenderNames(modified.GetUsers(), modified.GetChats())
				next := 0
				var data bytes.Buffer
				enc := json.NewEncoder(&data)
				for _, message := range messages {
					id := message.GetID()
					if id > 0 && (next == 0 || id < next) {
						next = id
					}
					record, ok := exportRecord(peers[key], message, senderNames)
					if !ok {
						g.Unavailable++
						continue
					}
					if record.Date.Before(start) {
						g.Complete = true
						continue
					}
					if !record.Date.Before(end) || seen[id] {
						continue
					}
					if e = enc.Encode(record); e != nil {
						f.Close()
						return e
					}
					seen[id] = true
					g.Count++
				}
				if _, e = f.Write(data.Bytes()); e != nil {
					f.Close()
					return e
				}
				if e = f.Sync(); e != nil {
					f.Close()
					return e
				}
				if next == 0 || (g.OffsetID > 0 && next >= g.OffsetID) {
					if !g.Complete {
						g.Error = "history_pagination_stalled"
					}
					break
				}
				g.OffsetID = next
				if err = save(); err != nil {
					f.Close()
					return err
				}
				if progress != nil {
					progress(i+1, len(keys), g.Count, "fetching")
				}
				if err = exportWait(ctx, 350*time.Millisecond); err != nil {
					f.Close()
					return err
				}
			}
			f.Close()
			if err = save(); err != nil {
				return err
			}
			if err = renderExportGroup(out, g); err != nil {
				return err
			}
			if progress != nil {
				status := "complete"
				if g.Error != "" {
					status = "incomplete"
				}
				progress(i+1, len(keys), g.Count, status)
			}
		}
		manifest.Complete = true
		for _, g := range manifest.Groups {
			if !g.Complete {
				manifest.Complete = false
			}
		}
		if err = save(); err != nil {
			return err
		}
		if err = renderExportIndex(out, manifest); err != nil {
			return err
		}
		return zipTextExport(out)
	})
}

func exportWait(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
func exportErrorCode(err error) string {
	for _, code := range []string{"CHANNEL_PRIVATE", "CHAT_ADMIN_REQUIRED", "CHANNEL_INVALID", "CHAT_ID_INVALID", "AUTH_KEY_UNREGISTERED", "SESSION_REVOKED"} {
		if tgerr.Is(err, code) {
			return code
		}
	}
	return "history_request_failed"
}
