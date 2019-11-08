#!/usr/bin/env python3
# Pull given image

import docker

client = docker.from_env()

print("\nContainer(s):")
for ctnr in client.containers.list(all=True):
    print("Id", ctnr.short_id, ctnr.attrs["Created"], ctnr.attrs["Args"])

