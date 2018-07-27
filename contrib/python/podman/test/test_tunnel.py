from __future__ import absolute_import

import time
import unittest
from unittest.mock import MagicMock, patch

from podman.libs.tunnel import Context, Portal, Tunnel


class TestTunnel(unittest.TestCase):
    def setUp(self):
        self.tunnel_01 = MagicMock(spec=Tunnel)
        self.tunnel_02 = MagicMock(spec=Tunnel)

    def test_portal_ops(self):
        portal = Portal(sweap=500)
        portal['unix:/01'] = self.tunnel_01
        portal['unix:/02'] = self.tunnel_02

        self.assertEqual(portal.get('unix:/01'), self.tunnel_01)
        self.assertEqual(portal.get('unix:/02'), self.tunnel_02)

        del portal['unix:/02']
        with self.assertRaises(KeyError):
            portal['unix:/02']
        self.assertEqual(len(portal), 1)

    def test_portal_reaping(self):
        portal = Portal(sweap=0.5)
        portal['unix:/01'] = self.tunnel_01
        portal['unix:/02'] = self.tunnel_02

        self.assertEqual(len(portal), 2)
        for entry in portal:
            self.assertIn(entry, (self.tunnel_01, self.tunnel_02))

        time.sleep(1)
        portal.reap()
        self.assertEqual(len(portal), 0)

    def test_portal_no_reaping(self):
        portal = Portal(sweap=500)
        portal['unix:/01'] = self.tunnel_01
        portal['unix:/02'] = self.tunnel_02

        portal.reap()
        self.assertEqual(len(portal), 2)
        for entry in portal:
            self.assertIn(entry, (self.tunnel_01, self.tunnel_02))

    @patch('subprocess.Popen')
    @patch('os.path.exists', return_value=True)
    @patch('weakref.finalize')
    def test_tunnel(self, mock_finalize, mock_exists, mock_Popen):
        context = Context(
            'unix:/01',
            'io.projectatomic.podman',
            '/tmp/user/socket',
            '/run/podman/socket',
            'user',
            'hostname',
            None,
            '~/.ssh/id_rsa',
        )
        tunnel = Tunnel(context).bore()

        cmd = [
            'ssh',
            '-fNT',
            '-q',
            '-L',
            '{}:{}'.format(context.local_socket, context.remote_socket),
            '-i',
            context.identity_file,
            '{}@{}'.format(context.username, context.hostname),
        ]

        mock_finalize.assert_called_once_with(tunnel, tunnel.close)
        mock_exists.assert_called_once_with(context.local_socket)
        mock_Popen.assert_called_once_with(cmd, close_fds=True)
