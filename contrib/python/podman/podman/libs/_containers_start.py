"""Exported method Container.start()."""
import logging
import os
import select
import signal
import socket
import sys
import termios
import tty

CONMON_BUFSZ = 8192


class Mixin:
    """Publish start() for inclusion in Container class."""

    def start(self):
        """Start container, return container on success.

        Will block if container has been detached.
        """
        with self._client() as podman:
            logging.debug('Starting Container "%s"', self._id)
            results = podman.StartContainer(self._id)
            logging.debug('Started Container "%s"', results['container'])

            if not hasattr(self, 'pseudo_tty') or self.pseudo_tty is None:
                return self._refresh(podman)

            logging.debug('Setting up PseudoTTY for Container "%s"',
                          results['container'])

            try:
                # save off the old settings for terminal
                tcoldattr = termios.tcgetattr(self.pseudo_tty.stdin)
                tty.setraw(self.pseudo_tty.stdin)

                # initialize container's window size
                self.resize_handler(None, sys._getframe(0))

                # catch any resizing events and send the resize info
                # to the control fifo "socket"
                signal.signal(signal.SIGWINCH, self.resize_handler)

            except termios.error:
                tcoldattr = None

            try:
                # TODO: Is socket.SOCK_SEQPACKET supported in Windows?
                with socket.socket(socket.AF_UNIX,
                                   socket.SOCK_SEQPACKET) as skt:
                    # Prepare socket for use with conmon/container
                    skt.connect(self.pseudo_tty.io_socket)

                    sources = [skt, self.pseudo_tty.stdin]
                    while sources:
                        logging.debug('Waiting on sources: %s', sources)
                        readable, _, _ = select.select(sources, [], [])

                        if skt in readable:
                            data = skt.recv(CONMON_BUFSZ)
                            if data:
                                # Remove source marker when writing
                                os.write(self.pseudo_tty.stdout, data[1:])
                            else:
                                sources.remove(skt)

                        if self.pseudo_tty.stdin in readable:
                            data = os.read(self.pseudo_tty.stdin, CONMON_BUFSZ)
                            if data:
                                skt.sendall(data)

                                if self.pseudo_tty.eot in data:
                                    sources.clear()
                            else:
                                sources.remove(self.pseudo_tty.stdin)
            finally:
                if tcoldattr:
                    termios.tcsetattr(self.pseudo_tty.stdin, termios.TCSADRAIN,
                                      tcoldattr)
                    signal.signal(signal.SIGWINCH, signal.SIG_DFL)
            return self._refresh(podman)
