"""Exported method Container.attach()."""

import collections
import fcntl
import logging
import struct
import sys
import termios


class Mixin:
    """Publish attach() for inclusion in Container class."""

    def attach(self, eot=4, stdin=None, stdout=None):
        """Attach to container's PID1 stdin and stdout.

        stderr is ignored.
        PseudoTTY work is done in start().
        """
        if stdin is None:
            stdin = sys.stdin.fileno()
        elif hasattr(stdin, 'fileno'):
            stdin = stdin.fileno()

        if stdout is None:
            stdout = sys.stdout.fileno()
        elif hasattr(stdout, 'fileno'):
            stdout = stdout.fileno()

        with self._client() as podman:
            attach = podman.GetAttachSockets(self._id)

        # This is the UDS where all the IO goes
        io_socket = attach['sockets']['io_socket']
        assert len(io_socket) <= 107,\
            'Path length for sockets too long. {} > 107'.format(
                len(io_socket)
            )

        # This is the control socket where resizing events are sent to conmon
        # attach['sockets']['control_socket']
        self.pseudo_tty = collections.namedtuple(
            'PseudoTTY',
            ['stdin', 'stdout', 'io_socket', 'control_socket', 'eot'])(
                stdin,
                stdout,
                attach['sockets']['io_socket'],
                attach['sockets']['control_socket'],
                eot,
            )

    @property
    def resize_handler(self):
        """Send the new window size to conmon."""

        def wrapped(signum, frame):  # pylint: disable=unused-argument
            packed = fcntl.ioctl(self.pseudo_tty.stdout, termios.TIOCGWINSZ,
                                 struct.pack('HHHH', 0, 0, 0, 0))
            rows, cols, _, _ = struct.unpack('HHHH', packed)
            logging.debug('Resize window(%dx%d) using %s', rows, cols,
                          self.pseudo_tty.control_socket)

            # TODO: Need some kind of timeout in case pipe is blocked
            with open(self.pseudo_tty.control_socket, 'w') as skt:
                # send conmon window resize message
                skt.write('1 {} {}\n'.format(rows, cols))

        return wrapped

    @property
    def log_handler(self):
        """Send command to reopen log to conmon."""

        def wrapped(signum, frame):  # pylint: disable=unused-argument
            with open(self.pseudo_tty.control_socket, 'w') as skt:
                # send conmon reopen log message
                skt.write('2\n')

        return wrapped
