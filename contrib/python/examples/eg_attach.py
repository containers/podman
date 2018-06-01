#!/usr/bin/env python3
"""Example: Run Alpine container and attach."""

import podman

print('{}\n'.format(__doc__))

with podman.Client() as client:
    id = client.images.pull('alpine:latest')
    img = client.images.get(id)
    cntr = img.create()
    cntr.start()

    try:
        cntr.attach()
    except BrokenPipeError:
        print('Container disconnected.')
