import random
import unittest

import requests
from dateutil.parser import parse

from .fixtures import APITestCase


class ContainerTestCase(APITestCase):
    def test_list(self):
        r = requests.get(self.uri("/containers/json"), timeout=5)
        self.assertEqual(r.status_code, 200, r.text)
        obj = r.json()
        self.assertEqual(len(obj), 1)

    def test_list_all(self):
        r = requests.get(self.uri("/containers/json?all=true"))
        self.assertEqual(r.status_code, 200, r.text)
        self.assertId(r.content)

    def test_inspect(self):
        r = requests.get(self.uri(self.resolve_container("/containers/{}/json")))
        self.assertEqual(r.status_code, 200, r.text)
        self.assertId(r.content)
        _ = parse(r.json()["Created"])

    def test_stats(self):
        r = requests.get(self.uri(self.resolve_container("/containers/{}/stats?stream=false")))
        self.assertIn(r.status_code, (200, 409), r.text)
        if r.status_code == 200:
            self.assertId(r.content)
        r = requests.get(self.uri(self.resolve_container("/containers/{}/stats?stream=false&one-shot=true")))
        self.assertIn(r.status_code, (200, 409), r.text)
        if r.status_code == 200:
            self.assertId(r.content)

    def test_delete(self):
        r = requests.delete(self.uri(self.resolve_container("/containers/{}?force=true")))
        self.assertEqual(r.status_code, 204, r.text)

    def test_stop(self):
        r = requests.post(self.uri(self.resolve_container("/containers/{}/start")))
        self.assertIn(r.status_code, (204, 304), r.text)

        r = requests.post(self.uri(self.resolve_container("/containers/{}/stop")))
        self.assertIn(r.status_code, (204, 304), r.text)

    def test_start(self):
        r = requests.post(self.uri(self.resolve_container("/containers/{}/stop")))
        self.assertIn(r.status_code, (204, 304), r.text)

        r = requests.post(self.uri(self.resolve_container("/containers/{}/start")))
        self.assertIn(r.status_code, (204, 304), r.text)

    def test_restart(self):
        r = requests.post(self.uri(self.resolve_container("/containers/{}/start")))
        self.assertIn(r.status_code, (204, 304), r.text)

        r = requests.post(self.uri(self.resolve_container("/containers/{}/restart")), timeout=5)
        self.assertEqual(r.status_code, 204, r.text)

    def test_resize(self):
        r = requests.post(self.uri(self.resolve_container("/containers/{}/resize?h=43&w=80")))
        self.assertIn(r.status_code, (200, 409), r.text)
        if r.status_code == 200:
            self.assertEqual(r.text, "", r.text)

    def test_attach(self):
        self.skipTest("FIXME: Test timeouts")
        r = requests.post(self.uri(self.resolve_container("/containers/{}/attach")), timeout=5)
        self.assertIn(r.status_code, (101, 500), r.text)

    def test_logs(self):
        r = requests.get(self.uri(self.resolve_container("/containers/{}/logs?stdout=true")))
        self.assertEqual(r.status_code, 200, r.text)

    def test_commit(self):
        r = requests.post(self.uri(self.resolve_container("/commit?container={}")))
        self.assertEqual(r.status_code, 200, r.text)
        self.assertId(r.content)

        obj = r.json()
        self.assertIsInstance(obj, dict)

    def test_prune(self):
        name = f"Container_{random.getrandbits(160):x}"

        r = requests.post(
            self.podman_url + f"/v1.40/containers/create?name={name}",
            json={
                "Cmd": ["cp", "/etc/motd", "/motd.size_test"],
                "Image": "alpine:latest",
                "NetworkDisabled": True,
            },
        )
        self.assertEqual(r.status_code, 201, r.text)
        create = r.json()

        r = requests.post(self.podman_url + f"/v1.40/containers/{create['Id']}/start")
        self.assertEqual(r.status_code, 204, r.text)

        r = requests.post(self.podman_url + f"/v1.40/containers/{create['Id']}/wait")
        self.assertEqual(r.status_code, 200, r.text)
        wait = r.json()
        self.assertEqual(wait["StatusCode"], 0, wait["Error"])

        prune = requests.post(self.podman_url + "/v1.40/containers/prune")
        self.assertEqual(prune.status_code, 200, prune.status_code)
        prune_payload = prune.json()
        self.assertGreater(prune_payload["SpaceReclaimed"], 0)
        self.assertIn(create["Id"], prune_payload["ContainersDeleted"])

        # Delete any orphaned containers
        r = requests.get(self.podman_url + "/v1.40/containers/json?all=true")
        self.assertEqual(r.status_code, 200, r.text)
        for self.resolve_container in r.json():
            requests.delete(
                self.podman_url + f"/v1.40/containers/{self.resolve_container['Id']}?force=true"
            )

        # Image prune here tied to containers freeing up
        prune = requests.post(self.podman_url + "/v1.40/images/prune")
        self.assertEqual(prune.status_code, 200, prune.text)
        prune_payload = prune.json()
        self.assertGreater(prune_payload["SpaceReclaimed"], 0)

        # FIXME need method to determine which image is going to be "pruned" to fix test
        # TODO should handler be recursive when deleting images?
        # self.assertIn(img["Id"], prune_payload["ImagesDeleted"][1]["Deleted"])

        # FIXME (@vrothberg): I commented this line out during the `libimage` migration.
        # It doesn't make sense to report anything to be deleted if the reclaimed space
        # is zero.  I think the test needs some rewrite.
        # self.assertIsNotNone(prune_payload["ImagesDeleted"][1]["Deleted"])

    def test_status(self):
        r = requests.post(
            self.podman_url + "/v1.40/containers/create?name=topcontainer",
            json={"Cmd": ["top"], "Image": "alpine:latest"},
        )
        self.assertEqual(r.status_code, 201, r.text)
        payload = r.json()
        container_id = payload["Id"]
        self.assertIsNotNone(container_id)

        r = requests.get(
            self.podman_url + "/v1.40/containers/json",
            params={"all": "true", "filters": f'{{"id":["{container_id}"]}}'},
        )
        self.assertEqual(r.status_code, 200, r.text)
        payload = r.json()
        self.assertEqual(payload[0]["Status"], "Created")

        r = requests.post(self.podman_url + f"/v1.40/containers/{container_id}/start")
        self.assertEqual(r.status_code, 204, r.text)

        r = requests.get(
            self.podman_url + "/v1.40/containers/json",
            params={"all": "true", "filters": f'{{"id":["{container_id}"]}}'},
        )
        self.assertEqual(r.status_code, 200, r.text)
        payload = r.json()
        self.assertTrue(str(payload[0]["Status"]).startswith("Up"))

        r = requests.post(self.podman_url + f"/v1.40/containers/{container_id}/pause")
        self.assertEqual(r.status_code, 204, r.text)

        r = requests.get(
            self.podman_url + "/v1.40/containers/json",
            params={"all": "true", "filters": f'{{"id":["{container_id}"]}}'},
        )
        self.assertEqual(r.status_code, 200, r.text)
        payload = r.json()
        self.assertTrue(str(payload[0]["Status"]).startswith("Up"))
        self.assertTrue(str(payload[0]["Status"]).endswith("(Paused)"))

        r = requests.post(self.podman_url + f"/v1.40/containers/{container_id}/unpause")
        self.assertEqual(r.status_code, 204, r.text)
        r = requests.post(self.podman_url + f"/v1.40/containers/{container_id}/stop")
        self.assertEqual(r.status_code, 204, r.text)

        r = requests.get(
            self.podman_url + "/v1.40/containers/json",
            params={"all": "true", "filters": f'{{"id":["{container_id}"]}}'},
        )
        self.assertEqual(r.status_code, 200, r.text)
        payload = r.json()
        self.assertTrue(str(payload[0]["Status"]).startswith("Exited"))

        r = requests.delete(self.podman_url + f"/v1.40/containers/{container_id}")
        self.assertEqual(r.status_code, 204, r.text)


if __name__ == "__main__":
    unittest.main()
