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
| `docker run`  | [`podman run`](./docs/source/markdown/podman-run.1.md) |
| `docker exec` | [`podman exec`](./docs/source/markdown/podman-exec.1.md) |
| `docker info` | [`podman info`](./docs/source/markdown/podman-info.1.md)  |
| `docker inspect` | [`podman inspect`](./docs/source/markdown/podman-inspect.1.md)       |
| `docker logs` | [`podman logs`](./docs/source/markdown/podman-logs.1.md)                 |
| `docker ps`   | [`podman ps`](./docs/source/markdown/podman-ps.1.md) |
| `docker stats`| [`podman stats`](./docs/source/markdown/podman-stats.1.md)|

## Development Transfer

There are other equivalents for these tools

| Existing Step | `Podman` (and friends) |
| :--- | :--- |
| `docker attach`  | [`podman attach`](./docs/source/markdown/podman-attach.1.md)    |
| `docker build`   | [`podman build`](./docs/source/markdown/podman-build.1.md)      |
| `docker commit`  | [`podman commit`](./docs/source/markdown/podman-commit.1.md)    |
| `docker container`|[`podman container`](./docs/source/markdown/podman-container.1.md) |
| `docker cp`      | [`podman cp`](./docs/source/markdown/podman-cp.1.md)            |
| `docker create`  | [`podman create`](./docs/source/markdown/podman-create.1.md)    |
| `docker diff`    | [`podman diff`](./docs/source/markdown/podman-diff.1.md)        |
| `docker events`  | [`podman events`](./docs/source/markdown/podman-events.1.md)    |
| `docker export`  | [`podman export`](./docs/source/markdown/podman-export.1.md)    |
| `docker history` | [`podman history`](./docs/source/markdown/podman-history.1.md)  |
| `docker image`   | [`podman image`](./docs/source/markdown/podman-image.1.md)        |
| `docker images`  | [`podman images`](./docs/source/markdown/podman-images.1.md)    |
| `docker import`  | [`podman import`](./docs/source/markdown/podman-import.1.md)    |
| `docker kill`    | [`podman kill`](./docs/source/markdown/podman-kill.1.md)        |
| `docker load`    | [`podman load`](./docs/source/markdown/podman-load.1.md)        |
| `docker login`   | [`podman login`](./docs/source/markdown/podman-login.1.md)      |
| `docker logout`  | [`podman logout`](./docs/source/markdown/podman-logout.1.md)    |
| `docker network create`  | [`podman network create`](./docs/source/markdown/podman-network-create.1.md)   |
| `docker network inspect` | [`podman network inspect`](./docs/source/markdown/podman-network-inspect.1.md) |
| `docker network ls`      | [`podman network ls`](./docs/source/markdown/podman-network-ls.1.md)           |
| `docker network rm`      | [`podman network rm`](./docs.source/markdown/podman-network-rm.1.md)           |
| `docker pause`   | [`podman pause`](./docs/source/markdown/podman-pause.1.md)      |
| `docker port`    | [`podman port`](./docs/source/markdown/podman-port.1.md)        |
| `docker ps`      | [`podman ps`](./docs/source/markdown/podman-ps.1.md)            |
| `docker pull`    | [`podman pull`](./docs/source/markdown/podman-pull.1.md)        |
| `docker push`    | [`podman push`](./docs/source/markdown/podman-push.1.md)        |
| `docker restart` | [`podman restart`](./docs/source/markdown/podman-restart.1.md)  |
| `docker rm`      | [`podman rm`](./docs/source/markdown/podman-rm.1.md)            |
| `docker rmi`     | [`podman rmi`](./docs/source/markdown/podman-rmi.1.md)          |
| `docker run`     | [`podman run`](./docs/source/markdown/podman-run.1.md)          |
| `docker save`    | [`podman save`](./docs/source/markdown/podman-save.1.md)        |
| `docker search`  | [`podman search`](./docs/source/markdown/podman-search.1.md)    |
| `docker start`   | [`podman start`](./docs/source/markdown/podman-start.1.md)      |
| `docker stop`    | [`podman stop`](./docs/source/markdown/podman-stop.1.md)        |
| `docker system df`     | [`podman system df`](./docs/source/markdown/podman-system-df.1.md)          |
| `docker system info`   | [`podman system info`](./docs/source/markdown/podman-system-info.1.md)      |
| `docker system prune`  | [`podman system prune`](./docs/source/markdown/podman-system-prune.1.md)    |
| `docker system`        | [`podman system`](./docs/source/markdown/podman-system.1.md)                |
| `docker tag`     | [`podman tag`](./docs/source/markdown/podman-tag.1.md)          |
| `docker top`     | [`podman top`](./docs/source/markdown/podman-top.1.md)          |
| `docker unpause` | [`podman unpause`](./docs/source/markdown/podman-unpause.1.md)  |
| `docker version` | [`podman version`](./docs/source/markdown/podman-version.1.md)  |
| `docker volume create` | [`podman volume create`](./docs/source/markdown/podman-volume-create.1.md)  |
| `docker volume inspect`| [`podman volume inspect`](./docs/source/markdown/podman-volume-inspect.1.md)|
| `docker volume ls`     | [`podman volume ls`](./docs/source/markdown/podman-volume-ls.1.md)          |
| `docker volume prune`  | [`podman volume prune`](./docs/source/markdown/podman-volume-prune.1.md)    |
| `docker volume rm`     | [`podman volume rm`](./docs/source/markdown/podman-volume-rm.1.md)          |
| `docker volume`  | [`podman volume`](./docs/source/markdown/podman-volume.1.md)			|
| `docker wait`          | [`podman wait`](./docs/source/markdown/podman-wait.1.md)		       |

**** Use mount to take advantage of the entire linux tool chain rather then just cp.  Read [`here`](./docs/podman-cp.1.md) for more information.

## Behavioural differences and pitfalls

These commands behave differently from the commands in Docker:

| Command | Description |
| :--- | :--- |
| `podman volume create` | While `docker volume create` is idempotent, `podman volume create` raises an error if the volume does already exist. See this [docker issue](https://github.com/moby/moby/issues/16068) for more information.|

## Missing commands in podman

Those Docker commands currently do not have equivalents in `podman`:

| Missing command | Description|
| :--- | :--- |
| `docker container rename`   | podman does not support `container rename` - or the `rename` shorthand. We recommend using `podman rm` and  `podman create` to create a container with a specific name.|
| `docker container update`  | podman does not support altering running containers. We recommend recreating containers with the correct arguments.|
| `docker node`     ||
| `docker plugin`   | podman does not support plugins.  We recommend you use alternative OCI Runtimes or OCI Runtime Hooks to alter behavior of podman.|
| `docker secret`   ||
| `docker service`  ||
| `docker stack`    ||
| `docker swarm`    | podman does not support swarm.  We support Kubernetes for orchestration using [CRI-O](https://github.com/cri-o/cri-o).|

## Missing commands in Docker

The following podman commands do not have a Docker equivalent:

* [`podman container checkpoint`](/docs/source/markdown/podman-container-checkpoint.1.md)
* [`podman container cleanup`](/docs/source/markdown/podman-container-cleanup.1.md)
* [`podman container exists`](/docs/source/markdown/podman-container-exists.1.md)
* [`podman container refresh`](/docs/source/markdown/podman-container-refresh.1.md)
* [`podman container restore`](/docs/source/markdown/podman-container-restore.1.md)
* [`podman container runlabel`](/docs/source/markdown/podman-container-runlabel.1.md)
* [`podman generate kube`](./docs/source/markdown/podman-generate-kube.1.md)
* [`podman generate`](./docs/source/markdown/podman-generate.1.md)
* [`podman healthcheck run`](/docs/source/markdown/podman-healthcheck-run.1.md)
* [`podman image exists`](./docs/source/markdown/podman-image-exists.1.md)
* [`podman image sign`](./docs/source/markdown/podman-image-sign.1.md)
* [`podman image trust`](./docs/source/markdown/podman-image-trust.1.md)
* [`podman mount`](./docs/source/markdown/podman-mount.1.md)
* [`podman play kube`](./docs/source/markdown/podman-play-kube.1.md)
* [`podman play`](./docs/source/markdown/podman-play.1.md)
* [`podman pod create`](./docs/source/markdown/podman-pod-create.1.md)
* [`podman pod exists`](./docs/source/markdown/podman-pod-exists.1.md)
* [`podman pod inspect`](./docs/source/markdown/podman-pod-inspect.1.md)
* [`podman pod kill`](./docs/source/markdown/podman-pod-kill.1.md)
* [`podman pod pause`](./docs/source/markdown/podman-pod-pause.1.md)
* [`podman pod ps`](./docs/source/markdown/podman-pod-ps.1.md)
* [`podman pod restart`](./docs/source/markdown/podman-pod-restart.1.md)
* [`podman pod rm`](./docs/source/markdown/podman-pod-rm.1.md)
* [`podman pod start`](./docs/source/markdown/podman-pod-start.1.md)
* [`podman pod stop`](./docs/source/markdown/podman-pod-stop.1.md)
* [`podman pod top`](./docs/source/markdown/podman-pod-top.1.md)
* [`podman pod unpause`](./docs/source/markdown/podman-pod-unpause.1.md)
* [`podman pod`](./docs/source/markdown/podman-pod.1.md)
* [`podman umount`](./docs/source/markdown/podman-umount.1.md)
