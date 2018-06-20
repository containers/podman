import os
import unittest
from urllib.parse import urlparse

import podman
import varlink


class TestSystem(unittest.TestCase):
    def setUp(self):
        self.host = os.environ['PODMAN_HOST']
        self.tmpdir = os.environ['TMPDIR']

    def tearDown(self):
        pass

    def test_bad_address(self):
        with self.assertRaisesRegex(varlink.client.ConnectionError,
                                    "Invalid address 'bad address'"):
            podman.Client('bad address')

    def test_ping(self):
        with podman.Client(self.host) as pclient:
            self.assertTrue(pclient.system.ping())

    def test_remote_ping(self):
        host = urlparse(self.host)
        remote_uri = 'ssh://root@localhost/{}'.format(host.path)

        local_uri = 'unix:{}/tunnel/podman.sock'.format(self.tmpdir)
        with podman.Client(
                uri=local_uri,
                remote_uri=remote_uri,
                identity_file=os.path.expanduser('~/.ssh/id_rsa'),
        ) as pclient:
            pclient.system.ping()

    def test_versions(self):
        with podman.Client(self.host) as pclient:
            # Values change with each build so we cannot test too much
            self.assertListEqual(
                sorted([
                    'built', 'client_version', 'git_commit', 'go_version',
                    'os_arch', 'version'
                ]), sorted(list(pclient.system.versions._fields)))
            pclient.system.versions
        self.assertIsNot(podman.__version__, '0.0.0')

    def test_info(self):
        with podman.Client(self.host) as pclient:
            actual = pclient.system.info()
            # Values change too much to do exhaustive testing
            self.assertIsNotNone(actual.podman['go_version'])
            self.assertListEqual(
                sorted([
                    'host', 'insecure_registries', 'podman', 'registries',
                    'store'
                ]), sorted(list(actual._fields)))


if __name__ == '__main__':
    unittest.main()
