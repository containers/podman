import os
import signal
import unittest
from test.podman_testcase import PodmanTestCase

import podman


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
        self.loadCache()

    def tearDown(self):
        pass

    def loadCache(self):
        self.containers = list(self.pclient.containers.list())

        self.alpine_ctnr = next(
            iter([c for c in self.containers if 'alpine' in c['image']] or []),
            None)

        if self.alpine_ctnr and self.alpine_ctnr.status != 'running':
            self.alpine_ctnr.start()

    def test_list(self):
        self.assertGreaterEqual(len(self.containers), 2)
        self.assertIsNotNone(self.alpine_ctnr)
        self.assertIn('alpine', self.alpine_ctnr.image)

    def test_delete_stopped(self):
        before = len(self.containers)

        self.alpine_ctnr.stop()
        target = self.alpine_ctnr.id
        actual = self.pclient.containers.delete_stopped()
        self.assertIn(target, actual)

        self.loadCache()
        after = len(self.containers)

        self.assertLess(after, before)
        TestContainers.setUpClass()

    def test_get(self):
        actual = self.pclient.containers.get(self.alpine_ctnr.id)
        for k in ['id', 'status', 'ports']:
            self.assertEqual(actual[k], self.alpine_ctnr[k])

        with self.assertRaises(podman.ContainerNotFound):
            self.pclient.containers.get("bozo")

    def test_attach(self):
        # StringIO does not support fileno() so we had to go old school
        input = os.path.join(self.tmpdir, 'test_attach.stdin')
        output = os.path.join(self.tmpdir, 'test_attach.stdout')

        with open(input, 'w+') as mock_in, open(output, 'w+') as mock_out:
            # double quote is indeed in the expected place
            mock_in.write('echo H"ello, World"; exit\n')
            mock_in.seek(0, 0)

            ctnr = self.pclient.images.get(self.alpine_ctnr.image).container(
                detach=True, tty=True)
            ctnr.attach(stdin=mock_in.fileno(), stdout=mock_out.fileno())
            ctnr.start()

            mock_out.flush()
            mock_out.seek(0, 0)
            output = mock_out.read()
            self.assertIn('Hello', output)

            ctnr.remove(force=True)

    def test_processes(self):
        actual = list(self.alpine_ctnr.processes())
        self.assertGreaterEqual(len(actual), 2)

    def test_start_stop_wait(self):
        ctnr = self.alpine_ctnr.stop()
        self.assertFalse(ctnr['running'])

        ctnr.start()
        self.assertTrue(ctnr.running)

        ctnr.stop()
        self.assertFalse(ctnr['containerrunning'])

        actual = ctnr.wait()
        self.assertGreaterEqual(actual, 0)

    def test_changes(self):
        actual = self.alpine_ctnr.changes()

        self.assertListEqual(
            sorted(['changed', 'added', 'deleted']), sorted(
                list(actual.keys())))

        # TODO: brittle, depends on knowing history of ctnr
        self.assertGreaterEqual(len(actual['changed']), 2)
        self.assertGreaterEqual(len(actual['added']), 2)
        self.assertEqual(len(actual['deleted']), 0)

    def test_kill(self):
        self.assertTrue(self.alpine_ctnr.running)
        ctnr = self.alpine_ctnr.kill(signal.SIGKILL)
        self.assertFalse(ctnr.running)

    def test_inspect(self):
        actual = self.alpine_ctnr.inspect()
        self.assertEqual(actual.id, self.alpine_ctnr.id)
        # TODO: Datetime values from inspect missing offset in CI instance
        # self.assertEqual(
        # datetime_parse(actual.created),
        # datetime_parse(self.alpine_ctnr.createdat))

    def test_export(self):
        target = os.path.join(self.tmpdir, 'alpine_export_ctnr.tar')

        actual = self.alpine_ctnr.export(target)
        self.assertEqual(actual, target)
        self.assertTrue(os.path.isfile(target))
        self.assertGreater(os.path.getsize(target), 0)

    def test_commit(self):
        # TODO: Test for STOPSIGNAL when supported by OCI
        # TODO: Test for message when supported by OCI
        details = self.pclient.images.get(self.alpine_ctnr.image).inspect()
        changes = ['ENV=' + i for i in details.containerconfig['env']]
        changes.append('CMD=/usr/bin/zsh')
        changes.append('ENTRYPOINT=/bin/sh date')
        changes.append('ENV=TEST=test_containers.TestContainers.test_commit')
        changes.append('EXPOSE=80')
        changes.append('EXPOSE=8888')
        changes.append('LABEL=unittest=test_commit')
        changes.append('USER=bozo:circus')
        changes.append('VOLUME=/data')
        changes.append('WORKDIR=/data/application')

        id = self.alpine_ctnr.commit(
            'alpine3', author='Bozo the clown', changes=changes, pause=True)
        img = self.pclient.images.get(id)
        self.assertIsNotNone(img)

        details = img.inspect()
        self.assertEqual(details.author, 'Bozo the clown')
        self.assertListEqual(['/usr/bin/zsh'], details.containerconfig['cmd'])
        self.assertListEqual(['/bin/sh date'],
                             details.containerconfig['entrypoint'])
        self.assertIn('TEST=test_containers.TestContainers.test_commit',
                      details.containerconfig['env'])
        self.assertTrue(
            [e for e in details.containerconfig['env'] if 'PATH=' in e])
        self.assertDictEqual({
            '80': {},
            '8888': {},
        }, details.containerconfig['exposedports'])
        self.assertDictEqual({'unittest': 'test_commit'}, details.labels)
        self.assertEqual('bozo:circus', details.containerconfig['user'])
        self.assertEqual({'/data': {}}, details.containerconfig['volumes'])
        self.assertEqual('/data/application',
                         details.containerconfig['workingdir'])

    def test_remove(self):
        before = len(self.containers)

        with self.assertRaises(podman.ErrorOccurred):
            self.alpine_ctnr.remove()

        self.assertEqual(
            self.alpine_ctnr.id, self.alpine_ctnr.remove(force=True))
        self.loadCache()
        after = len(self.containers)

        self.assertLess(after, before)
        TestContainers.setUpClass()

    def test_restart(self):
        self.assertTrue(self.alpine_ctnr.running)
        before = self.alpine_ctnr.runningfor

        ctnr = self.alpine_ctnr.restart()
        self.assertTrue(ctnr.running)

        after = self.alpine_ctnr.runningfor

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

        ctnr = self.alpine_ctnr.pause()
        self.assertEqual(ctnr.status, 'paused')

        ctnr = self.alpine_ctnr.unpause()
        self.assertTrue(ctnr.running)
        self.assertTrue(ctnr.status, 'running')

    def test_stats(self):
        self.assertTrue(self.alpine_ctnr.running)

        actual = self.alpine_ctnr.stats()
        self.assertEqual(self.alpine_ctnr.id, actual.id)
        self.assertEqual(self.alpine_ctnr.names, actual.name)

    def test_logs(self):
        self.assertTrue(self.alpine_ctnr.running)
        actual = list(self.alpine_ctnr.logs())
        self.assertIsNotNone(actual)


if __name__ == '__main__':
    unittest.main()
