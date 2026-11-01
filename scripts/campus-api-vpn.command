#!/bin/bash
# Dedicated campus host route. No full-tunnel MotionPro session is used.
set -euo pipefail
task_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
brew_path=$(command -v brew || true)
if [ -z "$brew_path" ]; then
  echo '找不到 Homebrew，请先安装 openconnect 和 vpn-slice。' >&2
  exit 1
fi
brew_prefix=$("$brew_path" --prefix)
vpn_client="$brew_prefix/bin/openconnect"
slice_client="$brew_prefix/bin/vpn-slice"
if [ ! -x "$vpn_client" ] || [ ! -x "$slice_client" ]; then
  echo '缺少依赖，请运行：brew install openconnect vpn-slice' >&2
  exit 1
fi
/usr/bin/python3 - "$task_root" <<'PY'
from pathlib import Path
import re,subprocess,sys
root=Path(sys.argv[1])
folder=root/'.local/native/activity-data/network-diagnostics'
if (folder/'split.lock').exists() or (folder/'split-session.json').exists():
    raise SystemExit('旧分流工具尚有恢复记录，请先运行 scripts/vpn-restore.command。')
if Path('/etc/resolver/agi.tongji.edu.cn').exists():
    raise SystemExit('此域名已有独立 DNS 配置，请先核对；未覆盖。')
result=subprocess.run(['/sbin/route','-n','get','1.1.1.1'],text=True,capture_output=True)
match=re.search(r'interface:\s*(\S+)',result.stdout)
if not match or not re.fullmatch(r'en\d+',match.group(1)):
    raise SystemExit('请先断开 MotionPro，并恢复普通网络后再运行。')
print('准备就绪：只为 agi.tongji.edu.cn 建立校园通道，不接管默认路由或默认 DNS。')
PY
if [ "${1:-}" = '--check' ]; then
  exit 0
fi
echo '请保持 MotionPro 断开，本工具会自行连接同一校园 VPN 网关。'
echo '先输入本机管理员密码，再按提示输入校园账号、密码及验证码。'
echo '这些凭据只在本机 Terminal 输入，不会保存到脚本。'
echo '保持窗口开启；按 Ctrl+C 断开并清理本次通道。'
# The restrictive flags intentionally omit --route-splits and --route-internal.
# vpn-slice resolves only this FQDN using VPN DNS and pins its hosts entry;
# it does not change the machine-wide default DNS or default route.
slice_hook="'$slice_client' --no-short-names --no-ns-hosts agi.tongji.edu.cn"
sudo "$slice_client" --self-test
# Gateway stated in the campus VPN manual supplied for this task.
sudo "$vpn_client" --protocol=array --script "$slice_hook" https://vpn.tongji.cn
read -r -p '连接已结束。按回车关闭窗口。' _campus_api_done
