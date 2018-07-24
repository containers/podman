"""Remote client command for deleting containers."""
import sys

import podman

from pypodman.lib import AbstractActionBase


class Rm(AbstractActionBase):
    """Class for removing containers from storage."""

    @classmethod
    def subparser(cls, parent):
        """Add Rm command to parent parser."""
        parser = parent.add_parser('rm', help='delete container(s)')
        parser.add_argument(
            '-f',
            '--force',
            action='store_true',
            help=('force delete of running container(s).'
                  ' (default: %(default)s)'))
        parser.add_argument(
            'targets', nargs='*', help='container id(s) to delete')
        parser.set_defaults(class_=cls, method='remove')

    def __init__(self, args):
        """Construct Rm class."""
        super().__init__(args)
        if not args.targets:
            raise ValueError('You must supply at least one container id'
                             ' or name to be deleted.')

    def remove(self):
        """Remove container(s)."""
        for id_ in self._args.targets:
            try:
                ctnr = self.client.containers.get(id_)
                ctnr.remove(self._args.force)
                print(id_)
            except podman.ContainerNotFound as e:
                sys.stdout.flush()
                print(
                    'Container {} not found.'.format(e.name),
                    file=sys.stderr,
                    flush=True)
            except podman.ErrorOccurred as e:
                sys.stdout.flush()
                print(
                    '{}'.format(e.reason).capitalize(),
                    file=sys.stderr,
                    flush=True)
