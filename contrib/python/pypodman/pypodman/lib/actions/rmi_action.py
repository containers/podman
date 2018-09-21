"""Remote client command for deleting images."""
import sys

import podman
from pypodman.lib import AbstractActionBase


class Rmi(AbstractActionBase):
    """Class for removing images from storage."""

    @classmethod
    def subparser(cls, parent):
        """Add Rmi command to parent parser."""
        parser = parent.add_parser('rmi', help='delete image(s)')
        parser.add_argument(
            '-f',
            '--force',
            action='store_true',
            help=('force delete of image(s) and associated containers.'
                  ' (default: %(default)s)'))
        parser.add_argument('targets', nargs='+', help='image id(s) to delete')
        parser.set_defaults(class_=cls, method='remove')

    def __init__(self, args):
        """Construct Rmi class."""
        super().__init__(args)

    def remove(self):
        """Remove image(s)."""
        for ident in self._args.targets:
            try:
                img = self.client.images.get(ident)
                img.remove(self._args.force)
                print(ident)
            except podman.ImageNotFound as e:
                sys.stdout.flush()
                print(
                    'Image {} not found.'.format(e.name),
                    file=sys.stderr,
                    flush=True)
            except podman.ErrorOccurred as e:
                sys.stdout.flush()
                print(
                    '{}'.format(e.reason).capitalize(),
                    file=sys.stderr,
                    flush=True)
