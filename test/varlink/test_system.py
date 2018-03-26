import unittest
from varlink import (Client, VarlinkError)


address = "unix:/run/podman/io.projectatomic.podman"
client = Client(address=address)

class SystemAPI(unittest.TestCase):
    def test_ping(self):
        podman = client.open("io.projectatomic.podman")
        response = podman.Ping()
        self.assertEqual("OK", response["ping"]["message"])

    def test_GetVersion(self):
        podman = client.open("io.projectatomic.podman")
        response = podman.GetVersion()
        for k in ["version", "go_version", "built", "os_arch"]:
            self.assertTrue(k in response["version"].keys())

if __name__ == '__main__':
    unittest.main()
