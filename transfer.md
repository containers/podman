![PODMAN logo](logo/podman-logo-source.svg)
# Podman Usage Transfer

This document outlines useful information for ops and dev transfer as it relates to infrastructure that utilizes `Podman`.

## Operational Transfer

## Abstract

Podman is a tool for managing Pods, Containers, and Container Images.  The CLI
for Podman is based on the Docker CLI, although Podman does not require a
runtime daemon to be running in order to function.

## System Tools

Many traditional tools will still be useful, such as `pstree`, `nsenter` and `lsns`.
As well as some systemd helpers like `systemd-cgls` and `systemd-cgtop` are still just as applicable.

## Equivalents

For many troubleshooting and information collection steps, there may be an existing pattern.
Following provides equivalent with `Podman` tools for gathering information or jumping into containers, for operational use.

| Existing Step | `Podman` (and friends) |
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

| Existing Step | `Podman` (and friends) |
| :--- | :--- |
| `docker attach`  | [`podman attach`](./docs/podman-attach.1.md)    |
| `docker cp`      | [`podman cp`](./docs/podman-cp.1.md)            |
| `docker build`   | [`podman build`](./docs/podman-build.1.md)      |
| `docker commit`  | [`podman commit`](./docs/podman-commit.1.md)    |
| `docker container`|[`podman container`](./docs/podman-container.1.md) |
| `docker create`  | [`podman create`](./docs/podman-create.1.md)    |
| `docker diff`    | [`podman diff`](./docs/podman-diff.1.md)        |
| `docker export`  | [`podman export`](./docs/podman-export.1.md)    |
| `docker history` | [`podman history`](./docs/podman-history.1.md)  |
| `docker image`   | [`podman image`](./docs/podman-image.1.md)        |
| `docker images`  | [`podman images`](./docs/podman-images.1.md)    |
| `docker import`  | [`podman import`](./docs/podman-import.1.md)    |
| `docker kill`    | [`podman kill`](./docs/podman-kill.1.md)        |
| `docker load`    | [`podman load`](./docs/podman-load.1.md)        |
| `docker login`   | [`podman login`](./docs/podman-login.1.md)      |
| `docker logout`  | [`podman logout`](./docs/podman-logout.1.md)    |
| `docker pause`   | [`podman pause`](./docs/podman-pause.1.md)      |
| `docker ps`      | [`podman ps`](./docs/podman-ps.1.md)            |
| `docker pull`    | [`podman pull`](./docs/podman-pull.1.md)        |
| `docker push`    | [`podman push`](./docs/podman-push.1.md)        |
| `docker port`    | [`podman port`](./docs/podman-port.1.md)        |
| `docker restart` | [`podman restart`](./docs/podman-restart.1.md)  |
| `docker rm`      | [`podman rm`](./docs/podman-rm.1.md)            |
| `docker rmi`     | [`podman rmi`](./docs/podman-rmi.1.md)          |
| `docker run`     | [`podman run`](./docs/podman-run.1.md)          |
| `docker save`    | [`podman save`](./docs/podman-save.1.md)        |
| `docker search`  | [`podman search`](./docs/podman-search.1.md)    |
| `docker start`   | [`podman start`](./docs/podman-start.1.md)      |
| `docker stop`    | [`podman stop`](./docs/podman-stop.1.md)        |
| `docker tag`     | [`podman tag`](./docs/podman-tag.1.md)          |
| `docker top`     | [`podman top`](./docs/podman-top.1.md)          |
| `docker unpause` | [`podman unpause`](./docs/podman-unpause.1.md)  |
| `docker version` | [`podman version`](./docs/podman-version.1.md)  |
| `docker volume`  | [`podman volume`](./docs/podman-volume.1.md)			|
| `docker volume create` | [`podman volume create`](./docs/podman-volume-create.1.md)  |
| `docker volume inspect`| [`podman volume inspect`](./docs/podman-volume-inspect.1.md)|
| `docker volume ls`     | [`podman volume ls`](./docs/podman-volume-ls.1.md)          |
| `docker volume prune`  | [`podman volume prune`](./docs/podman-volume-prune.1.md)    |
| `docker volume rm`     | [`podman volume rm`](./docs/podman-volume-rm.1.md)          |
| `docker system`        | [`podman system`](./docs/podman-system.1.md)                |
| `docker system prune`  | [`podman system prune`](./docs/podman-system-prune.1.md)    |
| `docker system info`   | [`podman system info`](./docs/podman-system-info.1.md)      |
| `docker wait`          | [`podman wait`](./docs/podman-wait.1.md)		       |

**** Use mount to take advantage of the entire linux tool chain rather then just cp.  Read [`here`](./docs/podman-cp.1.md) for more information.

## Missing commands in podman

Those Docker commands currently do not have equivalents in `podman`:

| Missing command | Description|
| :--- | :--- |
| `docker events`   ||
| `docker network`  ||
| `docker node`     ||
| `docker plugin`   | podman does not support plugins.  We recommend you use alternative OCI Runtimes or OCI Runtime Hooks to alter behavior of podman.|
| `docker rename`   | podman does not support rename, you need to use `podman rm` and  `podman create` to rename a container.|
| `docker secret`   ||
| `docker service`  ||
| `docker stack`    ||
| `docker swarm`    | podman does not support swarm.  We support Kubernetes for orchestration using [CRI-O](https://github.com/kubernetes-sigs/cri-o).|
| `docker volume`   | podman currently supports file volumes.  Future enhancement planned to support Docker Volumes Plugins

## Missing commands in Docker

The following podman commands do not have a Docker equivalent:

* [`podman generate`](./docs/podman-generate.1.md)
* [`podman generate kube`](./docs/podman-generate-kube.1.md)
* [`podman container checkpoint`](/docs/podman-container-checkpoint.1.md)
* [`podman container cleanup`](/docs/podman-container-cleanup.1.md)
* [`podman container exists`](/docs/podman-container-exists.1.md)
* [`podman container refresh`](/docs/podman-container-refresh.1.md)
* [`podman container runlabel`](/docs/podman-container-runlabel.1.md)
* [`podman container restore`](/docs/podman-container-restore.1.md)
* [`podman healthcheck run`](/docs/podman-healthcheck-run.1.md)
* [`podman image exists`](./docs/podman-image-exists.1.md)
* [`podman image sign`](./docs/podman-image-sign.1.md)
* [`podman image trust`](./docs/podman-image-trust.1.md)
* [`podman mount`](./docs/podman-mount.1.md)
* [`podman play`](./docs/podman-play.1.md)
* [`podman play kube`](./docs/podman-play-kube.1.md)
* [`podman pod`](./docs/podman-pod.1.md)
* [`podman pod create`](./docs/podman-pod-create.1.md)
* [`podman pod exists`](./docs/podman-pod-exists.1.md)
* [`podman pod inspect`](./docs/podman-pod-inspect.1.md)
* [`podman pod kill`](./docs/podman-pod-kill.1.md)
* [`podman pod pause`](./docs/podman-pod-pause.1.md)
* [`podman pod ps`](./docs/podman-pod-ps.1.md)
* [`podman pod restart`](./docs/podman-pod-restart.1.md)
* [`podman pod rm`](./docs/podman-pod-rm.1.md)
* [`podman pod start`](./docs/podman-pod-start.1.md)
* [`podman pod stop`](./docs/podman-pod-stop.1.md)
* [`podman pod top`](./docs/podman-pod-top.1.md)
* [`podman pod unpause`](./docs/podman-pod-unpause.1.md)
* [`podman varlink`](./docs/podman-varlink.1.md)
* [`podman umount`](./docs/podman-umount.1.md)
