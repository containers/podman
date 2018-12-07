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
        parser.add_flag(
            '--force',
            '-f',
            help='force delete of running container(s).')
        parser.add_argument(
            'targets', nargs='+', help='container id(s) to delete')
        parser.set_defaults(class_=cls, method='remove')

    def remove(self):
        """Remove container(s)."""
        for ident in self._args.targets:
            try:
                ctnr = self.client.containers.get(ident)
                ctnr.remove(self._args.force)
                print(ident)
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
