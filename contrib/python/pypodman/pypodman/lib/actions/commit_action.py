"""Remote client command for creating image from container."""
import sys

import podman
from pypodman.lib import AbstractActionBase, BooleanAction, ChangeAction


class Commit(AbstractActionBase):
    """Class for creating image from container."""

    @classmethod
    def subparser(cls, parent):
        """Add Commit command to parent parser."""
        parser = parent.add_parser(
            'commit',
            help='create image from container',
        )
        parser.add_argument(
            '--author',
            help='Set the author for the committed image',
        )
        parser.add_argument(
            '--change',
            '-c',
            action=ChangeAction,
        )
        parser.add_argument(
            '--format',
            '-f',
            choices=('oci', 'docker'),
            default='oci',
            type=str.lower,
            help='Set the format of the image manifest and metadata',
        )
        parser.add_argument(
            '--iidfile',
            metavar='PATH',
            help='Write the image ID to the file',
        )
        parser.add_argument(
            '--message',
            '-m',
            help='Set commit message for committed image',
        )
        parser.add_argument(
            '--pause',
            '-p',
            action=BooleanAction,
            default=True,
            help='Pause the container when creating an image',
        )
        parser.add_argument(
            '--quiet',
            '-q',
            action='store_true',
            help='Suppress output',
        )
        parser.add_argument(
            'container',
            nargs=1,
            help='container to use as source',
        )
        parser.add_argument(
            'image',
            nargs=1,
            help='image name to create',
        )
        parser.set_defaults(class_=cls, method='commit')

    def commit(self):
        """Create image from container."""
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
                ident = ctnr.commit(self.opts['image'][0], **self.opts)
                print(ident)
        except podman.ErrorOccurred as e:
            sys.stdout.flush()
            print(
                '{}'.format(e.reason).capitalize(),
                file=sys.stderr,
                flush=True)
            return 1
        return 0
