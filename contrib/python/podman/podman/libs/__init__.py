"""Support files for podman API implementation."""
import collections
import datetime
import functools

from dateutil.parser import parse as dateutil_parse

__all__ = [
    'cached_property',
    'datetime_format',
    'datetime_parse',
    'fold_keys',
]


def cached_property(fn):
    """Decorate property to cache return value."""
    return property(functools.lru_cache(maxsize=8)(fn))


class ConfigDict(collections.UserDict):
    """Silently ignore None values, only take key once."""

    def __init__(self, **kwargs):
        """Construct dictionary."""
        super().__init__(kwargs)

    def __setitem__(self, key, value):
        """Store unique, not None values."""
        if value is None:
            return

        if super().__contains__(key):
            return

        super().__setitem__(key, value)


class FoldedString(collections.UserString):
    """Foldcase sequences value."""

    def __init__(self, seq):
        super().__init__(seq)
        self.data.casefold()


def fold_keys():  # noqa: D202
    """Fold case of dictionary keys."""

    @functools.wraps(fold_keys)
    def wrapped(mapping):
        """Fold case of dictionary keys."""
        return {k.casefold(): v for (k, v) in mapping.items()}

    return wrapped


def datetime_parse(string):
    """Convert timestamps to datetime.

    tzinfo aware, if provided.
    """
    return dateutil_parse(string.upper(), fuzzy=True)


def datetime_format(dt):
    """Format datetime in consistent style."""
    if isinstance(dt, str):
        return datetime_parse(dt).isoformat()

    if isinstance(dt, datetime.datetime):
        return dt.isoformat()

    raise ValueError('Unable to format {}. Type {} not supported.'.format(
        dt, type(dt)))
