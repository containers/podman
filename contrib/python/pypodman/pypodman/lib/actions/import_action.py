"""Remote client command to import tarball as image filesystem."""
import sys

import podman
from pypodman.lib import AbstractActionBase, ChangeAction


class Import(AbstractActionBase):
    """Class for importing tarball as image filesystem."""

    @classmethod
    def subparser(cls, parent):
        """Add Import command to parent parser."""
        parser = parent.add_parser(
            'import',
            help='import tarball as image filesystem',
        )
        parser.add_argument(
            '--change',
            '-c',
            action=ChangeAction,
        )
        parser.add_argument(
            '--message',
            '-m',
            help='Set commit message for imported image.',
        )
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

    def import_(self):
        """Import tarball as image filesystem."""
        # ImportImage() validates it's parameters therefore we need to create
        # pristine dict() for keywords
        options = {}
        if 'message' in self.opts:
            options['message'] = self.opts['message']
        if 'change' in self.opts and self.opts['change']:
            options['changes'] = self.opts['change']

        reference = self.opts['reference'][0] if 'reference' in self.opts\
            else None

        try:
            ident = self.client.images.import_image(
                self.opts['source'][0],
                reference,
                **options,
            )
            print(ident)
        except podman.ErrorOccurred as e:
            sys.stdout.flush()
            print(
                '{}'.format(e.reason).capitalize(),
                file=sys.stderr,
                flush=True)
            return 1
        return 0
