import unittest

import requests
from .fixtures import APITestCase


class DistributionTestCase(APITestCase):
    def test_distribution_inspect(self):
        # Make sure the image exists
        r = requests.post(self.uri("/images/pull?reference=alpine:latest"), timeout=15)
        self.assertEqual(r.status_code, 200, r.text)

        r = requests.get(self.podman_url + "/v1.40/distribution/alpine/json")
        self.assertEqual(r.status_code, 200, r.text)

        result = r.json()
        self.assertIn("Descriptor", result)
        self.assertIn("Platforms", result)

        descriptor = result["Descriptor"]
        self.assertIn("mediaType", descriptor)
        self.assertIn("digest", descriptor)
        self.assertIn("size", descriptor)

        for platform in result["Platforms"]:
            self.assertIn("architecture", platform)
            self.assertIn("os", platform)

    def test_distribution_inspect_invalid_image(self):
        r = requests.get(self.podman_url + "/v1.40/distribution/nonexistentimage/json")
        self.assertEqual(r.status_code, 401, r.text)

if __name__ == "__main__":
    unittest.main()
