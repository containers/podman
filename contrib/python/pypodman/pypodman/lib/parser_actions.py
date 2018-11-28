"""
Supplimental argparse.Action converters and validaters.

The constructors are very verbose but remain for IDE support.
"""
import argparse
import copy
import os
import signal

# API defined by argparse.Action therefore shut up pylint
# pragma pylint: disable=redefined-builtin
# pragma pylint: disable=too-few-public-methods
# pragma pylint: disable=too-many-arguments


class ChangeAction(argparse.Action):
    """Convert and validate change argument."""

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
                 metavar='OPT=VALUE'):
        """Create ChangeAction object."""
        help = (help or '') + ('Apply change(s) to the new image.'
                               ' May be given multiple times.')
        if default is None:
            default = []

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
        """Convert and Validate input."""
        items = getattr(namespace, self.dest, None) or []
        items = copy.copy(items)

        choices = ('CMD', 'ENTRYPOINT', 'ENV', 'EXPOSE', 'LABEL', 'ONBUILD',
                   'STOPSIGNAL', 'USER', 'VOLUME', 'WORKDIR')

        opt, _ = values.split('=', 1)
        if opt not in choices:
            parser.error('Option "{}" is not supported by argument "{}",'
                         ' valid options are: {}'.format(
                             opt, option_string, ', '.join(choices)))
        items.append(values)
        setattr(namespace, self.dest, items)


class SignalAction(argparse.Action):
    """Validate input as a signal."""

    def __init__(self,
                 option_strings,
                 dest,
                 nargs=None,
                 const=None,
                 default=None,
                 type=str,
                 choices=None,
                 required=False,
                 help='The signal to send.'
                 ' It may be given as a name or a number.',
                 metavar='SIGNAL'):
        """Create SignalAction object."""
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

        if hasattr(signal, "Signals"):

            def _signal_number(signame):
                cooked = 'SIG{}'.format(signame)
                try:
                    return signal.Signals[cooked].value
                except ValueError:
                    pass
        else:

            def _signal_number(signame):
                cooked = 'SIG{}'.format(signame)
                for n, v in sorted(signal.__dict__.items()):
                    if n != cooked:
                        continue
                    if n.startswith("SIG") and not n.startswith("SIG_"):
                        return v

        self._signal_number = _signal_number

    def __call__(self, parser, namespace, values, option_string=None):
        """Validate input is a signal for platform."""
        if values.isdigit():
            signum = int(values)
            if signal.SIGRTMIN <= signum >= signal.SIGRTMAX:
                raise ValueError('"{}" is not a valid signal. {}-{}'.format(
                    values, signal.SIGRTMIN, signal.SIGRTMAX))
        else:
            signum = self._signal_number(values)
            if signum is None:
                parser.error(
                    '"{}" is not a valid signal,'
                    ' see your platform documentation.'.format(values))
        setattr(namespace, self.dest, signum)


class UnitAction(argparse.Action):
    """Validate number given is positive integer, with optional suffix."""

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
                 metavar='UNIT'):
        """Create UnitAction object."""
        help = (help or metavar or dest)\
            + ' (format: <number>[<unit>], where unit = b, k, m or g)'
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
        """Validate input as a UNIT."""
        try:
            val = int(values)
        except ValueError:
            if not values[:-1].isdigit():
                msg = ('{} must be a positive integer,'
                       ' with optional suffix').format(option_string)
                parser.error(msg)
            if not values[-1] in ('b', 'k', 'm', 'g'):
                msg = '{} only supports suffices of: b, k, m, g'.format(
                    option_string)
                parser.error(msg)
        else:
            if val <= 0:
                msg = '{} must be a positive integer'.format(option_string)
                parser.error(msg)

        setattr(namespace, self.dest, values)


class PositiveIntAction(argparse.Action):
    """Validate number given is positive integer."""

    def __init__(self,
                 option_strings,
                 dest,
                 nargs=None,
                 const=None,
                 default=None,
                 type=int,
                 choices=None,
                 required=False,
                 help='Must be a positive integer.',
                 metavar=None):
        """Create PositiveIntAction object."""
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
        """Validate input."""
        if values > 0:
            setattr(namespace, self.dest, values)
            return

        msg = '{} must be a positive integer'.format(option_string)
        parser.error(msg)


class PathAction(argparse.Action):
    """Expand user- and relative-paths."""

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
                 metavar='PATH'):
        """Create PathAction object."""
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
        """Resolve full path value on local filesystem."""
        setattr(namespace, self.dest,
                os.path.abspath(os.path.expanduser(values)))
