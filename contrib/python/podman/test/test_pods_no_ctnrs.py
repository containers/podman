import os
import unittest

import podman
import varlink

ident = None
pod = None


class TestPodsNoCtnrs(unittest.TestCase):
    def setUp(self):
        self.tmpdir = os.environ['TMPDIR']
        self.host = os.environ['PODMAN_HOST']

        self.pclient = podman.Client(self.host)

    def test_010_create(self):
        global ident

        actual = self.pclient.pods.create('pod0')
        self.assertIsNotNone(actual)
        ident = actual.id

    def test_015_list(self):
        global ident, pod

        actual = next(self.pclient.pods.list())
        self.assertEqual('pod0', actual.name)
        self.assertEqual(ident, actual.id)
        self.assertEqual('Created', actual.status)
        self.assertEqual('0', actual.numberofcontainers)
        self.assertFalse(actual.containersinfo)
        pod = actual

    def test_020_get(self):
        global ident, pod

        actual = self.pclient.pods.get(pod.id)
        self.assertEqual('pod0', actual.name)
        self.assertEqual(ident, actual.id)
        self.assertEqual('Created', actual.status)
        self.assertEqual('0', actual.numberofcontainers)
        self.assertFalse(actual.containersinfo)

    def test_025_inspect(self):
        global ident, pod

        details = pod.inspect()
        self.assertEqual(ident, details.id)
        self.assertEqual('pod0', details.config['name'])
        self.assertIsNone(details.containers)

    def test_030_ident_no_ctnrs(self):
        global ident, pod

        actual = pod.kill()
        self.assertEqual(pod, actual)

        actual = pod.pause()
        self.assertEqual(pod, actual)

        actual = pod.unpause()
        self.assertEqual(pod, actual)

        actual = pod.stop()
        self.assertEqual(pod, actual)

    def test_045_raises_no_ctnrs(self):
        global ident, pod

        with self.assertRaises(podman.NoContainersInPod):
            pod.start()

        with self.assertRaises(podman.NoContainersInPod):
            pod.restart()

        with self.assertRaises(podman.NoContainerRunning):
            next(pod.stats())

        with self.assertRaises(varlink.error.MethodNotImplemented):
            pod.top()

        with self.assertRaises(varlink.error.MethodNotImplemented):
            pod.wait()

    def test_999_remove(self):
        global ident, pod

        actual = pod.remove()
        self.assertEqual(ident, actual)

        with self.assertRaises(StopIteration):
            next(self.pclient.pods.list())
