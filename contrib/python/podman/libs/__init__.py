"""Support files for podman API implementation."""
import datetime
import threading
from dateutil.parser import parse as dateutil_parse


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
    """Convert timestamps to datetime.

    tzinfo aware, if provided.
    """
    return dateutil_parse(string.upper(), fuzzy=True)


def datetime_format(dt):
    """Format datetime in consistent style."""
    if isinstance(dt, str):
        return datetime_parse(dt).isoformat()
    elif isinstance(dt, datetime.datetime):
        return dt.isoformat()
    else:
        raise ValueError('Unable to format {}. Type {} not supported.'.format(
            dt, type(dt)))
