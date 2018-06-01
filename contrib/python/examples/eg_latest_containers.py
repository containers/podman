#!/usr/bin/env python3
"""Example: Show all containers created since midnight."""

from datetime import datetime, time, timezone

import podman

print('{}\n'.format(__doc__))


midnight = datetime.combine(datetime.today(), time.min, tzinfo=timezone.utc)

with podman.Client() as client:
    for c in client.containers.list():
        created_at = podman.datetime_parse(c.createdat)

        if created_at > midnight:
            print('{}: image: {} createdAt: {}'.format(
                c.id[:12], c.image[:32], podman.datetime_format(created_at)))
