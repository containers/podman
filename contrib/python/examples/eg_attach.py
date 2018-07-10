#!/usr/bin/env python3
"""Example: Run top on Alpine container."""

import podman

print('{}\n'.format(__doc__))

with podman.Client() as client:
    id = client.images.pull('alpine:latest')
    img = client.images.get(id)
    cntr = img.create(detach=True, tty=True, command=['/usr/bin/top'])
    cntr.attach(eot=4)

    try:
        cntr.start()
        print()
    except (BrokenPipeError, KeyboardInterrupt):
        print('\nContainer disconnected.')
