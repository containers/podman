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


class ImageNotFound(VarlinkErrorProxy):
    """Raised when Client can not find requested image."""


class PodNotFound(VarlinkErrorProxy):
    """Raised when Client can not find requested image."""


class PodContainerError(VarlinkErrorProxy):
    """Raised when a container fails requested pod operation."""


class NoContainerRunning(VarlinkErrorProxy):
    """Raised when no container is running in pod."""


class NoContainersInPod(VarlinkErrorProxy):
    """Raised when Client fails to connect to runtime."""


class ErrorOccurred(VarlinkErrorProxy):
    """Raised when an error occurs during the execution.

    See error() to see actual error text.
    """


class PodmanError(VarlinkErrorProxy):
    """Raised when Client fails to connect to runtime."""


ERROR_MAP = {
    'io.podman.ContainerNotFound': ContainerNotFound,
    'io.podman.ErrorOccurred': ErrorOccurred,
    'io.podman.ImageNotFound': ImageNotFound,
    'io.podman.NoContainerRunning': NoContainerRunning,
    'io.podman.NoContainersInPod': NoContainersInPod,
    'io.podman.PodContainerError': PodContainerError,
    'io.podman.PodNotFound': PodNotFound,
    'io.podman.RuntimeError': PodmanError,
}


def error_factory(exception):
    """Map Exceptions to a discrete type."""
    try:
        return ERROR_MAP[exception.error()](exception)
    except KeyError:
        return exception
