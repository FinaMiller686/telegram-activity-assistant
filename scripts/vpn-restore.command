#!/bin/bash
set -euo pipefail
task_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
echo '本工具仅恢复 vpn-split.command 保存的原有 DNS，并撤销它添加的路由。'
sudo /usr/bin/python3 "$task_root/vpn-split.py" --restore
read -r -p '按回车关闭窗口。' _vpn_restore_done
