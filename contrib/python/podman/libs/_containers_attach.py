"""Exported method Container.attach()."""

import fcntl
import os
import select
import signal
import socket
import struct
import sys
import termios
import tty

CONMON_BUFSZ = 8192


class Mixin:
    """Publish attach() for inclusion in Container class."""

    def attach(self, eot=4, stdin=None, stdout=None):
        """Attach to container's PID1 stdin and stdout.

        stderr is ignored.
        """
        if stdin is None:
            stdin = sys.stdin.fileno()

        if stdout is None:
            stdout = sys.stdout.fileno()

        with self._client() as podman:
            attach = podman.GetAttachSockets(self._id)

        # This is the UDS where all the IO goes
        io_socket = attach['sockets']['io_socket']
        assert len(io_socket) <= 107,\
            'Path length for sockets too long. {} > 107'.format(
                len(io_socket)
            )

        # This is the control socket where resizing events are sent to conmon
        ctl_socket = attach['sockets']['control_socket']

        def resize_handler(signum, frame):
            """Send the new window size to conmon.

            The method arguments are not used.
            """
            packed = fcntl.ioctl(stdout, termios.TIOCGWINSZ,
                                 struct.pack('HHHH', 0, 0, 0, 0))
            rows, cols, _, _ = struct.unpack('HHHH', packed)

            with open(ctl_socket, 'w') as skt:
                # send conmon window resize message
                skt.write('1 {} {}\n'.format(rows, cols))

        def log_handler(signum, frame):
            """Send command to reopen log to conmon.

            The method arguments are not used.
            """
            with open(ctl_socket, 'w') as skt:
                # send conmon reopen log message
                skt.write('2\n')

        try:
            # save off the old settings for terminal
            original_attr = termios.tcgetattr(stdout)
            tty.setraw(stdin)

            # initialize containers window size
            resize_handler(None, sys._getframe(0))

            # catch any resizing events and send the resize info
            # to the control fifo "socket"
            signal.signal(signal.SIGWINCH, resize_handler)
        except termios.error:
            original_attr = None

        try:
            # Prepare socket for communicating with conmon/container
            with socket.socket(socket.AF_UNIX, socket.SOCK_SEQPACKET) as skt:
                skt.connect(io_socket)
                skt.sendall(b'\n')

                sources = [skt, stdin]
                while sources:
                    readable, _, _ = select.select(sources, [], [])
                    for r in readable:
                        if r is skt:
                            data = r.recv(CONMON_BUFSZ)
                            if not data:
                                sources.remove(skt)

                            # Remove source marker when writing
                            os.write(stdout, data[1:])
                        elif r is stdin:
                            data = os.read(stdin, CONMON_BUFSZ)
                            if not data:
                                sources.remove(stdin)

                            skt.sendall(data)

                            if eot in data:
                                sources.clear()
                                break
                        else:
                            raise ValueError('Unknown source in select')
        finally:
            if original_attr:
                termios.tcsetattr(stdout, termios.TCSADRAIN, original_attr)
                signal.signal(signal.SIGWINCH, signal.SIG_DFL)
