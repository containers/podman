import json
import unittest
from multiprocessing import Process

import requests
from dateutil.parser import parse
from .fixtures import APITestCase


class ImageTestCase(APITestCase):
    def test_list(self):
        r = requests.get(self.podman_url + "/v1.40/images/json")
        self.assertEqual(r.status_code, 200, r.text)

        # See https://docs.docker.com/engine/api/v1.40/#operation/ImageList
        required_keys = (
            "Id",
            "ParentId",
            "RepoTags",
            "RepoDigests",
            "Created",
            "Size",
            "SharedSize",
            "VirtualSize",
            "Labels",
            "Containers",
        )
        images = r.json()
        self.assertIsInstance(images, list)
        for item in images:
            self.assertIsInstance(item, dict)
            for k in required_keys:
                self.assertIn(k, item)

    def test_inspect(self):
        r = requests.get(self.podman_url + "/v1.40/images/alpine/json")
        self.assertEqual(r.status_code, 200, r.text)

        # See https://docs.docker.com/engine/api/v1.40/#operation/ImageInspect
        required_keys = (
            "Id",
            "Parent",
            "Comment",
            "Created",
            "Container",
            "DockerVersion",
            "Author",
            "Architecture",
            "Os",
            "Size",
            "VirtualSize",
            "GraphDriver",
            "RootFS",
            "Metadata",
        )

        image = r.json()
        self.assertIsInstance(image, dict)
        for item in required_keys:
            self.assertIn(item, image)
        _ = parse(image["Created"])

    def test_delete(self):
        r = requests.delete(self.podman_url + "/v1.40/images/alpine?force=true")
        self.assertEqual(r.status_code, 200, r.text)
        self.assertIsInstance(r.json(), list)

    def test_pull(self):
        r = requests.post(self.uri("/images/pull?reference=alpine"), timeout=15)
        self.assertEqual(r.status_code, 200, r.status_code)
        text = r.text
        keys = {
            "error": False,
            "id": False,
            "images": False,
            "stream": False,
        }
        # Read and record stanza's from pull
        for line in str.splitlines(text):
            obj = json.loads(line)
            key_list = list(obj.keys())
            for k in key_list:
                keys[k] = True

        self.assertFalse(keys["error"], "Expected no errors")
        self.assertTrue(keys["id"], "Expected to find id stanza")
        self.assertTrue(keys["images"], "Expected to find images stanza")
        self.assertTrue(keys["stream"], "Expected to find stream progress stanza's")

    def test_create(self):
        r = requests.post(
            self.podman_url + "/v1.40/images/create?fromImage=alpine&platform=linux/amd64/v8",
            timeout=15,
        )
        self.assertEqual(r.status_code, 200, r.text)
        r = requests.post(
            self.podman_url + "/v1.40/images/create?fromSrc=-&repo=fedora&message=testing123",
            timeout=15,
        )
        self.assertEqual(r.status_code, 200, r.text)

    def test_search_compat(self):
        url = self.podman_url + "/v1.40/images/search"

        # Had issues with this test hanging when repositories not happy
        def do_search1():
            payload = {"term": "alpine"}
            r = requests.get(url, params=payload, timeout=5)
            self.assertEqual(r.status_code, 200, f"#1: {r.text}")
            self.assertIsInstance(r.json(), list)

        def do_search2():
            payload = {"term": "alpine", "limit": 1}
            r = requests.get(url, params=payload, timeout=5)
            self.assertEqual(r.status_code, 200, f"#2: {r.text}")

            results = r.json()
            self.assertIsInstance(results, list)
            self.assertEqual(len(results), 1)

        def do_search3():
            # FIXME: Research if quay.io supports is-official and which image is "official"
            return
            payload = {"term": "thanos", "filters": '{"is-official":["true"]}'}
            r = requests.get(url, params=payload, timeout=5)
            self.assertEqual(r.status_code, 200, f"#3: {r.text}")

            results = r.json()
            self.assertIsInstance(results, list)

            # There should be only one official image
            self.assertEqual(len(results), 1)

        def do_search4():
            headers = {"X-Registry-Auth": "null"}
            payload = {"term": "alpine"}
            r = requests.get(url, params=payload, headers=headers, timeout=5)
            self.assertEqual(r.status_code, 200, f"#4: {r.text}")

        def do_search5():
            headers = {"X-Registry-Auth": "invalid value"}
            payload = {"term": "alpine"}
            r = requests.get(url, params=payload, headers=headers, timeout=5)
            self.assertEqual(r.status_code, 400, f"#5: {r.text}")

        i = 1
        for fn in [do_search1, do_search2, do_search3, do_search4, do_search5]:
            with self.subTest(i=i):
                search = Process(target=fn)
                search.start()
                search.join(timeout=10)
                self.assertFalse(search.is_alive(), f"#{i} /images/search took too long")

        # search_methods = [do_search1, do_search2, do_search3, do_search4, do_search5]
        # for search_method in search_methods:
        #     search = Process(target=search_method)
        #     search.start()
        #     search.join(timeout=10)
        #     self.assertFalse(search.is_alive(), "/images/search took too long")

    def test_history(self):
        r = requests.get(self.podman_url + "/v1.40/images/alpine/history")
        self.assertEqual(r.status_code, 200, r.text)

        # See https://docs.docker.com/engine/api/v1.40/#operation/ImageHistory
        required_keys = ("Id", "Created", "CreatedBy", "Tags", "Size", "Comment")

        changes = r.json()
        self.assertIsInstance(changes, list)
        for change in changes:
            self.assertIsInstance(change, dict)
            for k in required_keys:
                self.assertIn(k, change)

    def test_tree(self):
        r = requests.get(self.uri("/images/alpine/tree"))
        self.assertEqual(r.status_code, 200, r.text)
        tree = r.json()
        self.assertTrue(tree["Tree"].startswith("Image ID:"), r.text)


if __name__ == "__main__":
    unittest.main()
