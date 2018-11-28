"""Base class for all actions of remote client."""
import abc
from functools import lru_cache

import podman


class AbstractActionBase(abc.ABC):
    """Base class for all actions of remote client."""

    @classmethod
    @abc.abstractmethod
    def subparser(cls, parent):
        """Define parser for this action.  Subclasses must implement.

        API:
        Use set_defaults() to set attributes "class_" and "method". These will
        be invoked as class_(parsed_args).method()
        """
        parent.add_flag(
            '--all',
            help='list all items.')
        parent.add_flag(
            '--truncate',
            '--trunc',
            default=True,
            help="Truncate id's and other long fields.")
        parent.add_flag(
            '--heading',
            default=True,
            help='Include table headings in the output.')
        parent.add_flag(
            '--quiet',
            help='List only the IDs.')

    def __init__(self, args):
        """Construct class."""
        # Dump all unset arguments before transmitting to service
        self._args = args
        self.opts = {
            k: v
            for k, v in vars(self._args).items() if v is not None
        }

    @property
    def remote_uri(self):
        """URI for remote side of connection."""
        return self._args.remote_uri

    @property
    def local_uri(self):
        """URI for local side of connection."""
        return self._args.local_uri

    @property
    def identity_file(self):
        """Key for authenication."""
        return self._args.identity_file

    @property
    @lru_cache(maxsize=1)
    def client(self):
        """Podman remote client for communicating."""
        if self._args.host is None:
            return podman.Client(uri=self.local_uri)
        return podman.Client(
            uri=self.local_uri,
            remote_uri=self.remote_uri,
            identity_file=self.identity_file)

    def __repr__(self):
        """Compute the “official” string representation of object."""
        return ("{}(local_uri='{}', remote_uri='{}',"
                " identity_file='{}')").format(
                    self.__class__,
                    self.local_uri,
                    self.remote_uri,
                    self.identity_file,
                )
