import json
import subprocess
import unittest

import requests
import sys
import time

from .podman import Podman


class APITestCase(unittest.TestCase):
    PODMAN_URL = "http://localhost:8080"
    podman = None  # initialized podman configuration for tests
    service = None  # podman service instance

    @classmethod
    def setUpClass(cls):
        super().setUpClass()

        APITestCase.podman = Podman()
        APITestCase.service = APITestCase.podman.open(
            "system", "service", "tcp:localhost:8080", "--time=0"
        )
        # give the service some time to be ready...
        time.sleep(2)

        returncode = APITestCase.service.poll()
        if returncode is not None:
            raise subprocess.CalledProcessError(returncode, "podman system service")

        r = requests.post(
            APITestCase.uri("/images/pull?reference=quay.io%2Flibpod%2Falpine%3Alatest")
        )
        if r.status_code != 200:
            raise subprocess.CalledProcessError(
                r.status_code, f"podman images pull quay.io/libpod/alpine:latest {r.text}"
            )

    @classmethod
    def tearDownClass(cls):
        APITestCase.service.terminate()
        stdout, stderr = APITestCase.service.communicate(timeout=0.5)
        if stdout:
            sys.stdout.write("\nService Stdout:\n" + stdout.decode("utf-8"))
        if stderr:
            sys.stderr.write("\nService Stderr:\n" + stderr.decode("utf-8"))
        return super().tearDownClass()

    def setUp(self):
        super().setUp()
        APITestCase.podman.run("run", "-d", "alpine", "top", check=True)

    def tearDown(self) -> None:
        APITestCase.podman.run("pod", "rm", "--all", "--force", check=True)
        APITestCase.podman.run("rm", "--all", "--force", check=True)
        super().tearDown()

    @property
    def podman_url(self):
        return "http://localhost:8080"

    @staticmethod
    def uri(path):
        return APITestCase.PODMAN_URL + "/v2.0.0/libpod" + path

    @staticmethod
    def compat_uri(path):
        return APITestCase.PODMAN_URL + "/v3.0.0/" + path

    def resolve_container(self, path):
        """Find 'first' container and return 'Id' formatted into given URI path."""

        try:
            r = requests.get(self.uri("/containers/json?all=true"))
            containers = r.json()
        except Exception as e:
            msg = f"Bad container response: {e}"
            if r is not None:
                msg += ": " + r.text
            raise self.failureException(msg)
        return path.format(containers[0]["Id"])

    def assertContainerExists(self, member, msg=None):  # pylint: disable=invalid-name
        r = requests.get(self.uri(f"/containers/{member}/exists"))
        if r.status_code == 404:
            if msg is None:
                msg = f"Container '{member}' does not exist."
            self.failureException(msg)

    def assertContainerNotExists(self, member, msg=None):  # pylint: disable=invalid-name
        r = requests.get(self.uri(f"/containers/{member}/exists"))
        if r.status_code == 204:
            if msg is None:
                msg = f"Container '{member}' exists."
            self.failureException(msg)

    def assertId(self, content):  # pylint: disable=invalid-name
        objects = json.loads(content)
        try:
            if isinstance(objects, dict):
                _ = objects["Id"]
            else:
                for item in objects:
                    _ = item["Id"]
        except KeyError:
            self.failureException("Failed in find 'Id' in return value.")
