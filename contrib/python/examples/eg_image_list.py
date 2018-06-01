#!/usr/bin/env python3
"""Example: Show all images on system."""

import podman

print('{}\n'.format(__doc__))

with podman.Client() as client:
    for img in client.images.list():
        print(img)
