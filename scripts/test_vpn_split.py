import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('vpn_split', Path(__file__).with_name('vpn-split.py'))
vpn = importlib.util.module_from_spec(spec)
spec.loader.exec_module(vpn)


class RecoveryTests(unittest.TestCase):
    def test_only_new_campus_dns_goes_into_tunnel(self):
        normal = ['192.0.2.53', '192.0.2.54']
        combined = ['203.0.113.53', '203.0.113.54'] + normal
        self.assertEqual(vpn.campus_nameservers(combined, normal), ['203.0.113.53', '203.0.113.54'])
        self.assertEqual(vpn.campus_nameservers(normal, normal), [])

    def test_restores_saved_dns_and_removes_only_own_routes(self):
        with tempfile.TemporaryDirectory() as tmp:
            resolver = Path(tmp) / 'resolver'
            resolver.write_text('fixture')
            session = vpn.Session()
            session.service = 'Wi-Fi'
            session.original_dns = []  # Originally provided by DHCP.
            session.ordinary_dns = ['192.0.2.53']
            session.dns_changed = True
            session.resolver_created = True
            session.resolver_text = 'fixture'
            session.routes = [('0.0.0.0/1', '192.0.2.1', 'en0', False), ('203.0.113.10', '', 'utun9', True)]
            commands = []
            table = [('0.0.0.0/1', '192.0.2.1', 'en0'), ('203.0.113.10', 'link#99', 'utun9'), ('198.51.100.8', '192.0.2.1', 'en0')]
            with patch.object(vpn, 'RESOLVER', resolver), patch.object(vpn, 'static_dns', return_value=session.ordinary_dns), patch.object(vpn, 'routing_table', return_value=table), patch.object(vpn, 'run', side_effect=lambda args: commands.append(args) or ''):
                self.assertTrue(session.cleanup())
            self.assertFalse(resolver.exists())
            self.assertIn(['/usr/sbin/networksetup', '-setdnsservers', 'Wi-Fi', 'Empty'], commands)
            self.assertEqual(sum('delete' in c for c in commands), 2)
            self.assertFalse(any('198.51.100.8' in c for c in commands))

    def test_preserves_configuration_changed_by_other_software(self):
        session = vpn.Session()
        session.service = 'Wi-Fi'
        session.ordinary_dns = ['192.0.2.53']
        session.dns_changed = True
        session.routes = [('0.0.0.0/1', '192.0.2.1', 'en0', False)]
        with patch.object(vpn, 'static_dns', return_value=['198.51.100.53']), patch.object(vpn, 'routing_table', return_value=[('0.0.0.0/1', '192.0.2.2', 'utun5')]), patch.object(vpn, 'run') as runner:
            self.assertFalse(session.cleanup())
            runner.assert_not_called()

    def test_existing_routes_are_never_overwritten(self):
        with patch.object(vpn, 'routing_table', return_value=[('0.0.0.0/1', '192.0.2.1', 'en0')]), patch.object(vpn, 'run') as runner:
            with self.assertRaises(RuntimeError):
                vpn.Session().add_route('0.0.0.0/1', '192.0.2.5', 'en1')
            runner.assert_not_called()


if __name__ == '__main__':
    unittest.main()
