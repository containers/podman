"""Remote client command for signaling podman containers."""
import sys

import podman
from pypodman.lib import AbstractActionBase, SignalAction


class Kill(AbstractActionBase):
    """Class for sending signal to main process in container."""

    @classmethod
    def subparser(cls, parent):
        """Add Kill command to parent parser."""
        parser = parent.add_parser('kill', help='signal container')
        parser.add_argument(
            '--signal',
            '-s',
            action=SignalAction,
            default=9,
            help='Signal to send to the container. (default: %(default)s)')
        parser.add_argument(
            'containers',
            nargs='+',
            help='containers to signal',
        )
        parser.set_defaults(class_=cls, method='kill')

    def kill(self):
        """Signal provided containers."""
        try:
            for ident in self._args.containers:
                try:
                    ctnr = self.client.containers.get(ident)
                    ctnr.kill(self._args.signal)
                except podman.ContainerNotFound as e:
                    sys.stdout.flush()
                    print(
                        'Container "{}" not found'.format(e.name),
                        file=sys.stderr,
                        flush=True)
                else:
                    print(ident)
        except podman.ErrorOccurred as e:
            sys.stdout.flush()
            print(
                '{}'.format(e.reason).capitalize(),
                file=sys.stderr,
                flush=True)
            return 1
