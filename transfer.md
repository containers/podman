# PODMAN Usage Transfer

This document outlines useful information for ops and dev transfer as it relates to infrastructure that utilizes PODMAN.

## Operational Transfer

## Abstract

The `crio` daemon is intended to provide the [CRI](https://github.com/kubernetes/community/blob/master/contributors/devel/container-runtime-interface.md) socket needed for Kubernetes to use for automating deployment, scaling, and management of containerized applications (See the document for [configuring kubernetes to use PODMAN](./kubernetes.md) for more information on that).
Therefore the `crioctl` command line is a client that interfaces to the same grpc socket as the kubernetes daemon would, for talking to the `crio` daemon.
In many ways `crioctl` is only as feature rich as the Kubernetes CRI requires.
There are additional tools e.g. `podman` and [`buildah`](https://github.com/projectatomic/buildah) that provide a feature rich set of commands for all operational needs in a Kubernetes environment.


## System Tools

Many traditional tools will still be useful, such as `pstree`, `nsenter` and `lsns`.
As well as some systemd helpers like `systemd-cgls` and `systemd-cgtop` are still just as applicable.

## Equivalents

For many troubleshooting and information collection steps, there may be an existing pattern.
Following provides equivalent with PODMAN tools for gathering information or jumping into containers, for operational use.

| Existing Step | PODMAN (and friends) |
| :--- | :--- |
| `docker run`  | [`podman run`](./docs/podman-run.1.md) |
| `docker exec` | [`podman exec`](./docs/podman-exec.1.md) |
| `docker info` | [`podman info`](./docs/podman-info.1.md)  |
| `docker inspect` | [`podman inspect`](./docs/podman-inspect.1.md)       |
| `docker logs` | [`podman logs`](./docs/podman-logs.1.md)                 |
| `docker ps`   | [`podman ps`](./docs/podman-ps.1.md) |
| `docker stats`| [`podman stats`](./docs/podman-stats.1.md)|

## Development Transfer

There are other equivalents for these tools

| Existing Step | PODMAN (and friends) |
| :--- | :--- |
| `docker attach` | [`podman exec`](./docs/podman-attach.1.md)|
| `docker build`  | [`podman build`](./docs/podman-build.1.md) |
| `docker commit` | [`podman commit`](./docs/podman-commit.1.md)|
| `docker cp`     | [`podman mount`](./docs/podman-cp.1.md) ****   |
| `docker create` | [`podman create`](./docs/podman-create.1.md)  |
| `docker diff`   | [`podman diff`](./docs/podman-diff.1.md)      |
| `docker export` | [`podman export`](./docs/podman-export.1.md)  |
| `docker history`| [`podman history`](./docs/podman-history.1.md)|
| `docker images` | [`podman images`](./docs/podman-images.1.md)  |
| `docker import` | [`podman import`](./docs/podman-import.1.md)  |
| `docker kill`   | [`podman kill`](./docs/podman-kill.1.md)      |
| `docker load`   | [`podman load`](./docs/podman-load.1.md)      |
| `docker login`  | [`podman login`](./docs/podman-login.1.md)    |
| `docker logout` | [`podman logout`](./docs/podman-logout.1.md)  |
| `docker pause`  | [`podman pause`](./docs/podman-pause.1.md)    |
| `docker ps`     | [`podman ps`](./docs/podman-ps.1.md)          |
| `docker pull`   | [`podman pull`](./docs/podman-pull.1.md)      |
| `docker push`   | [`podman push`](./docs/podman-push.1.md)      |
| `docker rm`     | [`podman rm`](./docs/podman-rm.1.md)          |
| `docker rmi`    | [`podman rmi`](./docs/podman-rmi.1.md)        |
| `docker run`    | [`podman run`](./docs/podman-run.1.md)        |
| `docker save`   | [`podman save`](./docs/podman-save.1.md)      |
| `docker start`  | [`podman start`](./docs/podman-start.1.md)    |
| `docker stop`   | [`podman stop`](./docs/podman-stop.1.md)      |
| `docker tag`    | [`podman tag`](./docs/podman-tag.1.md)        |
| `docker top`    | [`podman top`](./docs/podman-top.1.md)        |
| `docker unpause`| [`podman unpause`](./docs/podman-unpause.1.md)|
| `docker version`| [`podman version`](./docs/podman-version.1.md)|
| `docker wait`   | [`podman wait`](./docs/podman-wait.1.md)   |

**** Use mount to take advantage of the entire linux tool chain rather then just cp.  Read [`here`](./docs/podman-cp.1.md) for more information.

## Missing commands in podman

Those Docker commands currently do not have equivalents in `podman`:

| Missing command | Description|
| :--- | :--- |
| `docker container`||
| `docker events`   ||
| `docker image`    ||
| `docker network`  ||
| `docker node`     ||
| `docker plugin`   |podman does not support plugins.  We recommend you use alternative OCI Runtimes or OCI Runtime Hooks to alter behaviour of podman.|
| `docker port`     ||
| `docker rename`   | podman does not support rename, you need to use `podman rm` and  `podman create` to rename a container.|
| `docker restart`  | podman does not support restart.  We recommend that you put your podman containers into a systemd unit file and use it for restarting applications.|
| `docker search`   ||
| `docker secret`   ||
| `docker service`  ||
| `docker stack`    ||
| `docker swarm`    | podman does not support swarm.  We support Kubernetes for orchestration using [CRI-O](https://github.com/kubernetes-incubator/cri-o).|
| `docker system`   ||
| `docker volume`   | podman does not support volumes.  Volumes should be built on the host operating system and then volume mounted into the containers.|

## Missing commands in Docker

The following podman commands do not have a Docker equivalent:

* [`podman mount`](./docs/podman-mount.1.md)
* [`podman umount`](./docs/podman-umount.1.md)
