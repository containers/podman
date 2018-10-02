import os
from test.podman_testcase import PodmanTestCase

import podman
from podman import FoldedString

pod = None


class TestPodsCtnrs(PodmanTestCase):
    @classmethod
    def setUpClass(cls):
        # Populate storage
        super().setUpClass()

    @classmethod
    def tearDownClass(cls):
        super().tearDownClass()

    def setUp(self):
        self.tmpdir = os.environ['TMPDIR']
        self.host = os.environ['PODMAN_HOST']

        self.pclient = podman.Client(self.host)

    def test_010_populate(self):
        global pod

        pod = self.pclient.pods.create('pod1')
        self.assertEqual('pod1', pod.name)

        img = self.pclient.images.get('docker.io/library/alpine:latest')
        ctnr = img.container(pod=pod.id)

        pod.refresh()
        self.assertEqual('1', pod.numberofcontainers)
        self.assertEqual(ctnr.id, pod.containersinfo[0]['id'])

    def test_015_one_shot(self):
        global pod

        details = pod.inspect()
        state = FoldedString(details.containers[0]['state'])
        self.assertEqual(state, 'configured')

        pod = pod.start()
        status = FoldedString(pod.containersinfo[0]['status'])
        # Race on whether container is still running or finished
        self.assertIn(status, ('exited', 'running'))

        pod = pod.restart()
        status = FoldedString(pod.containersinfo[0]['status'])
        self.assertIn(status, ('exited', 'running'))

        killed = pod.kill()
        self.assertEqual(pod, killed)

    def test_999_remove(self):
        global pod

        ident = pod.remove(force=True)
        self.assertEqual(ident, pod.id)

        with self.assertRaises(StopIteration):
            next(self.pclient.pods.list())
