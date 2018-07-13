#!/usr/bin/env python3
"""Example: Show containers grouped by image id."""

from itertools import groupby

import podman

print('{}\n'.format(__doc__))

with podman.Client() as client:
    ctnrs = sorted(client.containers.list(), key=lambda k: k.imageid)
    for key, grp in groupby(ctnrs, key=lambda k: k.imageid):
        print('Image: {}'.format(key))
        for c in grp:
            print('     : container: {} created at: {}'.format(
                c.id[:12], podman.datetime_format(c.createdat)))
