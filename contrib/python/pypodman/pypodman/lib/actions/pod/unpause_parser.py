"""Remote client command for unpausing processes in pod."""
import sys

import podman
from pypodman.lib import AbstractActionBase
from pypodman.lib import query_model as query_pods


class UnpausePod(AbstractActionBase):
    """Class for unpausing containers in pod."""

    @classmethod
    def subparser(cls, parent):
        """Add Pod Unpause command to parent parser."""
        parser = parent.add_parser('unpause', help='unpause pod')
        parser.add_flag(
            '--all',
            '-a',
            help='Unpause all pods.')
        parser.add_argument(
            'pod', nargs='*', help='Pod to unpause. Or, use --all')
        parser.set_defaults(class_=cls, method='unpause')

    def __init__(self, args):
        """Construct Pod Unpause class."""
        if args.all and args.pod:
            raise ValueError('You may give a pod or use --all, but not both')
        super().__init__(args)

    def unpause(self):
        """Unpause containers in provided Pod."""
        idents = None if self._args.all else self._args.pod
        pods = query_pods(self.client.pods, idents)

        for pod in pods:
            try:
                pod.unpause()
                print(pod.id)
            except podman.PodNotFound as ex:
                print(
                    'Pod "{}" not found'.format(ex.name),
                    file=sys.stderr,
                    flush=True)
            except podman.ErrorOccurred as ex:
                print(
                    '{}'.format(ex.reason).capitalize(),
                    file=sys.stderr,
                    flush=True)
                return 1
        return 0
