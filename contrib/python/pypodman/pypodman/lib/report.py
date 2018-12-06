"""Report Manager."""
import string
import sys
from collections import namedtuple


class ReportFormatter(string.Formatter):
    """Custom formatter to default missing keys to '<none>'."""

    def get_value(self, key, args, kwargs):
        """Map missing key to value '<none>'."""
        try:
            if isinstance(key, int):
                return args[key]
            else:
                return kwargs[key]
        except KeyError:
            return '<none>'


class ReportColumn(namedtuple('ReportColumn', 'key display width default')):
    """Hold attributes of output column."""

    __slots__ = ()

    def __new__(cls, key, display, width, default=None):
        """Add defaults for attributes."""
        return super(ReportColumn, cls).__new__(cls, key, display, width,
                                                default)


class Report():
    """Report Manager."""

    def __init__(self, columns, heading=True, epilog=None, file=sys.stdout):
        """Construct Report.

        columns is a mapping for named fields to column headings.
        headers True prints headers on table.
        epilog will be printed when the report context is closed.
        """
        self._columns = columns
        self._file = file
        self._format_string = None
        self._formatter = ReportFormatter()
        self._heading = heading
        self.epilog = epilog

    def row(self, **fields):
        """Print row for report."""
        if self._heading:
            hdrs = {k: v.display for (k, v) in self._columns.items()}
            print(
                self._formatter.format(self._format_string, **hdrs),
                flush=True,
                file=self._file,
            )
            self._heading = False

        fields = {k: str(v) for k, v in fields.items()}
        print(self._formatter.format(self._format_string, **fields))

    def __enter__(self):
        """Return `self` upon entering the runtime context."""
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        """Leave Report context and print epilog if provided."""
        if self.epilog:
            print(self.epilog, flush=True, file=self._file)

    def layout(self, iterable, keys, truncate=True):
        """Use data and headings build format for table to fit."""
        fmt = []

        for key in keys:
            slice_ = [str(i.get(key, '')) for i in iterable]
            data_len = len(max(slice_, key=len))

            info = self._columns.get(key,
                                     ReportColumn(key, key.upper(), data_len))
            display_len = max(data_len, len(info.display))
            if truncate and info.width != 0:
                display_len = info.width

            fmt.append('{{{0}:{1}.{1}}}'.format(key, display_len))
        self._format_string = ' '.join(fmt)
