"""Remote client commands dealing with images."""
import operator
from collections import OrderedDict

import humanize

import podman
from pypodman.lib import AbstractActionBase, Report, ReportColumn


class Images(AbstractActionBase):
    """Class for Image manipulation."""

    @classmethod
    def subparser(cls, parent):
        """Add Images commands to parent parser."""
        parser = parent.add_parser('images', help='list images')
        super().subparser(parser)
        parser.add_argument(
            '--sort',
            choices=['created', 'id', 'repository', 'size', 'tag'],
            default='created',
            type=str.lower,
            help=('Change sort ordered of displayed images.'
                  ' (default: %(default)s)'))

        parser.add_flag(
            '--digests',
            help='Include digests with images.')
        parser.set_defaults(class_=cls, method='list')

    def __init__(self, args):
        """Construct Images class."""
        super().__init__(args)

        self.columns = OrderedDict({
            'name':
            ReportColumn('name', 'REPOSITORY', 0),
            'tag':
            ReportColumn('tag', 'TAG', 10),
            'id':
            ReportColumn('id', 'IMAGE ID', 12),
            'created':
            ReportColumn('created', 'CREATED', 12),
            'size':
            ReportColumn('size', 'SIZE', 8),
            'repoDigests':
            ReportColumn('repoDigests', 'DIGESTS', 35),
        })

    def list(self):
        """List images."""
        images = sorted(
            self.client.images.list(),
            key=operator.attrgetter(self._args.sort))
        if not images:
            return

        rows = list()
        for image in images:
            fields = dict(image)
            fields.update({
                'created':
                humanize.naturaldate(podman.datetime_parse(image.created)),
                'size':
                humanize.naturalsize(int(image.size)),
                'repoDigests':
                ' '.join(image.repoDigests),
            })

            for r in image.repoTags:
                name, tag = r.rsplit(':', 1)
                fields.update({
                    'name': name,
                    'tag': tag,
                })
            rows.append(fields)

        if not self._args.digests:
            del self.columns['repoDigests']

        with Report(self.columns, heading=self._args.heading) as report:
            report.layout(
                rows, self.columns.keys(), truncate=self._args.truncate)
            for row in rows:
                report.row(**row)
