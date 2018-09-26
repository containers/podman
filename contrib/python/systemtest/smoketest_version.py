#!/usr/bin/env python3

"""
Simple stand-alone system smoke-test against the podman version sub-command
"""

import os
import signal
import unittest
import subprocess


class TestVersion(unittest.TestCase):

    TIMEOUT_SECONDS = 5

    @staticmethod
    def fmt_popen(popen, stdout, stderr):
        """
        Format the exit code, stdout and stderr as a msg string for display
        """
        msg = '\n\tCMD: {0} \n\tEXIT: {1}\n\tSTDOUT: {2}\n\tSTDERR: {3}\n'
        return msg.format(popen.args, popen.returncode, stdout, stderr)

    def test_extra_args(self):
        """
        Verify the 'podman version' subcommand ignores extranious positional arguments
        """
        cmd = 'sudo podman version with extra args - --'
        was_killed = False
        popen = subprocess.Popen(cmd, shell=True, stdin=subprocess.PIPE,
                                 stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                 close_fds=True, preexec_fn=os.setsid)

        try:
            (stdout, stderr) = popen.communicate(timeout=self.TIMEOUT_SECONDS)
        except subprocess.TimeoutExpired:
            was_killed = True
            try: # Process group leader PID == SID
                os.killpg(popen.pid, signal.SIGTERM)
            finally:  # don't block on open pipes
                (stdout, stderr) = popen.communicate()

        msg = self.fmt_popen(popen, stdout, stderr)
        self.assertFalse(was_killed, msg=msg)
        self.assertEqual(popen.returncode, 0, msg=msg)


if __name__ == '__main__':
    unittest.main()
