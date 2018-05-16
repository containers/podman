"""Models for manipulating images in/to/from storage."""
import collections
import functools
import json


class Image(collections.UserDict):
    """Model for an Image."""

    def __init__(self, client, id, data):
        """Construct Image Model."""
        super(Image, self).__init__(data)
        for k, v in data.items():
            setattr(self, k, v)

        self._id = id
        self._client = client

        assert self._id == self.id,\
            'Requested image id({}) does not match store id({})'.format(
                self._id, self.id
            )

    def __getitem__(self, key):
        """Get items from parent dict."""
        return super().__getitem__(key)

    def export(self, dest, compressed=False):
        """Write image to dest, return True on success."""
        with self._client() as podman:
            results = podman.ExportImage(self.id, dest, compressed)
        return results['image']

    def history(self):
        """Retrieve image history."""
        with self._client() as podman:
            for r in podman.HistoryImage(self.id)['history']:
                yield collections.namedtuple('HistoryDetail', r.keys())(**r)

    def _lower_hook(self):
        """Convert all keys to lowercase."""

        @functools.wraps(self._lower_hook)
        def wrapped(input):
            return {k.lower(): v for (k, v) in input.items()}

        return wrapped

    def inspect(self):
        """Retrieve details about image."""
        with self._client() as podman:
            results = podman.InspectImage(self.id)
            obj = json.loads(results['image'], object_hook=self._lower_hook())
            return collections.namedtuple('ImageInspect', obj.keys())(**obj)

    def push(self, target, tlsverify=False):
        """Copy image to target, return id on success."""
        with self._client() as podman:
            results = podman.PushImage(self.id, target, tlsverify)
        return results['image']

    def remove(self, force=False):
        """Delete image, return id on success.

        force=True, stop any running containers using image.
        """
        with self._client() as podman:
            results = podman.RemoveImage(self.id, force)
        return results['image']

    def tag(self, tag):
        """Tag image."""
        with self._client() as podman:
            results = podman.TagImage(self.id, tag)
        return results['image']


class Images(object):
    """Model for Images collection."""

    def __init__(self, client):
        """Construct model for Images collection."""
        self._client = client

    def list(self):
        """List all images in the libpod image store."""
        with self._client() as podman:
            results = podman.ListImages()
        for img in results['images']:
            yield Image(self._client, img['id'], img)

    def create(self, *args, **kwargs):
        """Create image from configuration."""
        with self._client() as podman:
            results = podman.CreateImage()
        return results['image']

    def create_from(self, *args, **kwargs):
        """Create image from container."""
        # TODO: Should this be on container?
        with self._client() as podman:
            results = podman.CreateFromContainer()
        return results['image']

    def build(self, *args, **kwargs):
        """Build container from image.

        See podman-build.1.md for kwargs details.
        """
        with self._client() as podman:
            # TODO: Need arguments
            podman.BuildImage()

    def delete_unused(self):
        """Delete Images not associated with a container."""
        with self._client() as podman:
            results = podman.DeleteUnusedImages()
        return results['images']

    def import_image(self, source, reference, message=None, changes=None):
        """Read image tarball from source and save in image store."""
        with self._client() as podman:
            results = podman.ImportImage(source, reference, message, changes)
        return results['image']

    def pull(self, source):
        """Copy image from registry to image store."""
        with self._client() as podman:
            results = podman.PullImage(source)
        return results['id']

    def search(self, id, limit=25):
        """Search registries for id."""
        with self._client() as podman:
            results = podman.SearchImage(id)
        for img in results['images']:
            yield img
