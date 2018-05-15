"""Error classes and wrappers for VarlinkError."""
from varlink import VarlinkError


class VarlinkErrorProxy(VarlinkError):
    """Class to Proxy VarlinkError methods."""

    def __init__(self, obj):
        """Construct proxy from Exception."""
        self._obj = obj
        self.__module__ = 'libpod'

    def __getattr__(self, item):
        """Return item from proxied Exception."""
        return getattr(self._obj, item)


class ContainerNotFound(VarlinkErrorProxy):
    """Raised when Client can not find requested container."""

    pass


class ImageNotFound(VarlinkErrorProxy):
    """Raised when Client can not find requested image."""

    pass


class ErrorOccurred(VarlinkErrorProxy):
    """Raised when an error occurs during the execution.

    See error() to see actual error text.
    """

    pass


class RuntimeError(VarlinkErrorProxy):
    """Raised when Client fails to connect to runtime."""

    pass


error_map = {
    'io.projectatomic.podman.ContainerNotFound': ContainerNotFound,
    'io.projectatomic.podman.ErrorOccurred': ErrorOccurred,
    'io.projectatomic.podman.ImageNotFound': ImageNotFound,
    'io.projectatomic.podman.RuntimeError': RuntimeError,
}


def error_factory(exception):
    """Map Exceptions to a discrete type."""
    try:
        return error_map[exception.error()](exception)
    except KeyError:
        return exception
