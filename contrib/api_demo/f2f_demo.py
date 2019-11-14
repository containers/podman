#!/usr/bin/env python3

import docker
import os


def run(cmd):
    input("\n\n--> {}".format(cmd))
    os.system(cmd)


run("ps -ef |grep docke[r]")
run("ps -ef |grep container[d]")
print("DOCKER_HOST={}".format(os.environ["DOCKER_HOST"]))
input("\n\nNothing up our sleeves...")

client = docker.from_env()

run("podman images")
run("docker image ls")

input('\n\n--> client.images.pull("alpine:3.10")')
result = client.images.pull("alpine:3.10")
print("\nImage:")
print("Id", result.short_id, result.attrs["Created"], result.tags)

input("\n\n--> client.images.list()")
print("\nImage(s):")
for img in client.images.list(all=True):
    print("Id", img.short_id, img.attrs["Created"], img.tags)

run("podman images")
run("docker image ls")

input('\n\n--> client.api.create_container("alpine:3.10")')
result = client.api.create_container("alpine:3.10", "true")
print("\nContainer:")
for k in result:
    if result[k] is not None:
        print(k, result[k])

input("\n\n--> client.containers.list()")
print("\nContainer(s):")
for ctnr in client.containers.list():
    print("Id", ctnr.short_id, ctnr.attrs["Created"], ctnr.attrs["Command"])

ctnr.start()

run("podman ps --all")
run("docker ps --all")

try:
    input("\n\n--> client.api.remove_container(ctnr.id)")
    client.api.remove_container(ctnr.id, force=True)

    run("podman ps --all")
    run("docker ps --all")

    input('\n\n--> client.images.remove("alpine:3.10")')
    client.images.remove("alpine:3.10", force=True)

    run("podman images")
    run("docker image ls")
except Exception as e:
    print(e)
    exit

run("which docker")
run("ls -lh /usr/bin/docker")
