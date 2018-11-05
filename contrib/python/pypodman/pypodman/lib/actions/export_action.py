"""Remote client command for export container filesystem to tarball."""
import sys

import podman
from pypodman.lib import AbstractActionBase


class Export(AbstractActionBase):
    """Class for exporting container filesystem to tarball."""

    @classmethod
    def subparser(cls, parent):
        """Add Export command to parent parser."""
        parser = parent.add_parser(
            'export',
            help='export container to tarball',
        )
        parser.add_argument(
            '--output',
            '-o',
            metavar='PATH',
            nargs=1,
            required=True,
            help='Write to this file on host',
        )
        parser.add_argument(
            'container',
            nargs=1,
            help='container to use as source',
        )
        parser.set_defaults(class_=cls, method='export')

    def export(self):
        """Create tarball from container filesystem."""
        try:
            try:
                ctnr = self.client.containers.get(self._args.container[0])
            except podman.ContainerNotFound as e:
                sys.stdout.flush()
                print(
                    'Container {} not found.'.format(e.name),
                    file=sys.stderr,
                    flush=True)
                return 1
            else:
                ctnr.export(self._args.output[0])
        except podman.ErrorOccurred as e:
            sys.stdout.flush()
            print(
                '{}'.format(e.reason).capitalize(),
                file=sys.stderr,
                flush=True)
            return 1
        return 0
