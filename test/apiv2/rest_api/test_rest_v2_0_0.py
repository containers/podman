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
        TestApi.service = TestApi.podman.open("system", "service", "tcp:localhost:8080", "--time=0")
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

    # TODO Need to support Docker-py order of network/container creates
    def test_post_create_compat_connect(self):
        """Create network and container then connect to network"""
        net_default = requests.post(
            PODMAN_URL + "/v1.40/networks/create", json={"Name": "TestDefaultNetwork"}
        )
        self.assertEqual(net_default.status_code, 201, net_default.text)

        create = requests.post(
            PODMAN_URL + "/v1.40/containers/create?name=postCreate",
            json={
                "Cmd": ["top"],
                "Image": "alpine:latest",
                "NetworkDisabled": False,
                # FIXME adding these 2 lines cause: (This is sampled from docker-py)
                #   "network already exists","message":"container
                #  01306e499df5441560d70071a54342611e422a94de20865add50a9565fd79fb9 is already connected to CNI
                #  network \"TestDefaultNetwork\": network already exists"
                # "HostConfig": {"NetworkMode": "TestDefaultNetwork"},
                # "NetworkingConfig": {"EndpointsConfig": {"TestDefaultNetwork": None}},
                # FIXME These two lines cause:
                # CNI network \"TestNetwork\" not found","message":"error configuring network namespace for container
                # 369ddfa7d3211ebf1fbd5ddbff91bd33fa948858cea2985c133d6b6507546dff: CNI network \"TestNetwork\" not
                # found"
                # "HostConfig": {"NetworkMode": "TestNetwork"},
                # "NetworkingConfig": {"EndpointsConfig": {"TestNetwork": None}},
                # FIXME no networking defined cause: (note this error is from the container inspect below)
                # "internal libpod error","message":"network inspection mismatch: asked to join 2 CNI network(s) [
                # TestDefaultNetwork podman], but have information on 1 network(s): internal libpod error"
            },
        )
        self.assertEqual(create.status_code, 201, create.text)
        payload = json.loads(create.text)
        self.assertIsNotNone(payload["Id"])

        start = requests.post(PODMAN_URL + f"/v1.40/containers/{payload['Id']}/start")
        self.assertEqual(start.status_code, 204, start.text)

        connect = requests.post(
            PODMAN_URL + "/v1.40/networks/TestDefaultNetwork/connect",
            json={"Container": payload["Id"]},
        )
        self.assertEqual(connect.status_code, 200, connect.text)
        self.assertEqual(connect.text, "OK\n")

        inspect = requests.get(f"{PODMAN_URL}/v1.40/containers/{payload['Id']}/json")
        self.assertEqual(inspect.status_code, 200, inspect.text)

        payload = json.loads(inspect.text)
        self.assertFalse(payload["Config"].get("NetworkDisabled", False))

        self.assertEqual(
            "TestDefaultNetwork",
            payload["NetworkSettings"]["Networks"]["TestDefaultNetwork"]["NetworkID"],
        )
        # TODO restore this to test, when joining multiple networks possible
        # self.assertEqual(
        #     "TestNetwork",
        #     payload["NetworkSettings"]["Networks"]["TestNetwork"]["NetworkID"],
        # )
        # TODO Need to support network aliases
        # self.assertIn(
        #     "test_post_create",
        #     payload["NetworkSettings"]["Networks"]["TestNetwork"]["Aliases"],
        # )

    def test_post_create_compat(self):
        """Create network and connect container during create"""
        net = requests.post(PODMAN_URL + "/v1.40/networks/create", json={"Name": "TestNetwork"})
        self.assertEqual(net.status_code, 201, net.text)

        create = requests.post(
            PODMAN_URL + "/v1.40/containers/create?name=postCreate",
            json={
                "Cmd": ["date"],
                "Image": "alpine:latest",
                "NetworkDisabled": False,
                "HostConfig": {"NetworkMode": "TestNetwork"},
            },
        )
        self.assertEqual(create.status_code, 201, create.text)
        payload = json.loads(create.text)
        self.assertIsNotNone(payload["Id"])

        inspect = requests.get(f"{PODMAN_URL}/v1.40/containers/{payload['Id']}/json")
        self.assertEqual(inspect.status_code, 200, inspect.text)
        payload = json.loads(inspect.text)
        self.assertFalse(payload["Config"].get("NetworkDisabled", False))
        self.assertEqual(
            "TestNetwork",
            payload["NetworkSettings"]["Networks"]["TestNetwork"]["NetworkID"],
        )

    def test_commit(self):
        r = requests.post(_url(ctnr("/commit?container={}")))
        self.assertEqual(r.status_code, 200, r.text)

        obj = json.loads(r.content)
        self.assertIsInstance(obj, dict)
        self.assertIn("Id", obj)

    def test_images_compat(self):
        r = requests.get(PODMAN_URL + "/v1.40/images/json")
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
        objs = json.loads(r.content)
        self.assertIn(type(objs), (list,))
        for o in objs:
            self.assertIsInstance(o, dict)
            for k in required_keys:
                self.assertIn(k, o)

    def test_inspect_image_compat(self):
        r = requests.get(PODMAN_URL + "/v1.40/images/alpine/json")
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

        obj = json.loads(r.content)
        self.assertIn(type(obj), (dict,))
        for k in required_keys:
            self.assertIn(k, obj)
        _ = parse(obj["Created"])

    def test_delete_image_compat(self):
        r = requests.delete(PODMAN_URL + "/v1.40/images/alpine?force=true")
        self.assertEqual(r.status_code, 200, r.text)
        obj = json.loads(r.content)
        self.assertIn(type(obj), (list,))

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

    def test_search_compat(self):
        # Had issues with this test hanging when repositories not happy
        def do_search():
            r = requests.get(PODMAN_URL + "/v1.40/images/search?term=alpine", timeout=5)
            self.assertEqual(r.status_code, 200, r.text)
            objs = json.loads(r.text)
            self.assertIn(type(objs), (list,))

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

    def test_history_compat(self):
        r = requests.get(PODMAN_URL + "/v1.40/images/alpine/history")
        self.assertEqual(r.status_code, 200, r.text)

        # See https://docs.docker.com/engine/api/v1.40/#operation/ImageHistory
        required_keys = ("Id", "Created", "CreatedBy", "Tags", "Size", "Comment")

        objs = json.loads(r.content)
        self.assertIn(type(objs), (list,))
        for o in objs:
            self.assertIsInstance(o, dict)
            for k in required_keys:
                self.assertIn(k, o)


if __name__ == "__main__":
    unittest.main()
