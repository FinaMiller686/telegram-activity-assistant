#!/bin/bash
set -euo pipefail
export LC_ALL=en_US.UTF-8
project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
native_dir="$project_dir/.local/native"
data_dir="$native_dir/activity-data"
storage_tool="$project_dir/scripts/encrypted-storage.sh"
pg_prefix="$(brew --prefix postgresql@17)"
pgbin="$pg_prefix/bin"
pgport=55433
webport=3000
apiport=18080
mkdir -p "$native_dir"
chmod 700 "$project_dir/.local" "$native_dir"
umask 077

# Never load or print Telegram credentials. Those are entered in the local UI.
prepare_db() {
  if [ ! -f "$data_dir/db-password" ]; then echo "加密数据库凭据缺失，未初始化新数据库。" >&2; return 1; fi
  if [ ! -f "$data_dir/pgdata/PG_VERSION" ]; then
    echo "加密数据库文件缺失，未创建空数据库。" >&2
    return 1
  fi
}
start_db() {
  prepare_db || return 1
  if ! "$pgbin/pg_ctl" -D "$data_dir/pgdata" status >/dev/null 2>&1; then
    "$pgbin/pg_ctl" -D "$data_dir/pgdata" -l "$data_dir/postgres.log" -o "-h 127.0.0.1 -p $pgport" start >/dev/null || return 1
  fi
  local password; password=$(cat "$data_dir/db-password")
  PGPASSWORD="$password" "$pgbin/psql" -h 127.0.0.1 -p "$pgport" -U tgtldr -d tgtldr -Atqc 'select 1' >/dev/null || return 1
}
alive() {
  local pid_file="$1" expected="$2" pid command
  [ -f "$pid_file" ] || return 1
  pid=$(cat "$pid_file")
  [[ "$pid" =~ ^[0-9]+$ ]] || return 1
  kill -0 "$pid" 2>/dev/null || return 1
  command=$(ps -o command= -p "$pid" 2>/dev/null || true)
  if [ "$expected" = "$project_dir/web/.next/standalone/server.js" ] && [[ "$command" == next-server* ]]; then
    /usr/sbin/lsof -nP -a -p "$pid" -iTCP:"$webport" -sTCP:LISTEN -t >/dev/null 2>&1
    return
  fi
  [[ "$command" == *"$expected"* ]]
}
build() {
  command -v go >/dev/null
  command -v node >/dev/null
  command -v npm >/dev/null
  mkdir -p "$project_dir/app/bin" "$project_dir/app/.cache"
  (cd "$project_dir/app" && GOCACHE="$project_dir/app/.cache" go build -o "$project_dir/app/bin/tgtldr" ./cmd/server)
  (cd "$project_dir/web" && npm ci && npm run build && mkdir -p .next/standalone/.next && cp -R .next/static .next/standalone/.next/)
  echo '构建完成。'
}
start() {
  if [ ! -x "$project_dir/app/bin/tgtldr" ] || [ ! -f "$project_dir/web/.next/standalone/server.js" ]; then echo '请先运行 scripts/macos-native.sh build。' >&2; exit 1; fi
  "$storage_tool" mount || return 1
  "$storage_tool" verify || { echo '助手加密存储验证失败。' >&2; return 1; }
  if [ ! -f "$data_dir/.migration-complete" ]; then echo '加密存储尚未完成迁移，未启动。' >&2; return 1; fi
  start_db || return 1
  local password master
  password=$(cat "$data_dir/db-password")
  master=$("$storage_tool" get-master) || return 1
  if ! alive "$native_dir/app.pid" "$project_dir/app/bin/tgtldr"; then
    nohup env TGTLDR_STORAGE_PROTECTED=true TGTLDR_DATABASE_URL="postgres://tgtldr:$password@127.0.0.1:$pgport/tgtldr?sslmode=disable" TGTLDR_HTTP_ADDR="127.0.0.1:$apiport" TGTLDR_WEB_ORIGIN="http://localhost:$webport" TGTLDR_MASTER_KEY="$master" TGTLDR_MEDIA_DIR="$data_dir/media" "$project_dir/app/bin/tgtldr" >> "$data_dir/app.log" 2>&1 </dev/null &
    echo "$!" > "$native_dir/app.pid"
  fi
  if ! alive "$native_dir/web.pid" "$project_dir/web/.next/standalone/server.js"; then
    nohup env TGTLDR_INTERNAL_API_BASE_URL="http://127.0.0.1:$apiport" HOSTNAME=127.0.0.1 PORT="$webport" node "$project_dir/web/.next/standalone/server.js" >> "$data_dir/web.log" 2>&1 </dev/null &
    echo "$!" > "$native_dir/web.pid"
  fi
  for attempt in {1..30}; do
    if alive "$native_dir/app.pid" "$project_dir/app/bin/tgtldr" && alive "$native_dir/web.pid" "$project_dir/web/.next/standalone/server.js" && curl -fsS "http://127.0.0.1:$apiport/api/health" >/dev/null 2>&1 && curl -fsS "http://127.0.0.1:$webport/" >/dev/null 2>&1; then echo "已启动：http://localhost:$webport"; return; fi
    sleep 1
  done
  echo "启动失败，请检查 $native_dir 下的日志。" >&2; return 1
}
stop() {
  local service pid_file expected
  for service in web app; do
    pid_file="$native_dir/$service.pid"
    if [ "$service" = web ]; then expected="$project_dir/web/.next/standalone/server.js"; else expected="$project_dir/app/bin/tgtldr"; fi
    if alive "$pid_file" "$expected"; then kill "$(cat "$pid_file")"; fi
    rm -f "$pid_file"
  done
  if [ -f "$data_dir/pgdata/PG_VERSION" ] && "$pgbin/pg_ctl" -D "$data_dir/pgdata" status >/dev/null 2>&1; then "$pgbin/pg_ctl" -D "$data_dir/pgdata" stop -m fast >/dev/null; fi
  "$storage_tool" unmount || { echo "加密存储仍在使用，未卸载。" >&2; return 1; }
  echo '已停止。'
}
case "${1:-}" in
  build) build ;;
  start) start ;;
  stop) stop ;;
  status) for service in app web; do if [ "$service" = web ]; then expected="$project_dir/web/.next/standalone/server.js"; else expected="$project_dir/app/bin/tgtldr"; fi; if alive "$native_dir/$service.pid" "$expected"; then echo "$service: running"; else echo "$service: stopped"; fi; done ;;
  supervise) trap 'stop' EXIT INT TERM; while true; do start || true; sleep 30; done ;;
  *) echo "用法：$0 {build|start|stop|status|supervise}"; exit 2 ;;
esac
