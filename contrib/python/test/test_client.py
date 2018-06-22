from __future__ import absolute_import

import unittest
from unittest.mock import patch

import podman
from podman.client import BaseClient, Client, LocalClient, RemoteClient


class TestClient(unittest.TestCase):
    def setUp(self):
        pass

    @patch('podman.libs.system.System.ping', return_value=True)
    def test_local(self, mock_ping):
        p = Client(
            uri='unix:/run/podman',
            interface='io.projectatomic.podman',
        )

        self.assertIsInstance(p._client, LocalClient)
        self.assertIsInstance(p._client, BaseClient)

        mock_ping.assert_called_once()

    @patch('os.path.isfile', return_value=True)
    @patch('podman.libs.system.System.ping', return_value=True)
    def test_remote(self, mock_ping, mock_isfile):
        p = Client(
            uri='unix:/run/podman',
            interface='io.projectatomic.podman',
            remote_uri='ssh://user@hostname/run/podmain/podman',
            identity_file='~/.ssh/id_rsa')

        self.assertIsInstance(p._client, BaseClient)
        mock_ping.assert_called_once()
        mock_isfile.assert_called_once()
