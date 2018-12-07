"""Remote client command for inspecting podman objects."""
import json
import logging
import sys

import podman
from pypodman.lib import AbstractActionBase


class Inspect(AbstractActionBase):
    """Class for inspecting podman objects."""

    @classmethod
    def subparser(cls, parent):
        """Add Inspect command to parent parser."""
        parser = parent.add_parser('inspect', help='inspect objects')
        parser.add_argument(
            '--type',
            '-t',
            choices=('all', 'container', 'image'),
            default='all',
            type=str.lower,
            help='Type of object to inspect',
        )
        parser.add_flag(
            '--size',
            help='Display the total file size if the type is a container.')
        parser.add_argument(
            'objects',
            nargs='+',
            help='objects to inspect',
        )
        parser.set_defaults(class_=cls, method='inspect')

    def _get_container(self, ident):
        try:
            logging.debug("Getting container %s", ident)
            ctnr = self.client.containers.get(ident)
        except podman.ContainerNotFound:
            pass
        else:
            return ctnr.inspect()

    def _get_image(self, ident):
        try:
            logging.debug("Getting image %s", ident)
            img = self.client.images.get(ident)
        except podman.ImageNotFound:
            pass
        else:
            return img.inspect()

    def inspect(self):
        """Inspect provided podman objects."""
        output = []
        try:
            for ident in self._args.objects:
                obj = None

                if self._args.type in ('all', 'container'):
                    obj = self._get_container(ident)
                if obj is None and self._args.type in ('all', 'image'):
                    obj = self._get_image(ident)

                if obj is None:
                    if self._args.type == 'container':
                        msg = 'Container "{}" not found'.format(ident)
                    elif self._args.type == 'image':
                        msg = 'Image "{}" not found'.format(ident)
                    else:
                        msg = 'Object "{}" not found'.format(ident)
                    print(msg, file=sys.stderr, flush=True)
                else:
                    fields = obj._asdict()
                    if not self._args.size:
                        try:
                            del fields['sizerootfs']
                        except KeyError:
                            pass
                    output.append(fields)
        except podman.ErrorOccurred as e:
            sys.stdout.flush()
            print(
                '{}'.format(e.reason).capitalize(),
                file=sys.stderr,
                flush=True)
            return 1
        else:
            print(json.dumps(output, indent=2))
