"""Remote client command for pushing image elsewhere."""
import sys

import podman
from pypodman.lib import AbstractActionBase


class Push(AbstractActionBase):
    """Class for pushing images to repository."""

    @classmethod
    def subparser(cls, parent):
        """Add Push command to parent parser."""
        parser = parent.add_parser(
            'push',
            help='push image elsewhere',
        )
        parser.add_flag(
            '--tlsverify',
            help='Require HTTPS and verify certificates when'
            ' contacting registries.')
        parser.add_argument(
            'image', nargs=1, help='name or id of image to push')
        parser.add_argument(
            'tag',
            nargs=1,
            help='destination image id',
        )
        parser.set_defaults(class_=cls, method='push')

    def pull(self):
        """Store image elsewhere."""
        try:
            try:
                img = self.client.images.get(self._args.image[0])
            except podman.ImageNotFound as e:
                sys.stdout.flush()
                print(
                    'Image {} not found.'.format(e.name),
                    file=sys.stderr,
                    flush=True)
            else:
                img.push(self._args.tag[0], tlsverify=self._args.tlsverify)
                print(self._args.image[0])
        except podman.ErrorOccurred as e:
            sys.stdout.flush()
            print(
                '{}'.format(e.reason).capitalize(),
                file=sys.stderr,
                flush=True)
