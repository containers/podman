import itertools
import os
import unittest
from test.podman_testcase import PodmanTestCase

import podman


class TestImages(PodmanTestCase):
    @classmethod
    def setUpClass(cls):
        super().setUpClass()

    @classmethod
    def tearDownClass(cls):
        super().tearDownClass()

    def setUp(self):
        self.tmpdir = os.environ['TMPDIR']
        self.host = os.environ['PODMAN_HOST']

        self.pclient = podman.Client(self.host)
        self.images = self.loadCache()

    def tearDown(self):
        pass

    def loadCache(self):
        with podman.Client(self.host) as pclient:
            self.images = list(pclient.images.list())

        self.alpine_image = next(
            iter([
                i for i in self.images
                if 'docker.io/library/alpine:latest' in i['repoTags']
            ] or []), None)
        return self.images

    def test_list(self):
        actual = self.loadCache()
        self.assertGreaterEqual(len(actual), 2)
        self.assertIsNotNone(self.alpine_image)

    def test_build(self):
        with self.assertRaisesNotImplemented():
            self.pclient.images.build()

    def test_create(self):
        with self.assertRaisesNotImplemented():
            self.pclient.images.create()

    @unittest.skip('Code implemented')
    def test_create_from(self):
        with self.assertRaisesNotImplemented():
            self.pclient.images.create_from()

    def test_export(self):
        path = os.path.join(self.tmpdir, 'alpine_export.tar')
        target = 'oci-archive:{}:latest'.format(path)

        actual = self.alpine_image.export(target, False)
        self.assertTrue(actual)
        self.assertTrue(os.path.isfile(path))

    def test_history(self):
        count = 0
        for record in self.alpine_image.history():
            count += 1
            self.assertEqual(record.id, self.alpine_image.id)
        self.assertGreater(count, 0)

    def test_inspect(self):
        actual = self.alpine_image.inspect()
        self.assertEqual(actual.id, self.alpine_image.id)

    def test_push(self):
        path = '{}/alpine_push'.format(self.tmpdir)
        target = 'dir:{}'.format(path)
        self.alpine_image.push(target)

        self.assertTrue(os.path.isfile(os.path.join(path, 'manifest.json')))
        self.assertTrue(os.path.isfile(os.path.join(path, 'version')))

    def test_tag(self):
        self.assertEqual(self.alpine_image.id,
                         self.alpine_image.tag('alpine:fubar'))
        self.loadCache()
        self.assertIn('alpine:fubar', self.alpine_image.repoTags)

    def test_remove(self):
        before = self.loadCache()

        # assertRaises doesn't follow the import name :(
        with self.assertRaises(podman.ErrorOccurred):
            self.alpine_image.remove()

        # TODO: remove this block once force=True works
        with podman.Client(self.host) as pclient:
            for ctnr in pclient.containers.list():
                if 'alpine' in ctnr.image:
                    ctnr.stop()
                    ctnr.remove()

        actual = self.alpine_image.remove(force=True)
        self.assertEqual(self.alpine_image.id, actual)
        after = self.loadCache()

        self.assertLess(len(after), len(before))
        TestImages.setUpClass()
        self.loadCache()

    def test_import_delete_unused(self):
        before = self.loadCache()
        # create unused image, so we have something to delete
        source = os.path.join(self.tmpdir, 'alpine_gold.tar')
        new_img = self.pclient.images.import_image(source, 'alpine2:latest',
                                                   'unittest.test_import')
        after = self.loadCache()

        self.assertEqual(len(before) + 1, len(after))
        self.assertIsNotNone(
            next(iter([i for i in after if new_img in i['id']] or []), None))

        actual = self.pclient.images.delete_unused()
        self.assertIn(new_img, actual)

        after = self.loadCache()
        self.assertEqual(len(before), len(after))

        TestImages.setUpClass()
        self.loadCache()

    def test_pull(self):
        before = self.loadCache()
        actual = self.pclient.images.pull('prom/busybox:latest')
        after = self.loadCache()

        self.assertEqual(len(before) + 1, len(after))
        self.assertIsNotNone(
            next(iter([i for i in after if actual in i['id']] or []), None))

    def test_search(self):
        actual = self.pclient.images.search('alpine', 25)
        names, lengths = itertools.tee(actual)

        for img in names:
            self.assertIn('alpine', img['name'])
        self.assertTrue(0 < len(list(lengths)) <= 25)


if __name__ == '__main__':
    unittest.main()
