"""Remote client command to import tarball as image filesystem."""
import sys

import podman
from pypodman.lib import AbstractActionBase


class Import(AbstractActionBase):
    """Class for importing tarball as image filesystem."""

    @classmethod
    def subparser(cls, parent):
        """Add Import command to parent parser."""
        parser = parent.add_parser(
            'import', help='import tarball as image filesystem')
        parser.add_argument(
            '--change',
            '-c',
            action='append',
            choices=('CMD', 'ENTRYPOINT', 'ENV', 'EXPOSE', 'LABEL',
                     'STOPSIGNAL', 'USER', 'VOLUME', 'WORKDIR'),
            type=str.upper,
            help='Apply the following possible instructions',
        )
        parser.add_argument(
            '--message', '-m', help='Set commit message for imported image.')
        parser.add_argument(
            'source',
            metavar='PATH',
            nargs=1,
            help='tarball to use as source on remote system',
        )
        parser.add_argument(
            'reference',
            metavar='TAG',
            nargs='*',
            help='Optional tag for image. (default: None)',
        )
        parser.set_defaults(class_=cls, method='import_')

    def __init__(self, args):
        """Construct Import class."""
        super().__init__(args)

    def import_(self):
        """Import tarball as image filesystem."""
        try:
            ident = self.client.images.import_image(
                self.opts.source,
                self.opts.reference,
                message=self.opts.message,
                changes=self.opts.change)
            print(ident)
        except podman.ErrorOccurred as e:
            sys.stdout.flush()
            print(
                '{}'.format(e.reason).capitalize(),
                file=sys.stderr,
                flush=True)
            return 1
