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

// Serialize the whole send, including its receipt, across workers and processes.
// Telegram has no idempotency key for sendMessage: losing a successful response
// can still cause a duplicate on retry. Such attempts remain visible as unconfirmed.
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
	var stored string
	var done, ready bool
	if err = tx.QueryRow(ctx, `select content,sent_at is not null or skipped_at is not null,retry_at<=now() from activity_deliveries where key=$1`, key).Scan(&stored, &done, &ready); err != nil {
		return err
	}
	if done {
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
		// Only the current revision is retried; never replay an older failed card.
		if _, err = tx.Exec(ctx, `update activity_deliveries set skipped_at=now(),skip_reason='superseded' where key like $1 and key<>$2 and sent_at is null and skipped_at is null`, resource+":%", key); err != nil {
			return err
		}
		// A revision can change hidden extraction fields without changing the card.
		// Compare only with the most recent actual send, so A -> B -> A is delivered.
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
	// Persist a receipt even if the caller is cancelled just after Telegram replies.
	finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err != nil {
		code := "bot_send_failed"
		if err.Error() == "bot_network_failed" || err.Error() == "bot_response_invalid" {
			code = "delivery_unconfirmed"
		}
		_, saveErr := tx.Exec(finish, `update activity_deliveries set attempts=attempts+1,last_error_code=$2,retry_at=now()+interval '2 minutes'*least(attempts+1,30) where key=$1`, key, code)
		if saveErr == nil {
			saveErr = tx.Commit(finish)
		}
		return errors.Join(err, saveErr)
	}
	if _, err = tx.Exec(finish, `update activity_deliveries set sent_at=clock_timestamp(),bot_message_id=$2,attempts=attempts+1,last_error_code='' where key=$1`, key, receipt.MessageID); err != nil {
		return err
	}
	return tx.Commit(finish)
}
