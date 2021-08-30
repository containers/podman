import unittest

import requests
from .fixtures import APITestCase


class ManifestTestCase(APITestCase):
    def test_manifest_409(self):
        r = requests.post(self.uri("/manifests/create"), params={"name": "ThisIsAnInvalidImage"})
        self.assertEqual(r.status_code, 400, r.text)

    def test_manifest_inspect_multi_image(self):
        r = requests.get(self.uri("/manifests/quay.io/libpod/busybox:latest/json"))
        j = r.json()
        self.assertEqual(len(j.get("manifests", [])), 9)

    def test_manifest_inspect_single_image(self):
        r = requests.get(self.uri("/manifests/quay.io/libpod/busybox@sha256:c9249fdf56138f0d929e2080ae98ee9cb2946f71498fc1484288e6a935b5e5bc/json"))
        j = r.json()
        self.assertEqual(j.get("config", {}).get("mediaType", ""), "application/vnd.docker.container.image.v1+json")

    def test_manifest_inspect_non_existant_image(self):
        r = requests.get(self.uri("/manifests/quay.io/libpod/foobar:1234/json"))
        j = r.json()
        self.assertEqual(j.get("response", 200), 404)

    def test_manifest_inspect_non_existant_repository(self):
        r = requests.get(self.uri("/manifests/foo.bar/libpod/busybox:latest/json"))
        j = r.json()
        self.assertEqual(j.get("response", 200), 404)

if __name__ == "__main__":
    unittest.main()
