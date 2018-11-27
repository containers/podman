"""Models for manipulating images in/to/from storage."""
import collections
import copy
import json
import logging

from . import ConfigDict, fold_keys
from .containers import Container


class Image(collections.UserDict):
    """Model for an Image."""

    def __init__(self, client, id, data):
        """Construct Image Model."""
        super().__init__(data)
        for k, v in data.items():
            setattr(self, k, v)

        self._id = id
        self._client = client

        assert self._id == data['id'],\
            'Requested image id({}) does not match store id({})'.format(
                self._id, data['id']
            )

    @staticmethod
    def _split_token(values=None, sep='='):
        if not values:
            return {}
        return {
            k: v1 for k, v1 in (v0.split(sep, 1) for v0 in values)
        }

    def create(self, *args, **kwargs):
        """Create container from image.

        Pulls defaults from image.inspect()
        """
        details = self.inspect()

        config = ConfigDict(image_id=self._id, **kwargs)
        config['command'] = details.containerconfig.get('cmd')
        config['env'] = self._split_token(details.containerconfig.get('env'))
        config['image'] = copy.deepcopy(details.repotags[0])
        config['labels'] = copy.deepcopy(details.labels)
        config['net_mode'] = 'bridge'
        config['network'] = 'bridge'

        logging.debug('Image %s: create config: %s', self._id, config)
        with self._client() as podman:
            id_ = podman.CreateContainer(config)['container']
            cntr = podman.GetContainer(id_)
        return Container(self._client, id_, cntr['container'])

    container = create

    def export(self, dest, compressed=False):
        """Write image to dest, return id on success."""
        with self._client() as podman:
            results = podman.ExportImage(self._id, dest, compressed)
        return results['image']

    def history(self):
        """Retrieve image history."""
        with self._client() as podman:
            for r in podman.HistoryImage(self._id)['history']:
                yield collections.namedtuple('HistoryDetail', r.keys())(**r)

    def inspect(self):
        """Retrieve details about image."""
        with self._client() as podman:
            results = podman.InspectImage(self._id)
        obj = json.loads(results['image'], object_hook=fold_keys())
        return collections.namedtuple('ImageInspect', obj.keys())(**obj)

    def push(self, target, tlsverify=False):
        """Copy image to target, return id on success."""
        with self._client() as podman:
            results = podman.PushImage(self._id, target, tlsverify)
        return results['image']

    def remove(self, force=False):
        """Delete image, return id on success.

        force=True, stop any running containers using image.
        """
        with self._client() as podman:
            results = podman.RemoveImage(self._id, force)
        return results['image']

    def tag(self, tag):
        """Tag image."""
        with self._client() as podman:
            results = podman.TagImage(self._id, tag)
        return results['image']


class Images():
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

    def build(self, dockerfile=None, tags=None, **kwargs):
        """Build container from image.

        See podman-build.1.md for kwargs details.
        """
        if dockerfile is None:
            raise ValueError('"dockerfile" is a required argument.')
        elif not hasattr(dockerfile, '__iter__'):
            raise ValueError('"dockerfile" is required to be an iter.')

        if tags is None:
            raise ValueError('"tags" is a required argument.')
        elif not hasattr(tags, '__iter__'):
            raise ValueError('"tags" is required to be an iter.')

        config = ConfigDict(dockerfile=dockerfile, tags=tags, **kwargs)
        with self._client() as podman:
            result = podman.BuildImage(config)
        return self.get(result['image']['id']), \
            (line for line in result['image']['logs'])

    def delete_unused(self):
        """Delete Images not associated with a container."""
        with self._client() as podman:
            results = podman.DeleteUnusedImages()
        return results['images']

    def import_image(self, source, reference, message='', changes=None):
        """Read image tarball from source and save in image store."""
        with self._client() as podman:
            results = podman.ImportImage(source, reference, message, changes)
        return results['image']

    def pull(self, source):
        """Copy image from registry to image store."""
        with self._client() as podman:
            results = podman.PullImage(source)
        return results['id']

    def search(self, id_, limit=25):
        """Search registries for id."""
        with self._client() as podman:
            results = podman.SearchImage(id_, limit)
        for img in results['images']:
            yield collections.namedtuple('ImageSearch', img.keys())(**img)

    def get(self, id_):
        """Get Image from id."""
        with self._client() as podman:
            result = podman.GetImage(id_)
        return Image(self._client, result['image']['id'], result['image'])
