#!/usr/bin/env python3
# Pull given image

import docker

client = docker.from_env()

print("\nImage(s):")
for img in client.images.list(all=True):
    print("Id", img.short_id, img.attrs["Created"], img.tags)

