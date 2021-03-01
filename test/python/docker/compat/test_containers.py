import subprocess
import sys
import time
import unittest

from docker import DockerClient, errors

from test.python.docker import Podman
from test.python.docker.compat import common, constant


class TestContainers(unittest.TestCase):
    podman = None  # initialized podman configuration for tests
    service = None  # podman service instance
    topContainerId = ""

    def setUp(self):
        super().setUp()
        self.client = DockerClient(base_url="tcp://127.0.0.1:8080", timeout=15)
        TestContainers.podman.restore_image_from_cache(self.client)
        TestContainers.topContainerId = common.run_top_container(self.client)
        self.assertIsNotNone(TestContainers.topContainerId)

    def tearDown(self):
        common.remove_all_containers(self.client)
        common.remove_all_images(self.client)
        self.client.close()
        return super().tearDown()

    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        TestContainers.podman = Podman()
        TestContainers.service = TestContainers.podman.open(
            "system", "service", "tcp:127.0.0.1:8080", "--time=0"
        )
        # give the service some time to be ready...
        time.sleep(2)

        rc = TestContainers.service.poll()
        if rc is not None:
            raise subprocess.CalledProcessError(rc, "podman system service")

    @classmethod
    def tearDownClass(cls):
        TestContainers.service.terminate()
        stdout, stderr = TestContainers.service.communicate(timeout=0.5)
        if stdout:
            sys.stdout.write("\nContainers Service Stdout:\n" + stdout.decode("utf-8"))
        if stderr:
            sys.stderr.write("\nContainers Service Stderr:\n" + stderr.decode("utf-8"))

        TestContainers.podman.tear_down()
        return super().tearDownClass()

    def test_create_container(self):
        # Run a container with detach mode
        self.client.containers.create(image="alpine", detach=True)
        self.assertEqual(len(self.client.containers.list(all=True)), 2)

    def test_create_network(self):
        net = self.client.networks.create("testNetwork", driver="bridge")
        ctnr = self.client.containers.create(image="alpine", detach=True)

        #  TODO fix when ready
        # This test will not work until all connect|disconnect
        # code is fixed.
        # net.connect(ctnr)

        # nets = self.client.networks.list(greedy=True)
        # self.assertGreaterEqual(len(nets), 1)

        # TODO fix endpoint to include containers
        # for n in nets:
        #     if n.id == "testNetwork":
        #         self.assertEqual(ctnr.id, n.containers)
        # self.assertTrue(False, "testNetwork not found")

    def test_start_container(self):
        # Podman docs says it should give a 304 but returns with no response
        # # Start a already started container should return 304
        # response = self.client.api.start(container=TestContainers.topContainerId)
        # self.assertEqual(error.exception.response.status_code, 304)

        # Create a new container and validate the count
        self.client.containers.create(image=constant.ALPINE, name="container2")
        containers = self.client.containers.list(all=True)
        self.assertEqual(len(containers), 2)

    def test_start_container_with_random_port_bind(self):
        container = self.client.containers.create(
            image=constant.ALPINE,
            name="containerWithRandomBind",
            ports={"1234/tcp": None},
        )
        containers = self.client.containers.list(all=True)
        self.assertTrue(container in containers)

    def test_stop_container(self):
        top = self.client.containers.get(TestContainers.topContainerId)
        self.assertEqual(top.status, "running")

        # Stop a running container and validate the state
        top.stop()
        top.reload()
        self.assertIn(top.status, ("stopped", "exited"))

    def test_kill_container(self):
        top = self.client.containers.get(TestContainers.topContainerId)
        self.assertEqual(top.status, "running")

        # Kill a running container and validate the state
        top.kill()
        top.reload()
        self.assertIn(top.status, ("stopped", "exited"))

    def test_restart_container(self):
        # Validate the container state
        top = self.client.containers.get(TestContainers.topContainerId)
        top.stop()
        top.reload()
        self.assertIn(top.status, ("stopped", "exited"))

        # restart a running container and validate the state
        top.restart()
        top.reload()
        self.assertEqual(top.status, "running")

    def test_remove_container(self):
        # Remove container by ID with force
        top = self.client.containers.get(TestContainers.topContainerId)
        top.remove(force=True)
        self.assertEqual(len(self.client.containers.list()), 0)

    def test_remove_container_without_force(self):
        # Validate current container count
        self.assertTrue(len(self.client.containers.list()), 1)

        # Remove running container should throw error
        top = self.client.containers.get(TestContainers.topContainerId)
        with self.assertRaises(errors.APIError) as error:
            top.remove()
        self.assertEqual(error.exception.response.status_code, 500)

        # Remove container by ID without force
        top.stop()
        top.remove()
        self.assertEqual(len(self.client.containers.list()), 0)

    def test_pause_container(self):
        # Validate the container state
        top = self.client.containers.get(TestContainers.topContainerId)
        self.assertEqual(top.status, "running")

        # Pause a running container and validate the state
        top.pause()
        top.reload()
        self.assertEqual(top.status, "paused")

    def test_pause_stopped_container(self):
        # Stop the container
        top = self.client.containers.get(TestContainers.topContainerId)
        top.stop()

        # Pause exited container should throw error
        with self.assertRaises(errors.APIError) as error:
            top.pause()
        self.assertEqual(error.exception.response.status_code, 500)

    def test_unpause_container(self):
        top = self.client.containers.get(TestContainers.topContainerId)

        # Validate the container state
        top.pause()
        top.reload()
        self.assertEqual(top.status, "paused")

        # Pause a running container and validate the state
        top.unpause()
        top.reload()
        self.assertEqual(top.status, "running")

    def test_list_container(self):
        # Add container and validate the count
        self.client.containers.create(image="alpine", detach=True)
        containers = self.client.containers.list(all=True)
        self.assertEqual(len(containers), 2)

    def test_filters(self):
        self.skipTest("TODO Endpoint does not yet support filters")

        # List container with filter by id
        filters = {"id": TestContainers.topContainerId}
        ctnrs = self.client.containers.list(all=True, filters=filters)
        self.assertEqual(len(ctnrs), 1)

        # List container with filter by name
        filters = {"name": "top"}
        ctnrs = self.client.containers.list(all=True, filters=filters)
        self.assertEqual(len(ctnrs), 1)
