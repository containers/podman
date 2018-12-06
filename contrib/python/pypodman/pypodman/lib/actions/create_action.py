"""Remote client command for creating container from image."""
import sys

import podman
from pypodman.lib import AbstractActionBase

from ._create_args import CreateArguments


class Create(AbstractActionBase):
    """Class for creating container from image."""

    @classmethod
    def subparser(cls, parent):
        """Add Create command to parent parser."""
        parser = parent.add_parser(
            'create', help='create container from image')

        CreateArguments.add_arguments(parser)

        parser.add_argument('image', nargs=1, help='source image id')
        parser.add_argument(
            'command',
            nargs=parent.REMAINDER,
            help='command and args to run.',
        )
        parser.set_defaults(class_=cls, method='create')

    def __init__(self, args):
        """Construct Create class."""
        super().__init__(args)

        # image id used only on client
        del self.opts['image']

    def create(self):
        """Create container."""
        try:
            for ident in self._args.image:
                try:
                    img = self.client.images.get(ident)
                    img.container(**self.opts)
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
