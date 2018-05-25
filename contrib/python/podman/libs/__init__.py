"""Support files for podman API implementation."""
import collections
import datetime
import functools

from dateutil.parser import parse as dateutil_parse

__all__ = [
    'cached_property',
    'datetime_parse',
    'datetime_format',
]


def cached_property(fn):
    """Decorate property to cache return value."""
    return property(functools.lru_cache(maxsize=8)(fn))


class Config(collections.UserDict):
    """Silently ignore None values, only take key once."""

    def __init__(self, **kwargs):
        """Construct dictionary."""
        super(Config, self).__init__(kwargs)

    def __setitem__(self, key, value):
        """Store unique, not None values."""
        if value is None:
            return

        if super().__contains__(key):
            return

        super().__setitem__(key, value)


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
