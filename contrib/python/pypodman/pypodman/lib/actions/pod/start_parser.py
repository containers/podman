"""Remote client command for starting pod and container(s)."""

import sys

import podman
from pypodman.lib import AbstractActionBase
from pypodman.lib import query_model as query_pods


class StartPod(AbstractActionBase):
    """Class for starting pod and container(s)."""

    @classmethod
    def subparser(cls, parent):
        """Add Pod Start command to parent parser."""
        parser = parent.add_parser('start', help='start pod')
        parser.add_flag(
            '--all',
            '-a',
            help='Start all pods.')
        parser.add_argument(
            'pod', nargs='*', help='Pod to start. Or, use --all')
        parser.set_defaults(class_=cls, method='start')

    def __init__(self, args):
        """Construct StartPod object."""
        if args.all and args.pod:
            raise ValueError('You may give a pod or use --all, but not both')
        super().__init__(args)

    def start(self):
        """Start pod and container(s)."""
        idents = None if self._args.all else self._args.pod
        pods = query_pods(self.client.pods, idents)

        for pod in pods:
            try:
                pod.start()
            except podman.ErrorOccurred as ex:
                print(
                    '{}'.format(ex.reason).capitalize(),
                    file=sys.stderr,
                    flush=True)
                return 1
        return 0
