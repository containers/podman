"""
Supplimental argparse.Action converters and validaters.

The constructors are very verbose but remain for IDE support.
"""
import argparse
import copy
import os

# API defined by argparse.Action therefore shut up pylint
# pragma pylint: disable=redefined-builtin
# pragma pylint: disable=too-few-public-methods
# pragma pylint: disable=too-many-arguments


class BooleanValidate():
    """Validate value is boolean string."""

    def __call__(self, value):
        """Return True, False or raise ValueError."""
        val = value.capitalize()
        if val == 'False':
            return False
        elif val == 'True':
            return True
        else:
            raise ValueError('"{}" is not True or False'.format(value))


class BooleanAction(argparse.Action):
    """Convert and validate bool argument."""

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
                 metavar='{True,False}'):
        """Create BooleanAction object."""
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
        try:
            val = BooleanValidate()(values)
        except ValueError:
            parser.error('"{}" must be True or False.'.format(option_string))
        else:
            setattr(namespace, self.dest, val)


class ChangeAction(argparse.Action):
    """Convert and validate change argument."""

    def __init__(self,
                 option_strings,
                 dest,
                 nargs=None,
                 const=None,
                 default=[],
                 type=None,
                 choices=None,
                 required=False,
                 help=None,
                 metavar='OPT=VALUE'):
        """Create ChangeAction object."""
        help = (help or '') + ('Apply change(s) to the new image.'
                               ' May be given multiple times.')

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

        opt, val = values.split('=', 1)
        if opt not in choices:
            parser.error('Option "{}" is not supported by argument "{}",'
                         ' valid options are: {}'.format(
                             opt, option_string, ', '.join(choices)))
        items.append(values)
        setattr(namespace, self.dest, items)


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
