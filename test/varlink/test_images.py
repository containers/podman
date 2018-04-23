import unittest
from varlink import (Client, VarlinkError)


address = "unix:/run/podman/io.projectatomic.podman"
client = Client(address=address)


def runErrorTest(tfunc):
    try:
        tfunc()
    except VarlinkError as e:
        return e.error() == "org.varlink.service.MethodNotImplemented"
    return False


class ImagesAPI(unittest.TestCase):
    def test_ListImages(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.ListImages))

    def test_BuildImage(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.BuildImage))

    def test_CreateImage(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.CreateImage))

    def test_InspectImage(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.InspectImage))

    def test_HistoryImage(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.HistoryImage))

    def test_PushImage(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.PushImage))

    def test_TagImage(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.TagImage))

    def test_RemoveImage(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.TagImage))

    def test_SearchImage(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.SearchImage))

    def test_DeleteUnusedImages(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.DeleteUnusedImages))

    def test_CreateFromContainer(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.CreateFromContainer))

    def test_ImportImage(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.ImportImage))

    def test_ExportImage(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.ExportImage))

if __name__ == '__main__':
    unittest.main()
