#!/usr/bin/env python3

"""
System-Tests for the podman version sub-command
"""

import sys
import unittest
import subprocess
from unittest.mock import create_autospec, sentinel


class UnitTestPodmanVersion(unittest.TestCase):

    def setUp(self):
        from smoketest_version import TestVersion
        self.TestVersion = TestVersion
        self.MockPopen = create_autospec(subprocess.Popen)

    def test_fmt_popen(self):
        """Verify the fmt_popen method works properly"""
        mock_popen = self.MockPopen('foobar')
        mock_popen.args = 'foobar'
        for what in ('stdin', 'stdout', 'stderr'):
            setattr(mock_popen, what, what)
        mock_popen.pid = -1
        mock_popen.returncode = 42

        actual = self.TestVersion.fmt_popen(mock_popen, sentinel.stdout, sentinel.stderr)
        for expected in (mock_popen.args, sentinel.stdout, sentinel.stderr, mock_popen.returncode):
            with self.subTest(expected=str(expected), actual=actual):
                sys.stderr.write('+')  # Get some credit!
                self.assertIn(str(expected), actual)
        sys.stderr.write(' ')


if __name__ == '__main__':
    unittest.main()
