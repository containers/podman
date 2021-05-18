import json
import unittest

import requests
from .fixtures import APITestCase


class SystemTestCase(APITestCase):
    def test_info(self):
        r = requests.get(self.uri("/info"))
        self.assertEqual(r.status_code, 200, r.text)
        self.assertIsNotNone(r.content)
        _ = r.json()

        r = requests.get(self.podman_url + "/v1.40/info")
        self.assertEqual(r.status_code, 200, r.text)
        self.assertIsNotNone(r.content)
        _ = r.json()

    def test_events(self):
        r = requests.get(self.uri("/events?stream=false"))
        self.assertEqual(r.status_code, 200, r.text)
        self.assertIsNotNone(r.content)

        report = r.text.splitlines()
        self.assertGreater(len(report), 0, "No events found!")
        for line in report:
            obj = json.loads(line)
            # Actor.ID is uppercase for compatibility
            self.assertIn("ID", obj["Actor"])

    def test_ping(self):
        required_headers = (
            "API-Version",
            "Builder-Version",
            "Docker-Experimental",
            "Cache-Control",
            "Pragma",
            "Pragma",
        )

        def check_headers(req):
            for k in required_headers:
                self.assertIn(k, req.headers)

        r = requests.get(self.podman_url + "/_ping")
        self.assertEqual(r.status_code, 200, r.text)
        self.assertEqual(r.text, "OK")
        check_headers(r)

        r = requests.head(self.podman_url + "/_ping")
        self.assertEqual(r.status_code, 200, r.text)
        self.assertEqual(r.text, "")
        check_headers(r)

        r = requests.get(self.uri("/_ping"))
        self.assertEqual(r.status_code, 200, r.text)
        self.assertEqual(r.text, "OK")
        check_headers(r)

        r = requests.head(self.uri("/_ping"))
        self.assertEqual(r.status_code, 200, r.text)
        self.assertEqual(r.text, "")
        check_headers(r)

    def test_version(self):
        r = requests.get(self.podman_url + "/v1.40/version")
        self.assertEqual(r.status_code, 200, r.text)

        r = requests.get(self.uri("/version"))
        self.assertEqual(r.status_code, 200, r.text)

    def test_df(self):
        r = requests.get(self.podman_url + "/v1.40/system/df")
        self.assertEqual(r.status_code, 200, r.text)

        obj = r.json()
        self.assertIn("Images", obj)
        self.assertIn("Containers", obj)
        self.assertIn("Volumes", obj)
        self.assertIn("BuildCache", obj)

        r = requests.get(self.uri("/system/df"))
        self.assertEqual(r.status_code, 200, r.text)


if __name__ == "__main__":
    unittest.main()
