"""Remote client command for retrieving mounts from containers."""
import sys
from collections import OrderedDict

import podman
from pypodman.lib import AbstractActionBase, Report, ReportColumn


class Mount(AbstractActionBase):
    """Class for retrieving mounts from container."""

    @classmethod
    def subparser(cls, parent):
        """Add mount command to parent parser."""
        parser = parent.add_parser(
            'mount', help='retrieve mounts from containers.')
        super().subparser(parser)
        parser.add_argument(
            'containers',
            nargs='*',
            help='containers to list ports',
        )
        parser.set_defaults(class_=cls, method='mount')

    def __init__(self, args):
        """Construct Mount class."""
        super().__init__(args)

        self.columns = OrderedDict({
            'id':
            ReportColumn('id', 'CONTAINER ID', 14),
            'destination':
            ReportColumn('destination', 'DESTINATION', 0)
        })

    def mount(self):
        """Retrieve mounts from containers."""
        try:
            ctnrs = []
            if not self._args.containers:
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

        except podman.ErrorOccurred as e:
            sys.stdout.flush()
            print(
                '{}'.format(e.reason).capitalize(),
                file=sys.stderr,
                flush=True)
            return 1

        if not ctnrs:
            print(
                'Unable to find any containers.', file=sys.stderr, flush=True)
            return 1

        rows = list()
        for ctnr in ctnrs:
            details = ctnr.inspect()
            rows.append({
                'id': ctnr.id,
                'destination': details.graphdriver['data']['mergeddir']
            })

        with Report(self.columns, heading=self._args.heading) as report:
            report.layout(
                rows, self.columns.keys(), truncate=self._args.truncate)
            for row in rows:
                report.row(**row)
