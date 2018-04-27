import unittest

from podman_testcase import PodmanTestCase


class TestSystemAPI(PodmanTestCase):
    def test_ping(self):
        response = self.podman.Ping()
        self.assertEqual('OK', response['ping']['message'])

    def test_GetVersion(self):
        response = self.podman.GetVersion()
        self.assertTrue(set(
            ['version', 'go_version', 'built', 'os_arch']
        ).issubset(response['version'].keys()))


if __name__ == '__main__':
    unittest.main()
