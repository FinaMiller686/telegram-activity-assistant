package telegram

import (
	"context"
	"github.com/gotd/td/tg"
	"strings"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/activity"
)

func collectActivityText(text string, aiEnabled bool) bool {
	if aiEnabled {
		return strings.TrimSpace(text) != ""
	}
	return activity.Candidate(text)
}

// Do not retain ordinary conversation. An edit that is no longer a candidate
// withdraws any previously extracted source.
func (s *Service) discardNonCandidate(ctx context.Context, chatID int64, messageID int) error {
	var exists bool
	if err := s.store.Pool.QueryRow(ctx, `select exists(select 1 from activity_sources where chat_id=$1 and telegram_message_id=$2) or exists(select 1 from messages where chat_id=$1 and telegram_message_id=$2)`, chatID, messageID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return nil
	}
	tx, err := s.store.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock(451010)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `update activity_sources set deleted=true where chat_id=$1 and telegram_message_id=$2`, chatID, messageID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `delete from messages where chat_id=$1 and telegram_message_id=$2`, chatID, messageID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) deleteActivityMessages(ctx context.Context, accountID, telegramChatID int64, ids []int) error {
	// Non-channel message IDs are account-wide; channel deletions always include the channel ID.
	tx, e := s.store.Pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `select pg_advisory_xact_lock(451010)`); e != nil {
		return e
	}
	_, e = tx.Exec(ctx, `update activity_sources a set deleted=true from chats c where a.chat_id=c.id and c.collector_account_id=$1 and ($2::bigint=0 and c.chat_type='group' or c.telegram_chat_id=$2) and a.telegram_message_id=any($3)`, accountID, telegramChatID, ids)
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, `delete from messages m using chats c where m.chat_id=c.id and c.collector_account_id=$1 and ($2::bigint=0 and c.chat_type='group' or c.telegram_chat_id=$2) and m.telegram_message_id=any($3)`, accountID, telegramChatID, ids)
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (s *Service) recoverActivityHistory(ctx context.Context, accountID int64) {
	cfg, e := activity.New(s.store).Config(ctx)
	if e != nil {
		return
	}
	chats, e := s.store.Chats.List(ctx)
	if e != nil {
		return
	}
	now := s.clock.Now().In(activity.Singapore)
	for _, chat := range chats {
		if !chat.Enabled || chat.CollectorAccountID != accountID {
			continue
		}
		var latest *time.Time
		e = s.store.Pool.QueryRow(ctx, `select max(synced_at) from activity_sync where chat_id=$1`, chat.ID).Scan(&latest)
		if e != nil {
			continue
		}
		start := now.AddDate(0, 0, -7)
		if latest != nil {
			start = latest.In(activity.Singapore).AddDate(0, 0, -1)
		}
		floor := now.AddDate(0, 0, -30)
		if start.Before(floor) {
			start = floor
			activity.New(s.store).Record(ctx, "history", "gap_exceeds_30_days_manual_backfill_required")
		}
		if cfg.CollectSince != nil && start.Before(*cfg.CollectSince) {
			start = cfg.CollectSince.In(activity.Singapore)
		}
		if _, e = s.StartHistoryBackfill(chat, start.Format("2006-01-02"), now.Format("2006-01-02")); e != nil {
			activity.New(s.store).Record(ctx, "history", "backfill_start_failed")
		}
	}
}

func (s *Service) reconcileActivitySources(ctx context.Context, chatID int64, start, end, scanStarted time.Time, seen map[int]struct{}) error {
	ids := make([]int, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	tx, e := s.store.Pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	// A message received while the scan was running must not be mistaken for a deletion.
	_, e = tx.Exec(ctx, `update activity_sources a set deleted=true from messages m where a.chat_id=m.chat_id and a.telegram_message_id=m.telegram_message_id and m.chat_id=$1 and m.message_time >= $2 and m.message_time < $3 and not(m.telegram_message_id=any($4)) and m.created_at<$5`, chatID, start, end, ids, scanStarted)
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, `delete from messages where chat_id=$1 and message_time>=$2 and message_time<$3 and not(telegram_message_id=any($4)) and created_at<$5`, chatID, start, end, ids, scanStarted)
	if e != nil {
		return e
	}
	if end.After(scanStarted) {
		_, e = tx.Exec(ctx, `insert into activity_sync(chat_id,synced_at) values($1,$2) on conflict(chat_id) do update set synced_at=greatest(activity_sync.synced_at,excluded.synced_at)`, chatID, scanStarted)
		if e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}

// Audit upcoming event messages outside the recovery window. A failed API call never implies deletion.
func (s *Service) auditActivitySources(ctx context.Context, accountID int64, api *tg.Client) {
	rows, err := s.store.Pool.Query(ctx, `select c.telegram_chat_id,c.chat_type,coalesce(ac.telegram_access_hash,c.telegram_access_hash),a.telegram_message_id from activity_sources a join chats c on c.id=a.chat_id join activities v on v.id=a.activity_id left join telegram_account_chats ac on ac.chat_id=c.id and ac.telegram_account_id=$1 where c.enabled and c.collector_account_id=$1 and not a.deleted and (coalesce(v.data->>'date','')='' or v.data->>'date'>=(now() at time zone 'Asia/Singapore')::date::text) order by c.telegram_chat_id,a.telegram_message_id limit 5000`, accountID)
	if err != nil {
		activity.New(s.store).Record(ctx, "source_audit", "database_error")
		return
	}
	type source struct {
		chat   int64
		kind   string
		access int64
		id     int
	}
	groups := map[int64][]source{}
	for rows.Next() {
		var v source
		if err = rows.Scan(&v.chat, &v.kind, &v.access, &v.id); err != nil {
			rows.Close()
			return
		}
		groups[v.chat] = append(groups[v.chat], v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return
	}
	for _, sources := range groups {
		for start := 0; start < len(sources); start += 100 {
			end := start + 100
			if end > len(sources) {
				end = len(sources)
			}
			batch := sources[start:end]
			ids := make([]tg.InputMessageClass, 0, len(batch))
			missing := map[int]bool{}
			for _, v := range batch {
				ids = append(ids, &tg.InputMessageID{ID: v.id})
				missing[v.id] = true
			}
			var result tg.MessagesMessagesClass
			if batch[0].kind == "supergroup" {
				result, err = api.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{Channel: &tg.InputChannel{ChannelID: batch[0].chat, AccessHash: batch[0].access}, ID: ids})
			} else {
				result, err = api.MessagesGetMessages(ctx, ids)
			}
			if err != nil {
				activity.New(s.store).Record(ctx, "source_audit", "telegram_request_failed")
				return
			}
			modified, ok := result.AsModified()
			if !ok {
				activity.New(s.store).Record(ctx, "source_audit", "unexpected_response")
				return
			}
			for _, m := range modified.GetMessages() {
				if msg, ok := m.(*tg.Message); ok {
					delete(missing, msg.ID)
					if err = s.storeIncomingMessage(ctx, accountID, tg.Entities{}, msg); err != nil {
						activity.New(s.store).Record(ctx, "source_audit", "store_failed")
						return
					}
				}
			}
			deleted := []int{}
			for id := range missing {
				deleted = append(deleted, id)
			}
			if len(deleted) > 0 {
				peer := batch[0].chat
				if batch[0].kind == "group" {
					peer = 0
				}
				if err = s.deleteActivityMessages(ctx, accountID, peer, deleted); err != nil {
					return
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}
	activity.New(s.store).Record(ctx, "source_audit", "ok")
}
