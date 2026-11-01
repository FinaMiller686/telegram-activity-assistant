package activity

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type DeliveryRecord struct {
	Key       string     `json:"key"`
	Kind      string     `json:"kind"`
	Content   string     `json:"content"`
	State     string     `json:"state"`
	SentAt    *time.Time `json:"sentAt"`
	CreatedAt time.Time  `json:"createdAt"`
	RetryAt   time.Time  `json:"retryAt"`
	MessageID int64      `json:"messageId"`
}

type DeliveryReport struct {
	QueuedMessages        int              `json:"queuedMessages"`
	FailedJobs            int              `json:"failedJobs"`
	ExhaustedJobs         int              `json:"exhaustedJobs"`
	PendingDeliveries     int              `json:"pendingDeliveries"`
	RetryingDeliveries    int              `json:"retryingDeliveries"`
	UnconfirmedDeliveries int              `json:"unconfirmedDeliveries"`
	MonitoredGroups       int              `json:"monitoredGroups"`
	StaleGroups           int              `json:"staleGroups"`
	OldestSync            *time.Time       `json:"oldestSync"`
	BotReady              bool             `json:"botReady"`
	Recent                []DeliveryRecord `json:"recent"`
}

// These counters describe saved work and API acknowledgements, not completeness
// of Telegram history, model classification, or notifications on a phone.
func (s *Service) DeliveryReport(ctx context.Context) (DeliveryReport, error) {
	report := DeliveryReport{Recent: []DeliveryRecord{}}
	err := s.Store.Pool.QueryRow(ctx, `select count(*) filter(where error_code<>''),count(*) filter(where error_code<>'' and attempts>=5) from activity_jobs`).Scan(&report.FailedJobs, &report.ExhaustedJobs)
	if err != nil {
		return report, err
	}
	err = s.Store.Pool.QueryRow(ctx, `select count(*) from messages m join chats c on c.id=m.chat_id left join activity_jobs j on j.message_id=m.id where c.enabled and c.chat_type in ('group','supergroup','channel') and ((j.message_id is null and not exists(select 1 from activity_sources a where a.chat_id=m.chat_id and a.telegram_message_id=m.telegram_message_id and not a.deleted and a.fingerprint=md5(m.text_content||m.raw_json::text))) or j.fingerprint<>md5(m.text_content||m.raw_json::text))`).Scan(&report.QueuedMessages)
	if err != nil {
		return report, err
	}
	err = s.Store.Pool.QueryRow(ctx, `select count(*),count(*) filter(where s.synced_at is null or s.synced_at<now()-interval '45 minutes'),min(s.synced_at) from chats c left join activity_sync s on s.chat_id=c.id where c.enabled and c.chat_type in ('group','supergroup','channel')`).Scan(&report.MonitoredGroups, &report.StaleGroups, &report.OldestSync)
	if err != nil {
		return report, err
	}
	settings, err := s.Store.Settings.Get(ctx)
	if err != nil {
		return report, err
	}
	cfg, err := s.Config(ctx)
	if err != nil {
		return report, err
	}
	report.BotReady = settings.BotEnabled && settings.BotToken != "" && s.ownerReady(ctx, cfg.OwnerID)
	rows, err := s.Store.Pool.Query(ctx, `select key,content,created_at,sent_at,retry_at,skipped_at,skip_reason,attempts,last_error_code,coalesce(bot_message_id,0) from activity_deliveries order by created_at desc,key desc`)
	if err != nil {
		return report, err
	}
	known := map[string]bool{}
	for rows.Next() {
		var record DeliveryRecord
		var skipped *time.Time
		var reason, code string
		var attempts int
		if err = rows.Scan(&record.Key, &record.Content, &record.CreatedAt, &record.SentAt, &record.RetryAt, &skipped, &reason, &attempts, &code, &record.MessageID); err != nil {
			rows.Close()
			return report, err
		}
		known[record.Key] = true
		record.Kind = strings.SplitN(record.Key, ":", 2)[0]
		switch {
		case record.SentAt != nil:
			record.State = "sent"
		case skipped != nil:
			record.State = reason
		case code == "delivery_unconfirmed":
			record.State = "unconfirmed"
		case attempts > 0:
			record.State = "retrying"
		default:
			record.State = "pending"
		}
		if record.SentAt == nil && skipped == nil {
			// Digest parts are included in their parent task's pending count.
			if record.Kind != "digestpart" {
				report.PendingDeliveries++
			}
			if attempts > 0 {
				report.RetryingDeliveries++
			}
			if code == "delivery_unconfirmed" {
				report.UnconfirmedDeliveries++
			}
		}
		if len(report.Recent) < 30 {
			report.Recent = append(report.Recent, record)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return report, err
	}
	// A crash before queue insertion must not make an eligible card look sent.
	events, err := s.List(ctx)
	if err != nil {
		return report, err
	}
	now := time.Now()
	for _, event := range events {
		if event.Status != "ignored" && !event.Withdrawn && cardStillRelevant(event.Details, now) && !known[fmt.Sprintf("card:%d:%d", event.ID, event.Revision)] {
			report.PendingDeliveries++
		}
	}
	return report, nil
}
