"""Utilities for with-statement contexts.  See PEP 343."""

import abc

import _collections_abc

try:
    from contextlib import AbstractContextManager
except ImportError:
    # Copied from python3.7 library as "backport"
    class AbstractContextManager(abc.ABC):
        """An abstract base class for context managers."""

        def __enter__(self):
            """Return `self` upon entering the runtime context."""
            return self

        @abc.abstractmethod
        def __exit__(self, exc_type, exc_value, traceback):
            """Raise any exception triggered within the runtime context."""
            return None

        @classmethod
        def __subclasshook__(cls, C):
            """Check whether subclass is considered a subclass of this ABC."""
            if cls is AbstractContextManager:
                return _collections_abc._check_methods(C, "__enter__",
                                                       "__exit__")
            return NotImplemented
