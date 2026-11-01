#!/usr/bin/python3
"""Temporary, reversible routing and split DNS for one authorized campus service."""
import ipaddress
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import time

DOMAIN = 'agi.tongji.edu.cn'
ROOT = Path(__file__).resolve().parent.parent
DATA = ROOT / '.local/native/activity-data'
RESOLVER = Path('/etc/resolver') / DOMAIN
HALVES = ('0.0.0.0/1', '128.0.0.0/1')
STATE_KEYS = ('interface', 'gateway', 'destination', 'mask')
JOURNAL = DATA / 'network-diagnostics/split-session.json'
REPORT = DATA / 'network-diagnostics/split-result.json'


def run(args):
    try:
        result = subprocess.run(args, text=True, capture_output=True, timeout=12)
    except (OSError, subprocess.TimeoutExpired):
        raise RuntimeError('本机网络命令未完成，已停止操作。')
    if result.returncode:
        raise RuntimeError('网络配置命令被系统拒绝；不会覆盖已有路由。')
    return result.stdout


def route_to(target):
    output = run(['/sbin/route', '-n', 'get', target])
    result = {}
    for key in STATE_KEYS:
        match = re.search(r'^\s*' + key + r':\s*(.+)$', output, re.M)
        if match:
            result[key] = match.group(1).strip()
    return result


def tunnel(name):
    return bool(re.fullmatch(r'(?:utun|ppp|tun)\d+', name or ''))


def primary_dns():
    output = run(['/usr/sbin/scutil', '--dns'])
    block = re.split(r'resolver #\d+', output)[1]
    result = []
    for address in re.findall(r'nameserver\[\d+\]\s*:\s*(\S+)', block):
        try:
            ipaddress.ip_address(address)
            result.append(address)
        except ValueError:
            pass
    return result


def service_for(interface):
    text = run(['/usr/sbin/networksetup', '-listnetworkserviceorder'])
    current = ''
    for line in text.splitlines():
        match = re.match(r'^\(\d+\)\s+(.+)', line)
        if match:
            current = match.group(1)
        if current and re.search(r'Device:\s*' + re.escape(interface) + r'\)', line):
            return current
    raise RuntimeError('无法确定普通网络服务，未修改网络。')


def static_dns(service):
    result = []
    for line in run(['/usr/sbin/networksetup', '-getdnsservers', service]).splitlines():
        try:
            ipaddress.ip_address(line.strip())
            result.append(line.strip())
        except ValueError:
            pass
    return result


def campus_nameservers(current, ordinary):
    ordinary_set = {str(ipaddress.ip_address(v)) for v in ordinary}
    return list(dict.fromkeys(str(ipaddress.ip_address(v)) for v in current
                             if ':' not in v and not ipaddress.ip_address(v).is_loopback
                             and str(ipaddress.ip_address(v)) not in ordinary_set))


def reachable(url):
    # No cookies, credentials, or conversation contents. HTTP 403 can still prove TLS reachability.
    try:
        result = subprocess.run(['/usr/bin/curl', '--silent', '--connect-timeout', '4', '--max-time', '8',
                                 '--output', '/dev/null', '--write-out', '%{http_code}', url],
                                text=True, capture_output=True, timeout=10)
        status = result.stdout.strip()
        return {'connected': result.returncode == 0 and status.isdigit() and status != '000',
                'http_status': int(status) if status.isdigit() else 0, 'curl_code': result.returncode}
    except (OSError, subprocess.TimeoutExpired):
        return {'connected': False, 'http_status': 0, 'curl_code': -1}


def write_report(report):
    temporary = REPORT.with_suffix('.tmp')
    temporary.write_text(json.dumps(report), encoding='utf-8')
    temporary.chmod(0o600)
    uid = os.environ.get('SUDO_UID', '')
    if uid.isdigit() and int(uid) > 0:
        os.chown(temporary, int(uid), -1)
    temporary.replace(REPORT)


def routing_table():
    rows = []
    for line in run(['/usr/sbin/netstat', '-rn', '-f', 'inet']).splitlines():
        parts = line.split()
        if len(parts) < 4:
            continue
        dest = {'0/1': HALVES[0], '128.0/1': HALVES[1], '128/1': HALVES[1]}.get(parts[0], parts[0])
        interface = next((v for v in parts[3:] if re.fullmatch(r'(?:en|utun|ppp|tun|lo|bridge)\d+', v)), '')
        rows.append((dest, parts[1], interface))
    return rows


def host_addresses():
    output = run(['/usr/bin/dscacheutil', '-q', 'host', '-a', 'name', DOMAIN])
    values = re.findall(r'(?m)^ip_address:\s*(\S+)', output)
    result = sorted({str(ipaddress.IPv4Address(v)) for v in values})
    if not result:
        raise RuntimeError('校园网络无法解析同济接口，未应用分流。')
    return result


class Session:
    def __init__(self):
        self.routes = []
        self.service = ''
        self.original_dns = []
        self.ordinary_dns = []
        self.resolver_text = ''
        self.dns_changed = False
        self.resolver_created = False

    def persist(self):
        temporary = JOURNAL.with_suffix('.tmp')
        temporary.write_text(json.dumps(self.__dict__), encoding='utf-8')
        temporary.chmod(0o600)
        temporary.replace(JOURNAL)

    def add_route(self, destination, gateway, interface, host=False):
        if any(row[0] in (destination, destination + '/32') for row in routing_table()):
            if host and route_to(destination).get('interface') == interface:
                return
            raise RuntimeError('存在同名路由，未覆盖。请先停止其它分流工具。')
        args = ['/sbin/route', '-n', 'add', '-host' if host else '-net', destination]
        args += ['-interface', interface] if host else [gateway]
        run(args)
        self.routes.append((destination, gateway, interface, host))
        self.persist()

    def cleanup(self):
        failures = []
        try:
            if self.dns_changed and static_dns(self.service) == self.ordinary_dns:
                run(['/usr/sbin/networksetup', '-setdnsservers', self.service] + (self.original_dns or ['Empty']))
            if self.resolver_created and RESOLVER.exists() and RESOLVER.read_text() == self.resolver_text:
                RESOLVER.unlink()
        except (OSError, RuntimeError):
            failures.append('DNS')
        for destination, gateway, interface, host in reversed(self.routes):
            try:
                existing = [row for row in routing_table() if row[0] in (destination, destination + '/32')]
                if not existing:
                    continue
                if any(row[2] != interface or (not host and row[1] != gateway) for row in existing):
                    failures.append('路由已被其它程序修改')
                    continue
                run(['/sbin/route', '-n', 'delete', '-host' if host else '-net', destination])
            except RuntimeError:
                failures.append('路由')
        if failures:
            print('部分设置未能自动撤销：' + '、'.join(failures) + '。请断开 MotionPro 并检查网络设置。', flush=True)
        else:
            print('本次添加的分流设置已撤销。', flush=True)
        return not failures


def main():
    if '--apply' in sys.argv:
        raise SystemExit('旧版分流已停用，请运行 scripts/campus-api-vpn.command；恢复旧设置仍可使用 --restore。')
    if not any(arg in sys.argv for arg in ('--apply', '--restore')):
        print('用途：sudo /usr/bin/python3 scripts/vpn-split.py --apply')
        return
    if os.geteuid() != 0:
        raise SystemExit('需要本机管理员授权；请通过 vpn-split.command 启动，勿把密码发到聊天里。')
    if not DATA.is_mount():
        raise SystemExit('助手加密区未挂载，请先启动助手。')
    folder = DATA / 'network-diagnostics'
    folder.mkdir(mode=0o700, exist_ok=True)
    lock = folder / 'split.lock'
    if '--restore' in sys.argv:
        if lock.exists():
            pid = json.loads(lock.read_text()).get('pid')
            try:
                os.kill(int(pid), 0)
                raise SystemExit('分流工具仍在运行，请在原窗口按 Ctrl+C。')
            except ProcessLookupError:
                pass
        if not JOURNAL.exists():
            print('没有待恢复的分流设置。')
            return
        saved = json.loads(JOURNAL.read_text())
        recovery = Session()
        for key in recovery.__dict__:
            if key in saved:
                setattr(recovery, key, saved[key])
        if recovery.cleanup():
            JOURNAL.unlink(missing_ok=True)
            lock.unlink(missing_ok=True)
        return
    if lock.exists():
        raise SystemExit('已有分流工具或上次未正常退出。请先检查，未启动第二份。')
    if JOURNAL.exists():
        raise SystemExit('存在待恢复记录，请先运行 vpn-restore.command。')
    if RESOLVER.exists():
        raise SystemExit('此域名已有 DNS 配置，未覆盖。')
    normal = route_to('1.1.1.1')
    interface, gateway = normal.get('interface', ''), normal.get('gateway', '')
    if tunnel(interface) or not interface.startswith('en'):
        raise SystemExit('请先断开 MotionPro，并使用普通网络启动本工具。')
    try:
        ipaddress.IPv4Address(gateway)
    except ValueError:
        raise SystemExit('普通网络网关不明确，未修改网络。')
    if any(row[0] in HALVES for row in routing_table()):
        raise SystemExit('检测到其它软件的全局分流路由，未覆盖。')
    session = Session()
    session.service = service_for(interface)
    session.original_dns = static_dns(session.service)
    session.ordinary_dns = primary_dns()
    if not session.ordinary_dns:
        raise SystemExit('无法读取普通网络 DNS，未修改网络。')
    report = {'stage': 'waiting_vpn', 'baseline_chatgpt': reachable('https://chatgpt.com'), 'applied': False}
    write_report(report)
    os.umask(0o077)
    fd = os.open(str(lock), os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
    with os.fdopen(fd, 'w') as stream:
        json.dump({'pid': os.getpid()}, stream)
    session.persist()
    def interrupted(*_):
        raise KeyboardInterrupt()
    for sig in (signal.SIGINT, signal.SIGTERM, signal.SIGHUP):
        signal.signal(sig, interrupted)
    try:
        print('现在请自行连接 MotionPro。此窗口在断网期间仍会运行。', flush=True)
        while True:
            current = route_to('1.1.1.1')
            if tunnel(current.get('interface')):
                vpn = current['interface']
                break
            time.sleep(2)
        time.sleep(5)
        campus_dns = campus_nameservers(primary_dns(), session.ordinary_dns)
        if not campus_dns:
            raise RuntimeError('没有识别到校园 DNS，未应用分流。')
        report['campus_dns_distinct_from_ordinary'] = not bool(set(campus_dns) & set(session.ordinary_dns))
        write_report(report)
        for address in sorted(set(host_addresses() + campus_dns)):
            session.add_route(address, '', vpn, host=True)
        for prefix in HALVES:
            session.add_route(prefix, gateway, interface)
        RESOLVER.parent.mkdir(mode=0o755, exist_ok=True)
        session.resolver_text = ''.join('nameserver ' + server + '\n' for server in campus_dns) + 'timeout 3\n'
        with RESOLVER.open('x') as stream:
            stream.write(session.resolver_text)
        RESOLVER.chmod(0o644)
        session.resolver_created = True
        session.dns_changed = True
        session.persist()
        run(['/usr/sbin/networksetup', '-setdnsservers', session.service] + session.ordinary_dns)
        time.sleep(2)
        if route_to('1.1.1.1').get('interface') != interface:
            raise RuntimeError('普通流量未恢复，撤销分流。')
        if set(primary_dns()) != set(session.ordinary_dns):
            raise RuntimeError('VPN 仍在强制覆盖 DNS，撤销分流；需要使用学校支持的分流配置。')
        if any(route_to(ip).get('interface') != interface for ip in session.ordinary_dns if ':' not in ip):
            raise RuntimeError('普通 DNS 仍被 VPN 接管，撤销分流；需要学校支持的分流配置。')
        if any(route_to(ip).get('interface') != vpn for ip in host_addresses()):
            raise RuntimeError('同济接口未走校园 VPN，撤销分流。')
        run(['/usr/bin/dscacheutil', '-flushcache'])
        print('路由与 DNS 已设置，正在验证网络连通性……', flush=True)
        report['after_chatgpt'] = reachable('https://chatgpt.com')
        report['ordinary_dns_on_original_network'] = True
        report['campus_ai_on_vpn'] = True
        write_report(report)
        if report['baseline_chatgpt']['connected'] and not report['after_chatgpt']['connected']:
            raise RuntimeError('分流后的 GPT 网络探测失败，已停止并撤销设置。')
        report['applied'] = True
        report['stage'] = 'active'
        write_report(report)
        print('分流已开启：同济 AI 接口和校园 DNS 走 VPN，其余 IPv4 流量走原网络。', flush=True)
        print('请保持此窗口开启。断开 MotionPro 或按 Ctrl+C 会撤销本次设置。', flush=True)
        next_refresh = time.monotonic() + 30
        while vpn in run(['/sbin/ifconfig', '-l']).split():
            time.sleep(3)
            if vpn not in run(['/sbin/ifconfig', '-l']).split():
                break
            if route_to('1.1.1.1').get('interface') != interface or set(primary_dns()) != set(session.ordinary_dns):
                raise RuntimeError('VPN 再次覆盖网络设置，撤销分流；请使用学校支持的分流配置。')
            if time.monotonic() >= next_refresh:
                for address in host_addresses():
                    session.add_route(address, '', vpn, host=True)
                next_refresh = time.monotonic() + 30
    except KeyboardInterrupt:
        pass
    except (RuntimeError, OSError, IndexError, ValueError) as error:
        # Only locally generated messages are displayed; subprocess output is never echoed.
        print(str(error) if isinstance(error, RuntimeError) else '分流未完成，正在恢复原设置。', flush=True)
        report['stage'] = 'failed'
    finally:
        restored = session.cleanup()
        report['restored'] = restored
        report['stage'] = 'stopped' if report['stage'] != 'failed' else 'failed'
        write_report(report)
        if restored:
            JOURNAL.unlink(missing_ok=True)
        lock.unlink(missing_ok=True)


if __name__ == '__main__':
    main()
