"""Report Manager."""
import sys
from collections import namedtuple

from .future_abstract import AbstractContextManager


class ReportColumn(namedtuple('ReportColumn', 'key display width default')):
    """Hold attributes of output column."""

    __slots__ = ()

    def __new__(cls, key, display, width, default=None):
        """Add defaults for attributes."""
        return super(ReportColumn, cls).__new__(cls, key, display, width,
                                                default)


class Report(AbstractContextManager):
    """Report Manager."""

    def __init__(self, columns, heading=True, epilog=None, file=sys.stdout):
        """Construct Report.

        columns is a mapping for named fields to column headings.
        headers True prints headers on table.
        epilog will be printed when the report context is closed.
        """
        self._columns = columns
        self._file = file
        self._heading = heading
        self.epilog = epilog
        self._format = None

    def row(self, **fields):
        """Print row for report."""
        if self._heading:
            hdrs = {k: v.display for (k, v) in self._columns.items()}
            print(self._format.format(**hdrs), flush=True, file=self._file)
            self._heading = False
        fields = {k: str(v) for k, v in fields.items()}
        print(self._format.format(**fields))

    def __exit__(self, exc_type, exc_value, traceback):
        """Leave Report context and print epilog if provided."""
        if self.epilog:
            print(self.epilog, flush=True, file=self._file)

    def layout(self, iterable, keys, truncate=True):
        """Use data and headings build format for table to fit."""
        format = []

        for key in keys:
            value = max(map(lambda x: len(str(x.get(key, ''))), iterable))
            # print('key', key, 'value', value)

            if truncate:
                row = self._columns.get(
                    key, ReportColumn(key, key.upper(), len(key)))
                if value < row.width:
                    step = row.width if value == 0 else value
                    value = max(len(key), step)
                elif value > row.width:
                    value = row.width if row.width != 0 else value

            format.append('{{{0}:{1}.{1}}}'.format(key, value))
        self._format = ' '.join(format)
