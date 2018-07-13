#!/usr/bin/env python3
"""Example: Pull Fedora and inspect image and container."""

import podman

print('{}\n'.format(__doc__))

with podman.Client() as client:
    id = client.images.pull('registry.fedoraproject.org/fedora:28')
    img = client.images.get(id)
    print(img.inspect())

    cntr = img.create()
    print(cntr.inspect())

    cntr.remove()
