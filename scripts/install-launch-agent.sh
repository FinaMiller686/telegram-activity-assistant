#!/bin/bash
set -euo pipefail
project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
agent_dir="$HOME/Library/LaunchAgents"
mkdir -p "$agent_dir" "$project_dir/.local/native"
plist="$agent_dir/local.tgtldr.activity.plist"
if [ -e "$plist" ]; then
  echo "自动启动项已存在，未覆盖：$plist" >&2
  exit 1
fi
PROJECT_DIR="$project_dir" AGENT_PLIST="$plist" python3 - <<'PY'
import os, plistlib
root=os.environ['PROJECT_DIR']
config={'Label':'local.tgtldr.activity','ProgramArguments':['/bin/bash',root+'/scripts/macos-native.sh','supervise'],'RunAtLoad':True,'KeepAlive':True,'StandardOutPath':root+'/.local/native/launchd.log','StandardErrorPath':root+'/.local/native/launchd-error.log','EnvironmentVariables':{'PATH':'/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin','LC_ALL':'en_US.UTF-8'}}
with open(os.environ['AGENT_PLIST'],'wb') as f: plistlib.dump(config,f)
PY
launchctl bootout "gui/$(id -u)/local.tgtldr.activity" 2>/dev/null || true
launchctl bootstrap "gui/$(id -u)" "$plist"
echo '已设置登录后自动启动；睡眠或关机期间监听会暂停。'
