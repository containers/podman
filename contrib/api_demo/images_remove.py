#!/usr/bin/env python3

import docker

client = docker.from_env()
_ = client.images.pull("busybox", tag="latest")
client.images.remove("busybox", force=True)

