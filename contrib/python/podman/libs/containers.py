"""Models for manipulating containers and storage."""
import collections
import functools
import json
import signal


class Container(collections.UserDict):
    """Model for a container."""

    def __init__(self, client, id, data):
        """Construct Container Model."""
        super(Container, self).__init__(data)

        self._client = client
        self._id = id

        self._refresh(data)
        assert self._id == self.data['id'],\
            'Requested container id({}) does not match store id({})'.format(
                self._id, self.id
            )

    def __getitem__(self, key):
        """Get items from parent dict + apply aliases."""
        if key == 'running':
            key = 'containerrunning'
        return super().__getitem__(key)

    def _refresh(self, data):
        super().update(data)
        for k, v in data.items():
            setattr(self, k, v)
        setattr(self, 'running', data['containerrunning'])

    def refresh(self):
        """Refresh status fields for this container."""
        ctnr = Containers(self._client).get(self.id)
        self._refresh(ctnr)

    def attach(self, detach_key=None, no_stdin=False, sig_proxy=True):
        """Attach to running container."""
        with self._client() as podman:
            # TODO: streaming and port magic occur, need arguements
            podman.AttachToContainer()

    def processes(self):
        """Show processes running in container."""
        with self._client() as podman:
            results = podman.ListContainerProcesses(self.id)
        for p in results['container']:
            yield p

    def changes(self):
        """Retrieve container changes."""
        with self._client() as podman:
            results = podman.ListContainerChanges(self.id)
        return results['container']

    def kill(self, signal=signal.SIGTERM):
        """Send signal to container, return id if successful.

        default signal is signal.SIGTERM.
        """
        with self._client() as podman:
            results = podman.KillContainer(self.id, signal)
        return results['container']

    def _lower_hook(self):
        """Convert all keys to lowercase."""

        @functools.wraps(self._lower_hook)
        def wrapped(input):
            return {k.lower(): v for (k, v) in input.items()}

        return wrapped

    def inspect(self):
        """Retrieve details about containers."""
        with self._client() as podman:
            results = podman.InspectContainer(self.id)
            obj = json.loads(
                results['container'], object_hook=self._lower_hook())
            return collections.namedtuple('ContainerInspect',
                                          obj.keys())(**obj)

    def export(self, target):
        """Export container from store to tarball.

        TODO: should there be a compress option, like images?
        """
        with self._client() as podman:
            results = podman.ExportContainer(self.id, target)
        return results['tarfile']

    def start(self):
        """Start container, return id on success."""
        with self._client() as podman:
            results = podman.StartContainer(self.id)
        return results['container']

    def stop(self, timeout=25):
        """Stop container, return id on success."""
        with self._client() as podman:
            results = podman.StopContainer(self.id, timeout)
        return results['container']

    def remove(self, force=False):
        """Remove container, return id on success.

        force=True, stop running container.
        """
        with self._client() as podman:
            results = podman.RemoveContainer(self.id, force)
        return results['container']

    def restart(self, timeout=25):
        """Restart container with timeout, return id on success."""
        with self._client() as podman:
            results = podman.RestartContainer(self.id, timeout)
        return results['container']

    def rename(self, target):
        """Rename container, return id on success."""
        with self._client() as podman:
            # TODO: Need arguements
            results = podman.RenameContainer()
        # TODO: fixup objects cached information
        return results['container']

    def resize_tty(self, width, height):
        """Resize container tty."""
        with self._client() as podman:
            # TODO: magic re: attach(), arguements
            podman.ResizeContainerTty()

    def pause(self):
        """Pause container, return id on success."""
        with self._client() as podman:
            results = podman.PauseContainer(self.id)
        return results['container']

    def unpause(self):
        """Unpause container, return id on success."""
        with self._client() as podman:
            results = podman.UnpauseContainer(self.id)
        return results['container']

    def update_container(self, *args, **kwargs):
        """TODO: Update container..., return id on success."""
        with self._client() as podman:
            results = podman.UpdateContainer()
        self.refresh()
        return results['container']

    def wait(self):
        """Wait for container to finish, return 'returncode'."""
        with self._client() as podman:
            results = podman.WaitContainer(self.id)
        return int(results['exitcode'])

    def stats(self):
        """Retrieve resource stats from the container."""
        with self._client() as podman:
            results = podman.GetContainerStats(self.id)
        obj = results['container']
        return collections.namedtuple('StatDetail', obj.keys())(**obj)

    def logs(self, *args, **kwargs):
        """Retrieve container logs."""
        with self._client() as podman:
            results = podman.GetContainerLogs(self.id)
        for line in results:
            yield line


class Containers(object):
    """Model for Containers collection."""

    def __init__(self, client):
        """Construct model for Containers collection."""
        self._client = client

    def list(self):
        """List of containers in the container store."""
        with self._client() as podman:
            results = podman.ListContainers()
        for cntr in results['containers']:
            yield Container(self._client, cntr['id'], cntr)

    def delete_stopped(self):
        """Delete all stopped containers."""
        with self._client() as podman:
            results = podman.DeleteStoppedContainers()
        return results['containers']

    def create(self, *args, **kwargs):
        """Create container layer over the specified image.

        See podman-create.1.md for kwargs details.
        """
        with self._client() as podman:
            results = podman.CreateContainer()
        return results['id']

    def get(self, id):
        """Retrieve container details from store."""
        with self._client() as podman:
            cntr = podman.GetContainer(id)
        return Container(self._client, cntr['container']['id'],
                         cntr['container'])
