import os
import random
import unittest

import requests
from .fixtures import APITestCase


class VolumeTestCase(APITestCase):
    def test_volume(self):
        name = f"Volume_{random.getrandbits(160):x}"

        ls = requests.get(self.podman_url + "/v1.40/volumes")
        self.assertEqual(ls.status_code, 200, ls.text)

        # See https://docs.docker.com/engine/api/v1.40/#operation/VolumeList
        required_keys = (
            "Volumes",
            "Warnings",
        )

        volumes = ls.json()
        self.assertIsInstance(volumes, dict)
        for key in required_keys:
            self.assertIn(key, volumes)

        create = requests.post(self.podman_url + "/v1.40/volumes/create", json={"Name": name})
        self.assertEqual(create.status_code, 201, create.text)

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

        volume = create.json()
        self.assertIsInstance(volume, dict)
        for k in required_keys:
            self.assertIn(k, volume)
        self.assertEqual(volume["Name"], name)

        inspect = requests.get(self.podman_url + f"/v1.40/volumes/{name}")
        self.assertEqual(inspect.status_code, 200, inspect.text)

        volume = inspect.json()
        self.assertIsInstance(volume, dict)
        for k in required_keys:
            self.assertIn(k, volume)

        rm = requests.delete(self.podman_url + f"/v1.40/volumes/{name}")
        self.assertEqual(rm.status_code, 204, rm.text)

        # recreate volume with data and then prune it
        r = requests.post(self.podman_url + "/v1.40/volumes/create", json={"Name": name})
        self.assertEqual(create.status_code, 201, create.text)

        create = r.json()
        with open(os.path.join(create["Mountpoint"], "test_prune"), "w") as file:
            file.writelines(["This is a test\n", "This is a good test\n"])

        prune = requests.post(self.podman_url + "/v1.40/volumes/prune")
        self.assertEqual(prune.status_code, 200, prune.text)

        payload = prune.json()
        self.assertIn(name, payload["VolumesDeleted"])
        self.assertGreater(payload["SpaceReclaimed"], 0)


if __name__ == "__main__":
    unittest.main()
