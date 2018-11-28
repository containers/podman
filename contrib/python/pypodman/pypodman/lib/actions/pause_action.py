"""Remote client command for pausing processes in containers."""
import sys

import podman
from pypodman.lib import AbstractActionBase


class Pause(AbstractActionBase):
    """Class for pausing processes in container."""

    @classmethod
    def subparser(cls, parent):
        """Add Pause command to parent parser."""
        parser = parent.add_parser('pause', help='pause container processes')
        parser.add_argument(
            'containers',
            nargs='+',
            help='containers to pause',
        )
        parser.set_defaults(class_=cls, method='pause')

    def pause(self):
        """Pause provided containers."""
        try:
            for ident in self._args.containers:
                try:
                    ctnr = self.client.containers.get(ident)
                    ctnr.pause()
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
