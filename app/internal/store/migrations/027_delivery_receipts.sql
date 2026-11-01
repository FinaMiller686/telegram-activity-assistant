alter table activity_deliveries
 add column bot_message_id bigint,
 add column skipped_at timestamptz,
 add column skip_reason text not null default '',
 add column last_error_code text not null default '';
create index activity_delivery_pending on activity_deliveries(retry_at)
 where sent_at is null and skipped_at is null;
