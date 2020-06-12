
import unittest
import docker
import requests
import os
from docker import Client
from . import constant
from . import common

client = common.get_client()

class TestContainers(unittest.TestCase):

    podman = None

    def setUp(self):
        super().setUp()
        common.run_top_container()

    def tearDown(self):
        common.remove_all_containers()
        common.remove_all_images()
        return super().tearDown()

    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        common.enable_sock(cls)

    @classmethod
    def tearDownClass(cls):
        common.terminate_connection(cls)
        return super().tearDownClass()

    def test_inspect_container(self):
        # Inspect bogus container
        with self.assertRaises(requests.HTTPError):
            client.inspect_container("dummy")
        # Inspect valid container
        container = client.inspect_container(constant.TOP)
        self.assertIn(constant.TOP , container["Name"])


if __name__ == '__main__':
    # Setup temporary space
    unittest.main()
