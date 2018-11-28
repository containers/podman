"""Report on pod's containers' processes."""
import operator
from collections import OrderedDict

from pypodman.lib import AbstractActionBase, Report, ReportColumn


class ProcessesPod(AbstractActionBase):
    """Report on Pod's processes."""

    @classmethod
    def subparser(cls, parent):
        """Add Pod Ps command to parent parser."""
        parser = parent.add_parser('ps', help='list processes of pod')
        super().subparser(parser)

        parser.add_flag(
            '--ctr-names',
            help='Include container name in the info field.')
        parser.add_flag(
            '--ctr-ids',
            help='Include container ID in the info field.')
        parser.add_flag(
            '--ctr-status',
            help='Include container status in the info field.')
        parser.add_argument(
            '--format',
            choices=('json'),
            help='Pretty-print containers to JSON')
        parser.add_argument(
            '--sort',
            choices=('created', 'id', 'name', 'status', 'count'),
            default='created',
            type=str.lower,
            help='Sort on given field. (default: %(default)s)')
        parser.add_argument('--filter', help='Not Implemented')
        parser.set_defaults(class_=cls, method='processes')

    def __init__(self, args):
        """Construct ProcessesPod class."""
        if args.sort == 'created':
            args.sort = 'createdat'
        elif args.sort == 'count':
            args.sort = 'numberofcontainers'

        super().__init__(args)

        self.columns = OrderedDict({
            'id':
            ReportColumn('id', 'POD ID', 14),
            'name':
            ReportColumn('name', 'NAME', 30),
            'status':
            ReportColumn('status', 'STATUS', 8),
            'numberofcontainers':
            ReportColumn('numberofcontainers', 'NUMBER OF CONTAINERS', 0),
            'info':
            ReportColumn('info', 'CONTAINER INFO', 0),
        })

    def processes(self):
        """List pods."""
        pods = sorted(
            self.client.pods.list(), key=operator.attrgetter(self._args.sort))
        if not pods:
            return

        rows = list()
        for pod in pods:
            fields = dict(pod)
            if self._args.ctr_ids \
                    or self._args.ctr_names \
                    or self._args.ctr_status:
                keys = ('id', 'name', 'status', 'info')
                info = []
                for ctnr in pod.containersinfo:
                    ctnr_info = []
                    if self._args.ctr_ids:
                        ctnr_info.append(ctnr['id'])
                    if self._args.ctr_names:
                        ctnr_info.append(ctnr['name'])
                    if self._args.ctr_status:
                        ctnr_info.append(ctnr['status'])
                    info.append("[ {} ]".format(" ".join(ctnr_info)))
                fields.update({'info': " ".join(info)})
            else:
                keys = ('id', 'name', 'status', 'numberofcontainers')

            rows.append(fields)

        with Report(self.columns, heading=self._args.heading) as report:
            report.layout(rows, keys, truncate=self._args.truncate)
            for row in rows:
                report.row(**row)
