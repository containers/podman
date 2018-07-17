"""Utilities for with-statement contexts.  See PEP 343."""

import abc

try:
    from contextlib import AbstractContextManager
    assert AbstractContextManager
except ImportError:
    class AbstractContextManager(abc.ABC):
        """An abstract base class for context managers."""

        @abc.abstractmethod
        def __enter__(self):
            """Return `self` upon entering the runtime context."""
            return self

        @abc.abstractmethod
        def __exit__(self, exc_type, exc_value, traceback):
            """Raise any exception triggered within the runtime context."""
            return None
