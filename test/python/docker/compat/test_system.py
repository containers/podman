import subprocess
import sys
import time
import unittest

from docker import DockerClient

from test.python.docker import Podman, constant
from test.python.docker.compat import common


class TestSystem(unittest.TestCase):
    podman = None  # initialized podman configuration for tests
    service = None  # podman service instance
    topContainerId = ""

    def setUp(self):
        super().setUp()
        self.client = DockerClient(base_url="tcp://127.0.0.1:8080", timeout=15)

        TestSystem.podman.restore_image_from_cache(self.client)
        TestSystem.topContainerId = common.run_top_container(self.client)

    def tearDown(self):
        common.remove_all_containers(self.client)
        common.remove_all_images(self.client)
        self.client.close()
        return super().tearDown()

    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        TestSystem.podman = Podman()
        TestSystem.service = TestSystem.podman.open(
            "system", "service", "tcp:127.0.0.1:8080", "--time=0"
        )
        # give the service some time to be ready...
        time.sleep(2)

        returncode = TestSystem.service.poll()
        if returncode is not None:
            raise subprocess.CalledProcessError(returncode, "podman system service")

    @classmethod
    def tearDownClass(cls):
        TestSystem.service.terminate()
        stdout, stderr = TestSystem.service.communicate(timeout=0.5)
        if stdout:
            sys.stdout.write("\nImages Service Stdout:\n" + stdout.decode("utf-8"))
        if stderr:
            sys.stderr.write("\nImAges Service Stderr:\n" + stderr.decode("utf-8"))

        TestSystem.podman.tear_down()
        return super().tearDownClass()

    def test_Info(self):
        info = self.client.info()
        self.assertIsNotNone(info)
        self.assertEqual(info["RegistryConfig"]["IndexConfigs"]["localhost:5000"]["Secure"], False)
        self.assertEqual(info["RegistryConfig"]["IndexConfigs"]["localhost:5000"]["Mirrors"], ["mirror.localhost:5000"])

    def test_info_container_details(self):
        info = self.client.info()
        self.assertEqual(info["Containers"], 1)
        self.client.containers.create(image=constant.ALPINE)
        info = self.client.info()
        self.assertEqual(info["Containers"], 2)

    def test_version(self):
        version = self.client.version()
        self.assertIsNotNone(version["Platform"]["Name"])
