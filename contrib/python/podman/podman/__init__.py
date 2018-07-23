"""A client for communicating with a Podman server."""
import pkg_resources

from .client import Client
from .libs import datetime_format, datetime_parse
from .libs.errors import (ContainerNotFound, ErrorOccurred, ImageNotFound,
                          PodmanError)

try:
    __version__ = pkg_resources.get_distribution('podman').version
except Exception:
    __version__ = '0.0.0'

__all__ = [
    'Client',
    'ContainerNotFound',
    'datetime_format',
    'datetime_parse',
    'ErrorOccurred',
    'ImageNotFound',
    'PodmanError',
]
