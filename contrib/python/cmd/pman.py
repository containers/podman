import podman as p


class PodmanRemote(object):
    def __init__(self):
        self.args = None
        self._remote_uri= None
        self._local_uri= None
        self._identity_file= None
        self._client = None

    def set_args(self, args, local_uri, remote_uri, identity_file):
        self.args = args
        self._local_uri = local_uri
        self.remote_uri = remote_uri
        self._identity_file = identity_file

    @property
    def remote_uri(self):
        return self._remote_uri

    @property
    def local_uri(self):
        return self._local_uri

    @property
    def client(self):
        if self._client is None:
            self._client = p.Client(uri=self.local_uri, remote_uri=self.remote_uri, identity_file=self.identity_file)
        return self._client

    @remote_uri.setter
    def remote_uri(self, uri):
        self._remote_uri = uri

    @local_uri.setter
    def local_uri(self, uri):
        self._local_uri= uri

    @property
    def identity_file(self):
        return self._identity_file
