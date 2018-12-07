"""Remote client command for retrieving ports from containers."""
import sys

import podman
from pypodman.lib import AbstractActionBase


class Port(AbstractActionBase):
    """Class for retrieving ports from container."""

    @classmethod
    def subparser(cls, parent):
        """Add Port command to parent parser."""
        parser = parent.add_parser(
            'port', help='retrieve ports from containers')
        parser.add_flag(
            '--all',
            '-a',
            help='List all known port mappings for running containers')
        parser.add_argument(
            'containers',
            nargs='*',
            help='containers to list ports',
        )
        parser.set_defaults(class_=cls, method='port')

    def __init__(self, args):
        """Construct Port class."""
        if not args.all and not args.containers:
            raise ValueError('You must supply at least one'
                             ' container id or name, or --all.')
        super().__init__(args)

    def port(self):
        """Retrieve ports from containers."""
        try:
            ctnrs = []
            if self._args.all:
                ctnrs = self.client.containers.list()
            else:
                for ident in self._args.containers:
                    try:
                        ctnrs.append(self.client.containers.get(ident))
                    except podman.ContainerNotFound as e:
                        sys.stdout.flush()
                        print(
                            'Container "{}" not found'.format(e.name),
                            file=sys.stderr,
                            flush=True)

            for ctnr in ctnrs:
                print("{}\n{}".format(ctnr.id, ctnr.ports))

        except podman.ErrorOccurred as e:
            sys.stdout.flush()
            print(
                '{}'.format(e.reason).capitalize(),
                file=sys.stderr,
                flush=True)
            return 1
        return 0
