import json
import subprocess
import unittest

import requests
import sys
import time

from test.apiv2.rest_api import Podman

PODMAN_URL = "http://localhost:8080"


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

        r = requests.post(
            PODMAN_URL + "/v2.0.0/libpod/images/pull?reference=docker.io%2Falpine%3Alatest"
        )
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

    def test_issue8631(self):
        """Create and Start container. see https://github.com/containers/podman/issues/8631"""
        self.skipTest("Test skipped unless debugging issue#8631. Storage requirements exceed 25GB")

        pull = requests.post(
            PODMAN_URL + "/v1.40/images/create?fromImage=docker.io/kubevirtci/k8s-1.19"
        )
        self.assertEqual(pull.status_code, 200, pull.content)

        ls = requests.get(PODMAN_URL + "/v1.40/images/json")
        self.assertEqual(ls.status_code, 200, ls.content)

        create = requests.post(
            PODMAN_URL + "/v1.40/containers/create?name=kubevirt-dnsmasq2",
            json={
                "Hostname": "",
                "Domainname": "",
                "User": "",
                "AttachStdin": False,
                "AttachStdout": False,
                "AttachStderr": False,
                "ExposedPorts": {
                    "2201/tcp": {},
                    "443/tcp": {},
                    "5000/tcp": {},
                    "5901/tcp": {},
                    "6443/tcp": {},
                    "80/tcp": {},
                    "8443/tcp": {},
                },
                "Tty": False,
                "OpenStdin": False,
                "StdinOnce": False,
                "Env": ["NUM_NODES=1", "NUM_SECONDARY_NICS=0"],
                "Cmd": ["/bin/bash", "-c", "/dnsmasq.sh"],
                "Image": "docker.io/kubevirtci/k8s-1.19",
                "Volumes": None,
                "WorkingDir": "",
                "Entrypoint": None,
                "OnBuild": None,
                "Labels": None,
                "HostConfig": {
                    "Binds": None,
                    "ContainerIDFile": "",
                    "LogConfig": {"Type": "", "Config": None},
                    "NetworkMode": "",
                    "PortBindings": {},
                    "RestartPolicy": {"Name": "", "MaximumRetryCount": 0},
                    "AutoRemove": False,
                    "VolumeDriver": "",
                    "VolumesFrom": None,
                    "CapAdd": None,
                    "CapDrop": None,
                    "Dns": None,
                    "DnsOptions": None,
                    "DnsSearch": None,
                    "ExtraHosts": [
                        "nfs:192.168.66.2",
                        "registry:192.168.66.2",
                        "ceph:192.168.66.2",
                    ],
                    "GroupAdd": None,
                    "IpcMode": "",
                    "Cgroup": "",
                    "Links": None,
                    "OomScoreAdj": 0,
                    "PidMode": "",
                    "Privileged": True,
                    "PublishAllPorts": True,
                    "ReadonlyRootfs": False,
                    "SecurityOpt": None,
                    "UTSMode": "",
                    "UsernsMode": "",
                    "ShmSize": 0,
                    "ConsoleSize": [0, 0],
                    "Isolation": "",
                    "CpuShares": 0,
                    "Memory": 0,
                    "NanoCpus": 0,
                    "CgroupParent": "",
                    "BlkioWeight": 0,
                    "BlkioWeightDevice": None,
                    "BlkioDeviceReadBps": None,
                    "BlkioDeviceWriteBps": None,
                    "BlkioDeviceReadIOps": None,
                    "BlkioDeviceWriteIOps": None,
                    "CpuPeriod": 0,
                    "CpuQuota": 0,
                    "CpuRealtimePeriod": 0,
                    "CpuRealtimeRuntime": 0,
                    "CpusetCpus": "",
                    "CpusetMems": "",
                    "Devices": None,
                    "DiskQuota": 0,
                    "KernelMemory": 0,
                    "MemoryReservation": 0,
                    "MemorySwap": 0,
                    "MemorySwappiness": None,
                    "OomKillDisable": None,
                    "PidsLimit": 0,
                    "Ulimits": None,
                    "CpuCount": 0,
                    "CpuPercent": 0,
                    "IOMaximumIOps": 0,
                    "IOMaximumBandwidth": 0,
                    "Mounts": [
                        {"Type": "bind", "Source": "/lib/modules", "Target": "/lib/modules"}
                    ],
                },
                "NetworkingConfig": None,
            },
        )
        self.assertEqual(create.status_code, 201, create.text)
        payload = json.loads(create.text)
        ident = payload["Id"]

        start = requests.post(PODMAN_URL + f"/v1.40/containers/{ident}/start")
        self.assertEqual(start.status_code, 204, start.text)
