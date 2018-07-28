"""Error classes and wrappers for VarlinkError."""
from varlink import VarlinkError


class VarlinkErrorProxy(VarlinkError):
    """Class to Proxy VarlinkError methods."""

    def __init__(self, message, namespaced=False):
        """Construct proxy from Exception."""
        super().__init__(message.as_dict(), namespaced)
        self._message = message
        self.__module__ = 'libpod'

    def __getattr__(self, method):
        """Return attribute from proxied Exception."""
        if hasattr(self._message, method):
            return getattr(self._message, method)

        try:
            return self._message.parameters()[method]
        except KeyError:
            raise AttributeError('No such attribute: {}'.format(method))


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


class PodmanError(VarlinkErrorProxy):
    """Raised when Client fails to connect to runtime."""

    pass


ERROR_MAP = {
    'io.projectatomic.podman.ContainerNotFound': ContainerNotFound,
    'io.projectatomic.podman.ErrorOccurred': ErrorOccurred,
    'io.projectatomic.podman.ImageNotFound': ImageNotFound,
    'io.projectatomic.podman.RuntimeError': PodmanError,
}


def error_factory(exception):
    """Map Exceptions to a discrete type."""
    try:
        return ERROR_MAP[exception.error()](exception)
    except KeyError:
        return exception
