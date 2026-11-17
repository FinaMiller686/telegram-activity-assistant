package activity

import (
	"context"
	"fmt"
)

// RetryDelivery is an explicit user action after checking Telegram. The delay
// prevents a double click from reopening an ordinary in-flight 20-second send.
func (s *Service) RetryDelivery(ctx context.Context, key string) error {
	tag, err := s.Store.Pool.Exec(ctx, `update activity_deliveries set last_error_code='',retry_at=now() where key=$1 and sent_at is null and skipped_at is null and last_error_code in ('delivery_sending','delivery_unconfirmed') and retry_at<=now()`, key)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("delivery_not_retryable")
	}
	return nil
}
