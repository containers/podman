"""Remote client for reporting image history."""
import json
from collections import OrderedDict

import humanize

import podman
from pypodman.lib import AbstractActionBase, Report, ReportColumn


class History(AbstractActionBase):
    """Class for reporting Image History."""

    @classmethod
    def subparser(cls, parent):
        """Add History command to parent parser."""
        parser = parent.add_parser('history', help='report image history')
        super().subparser(parser)
        parser.add_flag(
            '--human',
            '-H',
            help='Display sizes and dates in human readable format.')
        parser.add_argument(
            '--format',
            choices=('json', 'table'),
            help="Alter the output for a format like 'json' or 'table'."
            " (default: table)")
        parser.add_argument(
            'image', nargs='+', help='image for history report')
        parser.set_defaults(class_=cls, method='history')

    def __init__(self, args):
        """Construct History class."""
        super().__init__(args)

        self.columns = OrderedDict({
            'id':
            ReportColumn('id', 'ID', 12),
            'created':
            ReportColumn('created', 'CREATED', 11),
            'createdBy':
            ReportColumn('createdBy', 'CREATED BY', 45),
            'size':
            ReportColumn('size', 'SIZE', 8),
            'comment':
            ReportColumn('comment', 'COMMENT', 0)
        })

    def history(self):
        """Report image history."""
        rows = list()
        for ident in self._args.image:
            for details in self.client.images.get(ident).history():
                fields = dict(details._asdict())

                if self._args.human:
                    fields.update({
                        'size':
                        humanize.naturalsize(details.size),
                        'created':
                        humanize.naturaldate(
                            podman.datetime_parse(details.created)),
                    })
                del fields['tags']

                rows.append(fields)

        if self._args.quiet:
            for row in rows:
                ident = row['id'][:12] if self._args.truncate else row['id']
                print(ident)
        elif self._args.format == 'json':
            print(json.dumps(rows, indent=2), flush=True)
        else:
            with Report(self.columns, heading=self._args.heading) as report:
                report.layout(
                    rows, self.columns.keys(), truncate=self._args.truncate)
                for row in rows:
                    report.row(**row)
