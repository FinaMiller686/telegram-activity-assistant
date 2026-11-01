#!/bin/bash
set -euo pipefail
task_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
/usr/bin/python3 "$task_root/vpn-diagnose.py"
read -r -p '按回车关闭窗口。' _vpn_diagnostic_done
