#!/bin/bash
set -euo pipefail
set +x
umask 077
task_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
data_dir="$task_root/.local/native/activity-data"
"$task_root/scripts/encrypted-storage.sh" verify || { echo '助手加密区未解锁，未导出。' >&2; exit 1; }
if [ ! -x "$task_root/app/bin/export-text" ]; then echo '请先编译 export-text。' >&2; exit 1; fi
export_dir="${1:-$data_dir/exports/telegram-text-$(date +%Y%m%d-%H%M%S)}"
case "$export_dir" in "$data_dir/exports/"*) ;; *) echo '请将导出保存到助手加密区的 exports 子目录。' >&2; exit 1;; esac
database_password=$(cat "$data_dir/db-password")
app_master=$("$task_root/scripts/encrypted-storage.sh" get-master)
TGTLDR_STORAGE_PROTECTED=true \
TGTLDR_DATABASE_URL="postgres://tgtldr:$database_password@127.0.0.1:55433/tgtldr?sslmode=disable" \
TGTLDR_MASTER_KEY="$app_master" \
"$task_root/app/bin/export-text" --output "$export_dir"
