"""Remote client command for pulling images."""
import sys

import podman
from pypodman.lib import AbstractActionBase


class Pull(AbstractActionBase):
    """Class for retrieving images from repository."""

    @classmethod
    def subparser(cls, parent):
        """Add Pull command to parent parser."""
        parser = parent.add_parser(
            'pull',
            help='retrieve image from repository',
        )
        parser.add_argument(
            'targets',
            nargs='+',
            help='image id(s) to retrieve.',
        )
        parser.set_defaults(class_=cls, method='pull')

    def __init__(self, args):
        """Construct Pull class."""
        super().__init__(args)

    def pull(self):
        """Retrieve image."""
        for ident in self._args.targets:
            try:
                self.client.images.pull(ident)
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
