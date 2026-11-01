#!/usr/bin/python3
"""Read-only before/after VPN diagnostics. No credentials or traffic contents."""
import datetime
import json
import os
from pathlib import Path
import re
import subprocess
import time

ROOT = Path(__file__).resolve().parent.parent
DATA = ROOT / '.local/native/activity-data'


def command(args, timeout=4):
    try:
        result = subprocess.run(args, capture_output=True, text=True, timeout=timeout)
        return result.stdout if result.returncode == 0 else ''
    except (subprocess.TimeoutExpired, OSError):
        return ''


def route(destination, ipv6=False):
    args = ['/sbin/route', '-n', 'get']
    if ipv6:
        args.append('-inet6')
    output = command(args + [destination])
    # Keep only information needed for route selection, not local interface addresses.
    found = {}
    for name in ('interface', 'gateway', 'flags'):
        match = re.search(r'^\s*' + name + r':\s*(.+)$', output, re.M)
        if match:
            found[name] = match.group(1).strip()
    return found


def resolvers():
    output = command(['/usr/sbin/scutil', '--dns'])
    result = []
    for block in re.split(r'resolver #\d+', output)[1:]:
        item = {}
        for line in block.splitlines():
            match = re.match(r'\s*(domain|search domain\[\d+\]|nameserver\[\d+\]|if_index|flags|options|order)\s*:\s*(.+)', line)
            if match:
                item[match.group(1)] = match.group(2)
        if item:
            result.append(item)
    return result


def state():
    proxy_output = command(['/usr/sbin/scutil', '--proxy'])
    proxy = {}
    for name in ('HTTPEnable', 'HTTPSEnable', 'SOCKSEnable', 'ProxyAutoConfigEnable'):
        match = re.search(r'\b' + name + r'\s*:\s*(\d)', proxy_output)
        proxy[name] = match.group(1) if match else '0'
    return {'default_ipv4': route('default'), 'default_ipv6': route('default', True),
            'public_ipv4': route('1.1.1.1'), 'dns': resolvers(), 'proxy_enabled': proxy,
            'interfaces': sorted(command(['/sbin/ifconfig', '-l']).split())}


def snapshot():
    result = state()
    result['time'] = datetime.datetime.now(datetime.timezone.utc).isoformat()
    result['targets'] = {}
    for host in ('agi.tongji.edu.cn', 'api.telegram.org'):
        output = command(['/usr/bin/dscacheutil', '-q', 'host', '-a', 'name', host], timeout=5)
        ips = re.findall(r'(?:ip_address|ipv6_address):\s*(\S+)', output)
        result['targets'][host] = {'resolved': bool(ips), 'routes': [route(ip, ':' in ip) for ip in ips[:4]], 'addresses': ips[:4]}
    return result


def main():
    if not DATA.is_mount() or not (DATA / '.migration-complete').exists():
        raise SystemExit('助手加密存储未挂载；未生成诊断记录。请先启动助手。')
    os.umask(0o077)
    folder = DATA / 'network-diagnostics'
    folder.mkdir(mode=0o700, exist_ok=True)
    path = folder / 'latest.json'
    stop = folder / 'stop'
    stop.unlink(missing_ok=True)
    print('此工具只读检查网络，不修改 DNS、路由、代理或 VPN。', flush=True)
    print('正在记录普通网络状态，请暂时保持 VPN 断开……', flush=True)
    baseline = snapshot()
    records = {'before': baseline, 'after': [], 'network_change_detected': False}

    def save():
        temporary = path.with_suffix('.tmp')
        temporary.write_text(json.dumps(records, ensure_ascii=False, indent=2), encoding='utf-8')
        temporary.chmod(0o600)
        temporary.replace(path)

    save()
    state_keys = ('default_ipv4', 'default_ipv6', 'public_ipv4', 'dns', 'proxy_enabled', 'interfaces')
    baseline_state = {key: baseline[key] for key in state_keys}
    print('现在请自行连接 MotionPro。即使聊天断线，本工具仍会运行。', flush=True)
    print('连接后保持约 30 秒，再断开 VPN；不要在这里输入密码。', flush=True)
    try:
        while not stop.exists():
            sample = snapshot()
            changed = {key: sample[key] for key in state_keys} != baseline_state
            if changed and not records['network_change_detected']:
                print('检测到网络变化，正在本机采集结果……', flush=True)
            records['network_change_detected'] = records['network_change_detected'] or changed
            # Capture even if a VPN changes only a specific host route or a DNS daemon.
            records['after'].append(sample)
            records['after'] = records['after'][-120:]
            save()
            time.sleep(5)
    except KeyboardInterrupt:
        pass
    save()
    print('诊断完成。请断开校园 VPN，恢复联网后告诉助手“诊断完成”。', flush=True)
    print('记录仅保存在助手的本机加密区，不会自动上传。', flush=True)


if __name__ == '__main__':
    main()
