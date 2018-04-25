import unittest

from varlink import VarlinkError

from podman_testcase import PodmanTestCase

MethodNotImplemented = 'org.varlink.service.MethodNotImplemented'


class TestContainersAPI(PodmanTestCase):
    def test_ListContainers(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.ListContainers()

    def test_CreateContainer(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.CreateContainer()

    def test_InspecContainer(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.InspectContainer()

    def test_ListContainerProcesses(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.ListContainerProcesses()

    def test_GetContainerLogs(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.GetContainerLogs()

    def test_ListContainerChanges(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.ListContainerChanges()

    def test_ExportContainer(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.ExportContainer()

    def test_GetContainerStats(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.GetContainerStats()

    def test_ResizeContainerTty(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.ResizeContainerTty()

    def test_StartContainer(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.StartContainer()

    def test_RestartContainer(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.RestartContainer()

    def test_KillContainer(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.KillContainer()

    def test_UpdateContainer(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.UpdateContainer()

    def test_RenameContainer(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.RenameContainer()

    def test_PauseContainer(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.PauseContainer()

    def test_UnpauseContainer(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.UnpauseContainer()

    def test_AttachToContainer(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.AttachToContainer()

    def test_WaitContainer(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.WaitContainer()

    def test_RemoveContainer(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.RemoveContainer()


if __name__ == '__main__':
    unittest.main()
