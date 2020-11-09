import json
import subprocess
import sys
import time
import unittest
from multiprocessing import Process

import requests
from dateutil.parser import parse

from test.apiv2.rest_api import Podman

PODMAN_URL = "http://localhost:8080"


def _url(path):
    return PODMAN_URL + "/v2.0.0/libpod" + path


def ctnr(path):
    try:
        r = requests.get(_url("/containers/json?all=true"))
        ctnrs = json.loads(r.text)
    except Exception as e:
        msg = f"Bad container response: {e}"
        if r is not None:
            msg = msg + " " + r.text
        sys.stderr.write(msg + "\n")
        raise
    return path.format(ctnrs[0]["Id"])


def validateObjectFields(buffer):
    objs = json.loads(buffer)
    if not isinstance(objs, dict):
        for o in objs:
            _ = o["Id"]
    else:
        _ = objs["Id"]
    return objs


class TestApi(unittest.TestCase):
    podman = None  # initialized podman configuration for tests
    service = None  # podman service instance

    def setUp(self):
        super().setUp()

        try:
            TestApi.podman.run("run", "alpine", "/bin/ls", check=True)
        except subprocess.CalledProcessError as e:
            if e.stdout:
                sys.stdout.write("\nRun Stdout:\n" + e.stdout.decode("utf-8"))
            if e.stderr:
                sys.stderr.write("\nRun Stderr:\n" + e.stderr.decode("utf-8"))
            raise

    @classmethod
    def setUpClass(cls):
        super().setUpClass()

        TestApi.podman = Podman()
        TestApi.service = TestApi.podman.open(
            "system", "service", "tcp:localhost:8080", "--time=0"
        )
        # give the service some time to be ready...
        time.sleep(2)

        returncode = TestApi.service.poll()
        if returncode is not None:
            raise subprocess.CalledProcessError(returncode, "podman system service")

        r = requests.post(_url("/images/pull?reference=docker.io%2Falpine%3Alatest"))
        if r.status_code != 200:
            raise subprocess.CalledProcessError(
                r.status_code, f"podman images pull docker.io/alpine:latest {r.text}"
            )

    @classmethod
    def tearDownClass(cls):
        TestApi.service.terminate()
        stdout, stderr = TestApi.service.communicate(timeout=0.5)
        if stdout:
            sys.stdout.write("\nService Stdout:\n" + stdout.decode("utf-8"))
        if stderr:
            sys.stderr.write("\nService Stderr:\n" + stderr.decode("utf-8"))
        return super().tearDownClass()

    def test_info(self):
        r = requests.get(_url("/info"))
        self.assertEqual(r.status_code, 200)
        self.assertIsNotNone(r.content)
        _ = json.loads(r.text)

    def test_events(self):
        r = requests.get(_url("/events?stream=false"))
        self.assertEqual(r.status_code, 200, r.text)
        self.assertIsNotNone(r.content)
        for line in r.text.splitlines():
            obj = json.loads(line)
            # Actor.ID is uppercase for compatibility
            _ = obj["Actor"]["ID"]

    def test_containers(self):
        r = requests.get(_url("/containers/json"), timeout=5)
        self.assertEqual(r.status_code, 200, r.text)
        obj = json.loads(r.text)
        self.assertEqual(len(obj), 0)

    def test_containers_all(self):
        r = requests.get(_url("/containers/json?all=true"))
        self.assertEqual(r.status_code, 200, r.text)
        validateObjectFields(r.text)

    def test_inspect_container(self):
        r = requests.get(_url(ctnr("/containers/{}/json")))
        self.assertEqual(r.status_code, 200, r.text)
        obj = validateObjectFields(r.content)
        _ = parse(obj["Created"])

    def test_stats(self):
        r = requests.get(_url(ctnr("/containers/{}/stats?stream=false")))
        self.assertIn(r.status_code, (200, 409), r.text)
        if r.status_code == 200:
            validateObjectFields(r.text)

    def test_delete_containers(self):
        r = requests.delete(_url(ctnr("/containers/{}")))
        self.assertEqual(r.status_code, 204, r.text)

    def test_stop_containers(self):
        r = requests.post(_url(ctnr("/containers/{}/start")))
        self.assertIn(r.status_code, (204, 304), r.text)

        r = requests.post(_url(ctnr("/containers/{}/stop")))
        self.assertIn(r.status_code, (204, 304), r.text)

    def test_start_containers(self):
        r = requests.post(_url(ctnr("/containers/{}/stop")))
        self.assertIn(r.status_code, (204, 304), r.text)

        r = requests.post(_url(ctnr("/containers/{}/start")))
        self.assertIn(r.status_code, (204, 304), r.text)

    def test_restart_containers(self):
        r = requests.post(_url(ctnr("/containers/{}/start")))
        self.assertIn(r.status_code, (204, 304), r.text)

        r = requests.post(_url(ctnr("/containers/{}/restart")), timeout=5)
        self.assertEqual(r.status_code, 204, r.text)

    def test_resize(self):
        r = requests.post(_url(ctnr("/containers/{}/resize?h=43&w=80")))
        self.assertIn(r.status_code, (200, 409), r.text)
        if r.status_code == 200:
            self.assertIsNone(r.text)

    def test_attach_containers(self):
        self.skipTest("FIXME: Test timeouts")
        r = requests.post(_url(ctnr("/containers/{}/attach")), timeout=5)
        self.assertIn(r.status_code, (101, 500), r.text)

    def test_logs_containers(self):
        r = requests.get(_url(ctnr("/containers/{}/logs?stdout=true")))
        self.assertEqual(r.status_code, 200, r.text)

    def test_post_create(self):
        self.skipTest("TODO: create request body")
        r = requests.post(_url("/containers/create?args=True"))
        self.assertEqual(r.status_code, 200, r.text)
        json.loads(r.text)

    def test_commit(self):
        r = requests.post(_url(ctnr("/commit?container={}")))
        self.assertEqual(r.status_code, 200, r.text)
        validateObjectFields(r.text)

    def test_images(self):
        r = requests.get(_url("/images/json"))
        self.assertEqual(r.status_code, 200, r.text)
        validateObjectFields(r.content)

    def test_inspect_image(self):
        r = requests.get(_url("/images/alpine/json"))
        self.assertEqual(r.status_code, 200, r.text)
        obj = validateObjectFields(r.content)
        _ = parse(obj["Created"])

    def test_delete_image(self):
        r = requests.delete(_url("/images/alpine?force=true"))
        self.assertEqual(r.status_code, 200, r.text)
        json.loads(r.text)

    def test_pull(self):
        r = requests.post(_url("/images/pull?reference=alpine"), timeout=15)
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

    def test_search(self):
        # Had issues with this test hanging when repositories not happy
        def do_search():
            r = requests.get(_url("/images/search?term=alpine"), timeout=5)
            self.assertEqual(r.status_code, 200, r.text)
            json.loads(r.text)

        search = Process(target=do_search)
        search.start()
        search.join(timeout=10)
        self.assertFalse(search.is_alive(), "/images/search took too long")

    def test_ping(self):
        r = requests.get(PODMAN_URL + "/_ping")
        self.assertEqual(r.status_code, 200, r.text)

        r = requests.head(PODMAN_URL + "/_ping")
        self.assertEqual(r.status_code, 200, r.text)

        r = requests.get(_url("/_ping"))
        self.assertEqual(r.status_code, 200, r.text)

        r = requests.get(_url("/_ping"))
        self.assertEqual(r.status_code, 200, r.text)


if __name__ == "__main__":
    unittest.main()
