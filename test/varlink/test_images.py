import unittest

from varlink import VarlinkError

from podman_testcase import PodmanTestCase

MethodNotImplemented = 'org.varlink.service.MethodNotImplemented'


class TestImagesAPI(PodmanTestCase):
    def test_ListImages(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.ListImages()

    def test_BuildImage(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.BuildImage()

    def test_CreateImage(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.CreateImage()

    def test_InspectImage(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.InspectImage()

    def test_HistoryImage(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.HistoryImage()

    def test_PushImage(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.PushImage()

    def test_TagImage(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.TagImage()

    def test_RemoveImage(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.TagImage()

    def test_SearchImage(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.SearchImage()

    def test_DeleteUnusedImages(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.DeleteUnusedImages()

    def test_CreateFromContainer(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.CreateFromContainer()

    def test_ImportImage(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.ImportImage()

    def test_ExportImage(self):
        with self.assertRaisesRegex(VarlinkError, MethodNotImplemented):
            self.podman.ExportImage()


if __name__ == '__main__':
    unittest.main()
