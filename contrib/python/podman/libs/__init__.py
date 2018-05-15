"""Support files for podman API implementation."""
import datetime
import re
import threading

__all__ = [
    'cached_property',
    'datetime_parse',
    'datetime_format',
]


class cached_property(object):
    """cached_property() - computed once per instance, cached as attribute.

    Maybe this will make a future version of python.
    """

    def __init__(self, func):
        """Construct context manager."""
        self.func = func
        self.__doc__ = func.__doc__
        self.lock = threading.RLock()

    def __get__(self, instance, cls=None):
        """Retrieve previous value, or call func()."""
        if instance is None:
            return self

        attrname = self.func.__name__
        try:
            cache = instance.__dict__
        except AttributeError:  # objects with __slots__ have no __dict__
            msg = ("No '__dict__' attribute on {}"
                   " instance to cache {} property.").format(
                       repr(type(instance).__name__), repr(attrname))
            raise TypeError(msg) from None

        with self.lock:
            # check if another thread filled cache while we awaited lock
            if attrname not in cache:
                cache[attrname] = self.func(instance)
        return cache[attrname]


def datetime_parse(string):
    """Convert timestamp to datetime.

    Because date/time parsing in python is still pedantically stupid,
    we rip the input string apart throwing out the stop characters etc;
    then rebuild a string strptime() can parse. Igit!

    - Python >3.7 will address colons in the UTC offset.
    - There is no ETA on microseconds > 6 digits.
    - And giving an offset and timezone name...

    # match: 2018-05-08T14:12:53.797795191-07:00
    # match: 2018-05-08T18:24:52.753227-07:00
    # match: 2018-05-08 14:12:53.797795191 -0700 MST
    # match: 2018-05-09T10:45:57.576002  (python isoformat())

    Some people, when confronted with a problem, think “I know,
    I'll use regular expressions.”  Now they have two problems.
      -- Jamie Zawinski
    """
    ts = re.compile(r'^(\d+)-(\d+)-(\d+)'
                    r'[ T]?(\d+):(\d+):(\d+).(\d+)'
                    r' *([-+][\d:]{4,5})? *')

    x = ts.match(string)
    if x is None:
        raise ValueError('Unable to parse {}'.format(string))

    # converting everything to int() not worth the readablity hit
    igit_proof = '{}T{}.{}{}'.format(
        '-'.join(x.group(1, 2, 3)),
        ':'.join(x.group(4, 5, 6)),
        x.group(7)[0:6],
        x.group(8).replace(':', '') if x.group(8) else '',
    )

    format = '%Y-%m-%dT%H:%M:%S.%f'
    if x.group(8):
        format += '%z'
    return datetime.datetime.strptime(igit_proof, format)


def datetime_format(dt):
    """Format datetime in consistent style."""
    if isinstance(dt, str):
        return datetime_parse(dt).isoformat()
    elif isinstance(dt, datetime.datetime):
        return dt.isoformat()
    else:
        raise ValueError('Unable to format {}. Type {} not supported.'.format(
            dt, type(dt)))
