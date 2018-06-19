"""A client for communicating with a Podman varlink service."""
import os
from urllib.parse import urlparse

from varlink import Client as VarlinkClient
from varlink import VarlinkError

from .libs import cached_property
from .libs.containers import Containers
from .libs.errors import error_factory
from .libs.images import Images
from .libs.system import System
from .libs.tunnel import Context, Portal, Tunnel


class BaseClient(object):
    """Context manager for API workers to access varlink."""

    def __call__(self):
        """Support being called for old API."""
        return self

    @classmethod
    def factory(cls,
                uri=None,
                interface='io.projectatomic.podman',
                *args,
                **kwargs):
        """Construct a Client based on input."""
        if uri is None:
            raise ValueError('uri is required and cannot be None')
        if interface is None:
            raise ValueError('interface is required and cannot be None')

        local_path = urlparse(uri).path
        if local_path == '':
            raise ValueError('path is required for uri, format'
                             ' "unix://path_to_socket"')

        if kwargs.get('remote_uri') or kwargs.get('identity_file'):
            # Remote access requires the full tuple of information
            if kwargs.get('remote_uri') is None:
                raise ValueError('remote is required, format'
                                 ' "ssh://user@hostname/path_to_socket".')
            remote = urlparse(kwargs['remote_uri'])
            if remote.username is None:
                raise ValueError('username is required for remote_uri, format'
                                 ' "ssh://user@hostname/path_to_socket".')
            if remote.path == '':
                raise ValueError('path is required for remote_uri, format'
                                 ' "ssh://user@hostname/path_to_socket".')
            if remote.hostname is None:
                raise ValueError('hostname is required for remote_uri, format'
                                 ' "ssh://user@hostname/path_to_socket".')

            if kwargs.get('identity_file') is None:
                raise ValueError('identity_file is required.')

            if not os.path.isfile(kwargs['identity_file']):
                raise ValueError('identity_file "{}" not found.'.format(
                    kwargs['identity_file']))
            return RemoteClient(
                Context(uri, interface, local_path, remote.path,
                        remote.username, remote.hostname,
                        kwargs['identity_file']))
        else:
            return LocalClient(
                Context(uri, interface, None, None, None, None, None))


class LocalClient(BaseClient):
    """Context manager for API workers to access varlink."""

    def __init__(self, context):
        """Construct LocalClient."""
        self._context = context

    def __enter__(self):
        """Enter context for LocalClient."""
        self._client = VarlinkClient(address=self._context.uri)
        self._iface = self._client.open(self._context.interface)
        return self._iface

    def __exit__(self, e_type, e, e_traceback):
        """Cleanup context for LocalClient."""
        if hasattr(self._client, 'close'):
            self._client.close()
        self._iface.close()

        if isinstance(e, VarlinkError):
            raise error_factory(e)


class RemoteClient(BaseClient):
    """Context manager for API workers to access remote varlink."""

    def __init__(self, context):
        """Construct RemoteCLient."""
        self._context = context
        self._portal = Portal()

    def __enter__(self):
        """Context manager for API workers to access varlink."""
        tunnel = self._portal.get(self._context.uri)
        if tunnel is None:
            tunnel = Tunnel(self._context).bore(self._context.uri)
            self._portal[self._context.uri] = tunnel

        try:
            self._client = VarlinkClient(address=self._context.uri)
            self._iface = self._client.open(self._context.interface)
            return self._iface
        except Exception:
            self._close_tunnel(self._context.uri)
            raise

    def __exit__(self, e_type, e, e_traceback):
        """Cleanup context for RemoteClient."""
        if hasattr(self._client, 'close'):
            self._client.close()
        self._iface.close()

        # set timer to shutdown ssh tunnel
        if isinstance(e, VarlinkError):
            raise error_factory(e)


class Client(object):
    """A client for communicating with a Podman varlink service.

    Example:

        >>> import podman
        >>> c = podman.Client()
        >>> c.system.versions

    Example remote podman:

        >>> import podman
        >>> c = podman.Client(uri='unix:/tmp/podman.sock',
                              remote_uri='ssh://user@host/run/podman/io.projectatomic.podman',
                              identity_file='~/.ssh/id_rsa')
    """

    def __init__(self,
                 uri='unix:/run/podman/io.projectatomic.podman',
                 interface='io.projectatomic.podman',
                 **kwargs):
        """Construct a podman varlink Client.

        uri from default systemd unit file.
        interface from io.projectatomic.podman.varlink, do not change unless
            you are a varlink guru.
        """
        self._client = BaseClient.factory(uri, interface, **kwargs)

        # Quick validation of connection data provided
        try:
            if not System(self._client).ping():
                raise ValueError('Failed varlink connection "{}/{}"'.format(
                    uri, interface))
        except FileNotFoundError:
            raise ValueError('Failed varlink connection "{}/{}".'
                             ' Is podman service running?'.format(
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
