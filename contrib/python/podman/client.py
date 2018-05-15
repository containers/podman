"""A client for communicating with a Podman varlink service."""
import contextlib
import functools

from varlink import Client as VarlinkClient
from varlink import VarlinkError

from .libs import cached_property
from .libs.containers import Containers
from .libs.errors import error_factory
from .libs.images import Images
from .libs.system import System


class Client(object):
    """A client for communicating with a Podman varlink service.

    Example:

        >>> import podman
        >>> c = podman.Client()
        >>> c.system.versions
    """

    # TODO: Port to contextlib.AbstractContextManager once
    #   Python >=3.6 required

    def __init__(self,
                 uri='unix:/run/podman/io.projectatomic.podman',
                 interface='io.projectatomic.podman'):
        """Construct a podman varlink Client.

        uri from default systemd unit file.
        interface from io.projectatomic.podman.varlink, do not change unless
            you are a varlink guru.
        """
        self._podman = None

        @contextlib.contextmanager
        def _podman(uri, interface):
            """Context manager for API children to access varlink."""
            client = VarlinkClient(address=uri)
            try:
                iface = client.open(interface)
                yield iface
            except VarlinkError as e:
                raise error_factory(e) from e
            finally:
                if hasattr(client, 'close'):
                    client.close()
                iface.close()

        self._client = functools.partial(_podman, uri, interface)

        # Quick validation of connection data provided
        if not System(self._client).ping():
            raise ValueError('Failed varlink connection "{}/{}"'.format(
                uri, interface))

    def __enter__(self):
        """Return `self` upon entering the runtime context."""
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        """Raise any exception triggered within the runtime context."""
        return None

    @cached_property
    def system(self):
        """Manage system model for podman."""
        return System(self._client)

    @cached_property
    def images(self):
        """Manage images model for libpod."""
        return Images(self._client)

    @cached_property
    def containers(self):
        """Manage containers model for libpod."""
        return Containers(self._client)
