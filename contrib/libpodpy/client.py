
from varlink import Client
from libpodpy.images import Images
from libpodpy.system import System
from libpodpy.containers import Containers

class LibpodClient(object):


    """
    A client for communicating with a Docker server.

    Example:

        >>> from libpodpy import client
        >>> c = client.LibpodClient("unix:/run/podman/io.projectatomic.podman")

    Args:
        Requires the varlink URI for libpod
    """

    def __init__(self, varlink_uri):
        c = Client(address=varlink_uri)
        self.conn = c.open("io.projectatomic.podman")

    @property
    def images(self):
        """
        An object for managing images through libpod
        """
        return Images(self.conn)

    @property
    def system(self):
        """
        An object for system related calls through libpod
        """
        return System(self.conn)

    @property
    def containers(self):
        """
        An object for managing containers through libpod
        """
        return Containers(self.conn)
