#!/bin/bash
set -euo pipefail
set +x
project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
native_dir="${ACTIVITY_NATIVE_DIR:-$project_dir/.local/native}"
image="$native_dir/activity-data.sparsebundle"
mount_dir="$native_dir/activity-data"
account=$(printf '%s' "$native_dir" | /usr/bin/shasum -a 256 | cut -c1-32)
volume_service='local.activity-assistant.volume.v1'
master_service='local.activity-assistant.master.v1'
umask 077
mkdir -p "$native_dir"
chmod 700 "$native_dir"
keychain_get() { /usr/bin/security find-generic-password -a "$account" -s "$1" -w 2>/dev/null; }
keychain_store() {
  local service="$1" secret="$2" saved
  if saved=$(keychain_get "$service"); then [ "$saved" = "$secret" ] || { echo '钥匙串中已有不同的助手钥匙，未覆盖。' >&2; return 1; }; return 0; fi
  printf 'add-generic-password -a %s -s %s -w %s\n' "$account" "$service" "$secret" | /usr/bin/security -i >/dev/null 2>&1
  saved=$(keychain_get "$service") || return 1
  [ "$saved" = "$secret" ]
}
image_is_encrypted() {
  /usr/bin/hdiutil isencrypted -plist "$image" 2>/dev/null | /usr/bin/python3 -c 'import plistlib,sys; d=plistlib.loads(sys.stdin.buffer.read()); sys.exit(0 if d.get("encrypted") is True else 1)'
}
image_is_mounted() {
  /usr/bin/hdiutil info -plist 2>/dev/null | /usr/bin/python3 -c 'import os,plistlib,sys; image=os.path.realpath(sys.argv[1]); mount=os.path.realpath(sys.argv[2]); d=plistlib.loads(sys.stdin.buffer.read()); sys.exit(0 if any(os.path.realpath(i.get("image-path", "")) == image and i.get("image-encrypted") is True and any(os.path.realpath(e.get("mount-point", "")) == mount for e in i.get("system-entities", [])) for i in d.get("images", [])) else 1)' "$image" "$mount_dir"
}
init() {
  if [ -e "$image" ]; then image_is_mounted || image_is_encrypted || { echo '已有存储映像未通过加密检查。' >&2; return 1; }; keychain_get "$volume_service" >/dev/null || { echo '找不到该加密映像的钥匙；未创建或覆盖。' >&2; return 1; }; return 0; fi
  if keychain_get "$volume_service" >/dev/null; then echo '发现已有助手钥匙但映像缺失；未覆盖，请先恢复映像。' >&2; return 1; fi
  local secret; secret=$(openssl rand -hex 32)
  keychain_store "$volume_service" "$secret" || { echo '无法将加密钥匙保存到 macOS 钥匙串。' >&2; return 1; }
  if ! printf '%s\n' "$secret" | /usr/bin/hdiutil create -size 4g -type SPARSEBUNDLE -fs HFS+J -volname ActivityAssistant -encryption AES-256 -stdinpass "$image" >"$native_dir/encrypted-create.log" 2>&1; then
    echo '无法创建加密映像；已保留钥匙以便恢复或排查。' >&2; return 1
  fi
  image_is_encrypted || { echo '映像未通过加密验证。' >&2; return 1; }
}
mount_image() {
  if image_is_mounted; then return 0; fi
  [ -e "$image" ] && image_is_encrypted || { echo '助手加密映像不可用。' >&2; return 1; }
  mkdir -p "$mount_dir"
  if [ -n "$(ls -A "$mount_dir")" ]; then echo '挂载点不是空目录，未覆盖。' >&2; return 1; fi
  if ! keychain_get "$volume_service" | /usr/bin/hdiutil attach -nobrowse -mountpoint "$mount_dir" -stdinpass "$image" >"$native_dir/encrypted-mount.log" 2>&1; then
    echo '无法解锁助手数据，请确认登录钥匙串已解锁。' >&2; return 1
  fi
  image_is_mounted || { echo '加密映像挂载验证失败。' >&2; return 1; }
}
case "${1:-}" in
  init) init ;;
  mount) mount_image ;;
  verify) image_is_mounted && keychain_get "$volume_service" >/dev/null && keychain_get "$master_service" >/dev/null ;;
  import-master)
    [ -f "$native_dir/master.key" ] || { echo '未找到原有助手主密钥。' >&2; exit 1; }
    master=$(cat "$native_dir/master.key")
    keychain_store "$master_service" "$master"
    ;;
  init-master)
    if keychain_get "$master_service" >/dev/null; then exit 0; fi
    if [ -e "$native_dir/master.key" ] || [ -e "$native_dir/pgdata" ] || [ -e "$mount_dir/pgdata" ]; then
      echo '发现既有助手数据；未创建新主密钥，请先迁移或恢复旧密钥。' >&2; exit 1
    fi
    master=$(openssl rand -base64 32)
    keychain_store "$master_service" "$master"
    ;;
  get-master) keychain_get "$master_service" ;;
  unmount) if image_is_mounted; then /usr/bin/hdiutil detach "$mount_dir" >/dev/null; fi ;;
  *) echo "用法：$0 {init|mount|verify|import-master|init-master|get-master|unmount}" >&2; exit 2 ;;
esac
