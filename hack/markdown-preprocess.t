#!/usr/bin/env python3

"""
Tests for markdown-preprocess
"""

import unittest

# https://stackoverflow.com/questions/66665217/how-to-import-a-python-script-without-a-py-extension
from importlib.util import spec_from_loader, module_from_spec
from importlib.machinery import SourceFileLoader

spec = spec_from_loader("mp", SourceFileLoader("mp", "hack/markdown-preprocess"))
mp = module_from_spec(spec)
spec.loader.exec_module(mp)

class TestPodReplacer(unittest.TestCase):
    def test_basic(self):
        """basic pod|container and vice-versa"""
        s = '<<container|pod>>'
        self.assertEqual(mp.replace_type(s, 'pod'), 'pod')
        self.assertEqual(mp.replace_type(s, 'container'), 'container')
        s = '<<container|pod>>'
        self.assertEqual(mp.replace_type(s, 'pod'), 'pod')
        self.assertEqual(mp.replace_type(s, 'container'), 'container')

    def test_case_insensitive(self):
        """test case-insensitive replacement of Pod, Container"""
        s = '<<Pod|Container>>'
        self.assertEqual(mp.replace_type(s, 'pod'), 'Pod')
        self.assertEqual(mp.replace_type(s, 'container'), 'Container')
        s = '<<Container|Pod>>'
        self.assertEqual(mp.replace_type(s, 'pod'), 'Pod')
        self.assertEqual(mp.replace_type(s, 'container'), 'Container')

    def test_dont_care_about_podman(self):
        """we ignore 'podman'"""
        self.assertEqual(mp.replace_type('<<podman container|pod in podman>>', 'container'), 'podman container')

    def test_exception_both(self):
        """test that 'pod' on both sides raises exception"""
        with self.assertRaisesRegex(Exception, "in both left and right sides"):
            mp.replace_type('<<pod 123|pod 321>>', 'pod')

    def test_exception_neither(self):
        """test that 'pod' on neither side raises exception"""
        with self.assertRaisesRegex(Exception, "in either side"):
            mp.replace_type('<<container 123|container 321>>', 'pod')

class TestPodmanSubcommand(unittest.TestCase):
    def test_basic(self):
        """podman subcommand basic test"""
        self.assertEqual(mp.podman_subcommand("podman-foo.1.md.in"), "foo")
        self.assertEqual(mp.podman_subcommand("podman-foo-bar.1.md.in"), "foo bar")


if __name__ == '__main__':
    unittest.main()
