#!/usr/bin/env python3

"""
Test basic podman workflows
"""

import unittest
import subprocess


class TestBasicWorkflow(unittest.TestCase):

    @unittest.skip("Stub")
    def test_pull_run(self):
        """
        Verify podman subcommand sequence: pull, run, kill, rm, rmi
        """
        pass

    @unittest.skip("Stub")
    def test_create_start(self):
        """
        Verify podman subcommand sequence: create, start, stop, rm, rmi
        """
        pass

    @unittest.skip("Stub")
    def test_run_exec(self):
        """
        Verify podman subcommand sequence: run, exec, kill, rm, rmi
        """
        pass

    @unittest.skip("Stub")
    def test_build_run(self):
        """
        Verify podman subcommand sequence: build, run, stop, rm, rmi
        """
        pass


if __name__ == '__main__':
    unittest.main()
