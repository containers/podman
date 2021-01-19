import json
import os
import random
import string
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

        info = requests.get(PODMAN_URL + "/v1.40/info")
        self.assertEqual(info.status_code, 200, info.content)
        _ = json.loads(info.text)

    def test_events(self):
        r = requests.get(_url("/events?stream=false"))
        self.assertEqual(r.status_code, 200, r.text)
        self.assertIsNotNone(r.content)

        report = r.text.splitlines()
        self.assertGreater(len(report), 0, "No events found!")
        for line in report:
            obj = json.loads(line)
            # Actor.ID is uppercase for compatibility
            self.assertIn("ID", obj["Actor"])

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
            self.assertEqual(r.text, "", r.text)

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
            PODMAN_URL + "/v1.40/containers/create?name=postCreateConnect",
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

        r = requests.get(PODMAN_URL + "/_ping")
        self.assertEqual(r.status_code, 200, r.text)
        self.assertEqual(r.text, "OK")
        check_headers(r)

        r = requests.head(PODMAN_URL + "/_ping")
        self.assertEqual(r.status_code, 200, r.text)
        self.assertEqual(r.text, "")
        check_headers(r)

        r = requests.get(_url("/_ping"))
        self.assertEqual(r.status_code, 200, r.text)
        self.assertEqual(r.text, "OK")
        check_headers(r)

        r = requests.head(_url("/_ping"))
        self.assertEqual(r.status_code, 200, r.text)
        self.assertEqual(r.text, "")
        check_headers(r)

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

    def test_network_compat(self):
        name = "Network_" + "".join(random.choice(string.ascii_letters) for i in range(10))

        # Cannot test for 0 existing networks because default "podman" network always exists

        create = requests.post(PODMAN_URL + "/v1.40/networks/create", json={"Name": name})
        self.assertEqual(create.status_code, 201, create.content)
        obj = json.loads(create.content)
        self.assertIn(type(obj), (dict,))
        self.assertIn("Id", obj)
        ident = obj["Id"]
        self.assertNotEqual(name, ident)

        ls = requests.get(PODMAN_URL + "/v1.40/networks")
        self.assertEqual(ls.status_code, 200, ls.content)
        objs = json.loads(ls.content)
        self.assertIn(type(objs), (list,))

        found = False
        for network in objs:
            if network["Name"] == name:
                found = True
        self.assertTrue(found, f"Network {name} not found")

        inspect = requests.get(PODMAN_URL + f"/v1.40/networks/{ident}")
        self.assertEqual(inspect.status_code, 200, inspect.content)
        obj = json.loads(create.content)
        self.assertIn(type(obj), (dict,))

        inspect = requests.delete(PODMAN_URL + f"/v1.40/networks/{ident}")
        self.assertEqual(inspect.status_code, 204, inspect.content)
        inspect = requests.get(PODMAN_URL + f"/v1.40/networks/{ident}")
        self.assertEqual(inspect.status_code, 404, inspect.content)

        prune = requests.post(PODMAN_URL + "/v1.40/networks/prune")
        self.assertEqual(prune.status_code, 404, prune.content)

    def test_volumes_compat(self):
        name = "Volume_" + "".join(random.choice(string.ascii_letters) for i in range(10))

        ls = requests.get(PODMAN_URL + "/v1.40/volumes")
        self.assertEqual(ls.status_code, 200, ls.content)

        # See https://docs.docker.com/engine/api/v1.40/#operation/VolumeList
        required_keys = (
            "Volumes",
            "Warnings",
        )

        obj = json.loads(ls.content)
        self.assertIn(type(obj), (dict,))
        for k in required_keys:
            self.assertIn(k, obj)

        create = requests.post(PODMAN_URL + "/v1.40/volumes/create", json={"Name": name})
        self.assertEqual(create.status_code, 201, create.content)

        # See https://docs.docker.com/engine/api/v1.40/#operation/VolumeCreate
        # and https://docs.docker.com/engine/api/v1.40/#operation/VolumeInspect
        required_keys = (
            "Name",
            "Driver",
            "Mountpoint",
            "Labels",
            "Scope",
            "Options",
        )

        obj = json.loads(create.content)
        self.assertIn(type(obj), (dict,))
        for k in required_keys:
            self.assertIn(k, obj)
        self.assertEqual(obj["Name"], name)

        inspect = requests.get(PODMAN_URL + f"/v1.40/volumes/{name}")
        self.assertEqual(inspect.status_code, 200, inspect.content)

        obj = json.loads(create.content)
        self.assertIn(type(obj), (dict,))
        for k in required_keys:
            self.assertIn(k, obj)

        rm = requests.delete(PODMAN_URL + f"/v1.40/volumes/{name}")
        self.assertEqual(rm.status_code, 204, rm.content)

        # recreate volume with data and then prune it
        r = requests.post(PODMAN_URL + "/v1.40/volumes/create", json={"Name": name})
        self.assertEqual(create.status_code, 201, create.content)
        create = json.loads(r.content)
        with open(os.path.join(create["Mountpoint"], "test_prune"), "w") as file:
            file.writelines(["This is a test\n", "This is a good test\n"])

        prune = requests.post(PODMAN_URL + "/v1.40/volumes/prune")
        self.assertEqual(prune.status_code, 200, prune.content)
        payload = json.loads(prune.content)
        self.assertIn(name, payload["VolumesDeleted"])
        self.assertGreater(payload["SpaceReclaimed"], 0)

    def test_auth_compat(self):
        r = requests.post(
            PODMAN_URL + "/v1.40/auth",
            json={
                "username": "bozo",
                "password": "wedontneednopasswords",
                "serveraddress": "https://localhost/v1.40/",
            },
        )
        self.assertEqual(r.status_code, 404, r.content)

    def test_version(self):
        r = requests.get(PODMAN_URL + "/v1.40/version")
        self.assertEqual(r.status_code, 200, r.content)

        r = requests.get(_url("/version"))
        self.assertEqual(r.status_code, 200, r.content)

    def test_df_compat(self):
        r = requests.get(PODMAN_URL + "/v1.40/system/df")
        self.assertEqual(r.status_code, 200, r.content)

        obj = json.loads(r.content)
        self.assertIn("Images", obj)
        self.assertIn("Containers", obj)
        self.assertIn("Volumes", obj)
        self.assertIn("BuildCache", obj)

    def test_prune_compat(self):
        name = "Ctnr_" + "".join(random.choice(string.ascii_letters) for i in range(10))

        r = requests.post(
            PODMAN_URL + f"/v1.40/containers/create?name={name}",
            json={
                "Cmd": ["cp", "/etc/motd", "/motd.size_test"],
                "Image": "alpine:latest",
                "NetworkDisabled": True,
            },
        )
        self.assertEqual(r.status_code, 201, r.text)
        create = json.loads(r.text)

        r = requests.post(PODMAN_URL + f"/v1.40/containers/{create['Id']}/start")
        self.assertEqual(r.status_code, 204, r.text)

        r = requests.post(PODMAN_URL + f"/v1.40/containers/{create['Id']}/wait")
        self.assertEqual(r.status_code, 200, r.text)
        wait = json.loads(r.text)
        self.assertEqual(wait["StatusCode"], 0, wait["Error"]["Message"])

        prune = requests.post(PODMAN_URL + "/v1.40/containers/prune")
        self.assertEqual(prune.status_code, 200, prune.status_code)
        prune_payload = json.loads(prune.text)
        self.assertGreater(prune_payload["SpaceReclaimed"], 0)
        self.assertIn(create["Id"], prune_payload["ContainersDeleted"])

        # Delete any orphaned containers
        r = requests.get(PODMAN_URL + "/v1.40/containers/json?all=true")
        self.assertEqual(r.status_code, 200, r.text)
        for ctnr in json.loads(r.text):
            requests.delete(PODMAN_URL + f"/v1.40/containers/{ctnr['Id']}?force=true")

        prune = requests.post(PODMAN_URL + "/v1.40/images/prune")
        self.assertEqual(prune.status_code, 200, prune.text)
        prune_payload = json.loads(prune.text)
        self.assertGreater(prune_payload["SpaceReclaimed"], 0)

        # FIXME need method to determine which image is going to be "pruned" to fix test
        # TODO should handler be recursive when deleting images?
        # self.assertIn(img["Id"], prune_payload["ImagesDeleted"][1]["Deleted"])
        self.assertIsNotNone(prune_payload["ImagesDeleted"][1]["Deleted"])

    def test_status_compat(self):
        r = requests.post(
            PODMAN_URL + "/v1.40/containers/create?name=topcontainer",
            json={"Cmd": ["top"], "Image": "alpine:latest"},
        )
        self.assertEqual(r.status_code, 201, r.text)
        payload = json.loads(r.text)
        container_id = payload["Id"]
        self.assertIsNotNone(container_id)

        r = requests.get(
            PODMAN_URL + "/v1.40/containers/json",
            params={"all": "true", "filters": f'{{"id":["{container_id}"]}}'},
        )
        self.assertEqual(r.status_code, 200, r.text)
        payload = json.loads(r.text)
        self.assertEqual(payload[0]["Status"], "Created")

        r = requests.post(PODMAN_URL + f"/v1.40/containers/{container_id}/start")
        self.assertEqual(r.status_code, 204, r.text)

        r = requests.get(
            PODMAN_URL + "/v1.40/containers/json",
            params={"all": "true", "filters": f'{{"id":["{container_id}"]}}'},
        )
        self.assertEqual(r.status_code, 200, r.text)
        payload = json.loads(r.text)
        self.assertTrue(str(payload[0]["Status"]).startswith("Up"))

        r = requests.post(PODMAN_URL + f"/v1.40/containers/{container_id}/pause")
        self.assertEqual(r.status_code, 204, r.text)

        r = requests.get(
            PODMAN_URL + "/v1.40/containers/json",
            params={"all": "true", "filters": f'{{"id":["{container_id}"]}}'},
        )
        self.assertEqual(r.status_code, 200, r.text)
        payload = json.loads(r.text)
        self.assertTrue(str(payload[0]["Status"]).startswith("Up"))
        self.assertTrue(str(payload[0]["Status"]).endswith("(Paused)"))

        r = requests.post(PODMAN_URL + f"/v1.40/containers/{container_id}/unpause")
        self.assertEqual(r.status_code, 204, r.text)
        r = requests.post(PODMAN_URL + f"/v1.40/containers/{container_id}/stop")
        self.assertEqual(r.status_code, 204, r.text)

        r = requests.get(
            PODMAN_URL + "/v1.40/containers/json",
            params={"all": "true", "filters": f'{{"id":["{container_id}"]}}'},
        )
        self.assertEqual(r.status_code, 200, r.text)
        payload = json.loads(r.text)
        self.assertTrue(str(payload[0]["Status"]).startswith("Exited"))

        r = requests.delete(PODMAN_URL + f"/v1.40/containers/{container_id}")
        self.assertEqual(r.status_code, 204, r.text)

    def test_pod_start_conflict(self):
        """Verify issue #8865"""

        pod_name = list()
        pod_name.append("Pod_" + "".join(random.choice(string.ascii_letters) for i in range(10)))
        pod_name.append("Pod_" + "".join(random.choice(string.ascii_letters) for i in range(10)))

        r = requests.post(
            _url("/pods/create"),
            json={
                "name": pod_name[0],
                "no_infra": False,
                "portmappings": [{"host_ip": "127.0.0.1", "host_port": 8889, "container_port": 89}],
            },
        )
        self.assertEqual(r.status_code, 201, r.text)
        r = requests.post(
            _url("/containers/create"),
            json={
                "pod": pod_name[0],
                "image": "docker.io/alpine:latest",
                "command": ["top"],
            },
        )
        self.assertEqual(r.status_code, 201, r.text)

        r = requests.post(
            _url("/pods/create"),
            json={
                "name": pod_name[1],
                "no_infra": False,
                "portmappings": [{"host_ip": "127.0.0.1", "host_port": 8889, "container_port": 89}],
            },
        )
        self.assertEqual(r.status_code, 201, r.text)
        r = requests.post(
            _url("/containers/create"),
            json={
                "pod": pod_name[1],
                "image": "docker.io/alpine:latest",
                "command": ["top"],
            },
        )
        self.assertEqual(r.status_code, 201, r.text)

        r = requests.post(_url(f"/pods/{pod_name[0]}/start"))
        self.assertEqual(r.status_code, 200, r.text)

        r = requests.post(_url(f"/pods/{pod_name[1]}/start"))
        self.assertEqual(r.status_code, 409, r.text)

        start = json.loads(r.text)
        self.assertGreater(len(start["Errs"]), 0, r.text)


if __name__ == "__main__":
    unittest.main()
