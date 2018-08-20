"""Remote client command for attaching to a container."""
import sys

import podman
from pypodman.lib import AbstractActionBase


class Attach(AbstractActionBase):
    """Class for attaching to a running container."""

    @classmethod
    def subparser(cls, parent):
        """Add Attach command to parent parser."""
        parser = parent.add_parser('attach', help='attach to container')
        parser.add_argument(
            '--image',
            help='image to instantiate and attach to',
        )
        parser.add_argument(
            'command',
            nargs='*',
            help='image to instantiate and attach to',
        )
        parser.set_defaults(class_=cls, method='attach')

    def __init__(self, args):
        """Construct Attach class."""
        super().__init__(args)
        if not args.image:
            raise ValueError('You must supply one image id'
                             ' or name to be attached.')

    def attach(self):
        """Attach to instantiated image."""
        args = {
            'detach': True,
            'tty': True,
        }
        if self._args.command:
            args['command'] = self._args.command

        try:
            try:
                ident = self.client.images.pull(self._args.image)
                img = self.client.images.get(ident)
            except podman.ImageNotFound as e:
                sys.stdout.flush()
                print(
                    'Image {} not found.'.format(e.name),
                    file=sys.stderr,
                    flush=True)
                return 1

            ctnr = img.create(**args)
            ctnr.attach(eot=4)

            try:
                ctnr.start()
                print()
            except (BrokenPipeError, KeyboardInterrupt):
                print('\nContainer disconnected.')
        except podman.ErrorOccurred as e:
            sys.stdout.flush()
            print(
                '{}'.format(e.reason).capitalize(),
                file=sys.stderr,
                flush=True)
            return 1
