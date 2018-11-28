"""Remote podman client support library."""
import sys

import podman
from pypodman.lib.action_base import AbstractActionBase
from pypodman.lib.parser_actions import (ChangeAction, PathAction,
                                         PositiveIntAction, SignalAction,
                                         UnitAction)
from pypodman.lib.podman_parser import PodmanArgumentParser
from pypodman.lib.report import Report, ReportColumn

# Silence pylint overlording...
assert ChangeAction
assert PathAction
assert PositiveIntAction
assert SignalAction
assert UnitAction

__all__ = [
    'AbstractActionBase',
    'PodmanArgumentParser',
    'Report',
    'ReportColumn',
]


def query_model(model, identifiers=None):
    """Retrieve all (default) or given model(s)."""
    objs = []
    if identifiers is None:
        objs.extend(model.list())
    else:
        try:
            for ident in identifiers:
                objs.append(model.get(ident))
        except (
                podman.PodNotFound,
                podman.ImageNotFound,
                podman.ContainerNotFound,
        ) as ex:
            print(
                '"{}" not found'.format(ex.name), file=sys.stderr, flush=True)
    return objs
