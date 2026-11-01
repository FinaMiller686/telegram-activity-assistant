#!/bin/bash
set -euo pipefail
set +x
export LC_ALL=en_US.UTF-8
project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
native_dir="${ACTIVITY_NATIVE_DIR:-$project_dir/.local/native}"
data_dir="$native_dir/activity-data"
port="${ACTIVITY_DATABASE_PORT:-55433}"
storage="$project_dir/scripts/encrypted-storage.sh"
pgbin="$(brew --prefix postgresql@17)/bin"
umask 077
if [ -e "$native_dir/pgdata" ] || [ -e "$native_dir/master.key" ]; then
  echo '发现旧助手数据；请先迁移，未覆盖任何文件。' >&2; exit 1
fi
"$storage" init
"$storage" mount
if [ -f "$data_dir/.migration-complete" ]; then "$storage" verify; echo '助手加密存储已初始化。'; exit 0; fi
if [ -e "$data_dir/pgdata" ] || [ -e "$data_dir/db-password" ]; then echo '发现未完成的数据目录；未覆盖，请先恢复。' >&2; exit 1; fi
"$storage" init-master
"$storage" verify || { echo '加密存储验证失败。' >&2; exit 1; }
openssl rand -hex 32 > "$data_dir/db-password"
chmod 600 "$data_dir/db-password"
"$pgbin/initdb" -D "$data_dir/pgdata" -U tgtldr --auth-local=trust --auth-host=scram-sha-256 --pwfile="$data_dir/db-password" --no-instructions > "$data_dir/initdb.log"
"$pgbin/pg_ctl" -D "$data_dir/pgdata" -l "$data_dir/postgres.log" -o "-h 127.0.0.1 -p $port" start >/dev/null
trap '"$pgbin/pg_ctl" -D "$data_dir/pgdata" stop -m fast >/dev/null 2>&1 || true' EXIT
password=$(cat "$data_dir/db-password")
PGPASSWORD="$password" "$pgbin/createdb" -h 127.0.0.1 -p "$port" -U tgtldr tgtldr
PGPASSWORD="$password" "$pgbin/psql" -h 127.0.0.1 -p "$port" -U tgtldr -d tgtldr -Atqc 'select 1' >/dev/null
printf 'initialized\n' > "$data_dir/.migration-complete"
chmod 600 "$data_dir/.migration-complete"
"$pgbin/pg_ctl" -D "$data_dir/pgdata" stop -m fast >/dev/null
trap - EXIT
echo '助手专用加密存储已初始化。'
