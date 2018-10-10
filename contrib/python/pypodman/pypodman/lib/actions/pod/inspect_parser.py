"""Remote client command for inspecting pods."""
import json
import sys

import podman
from pypodman.lib import AbstractActionBase


class InspectPod(AbstractActionBase):
    """Class for reporting on pods and their containers."""

    @classmethod
    def subparser(cls, parent):
        """Add Pod Inspect command to parent parser."""
        parser = parent.add_parser(
            'inspect',
            help='configuration and state information about a given pod')
        parser.add_argument('pod', nargs='+', help='pod(s) to inspect')
        parser.set_defaults(class_=cls, method='inspect')

    def inspect(self):
        """Report on provided pods."""
        output = {}
        try:
            for ident in self._args.pod:
                try:
                    pod = self.client.pods.get(ident)
                except podman.PodNotFound:
                    print(
                        'Pod "{}" not found.'.format(ident),
                        file=sys.stdout,
                        flush=True)
                output.update(pod.inspect()._asdict())
        except podman.ErrorOccurred as e:
            sys.stdout.flush()
            print(
                '{}'.format(e.reason).capitalize(),
                file=sys.stderr,
                flush=True)
            return 1
        else:
            print(json.dumps(output, indent=2))
        return 0
