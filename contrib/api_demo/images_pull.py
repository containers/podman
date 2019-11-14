#!/usr/bin/env python3

import docker

client = docker.from_env()

image = client.images.pull("busybox", tag="latest")
print("Image:")
print("Id", image.short_id, image.attrs["Created"], image.tags)

