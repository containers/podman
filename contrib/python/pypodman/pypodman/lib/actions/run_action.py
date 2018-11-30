"""Remote client command for run a command in a new container."""
import logging
import sys

import podman
from pypodman.lib import AbstractActionBase

from ._create_args import CreateArguments


class Run(AbstractActionBase):
    """Class for running a command in a container."""

    @classmethod
    def subparser(cls, parent):
        """Add Run command to parent parser."""
        parser = parent.add_parser('run', help='Run container from image')

        CreateArguments.add_arguments(parser)

        parser.add_argument('image', nargs=1, help='source image id.')
        parser.add_argument(
            'command',
            nargs=parent.REMAINDER,
            help='command and args to run.',
        )
        parser.set_defaults(class_=cls, method='run')

    def __init__(self, args):
        """Construct Run class."""
        super().__init__(args)
        if args.detach and args.rm:
            raise ValueError('Incompatible options: --detach and --rm')

        # image id used only on client
        del self.opts['image']

    def run(self):
        """Run container."""
        for ident in self._args.image:
            try:
                try:
                    img = self.client.images.get(ident)
                    ctnr = img.container(**self.opts)
                except podman.ImageNotFound as e:
                    sys.stdout.flush()
                    print(
                        'Image {} not found.'.format(e.name),
                        file=sys.stderr,
                        flush=True)
                    continue
                else:
                    logging.debug('New container created "{}"'.format(ctnr.id))

                if self._args.detach:
                    ctnr.start()
                    print(ctnr.id)
                else:
                    ctnr.attach(eot=4)
                    ctnr.start()
                    print(ctnr.id)

                    if self._args.rm:
                        ctnr.remove(force=True)
            except (BrokenPipeError, KeyboardInterrupt):
                print('\nContainer "{}" disconnected.'.format(ctnr.id))
            except podman.ErrorOccurred as e:
                sys.stdout.flush()
                print(
                    'Run for container "{}" failed: {} {}'.format(
                        ctnr.id, repr(e), e.reason.capitalize()),
                    file=sys.stderr,
                    flush=True)
