"""Remote client command for searching registries for an image."""
import argparse
import sys
from collections import OrderedDict

import podman
from pypodman.lib import (AbstractActionBase, PositiveIntAction, Report,
                          ReportColumn)


class FilterAction(argparse.Action):
    """Parse filter argument components."""

    def __init__(self,
                 option_strings,
                 dest,
                 nargs=None,
                 const=None,
                 default=None,
                 type=None,
                 choices=None,
                 required=False,
                 help=None,
                 metavar='FILTER'):
        """Create FilterAction object."""
        help = (help or '') + (' (format: stars=##'
                               ' or is-automated=[True|False]'
                               ' or is-official=[True|False])')
        super().__init__(
            option_strings=option_strings,
            dest=dest,
            nargs=nargs,
            const=const,
            default=default,
            type=type,
            choices=choices,
            required=required,
            help=help,
            metavar=metavar)

    def __call__(self, parser, namespace, values, option_string=None):
        """
        Convert and Validate input.

        Note: side effects
        1) self.dest value is set to subargument dest
        2) new attribute self.dest + '_value' is created with 2nd value.
        """
        opt, val = values.split('=', 1)
        if opt == 'stars':
            msg = ('{} option "stars" requires'
                   ' a positive integer').format(self.dest)
            try:
                val = int(val)
            except ValueError:
                parser.error(msg)

            if val < 0:
                parser.error(msg)
        elif opt == 'is-automated':
            if val.capitalize() in ('True', 'False'):
                val = bool(val)
            else:
                msg = ('{} option "is-automated"'
                       ' must be True or False.'.format(self.dest))
                parser.error(msg)
        elif opt == 'is-official':
            if val.capitalize() in ('True', 'False'):
                val = bool(val)
            else:
                msg = ('{} option "is-official"'
                       ' must be True or False.'.format(self.dest))
                parser.error(msg)
        else:
            msg = ('{} only supports one of the following options:\n'
                   '  stars, is-automated, or is-official').format(self.dest)
            parser.error(msg)
        setattr(namespace, self.dest, opt)
        setattr(namespace, self.dest + '_value', val)


class Search(AbstractActionBase):
    """Class for searching registries for an image."""

    @classmethod
    def subparser(cls, parent):
        """Add Search command to parent parser."""
        parser = parent.add_parser('search', help='search for images')
        super().subparser(parser)
        parser.add_argument(
            '--filter',
            '-f',
            action=FilterAction,
            help='Filter output based on conditions provided.')
        parser.add_argument(
            '--limit',
            action=PositiveIntAction,
            default=25,
            help='Limit the number of results.'
            ' (default: %(default)s)')
        parser.add_argument('term', nargs=1, help='search term for image')
        parser.set_defaults(class_=cls, method='search')

    def __init__(self, args):
        """Construct Search class."""
        super().__init__(args)

        self.columns = OrderedDict({
            'name':
            ReportColumn('name', 'NAME', 44),
            'description':
            ReportColumn('description', 'DESCRIPTION', 44),
            'star_count':
            ReportColumn('star_count', 'STARS', 5),
            'is_official':
            ReportColumn('is_official', 'OFFICIAL', 8),
            'is_automated':
            ReportColumn('is_automated', 'AUTOMATED', 9),
        })

    def search(self):
        """Search registries for image."""
        try:
            rows = list()
            for entry in self.client.images.search(
                    self._args.term[0], limit=self._args.limit):

                if self._args.filter == 'is-official':
                    if self._args.filter_value != entry.is_official:
                        continue
                elif self._args.filter == 'is-automated':
                    if self._args.filter_value != entry.is_automated:
                        continue
                elif self._args.filter == 'stars':
                    if self._args.filter_value > entry.star_count:
                        continue

                fields = dict(entry._asdict())

                status = '[OK]' if entry.is_official else ''
                fields['is_official'] = status

                status = '[OK]' if entry.is_automated else ''
                fields['is_automated'] = status

                if self._args.truncate:
                    fields.update({'name': entry.name[-44:]})
                rows.append(fields)

            with Report(self.columns, heading=self._args.heading) as report:
                report.layout(
                    rows, self.columns.keys(), truncate=self._args.truncate)
                for row in rows:
                    report.row(**row)
        except podman.ErrorOccurred as e:
            sys.stdout.flush()
            print(
                '{}'.format(e.reason).capitalize(),
                file=sys.stderr,
                flush=True)
