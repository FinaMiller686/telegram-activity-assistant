create table activity_config (
 id integer primary key check(id=1), ai_enabled boolean not null default false,
 external_ai_allowed boolean not null default false, digest_time text not null default '21:00',
 reminder_hours integer not null default 24 check(reminder_hours between 1 and 168),
 retention_days integer not null default 14 check(retention_days between 1 and 365),
 owner_id bigint not null default 0 check(owner_id>=0), bot_offset bigint not null default 0
);
insert into activity_config(id) values(1);
create table activity_consents (
 chat_id bigint primary key references chats(id) on delete cascade,
 evidence text not null, message_ids integer[] not null, expires_at timestamptz not null, recorded_at timestamptz not null default now()
);
create table activities (
 id bigint generated always as identity primary key, data jsonb not null,
 status text not null default 'new' check(status in ('new','going','later','ignored','registered','attended')),
 revision integer not null default 1, created_at timestamptz not null default now(), updated_at timestamptz not null default now()
);
create table activity_sources (
 chat_id bigint not null references chats(id) on delete cascade, telegram_message_id integer not null,
 activity_id bigint not null references activities(id) on delete cascade,
 link text not null default '', deleted boolean not null default false,
 primary key(chat_id,telegram_message_id)
);
create index activity_sources_event on activity_sources(activity_id);
create table activity_jobs (
 message_id bigint primary key references messages(id) on delete cascade,
 fingerprint text not null, attempts integer not null default 0,
 next_attempt timestamptz not null default now(), error_code text not null default '', updated_at timestamptz not null default now()
);
create table activity_deliveries (
 key text primary key, content text not null default '', sent_at timestamptz, created_at timestamptz not null default now()
);
create table activity_runtime (
 name text primary key, code text not null, updated_at timestamptz not null default now()
);
update app_settings set default_timezone='Asia/Singapore', auto_download_attachments=false;
update chats set summary_timezone='Asia/Singapore';
alter table chats alter column summary_timezone set default 'Asia/Singapore';
alter table activity_deliveries add column attempts integer not null default 0, add column retry_at timestamptz not null default now();
update app_settings set openai_base_url='http://host.docker.internal:11434/v1',openai_model='qwen3:8b' where openai_api_key='';
create table activity_sync(chat_id bigint primary key references chats(id) on delete cascade,synced_at timestamptz not null);
