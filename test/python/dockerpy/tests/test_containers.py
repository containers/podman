import os
import time
import unittest

import requests

from . import common, constant

client = common.get_client()


class TestContainers(unittest.TestCase):
    topContainerId = ""

    def setUp(self):
        super().setUp()
        common.restore_image_from_cache(self)
        TestContainers.topContainerId = common.run_top_container()

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
        common.flush_image_cache(cls)
        return super().tearDownClass()

    def test_inspect_container(self):
        # Inspect bogus container
        with self.assertRaises(requests.HTTPError) as error:
            client.inspect_container("dummy")
        self.assertEqual(error.exception.response.status_code, 404)
        # Inspect valid container by name
        container = client.inspect_container(constant.TOP)
        self.assertIn(TestContainers.topContainerId, container["Id"])
        # Inspect valid container by Id
        container = client.inspect_container(TestContainers.topContainerId)
        self.assertIn(constant.TOP, container["Name"])

    def test_create_container(self):
        # Run a container with detach mode
        container = client.create_container(image="alpine", detach=True)
        self.assertEqual(len(container), 2)

    def test_start_container(self):
        # Start bogus container
        with self.assertRaises(requests.HTTPError) as error:
            client.start("dummy")
        self.assertEqual(error.exception.response.status_code, 404)

        # Podman docs says it should give a 304 but returns with no response
        # # Start a already started container should return 304
        # response = client.start(container=TestContainers.topContainerId)
        # self.assertEqual(error.exception.response.status_code, 304)

        # Create a new container and validate the count
        client.create_container(image=constant.ALPINE, name="container2")
        containers = client.containers(quiet=True, all=True)
        self.assertEqual(len(containers), 2)

    def test_stop_container(self):
        # Stop bogus container
        with self.assertRaises(requests.HTTPError) as error:
            client.stop("dummy")
        self.assertEqual(error.exception.response.status_code, 404)

        # Validate the container state
        container = client.inspect_container(constant.TOP)
        self.assertEqual(container["State"]["Status"], "running")

        # Stop a running container and validate the state
        client.stop(TestContainers.topContainerId)
        container = client.inspect_container(constant.TOP)
        self.assertIn(
            container["State"]["Status"],
            "stopped exited",
        )

    def test_restart_container(self):
        # Restart bogus container
        with self.assertRaises(requests.HTTPError) as error:
            client.restart("dummy")
        self.assertEqual(error.exception.response.status_code, 404)

        # Validate the container state
        client.stop(TestContainers.topContainerId)
        container = client.inspect_container(constant.TOP)
        self.assertEqual(container["State"]["Status"], "stopped")

        # restart a running container and validate the state
        client.restart(TestContainers.topContainerId)
        container = client.inspect_container(constant.TOP)
        self.assertEqual(container["State"]["Status"], "running")

    def test_remove_container(self):
        # Remove bogus container
        with self.assertRaises(requests.HTTPError) as error:
            client.remove_container("dummy")
        self.assertEqual(error.exception.response.status_code, 404)

        # Remove container by ID with force
        client.remove_container(TestContainers.topContainerId, force=True)
        containers = client.containers()
        self.assertEqual(len(containers), 0)

    def test_remove_container_without_force(self):
        # Validate current container count
        containers = client.containers()
        self.assertTrue(len(containers), 1)

        # Remove running container should throw error
        with self.assertRaises(requests.HTTPError) as error:
            client.remove_container(TestContainers.topContainerId)
        self.assertEqual(error.exception.response.status_code, 500)

        # Remove container by ID with force
        client.stop(TestContainers.topContainerId)
        client.remove_container(TestContainers.topContainerId)
        containers = client.containers()
        self.assertEqual(len(containers), 0)

    def test_pause_container(self):
        # Pause bogus container
        with self.assertRaises(requests.HTTPError) as error:
            client.pause("dummy")
        self.assertEqual(error.exception.response.status_code, 404)

        # Validate the container state
        container = client.inspect_container(constant.TOP)
        self.assertEqual(container["State"]["Status"], "running")

        # Pause a running container and validate the state
        client.pause(container)
        container = client.inspect_container(constant.TOP)
        self.assertEqual(container["State"]["Status"], "paused")

    def test_pause_stoped_container(self):
        # Stop the container
        client.stop(TestContainers.topContainerId)

        # Pause exited container should trow error
        with self.assertRaises(requests.HTTPError) as error:
            client.pause(TestContainers.topContainerId)
        self.assertEqual(error.exception.response.status_code, 500)

    def test_unpause_container(self):
        # Unpause bogus container
        with self.assertRaises(requests.HTTPError) as error:
            client.unpause("dummy")
        self.assertEqual(error.exception.response.status_code, 404)

        # Validate the container state
        client.pause(TestContainers.topContainerId)
        container = client.inspect_container(constant.TOP)
        self.assertEqual(container["State"]["Status"], "paused")

        # Pause a running container and validate the state
        client.unpause(TestContainers.topContainerId)
        container = client.inspect_container(constant.TOP)
        self.assertEqual(container["State"]["Status"], "running")

    def test_list_container(self):

        # Add container and validate the count
        client.create_container(image="alpine", detach=True)
        containers = client.containers(all=True)
        self.assertEqual(len(containers), 2)

        # Not working for now......checking
        # # List container with filter by id
        # filters = {'id':TestContainers.topContainerId}
        # filteredContainers = client.containers(all=True,filters = filters)
        # self.assertEqual(len(filteredContainers) , 1)

        # # List container with filter by name
        # filters = {'name':constant.TOP}
        # filteredContainers = client.containers(all=True,filters = filters)
        # self.assertEqual(len(filteredContainers) , 1)

    @unittest.skip("Not Supported yet")
    def test_rename_container(self):
        # rename bogus container
        with self.assertRaises(requests.HTTPError) as error:
            client.rename(container="dummy", name="newname")
        self.assertEqual(error.exception.response.status_code, 404)
