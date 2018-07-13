"""Remote client commands dealing with containers."""
import operator
from collections import OrderedDict

import humanize
import podman

from .. import AbstractActionBase, Report, ReportColumn


class Ps(AbstractActionBase):
    """Class for Container manipulation."""

    @classmethod
    def subparser(cls, parent):
        """Add Images command to parent parser."""
        parser = parent.add_parser('ps', help='list containers')
        super().subparser(parser)
        parser.add_argument(
            '--sort',
            choices=[
                'createdat', 'id', 'image', 'names', 'runningfor', 'size',
                'status'
            ],
            default='createdat',
            type=str.lower,
            help=('Change sort ordered of displayed containers.'
                  ' (default: %(default)s)'))
        parser.set_defaults(class_=cls, method='list')

    def __init__(self, args):
        """Construct Ps class."""
        super().__init__(args)

        self.columns = OrderedDict({
            'id':
            ReportColumn('id', 'CONTAINER ID', 14),
            'image':
            ReportColumn('image', 'IMAGE', 30),
            'command':
            ReportColumn('column', 'COMMAND', 20),
            'createdat':
            ReportColumn('createdat', 'CREATED', 12),
            'status':
            ReportColumn('status', 'STATUS', 10),
            'ports':
            ReportColumn('ports', 'PORTS', 28),
            'names':
            ReportColumn('names', 'NAMES', 18)
        })

    def list(self):
        """List containers."""
        # TODO: Verify sorting on dates and size
        ctnrs = sorted(
            self.client.containers.list(),
            key=operator.attrgetter(self._args.sort))
        if len(ctnrs) == 0:
            return 0

        rows = list()
        for ctnr in ctnrs:
            fields = dict(ctnr)
            fields.update({
                'command':
                ' '.join(ctnr.command),
                'createdat':
                humanize.naturaldate(podman.datetime_parse(ctnr.createdat)),
            })
            rows.append(fields)

        with Report(self.columns, heading=self._args.heading) as report:
            report.layout(
                rows, self.columns.keys(), truncate=self._args.truncate)
            for row in rows:
                report.row(**row)
