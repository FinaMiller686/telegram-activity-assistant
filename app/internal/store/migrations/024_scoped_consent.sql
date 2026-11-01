alter table activity_consents add column if not exists message_ids integer[] not null default '{}';
create table if not exists activity_sync(chat_id bigint primary key references chats(id) on delete cascade,synced_at timestamptz not null);
create table if not exists activity_consented_messages(
 chat_id bigint not null references activity_consents(chat_id) on delete cascade,
 telegram_message_id integer not null,
 fingerprint text not null,
 primary key(chat_id,telegram_message_id)
);
