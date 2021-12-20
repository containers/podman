import collections
import io
import os
import subprocess
import sys
import time
import unittest

from docker import DockerClient, errors
from docker.errors import APIError

from test.python.docker import Podman
from test.python.docker.compat import common, constant


class TestImages(unittest.TestCase):
    podman = None  # initialized podman configuration for tests
    service = None  # podman service instance

    def setUp(self):
        super().setUp()
        self.client = DockerClient(base_url="tcp://127.0.0.1:8080", timeout=15)

        TestImages.podman.restore_image_from_cache(self.client)

    def tearDown(self):
        common.remove_all_images(self.client)
        self.client.close()
        return super().tearDown()

    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        TestImages.podman = Podman()
        TestImages.service = TestImages.podman.open(
            "system", "service", "tcp:127.0.0.1:8080", "--time=0"
        )
        # give the service some time to be ready...
        time.sleep(2)

        returncode = TestImages.service.poll()
        if returncode is not None:
            raise subprocess.CalledProcessError(returncode, "podman system service")

    @classmethod
    def tearDownClass(cls):
        TestImages.service.terminate()
        stdout, stderr = TestImages.service.communicate(timeout=0.5)
        if stdout:
            sys.stdout.write("\nImages Service Stdout:\n" + stdout.decode("utf-8"))
        if stderr:
            sys.stderr.write("\nImAges Service Stderr:\n" + stderr.decode("utf-8"))

        TestImages.podman.tear_down()
        return super().tearDownClass()

    def test_tag_valid_image(self):
        """Validates if the image is tagged successfully"""
        alpine = self.client.images.get(constant.ALPINE)
        self.assertTrue(alpine.tag("demo", constant.ALPINE_SHORTNAME))

        alpine = self.client.images.get(constant.ALPINE)
        for t in alpine.tags:
            self.assertIn("alpine", t)

    # @unittest.skip("doesn't work now")
    def test_retag_valid_image(self):
        """Validates if name updates when the image is retagged"""
        alpine = self.client.images.get(constant.ALPINE)
        self.assertTrue(alpine.tag("demo", "rename"))

        alpine = self.client.images.get(constant.ALPINE)
        self.assertNotIn("demo:test", alpine.tags)

    def test_list_images(self):
        """List images"""
        self.assertEqual(len(self.client.images.list()), 1)

        # Add more images
        self.client.images.pull(constant.BB)
        self.assertEqual(len(self.client.images.list()), 2)
        self.assertEqual(len(self.client.images.list(all=True)), 2)

        # List images with filter
        self.assertEqual(len(self.client.images.list(filters={"reference": "alpine"})), 1)

    def test_search_image(self):
        """Search for image"""
        for r in self.client.images.search("alpine"):
            self.assertIn("alpine", r["Name"])

    def test_search_bogus_image(self):
        """Search for bogus image should throw exception"""
        try:
            r = self.client.images.search("bogus/bogus")
        except:
            return
        self.assertTrue(len(r) == 0)

    def test_remove_image(self):
        """Remove image"""
        # Check for error with wrong image name
        with self.assertRaises(errors.NotFound):
            self.client.images.remove("dummy")
        self.assertEqual(len(self.client.images.list()), 1)

        self.client.images.remove(constant.ALPINE)
        self.assertEqual(len(self.client.images.list()), 0)

    def test_image_history(self):
        """Image history"""
        img = self.client.images.get(constant.ALPINE)
        history = img.history()
        image_id = img.id[7:] if img.id.startswith("sha256:") else img.id

        found = False
        for change in history:
            found |= image_id in change.values()
        self.assertTrue(found, f"image id {image_id} not found in history")

    def test_get_image_exists_not(self):
        """Negative test for get image"""
        with self.assertRaises(errors.NotFound):
            response = self.client.images.get("image_does_not_exists")
            collections.deque(response)

    def test_save_image(self):
        """Export Image"""
        image = self.client.images.pull(constant.BB)

        file = os.path.join(TestImages.podman.image_cache, "busybox.tar")
        with open(file, mode="wb") as tarball:
            for frame in image.save(named=True):
                tarball.write(frame)
        sz = os.path.getsize(file)
        self.assertGreater(sz, 0)

    def test_load_image(self):
        """Import|Load Image"""
        self.assertEqual(len(self.client.images.list()), 1)

        image = self.client.images.pull(constant.BB)
        file = os.path.join(TestImages.podman.image_cache, "busybox.tar")
        with open(file, mode="wb") as tarball:
            for frame in image.save():
                tarball.write(frame)

        with open(file, mode="rb") as saved:
            _ = self.client.images.load(saved)

        self.assertEqual(len(self.client.images.list()), 2)

    def test_load_corrupt_image(self):
        """Import|Load Image failure"""
        tarball = io.BytesIO("This is a corrupt tarball".encode("utf-8"))
        with self.assertRaises(APIError):
            self.client.images.load(tarball)

    def test_build_image(self):
        labels = {"apple": "red", "grape": "green"}
        _ = self.client.images.build(
            path="test/python/docker/build_labels", labels=labels, tag="labels", isolation="default"
        )
        image = self.client.images.get("labels")
        self.assertEqual(image.labels["apple"], labels["apple"])
        self.assertEqual(image.labels["grape"], labels["grape"])


if __name__ == "__main__":
    # Setup temporary space
    unittest.main()
