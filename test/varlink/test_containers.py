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


class ContainersAPI(unittest.TestCase):
    def test_ListContainers(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.ListContainers))

    def test_CreateContainer(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.CreateContainer))

    def test_InspecContainer(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.InspectContainer))

    def test_ListContainerProcesses(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.ListContainerProcesses))

    def test_GetContainerLogs(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.GetContainerLogs))

    def test_ListContainerChanges(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.ListContainerChanges))

    def test_ExportContainer(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.ExportContainer))

    def test_GetContainerStats(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.GetContainerStats))

    def test_ResizeContainerTty(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.ResizeContainerTty))

    def test_StartContainer(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.StartContainer))

    def test_RestartContainer(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.RestartContainer))

    def test_KillContainer(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.KillContainer))

    def test_UpdateContainer(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.UpdateContainer))

    def test_RenameContainer(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.RenameContainer))

    def test_PauseContainer(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.PauseContainer))

    def test_UnpauseContainer(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.UnpauseContainer))

    def test_AttachToContainer(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.AttachToContainer))

    def test_WaitContainer(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.WaitContainer))

    def test_RemoveContainer(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.RemoveContainer))

    def test_DeleteContainer(self):
        podman = client.open("io.projectatomic.podman")
        self.assertTrue(runErrorTest(podman.DeleteContainer))

if __name__ == '__main__':
    unittest.main()
