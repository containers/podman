"""Remote client command for reporting on Podman service."""
import json
import sys

import podman
import yaml
from pypodman.lib import AbstractActionBase


class Info(AbstractActionBase):
    """Class for reporting on Podman Service."""

    @classmethod
    def subparser(cls, parent):
        """Add Info command to parent parser."""
        parser = parent.add_parser(
            'info', help='report info on podman service')
        parser.add_argument(
            '--format',
            choices=('json', 'yaml'),
            help="Alter the output for a format like 'json' or 'yaml'."
            " (default: yaml)")
        parser.set_defaults(class_=cls, method='info')

    def info(self):
        """Report on Podman Service."""
        try:
            info = self.client.system.info()
        except podman.ErrorOccurred as e:
            sys.stdout.flush()
            print(
                '{}'.format(e.reason).capitalize(),
                file=sys.stderr,
                flush=True)
            return 1
        else:
            if self._args.format == 'json':
                print(json.dumps(info._asdict(), indent=2), flush=True)
            else:
                print(
                    yaml.dump(
                        dict(info._asdict()),
                        canonical=False,
                        default_flow_style=False),
                    flush=True)
