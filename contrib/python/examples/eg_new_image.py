#!/usr/bin/env python3
"""Example: Create new image from container."""

import sys

import podman


def print_history(details):
    """Format history data from an image, in a table."""
    for i, r in enumerate(details):
        print(
            '{}: {} {} {}'.format(i, r.id[:12],
                                  podman.datetime_format(r.created), r.tags),
            sep='\n')
    print("-" * 25)


print('{}\n'.format(__doc__))

with podman.Client() as client:
    ctnr = next(
        (c for c in client.containers.list() if 'alpine' in c['image']), None)

    if ctnr:
        print_history(client.images.get(ctnr.imageid).history())

        # Make changes as we save the container to a new image
        id = ctnr.commit('alpine-ash', changes=['CMD=/bin/ash'])
        print_history(client.images.get(id).history())
    else:
        print('Unable to find "alpine" container.', file=sys.stderr)
