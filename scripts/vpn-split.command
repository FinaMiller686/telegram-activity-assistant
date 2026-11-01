#!/bin/bash
set -euo pipefail
task_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
echo '旧的全局网络恢复方案已停用，改用仅接管同济 API 的专用通道。'
exec /bin/bash "$task_root/campus-api-vpn.command" "$@"
