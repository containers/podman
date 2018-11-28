"""Remote client command for pod subcommands."""
import inspect
import logging
import sys

from pypodman.lib import AbstractActionBase

# pylint: disable=wildcard-import
# pylint: disable=unused-wildcard-import
from .pod import *


class Pod(AbstractActionBase):
    """Class for creating a pod."""

    @classmethod
    def subparser(cls, parent):
        """Add Pod Create command to parent parser."""
        pod_parser = parent.add_parser(
            'pod',
            help='pod commands.'
            ' For subcommands, see: %(prog)s pod --help')
        subparser = pod_parser.add_subparsers()

        # pull in plugin(s) code for each subcommand
        for name, obj in inspect.getmembers(
                sys.modules['pypodman.lib.actions.pod'],
                predicate=inspect.isclass):
            if hasattr(obj, 'subparser'):
                try:
                    obj.subparser(subparser)
                except NameError as e:
                    logging.critical(e)
                    logging.warning(
                        'See subparser configuration for Class "%s"', name)
                    sys.exit(3)
