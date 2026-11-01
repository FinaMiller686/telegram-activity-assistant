#!/bin/sh
set -eu
project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_dir"
umask 077
if [ -f .env ] && grep -Eq '^TGTLDR_DB_PASSWORD=[^[:space:]]+' .env; then
  chmod 600 .env
  echo '.env 已存在且包含数据库密码；未覆盖。'
  exit 0
fi
password=$(openssl rand -hex 32)
ACTIVITY_DB_PASSWORD="$password" python3 - <<'PY'
import os, pathlib, re
path = pathlib.Path('.env')
content = path.read_text() if path.exists() else 'TGTLDR_HOST_WEB_PORT=3000\n'
line = 'TGTLDR_DB_PASSWORD=' + os.environ['ACTIVITY_DB_PASSWORD']
if re.search(r'^TGTLDR_DB_PASSWORD=.*$', content, flags=re.M):
    content = re.sub(r'^TGTLDR_DB_PASSWORD=.*$', lambda _: line, content, count=1, flags=re.M)
else:
    content += ('\n' if content and not content.endswith('\n') else '') + line + '\n'
tmp = pathlib.Path('.env.new')
with tmp.open('w') as f:
    os.chmod(tmp, 0o600)
    f.write(content)
os.replace(tmp, path)
PY
echo '已创建或补全受保护的本地 .env。'
