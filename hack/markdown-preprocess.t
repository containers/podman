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

pp = mp.Preprocessor()

class TestPodReplacer(unittest.TestCase):
    def check_4_way(self, containerstring: str, podstring: str):
        types = ['container', 'pod']
        strings = [ containerstring, podstring ]
        for i in 0, 1:
            pp.pod_or_container = types[i]
            for j in 0, 1:
                s = '<<' + strings[j] + '|' + strings[(j+1)%2] + '>>'
                self.assertEqual(pp.replace_type(s), strings[i])

    def test_basic(self):
        """basic pod|container and vice-versa"""
        self.check_4_way('container', 'pod')

    def test_case_insensitive(self):
        """test case-insensitive replacement of Pod, Container"""
        self.check_4_way('Container', 'Pod')

    def test_dont_care_about_podman(self):
        """we ignore 'podman'"""
        self.check_4_way('podman container', 'pod in podman')

    def test_not_at_beginning(self):
        """oops - test for 'pod' other than at beginning of string"""
        self.check_4_way('container', 'container or pod')

    def test_blank(self):
        """test that either side of '|' can be empty"""
        s_lblank = 'abc container<<| or pod>> def'
        s_rblank = 'abc container<< or pod|>> def'

        pp.pod_or_container = 'container'
        self.assertEqual(pp.replace_type(s_lblank), 'abc container def')
        self.assertEqual(pp.replace_type(s_rblank), 'abc container def')

        pp.pod_or_container = 'pod'
        self.assertEqual(pp.replace_type(s_lblank), 'abc container or pod def')
        self.assertEqual(pp.replace_type(s_rblank), 'abc container or pod def')

    def test_exception_both(self):
        """test that 'pod' on both sides raises exception"""
        for word in ['pod', 'container']:
            pp.pod_or_container = word
            with self.assertRaisesRegex(Exception, "in both left and right sides"):
                pp.replace_type('<<pod 123|pod 321>>')

    def test_exception_neither(self):
        """test that 'pod' on neither side raises exception"""
        for word in ['pod', 'container']:
            pp.pod_or_container = word
            with self.assertRaisesRegex(Exception, "in either side"):
                pp.replace_type('<<container 123|container 321>>')

class TestPodmanSubcommand(unittest.TestCase):
    def test_basic(self):
        """podman subcommand basic test"""
        pp.infile = 'podman-foo.1.md.in'
        self.assertEqual(pp.podman_subcommand(), "foo")

        pp.infile = 'podman-foo-bar.1.md.in'
        self.assertEqual(pp.podman_subcommand(), "foo bar")

        pp.infile = 'podman-pod-rm.1.md.in'
        self.assertEqual(pp.podman_subcommand(), "rm")
        self.assertEqual(pp.podman_subcommand("full"), "pod rm")

if __name__ == '__main__':
    unittest.main()
