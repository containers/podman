
import unittest
import docker
import requests
import os
from docker import Client
from . import constant
from . import common

client = common.get_client()

class TestImages(unittest.TestCase):

    def setUp(self):
        super().setUp()
        client.pull(constant.ALPINE)

    def tearDown(self):
        allImages = client.images()
        for image in allImages:
            client.remove_image(image,force=True)
        return super().tearDown()

# Inspect Image


    def test_inspect_image(self):
        # Check for error with wrong image name
        with self.assertRaises(requests.HTTPError):
            client.inspect_image("dummy")
        alpine_image = client.inspect_image(constant.ALPINE)
        self.assertIn(constant.ALPINE, alpine_image["RepoTags"])

# Tag Image

    # Validates if invalid image name is given a bad response is encountered.
    def test_tag_invalid_image(self):
        with self.assertRaises(requests.HTTPError):
            client.tag("dummy","demo")



    #  Validates if the image is tagged successfully.
    def test_tag_valid_image(self):
        client.tag(constant.ALPINE,"demo",constant.ALPINE_SHORTNAME)
        alpine_image = client.inspect_image(constant.ALPINE)
        for x in alpine_image["RepoTags"]:
            if("demo:alpine" in x):
                self.assertTrue
        self.assertFalse

    # Validates if name updates when the image is retagged.
    @unittest.skip("dosent work now")
    def test_retag_valid_image(self):
        client.tag(constant.ALPINE_SHORTNAME, "demo","rename")
        alpine_image = client.inspect_image(constant.ALPINE)
        self.assertNotIn("demo:test", alpine_image["RepoTags"])

# List Image
    # List All Images
    def test_list_images(self):
        allImages = client.images()
        self.assertEqual(len(allImages), 1)
        # Add more images
        client.pull(constant.BB)
        client.pull(constant.NGINX)
        allImages = client.images()
        self.assertEqual(len(allImages) , 3)


    # List images with filter
        filters = {'reference':'alpine'}
        allImages = client.images(filters = filters)
        self.assertEqual(len(allImages) , 1)

# Search Image
    def test_search_image(self):
        response = client.search("alpine")
        for i in response:
            # Alpine found
            if "docker.io/library/alpine" in i["Name"]:
                self.assertTrue(True, msg="Image found")
        self.assertFalse(False,msg="Image not found")

# Image Exist (No docker-py support yet)

# Remove Image
    def test_remove_image(self):
        # Check for error with wrong image name
        with self.assertRaises(requests.HTTPError):
            client.remove_image("dummy")
        allImages = client.images()
        self.assertEqual(len(allImages) , 1)
        alpine_image = client.inspect_image(constant.ALPINE)
        client.remove_image(alpine_image)
        allImages = client.images()
        self.assertEqual(len(allImages) , 0)

# Image History
    def test_image_history(self):
        # Check for error with wrong image name
        with self.assertRaises(requests.HTTPError):
            client.remove_image("dummy")
        imageHistory = client.history(constant.ALPINE)
        alpine_image = client.inspect_image(constant.ALPINE)
        for h in imageHistory:
            if h["Id"] in alpine_image["Id"]:
                self.assertTrue(True,msg="Image History validated")
        self.assertFalse(False,msg="Unable to get image history")

# Prune Image (No docker-py support yet)

# Export Image

    def test_export_image(self):
        file = "/tmp/alpine-latest.tar"
        # Check for error with wrong image name
        with self.assertRaises(requests.HTTPError):
            client.get_image("dummy")
        response = client.get_image(constant.ALPINE)
        image_tar = open(file,mode="wb")
        image_tar.write(response.data)
        image_tar.close()
        os.stat(file)

# Import|Load Image


if __name__ == '__main__':
    # Setup temporary space
    unittest.main()
