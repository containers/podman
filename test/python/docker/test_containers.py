import subprocess
import sys
import time
import unittest

from docker import APIClient, errors

from test.python.docker import Podman, common, constant


class TestContainers(unittest.TestCase):
    podman = None  # initialized podman configuration for tests
    service = None  # podman service instance
    topContainerId = ""

    def setUp(self):
        super().setUp()
        self.client = APIClient(base_url="tcp://127.0.0.1:8080", timeout=15)
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

    def test_inspect_container(self):
        # Inspect bogus container
        with self.assertRaises(errors.NotFound) as error:
            self.client.inspect_container("dummy")
        self.assertEqual(error.exception.response.status_code, 404)

        # Inspect valid container by Id
        container = self.client.inspect_container(TestContainers.topContainerId)
        self.assertIn("top", container["Name"])

        # Inspect valid container by name
        container = self.client.inspect_container("top")
        self.assertIn(TestContainers.topContainerId, container["Id"])

    def test_create_container(self):
        # Run a container with detach mode
        container = self.client.create_container(image="alpine", detach=True)
        self.assertEqual(len(container), 2)

    def test_start_container(self):
        # Start bogus container
        with self.assertRaises(errors.NotFound) as error:
            self.client.start("dummy")
        self.assertEqual(error.exception.response.status_code, 404)

        # Podman docs says it should give a 304 but returns with no response
        # # Start a already started container should return 304
        # response = self.client.start(container=TestContainers.topContainerId)
        # self.assertEqual(error.exception.response.status_code, 304)

        # Create a new container and validate the count
        self.client.create_container(image=constant.ALPINE, name="container2")
        containers = self.client.containers(quiet=True, all=True)
        self.assertEqual(len(containers), 2)

    def test_stop_container(self):
        # Stop bogus container
        with self.assertRaises(errors.NotFound) as error:
            self.client.stop("dummy")
        self.assertEqual(error.exception.response.status_code, 404)

        # Validate the container state
        container = self.client.inspect_container("top")
        self.assertEqual(container["State"]["Status"], "running")

        # Stop a running container and validate the state
        self.client.stop(TestContainers.topContainerId)
        container = self.client.inspect_container("top")
        self.assertIn(
            container["State"]["Status"],
            "stopped exited",
        )

    def test_restart_container(self):
        # Restart bogus container
        with self.assertRaises(errors.NotFound) as error:
            self.client.restart("dummy")
        self.assertEqual(error.exception.response.status_code, 404)

        # Validate the container state
        self.client.stop(TestContainers.topContainerId)
        container = self.client.inspect_container("top")
        self.assertEqual(container["State"]["Status"], "stopped")

        # restart a running container and validate the state
        self.client.restart(TestContainers.topContainerId)
        container = self.client.inspect_container("top")
        self.assertEqual(container["State"]["Status"], "running")

    def test_remove_container(self):
        # Remove bogus container
        with self.assertRaises(errors.NotFound) as error:
            self.client.remove_container("dummy")
        self.assertEqual(error.exception.response.status_code, 404)

        # Remove container by ID with force
        self.client.remove_container(TestContainers.topContainerId, force=True)
        containers = self.client.containers()
        self.assertEqual(len(containers), 0)

    def test_remove_container_without_force(self):
        # Validate current container count
        containers = self.client.containers()
        self.assertTrue(len(containers), 1)

        # Remove running container should throw error
        with self.assertRaises(errors.APIError) as error:
            self.client.remove_container(TestContainers.topContainerId)
        self.assertEqual(error.exception.response.status_code, 500)

        # Remove container by ID with force
        self.client.stop(TestContainers.topContainerId)
        self.client.remove_container(TestContainers.topContainerId)
        containers = self.client.containers()
        self.assertEqual(len(containers), 0)

    def test_pause_container(self):
        # Pause bogus container
        with self.assertRaises(errors.NotFound) as error:
            self.client.pause("dummy")
        self.assertEqual(error.exception.response.status_code, 404)

        # Validate the container state
        container = self.client.inspect_container("top")
        self.assertEqual(container["State"]["Status"], "running")

        # Pause a running container and validate the state
        self.client.pause(container["Id"])
        container = self.client.inspect_container("top")
        self.assertEqual(container["State"]["Status"], "paused")

    def test_pause_stopped_container(self):
        # Stop the container
        self.client.stop(TestContainers.topContainerId)

        # Pause exited container should trow error
        with self.assertRaises(errors.APIError) as error:
            self.client.pause(TestContainers.topContainerId)
        self.assertEqual(error.exception.response.status_code, 500)

    def test_unpause_container(self):
        # Unpause bogus container
        with self.assertRaises(errors.NotFound) as error:
            self.client.unpause("dummy")
        self.assertEqual(error.exception.response.status_code, 404)

        # Validate the container state
        self.client.pause(TestContainers.topContainerId)
        container = self.client.inspect_container("top")
        self.assertEqual(container["State"]["Status"], "paused")

        # Pause a running container and validate the state
        self.client.unpause(TestContainers.topContainerId)
        container = self.client.inspect_container("top")
        self.assertEqual(container["State"]["Status"], "running")

    def test_list_container(self):
        # Add container and validate the count
        self.client.create_container(image="alpine", detach=True)
        containers = self.client.containers(all=True)
        self.assertEqual(len(containers), 2)

    def test_filters(self):
        self.skipTest("TODO Endpoint does not yet support filters")

        # List container with filter by id
        filters = {"id": TestContainers.topContainerId}
        ctnrs = self.client.containers(all=True, filters=filters)
        self.assertEqual(len(ctnrs), 1)

        # List container with filter by name
        filters = {"name": "top"}
        ctnrs = self.client.containers(all=True, filters=filters)
        self.assertEqual(len(ctnrs), 1)

    def test_rename_container(self):
        # rename bogus container
        with self.assertRaises(errors.APIError) as error:
            self.client.rename(container="dummy", name="newname")
        self.assertEqual(error.exception.response.status_code, 404)
