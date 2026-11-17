package activity

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Claim a send durably before calling Telegram. A lost response or process crash
// leaves a visible, paused attempt; it must never trigger an automatic resend.
func (s *Service) sendOnce(ctx context.Context, key, content string, keyboard any, owner int64, token string) error {
	tx, err := s.Store.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	resource := key
	parts := strings.Split(key, ":")
	isCard := len(parts) == 3 && parts[0] == "card"
	if isCard {
		resource = strings.Join(parts[:2], ":")
	}
	if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock(hashtextextended($1,451011))`, resource); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `insert into activity_deliveries(key,content) values($1,$2) on conflict do nothing`, key, trimMessage(content)); err != nil {
		return err
	}
	var stored, code string
	var done, ready bool
	if err = tx.QueryRow(ctx, `select content,sent_at is not null or skipped_at is not null,retry_at<=now(),last_error_code from activity_deliveries where key=$1`, key).Scan(&stored, &done, &ready, &code); err != nil {
		return err
	}
	if done || deliveryNeedsReview(code) {
		return tx.Commit(ctx)
	}
	if isCard {
		id, idErr := strconv.ParseInt(parts[1], 10, 64)
		revision, revisionErr := strconv.Atoi(parts[2])
		if idErr != nil || revisionErr != nil {
			return fmt.Errorf("invalid_delivery_key")
		}
		var current int
		err = tx.QueryRow(ctx, `select revision from activities where id=$1`, id).Scan(&current)
		if err != nil && err != pgx.ErrNoRows {
			return err
		}
		if err == pgx.ErrNoRows || revision < current {
			_, err = tx.Exec(ctx, `update activity_deliveries set skipped_at=now(),skip_reason='superseded',last_error_code='' where key=$1`, key)
			if err != nil {
				return err
			}
			return tx.Commit(ctx)
		}
		// A newer revision cannot overtake a send still in flight or hide an
		// uncertain earlier attempt. A confirmed receipt releases the next revision.
		var unresolved bool
		if err = tx.QueryRow(ctx, `select exists(select 1 from activity_deliveries where key like $1 and sent_at is null and skipped_at is null and last_error_code in ('delivery_sending','delivery_unconfirmed'))`, resource+":%").Scan(&unresolved); err != nil {
			return err
		}
		if unresolved {
			return tx.Commit(ctx)
		}
		if _, err = tx.Exec(ctx, `update activity_deliveries set skipped_at=now(),skip_reason='superseded' where key like $1 and key<>$2 and sent_at is null and skipped_at is null`, resource+":%", key); err != nil {
			return err
		}
		// Compare with the latest actual send so a real A -> B -> A update
		// remains visible, while invisible extraction changes do not resend a card.
		var previous string
		err = tx.QueryRow(ctx, `select content from activity_deliveries where key like $1 and sent_at is not null order by sent_at desc,created_at desc,key desc limit 1`, resource+":%").Scan(&previous)
		if err != nil && err != pgx.ErrNoRows {
			return err
		}
		if err == nil && trimMessage(previous) == trimMessage(stored) {
			_, err = tx.Exec(ctx, `update activity_deliveries set skipped_at=now(),skip_reason='unchanged',last_error_code='' where key=$1`, key)
			if err != nil {
				return err
			}
			return tx.Commit(ctx)
		}
	}
	if !ready {
		return tx.Commit(ctx)
	}
	var attempt int
	if err = tx.QueryRow(ctx, `update activity_deliveries set attempts=attempts+1,last_error_code='delivery_sending',retry_at=now()+interval '2 minutes' where key=$1 returning attempts`, key).Scan(&attempt); err != nil {
		return err
	}
	// No network side effect is allowed unless this claim has committed.
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	payload := map[string]any{"chat_id": owner, "text": trimMessage(stored), "disable_web_page_preview": true}
	if keyboard != nil {
		payload["reply_markup"] = keyboard
	}
	var receipt struct {
		MessageID int64 `json:"message_id"`
	}
	err = s.bot.Call(ctx, token, "sendMessage", payload, &receipt)
	if err == nil && receipt.MessageID <= 0 {
		err = fmt.Errorf("bot_response_invalid")
	}
	finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err != nil {
		// Only an explicit rejection is safe to retry automatically. Network,
		// malformed-response and server errors may follow a successful send.
		_, saveErr := s.Store.Pool.Exec(finish, `update activity_deliveries set last_error_code=$2,retry_at=now()+interval '2 minutes'*least(attempts,30) where key=$1 and attempts=$3 and sent_at is null`, key, deliveryFailureCode(err), attempt)
		return errors.Join(err, saveErr)
	}
	tag, err := s.Store.Pool.Exec(finish, `update activity_deliveries set sent_at=clock_timestamp(),bot_message_id=$2,last_error_code='',skipped_at=null,skip_reason='' where key=$1 and attempts=$3 and sent_at is null`, key, receipt.MessageID, attempt)
	if err != nil {
		// The durable claim remains paused even when saving the receipt fails.
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("delivery_receipt_changed")
	}
	return nil
}
