import os
import time
import unittest
from test.podman_testcase import PodmanTestCase

import podman
from podman import datetime_parse


class TestContainers(PodmanTestCase):
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
        self.ctns = self.loadCache()
        # TODO: Change to start() when Implemented
        self.alpine_ctnr.restart()

    def tearDown(self):
        pass

    def loadCache(self):
        with podman.Client(self.host) as pclient:
            self.ctns = list(pclient.containers.list())

        self.alpine_ctnr = next(
            iter([c for c in self.ctns if 'alpine' in c['image']] or []), None)
        return self.ctns

    def test_list(self):
        actual = self.loadCache()
        self.assertGreaterEqual(len(actual), 2)
        self.assertIsNotNone(self.alpine_ctnr)
        self.assertIn('alpine', self.alpine_ctnr.image)

    def test_delete_stopped(self):
        before = self.loadCache()
        self.assertEqual(self.alpine_ctnr.id, self.alpine_ctnr.stop())
        actual = self.pclient.containers.delete_stopped()
        self.assertIn(self.alpine_ctnr.id, actual)
        after = self.loadCache()

        self.assertLess(len(after), len(before))
        TestContainers.setUpClass()
        self.loadCache()

    def test_create(self):
        with self.assertRaisesNotImplemented():
            self.pclient.containers.create()

    def test_get(self):
        actual = self.pclient.containers.get(self.alpine_ctnr.id)
        for k in ['id', 'status', 'ports']:
            self.assertEqual(actual[k], self.alpine_ctnr[k])

        with self.assertRaises(podman.ContainerNotFound):
            self.pclient.containers.get("bozo")

    def test_attach(self):
        with self.assertRaisesNotImplemented():
            self.alpine_ctnr.attach()

    def test_processes(self):
        actual = list(self.alpine_ctnr.processes())
        self.assertGreaterEqual(len(actual), 2)

    def test_start_stop_wait(self):
        self.assertEqual(self.alpine_ctnr.id, self.alpine_ctnr.stop())
        self.alpine_ctnr.refresh()
        self.assertFalse(self.alpine_ctnr['running'])

        self.assertEqual(self.alpine_ctnr.id, self.alpine_ctnr.restart())
        self.alpine_ctnr.refresh()
        self.assertTrue(self.alpine_ctnr.running)

        self.assertEqual(self.alpine_ctnr.id, self.alpine_ctnr.stop())
        self.alpine_ctnr.refresh()
        self.assertFalse(self.alpine_ctnr['containerrunning'])

        actual = self.alpine_ctnr.wait()
        self.assertEqual(0, actual)

    def test_changes(self):
        actual = self.alpine_ctnr.changes()

        self.assertListEqual(
            sorted(['changed', 'added', 'deleted']), sorted(
                list(actual.keys())))

        # TODO: brittle, depends on knowing history of ctnr
        self.assertGreaterEqual(len(actual['changed']), 2)
        self.assertGreaterEqual(len(actual['added']), 3)
        self.assertEqual(len(actual['deleted']), 0)

    def test_kill(self):
        self.assertTrue(self.alpine_ctnr.running)
        self.assertEqual(self.alpine_ctnr.id, self.alpine_ctnr.kill(9))
        time.sleep(2)

        self.alpine_ctnr.refresh()
        self.assertFalse(self.alpine_ctnr.running)

    def test_inspect(self):
        actual = self.alpine_ctnr.inspect()
        self.assertEqual(actual.id, self.alpine_ctnr.id)
        # TODO: Datetime values from inspect missing offset in CI instance
        # self.assertEqual(
        #     datetime_parse(actual.created),
        #     datetime_parse(self.alpine_ctnr.createdat))

    def test_export(self):
        target = os.path.join(self.tmpdir, 'alpine_export_ctnr.tar')

        actual = self.alpine_ctnr.export(target)
        self.assertEqual(actual, target)
        self.assertTrue(os.path.isfile(target))
        self.assertGreater(os.path.getsize(target), 0)

    def test_commit(self):
        # TODO: Test for STOPSIGNAL when supported by OCI
        # TODO: Test for message when supported by OCI
        # TODO: Test for EXPOSE when issue#795 fixed
        #       'EXPOSE=8888/tcp, 8888/udp'
        id = self.alpine_ctnr.commit(
            'alpine3',
            author='Bozo the clown',
            changes=[
                'CMD=/usr/bin/zsh',
                'ENTRYPOINT=/bin/sh date',
                'ENV=TEST=test_containers.TestContainers.test_commit',
                'LABEL=unittest=test_commit',
                'USER=bozo:circus',
                'VOLUME=/data',
                'WORKDIR=/data/application',
            ],
            pause=True)
        img = self.pclient.images.get(id)
        self.assertIsNotNone(img)

        details = img.inspect()
        self.assertEqual(details.author, 'Bozo the clown')
        self.assertListEqual(['/usr/bin/zsh'], details.containerconfig['cmd'])
        self.assertListEqual(['/bin/sh date'],
                             details.containerconfig['entrypoint'])
        self.assertListEqual(
            ['TEST=test_containers.TestContainers.test_commit'],
            details.containerconfig['env'])
        # self.assertDictEqual({
        #     '8888/tcp': {}
        # }, details.containerconfig['exposedports'])
        self.assertDictEqual({'unittest': 'test_commit'}, details.labels)
        self.assertEqual('bozo:circus', details.containerconfig['user'])
        self.assertEqual({'/data': {}}, details.containerconfig['volumes'])
        self.assertEqual('/data/application',
                         details.containerconfig['workingdir'])

    def test_remove(self):
        before = self.loadCache()

        with self.assertRaises(podman.ErrorOccurred):
            self.alpine_ctnr.remove()

        self.assertEqual(
            self.alpine_ctnr.id, self.alpine_ctnr.remove(force=True))
        after = self.loadCache()

        self.assertLess(len(after), len(before))
        TestContainers.setUpClass()
        self.loadCache()

    def test_restart(self):
        self.assertTrue(self.alpine_ctnr.running)
        before = self.alpine_ctnr.runningfor

        self.assertEqual(self.alpine_ctnr.id, self.alpine_ctnr.restart())

        self.alpine_ctnr.refresh()
        after = self.alpine_ctnr.runningfor
        self.assertTrue(self.alpine_ctnr.running)

        # TODO: restore check when restart zeros counter
        # self.assertLess(after, before)

    def test_rename(self):
        with self.assertRaisesNotImplemented():
            self.alpine_ctnr.rename('new_alpine')

    def test_resize_tty(self):
        with self.assertRaisesNotImplemented():
            self.alpine_ctnr.resize_tty(132, 43)

    def test_pause_unpause(self):
        self.assertTrue(self.alpine_ctnr.running)

        self.assertEqual(self.alpine_ctnr.id, self.alpine_ctnr.pause())
        self.alpine_ctnr.refresh()
        self.assertFalse(self.alpine_ctnr.running)

        self.assertEqual(self.alpine_ctnr.id, self.alpine_ctnr.unpause())
        self.alpine_ctnr.refresh()
        self.assertTrue(self.alpine_ctnr.running)

    def test_stats(self):
        self.alpine_ctnr.restart()
        actual = self.alpine_ctnr.stats()
        self.assertEqual(self.alpine_ctnr.id, actual.id)
        self.assertEqual(self.alpine_ctnr.names, actual.name)

    def test_logs(self):
        self.alpine_ctnr.restart()
        actual = list(self.alpine_ctnr.logs())
        self.assertIsNotNone(actual)


if __name__ == '__main__':
    unittest.main()
