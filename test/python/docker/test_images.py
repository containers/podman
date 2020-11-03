import collections
import os
import subprocess
import sys
import time
import unittest

from docker import APIClient, errors

from test.python.docker import Podman, common, constant


class TestImages(unittest.TestCase):
    podman = None  # initialized podman configuration for tests
    service = None  # podman service instance

    def setUp(self):
        super().setUp()
        self.client = APIClient(base_url="tcp://127.0.0.1:8080", timeout=15)

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

    def test_inspect_image(self):
        """Inspect Image"""
        # Check for error with wrong image name
        with self.assertRaises(errors.NotFound):
            self.client.inspect_image("dummy")
        alpine_image = self.client.inspect_image(constant.ALPINE)
        self.assertIn(constant.ALPINE, alpine_image["RepoTags"])

    def test_tag_invalid_image(self):
        """Tag Image

        Validates if invalid image name is given a bad response is encountered
        """
        with self.assertRaises(errors.NotFound):
            self.client.tag("dummy", "demo")

    def test_tag_valid_image(self):
        """Validates if the image is tagged successfully"""
        self.client.tag(constant.ALPINE, "demo", constant.ALPINE_SHORTNAME)
        alpine_image = self.client.inspect_image(constant.ALPINE)
        for x in alpine_image["RepoTags"]:
            self.assertIn("alpine", x)

    # @unittest.skip("doesn't work now")
    def test_retag_valid_image(self):
        """Validates if name updates when the image is retagged"""
        self.client.tag(constant.ALPINE_SHORTNAME, "demo", "rename")
        alpine_image = self.client.inspect_image(constant.ALPINE)
        self.assertNotIn("demo:test", alpine_image["RepoTags"])

    def test_list_images(self):
        """List images"""
        all_images = self.client.images()
        self.assertEqual(len(all_images), 1)
        # Add more images
        self.client.pull(constant.BB)
        all_images = self.client.images()
        self.assertEqual(len(all_images), 2)

        # List images with filter
        filters = {"reference": "alpine"}
        all_images = self.client.images(filters=filters)
        self.assertEqual(len(all_images), 1)

    def test_search_image(self):
        """Search for image"""
        response = self.client.search("libpod/alpine")
        for i in response:
            self.assertIn("quay.io/libpod/alpine", i["Name"])

    def test_remove_image(self):
        """Remove image"""
        # Check for error with wrong image name
        with self.assertRaises(errors.NotFound):
            self.client.remove_image("dummy")
        all_images = self.client.images()
        self.assertEqual(len(all_images), 1)

        alpine_image = self.client.inspect_image(constant.ALPINE)
        self.client.remove_image(alpine_image["Id"])
        all_images = self.client.images()
        self.assertEqual(len(all_images), 0)

    def test_image_history(self):
        """Image history"""
        # Check for error with wrong image name
        with self.assertRaises(errors.NotFound):
            self.client.history("dummy")

        # NOTE: history() has incorrect return type hint
        history = self.client.history(constant.ALPINE)
        alpine_image = self.client.inspect_image(constant.ALPINE)
        image_id = (
            alpine_image["Id"][7:]
            if alpine_image["Id"].startswith("sha256:")
            else alpine_image["Id"]
        )

        found = False
        for change in history:
            found |= image_id in change.values()
        self.assertTrue(found, f"image id {image_id} not found in history")

    def test_get_image_exists_not(self):
        """Negative test for get image"""
        with self.assertRaises(errors.NotFound):
            response = self.client.get_image("image_does_not_exists")
            collections.deque(response)

    def test_export_image(self):
        """Export Image"""
        self.client.pull(constant.BB)
        image = self.client.get_image(constant.BB)

        file = os.path.join(TestImages.podman.image_cache, "busybox.tar")
        with open(file, mode="wb") as tarball:
            for frame in image:
                tarball.write(frame)
        sz = os.path.getsize(file)
        self.assertGreater(sz, 0)

    def test_import_image(self):
        """Import|Load Image"""
        all_images = self.client.images()
        self.assertEqual(len(all_images), 1)

        file = os.path.join(TestImages.podman.image_cache, constant.ALPINE_TARBALL)
        self.client.import_image_from_file(filename=file)

        all_images = self.client.images()
        self.assertEqual(len(all_images), 2)


if __name__ == "__main__":
    # Setup temporary space
    unittest.main()
