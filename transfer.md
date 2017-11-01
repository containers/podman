# CRI-O Usage Transfer

This document outlines useful information for ops and dev transfer as it relates to infrastructure that utilizes CRI-O.

## Operational Transfer

## Abstract

The `crio` daemon is intended to provide the [CRI](https://github.com/kubernetes/community/blob/master/contributors/devel/container-runtime-interface.md) socket needed for Kubernetes to use for automating deployment, scaling, and management of containerized applications (See the document for [configuring kubernetes to use CRI-O](./kubernetes.md) for more information on that).
Therefore the `crioctl` command line is a client that interfaces to the same grpc socket as the kubernetes daemon would, for talking to the `crio` daemon.
In many ways `crioctl` is only as feature rich as the Kubernetes CRI requires.
There are additional tools e.g. `kpod` and [`buildah`](https://github.com/projectatomic/buildah) that provide a feature rich set of commands for all operational needs in a Kubernetes environment.


## System Tools

Many traditional tools will still be useful, such as `pstree`, `nsenter` and `lsns`.
As well as some systemd helpers like `systemd-cgls` and `systemd-cgtop` are still just as applicable.

## Equivalents

For many troubleshooting and information collection steps, there may be an existing pattern.
Following provides equivalent with CRI-O tools for gathering information or jumping into containers, for operational use.

| Existing Step | CRI-O (and friends) |
| :---: | :---: |
| `docker exec` | [`crioctl ctr exec`](./docs/crio.8.md) |
| `docker info` | [`kpod info`](./docs/kpod-info.1.md)  |
| `docker inspect` | [`kpod inspect`](./docs/kpod-inspect.1.md)       |
| `docker logs` | [`kpod logs`](./docs/kpod-logs.1.md)                 |
| `docker ps` | [`crioctl ctr list`](./docs/crio.8.md) or [`runc list`](https://github.com/opencontainers/runc/blob/master/man/runc-list.8.md) |
| `docker stats` | [`kpod stats`](./docs/kpod-stats.1.md) or `crioctl ctr status`|

If you were already using steps like `kubectl exec` (or `oc exec` on OpenShift), they will continue to function the same way.

## Development Transfer

There are other equivalents for these tools

| Existing Step | CRI-O (and friends) |
| :---: | :---: |
| `docker attach` | [`kpod exec`](./docs/kpod-attach.1.md) ***|
| `docker build`  | [`buildah bud`](https://github.com/projectatomic/buildah/blob/master/docs/buildah-bud.md) |
| `docker cp`     | [`kpod mount`](./docs/kpod-cp.1.md) ****   |
| `docker create` | [`kpod create`](./docs/kpod-create.1.md)  |
| `docker diff`   | [`kpod diff`](./docs/kpod-diff.1.md)      |
| `docker export` | [`kpod export`](./docs/kpod-export.1.md)  |
| `docker history`| [`kpod history`](./docs/kpod-history.1.md)|
| `docker images` | [`kpod images`](./docs/kpod-images.1.md)  |
| `docker kill`   | [`kpod kill`](./docs/kpod-kill.1.md)      |
| `docker load`   | [`kpod load`](./docs/kpod-load.1.md)      |
| `docker login`  | [`kpod login`](./docs/kpod-login.1.md)    |
| `docker logout` | [`kpod logout`](./docs/kpod-logout.1.md)  |
| `docker pause`  | [`kpod pause`](./docs/kpod-pause.1.md)    |
| `docker ps`     | [`kpod ps`](./docs/kpod-ps.1.md)          |
| `docker pull`   | [`kpod pull`](./docs/kpod-pull.1.md)      |
| `docker push`   | [`kpod push`](./docs/kpod-push.1.md)      |
| `docker rename` | [`kpod rename`](./docs/kpod-rename.1.md)  |
| `docker rm`     | [`kpod rm`](./docs/kpod-rm.1.md)          |
| `docker rmi`    | [`kpod rmi`](./docs/kpod-rmi.1.md)        |
| `docker run`    | [`kpod run`](./docs/kpod-run.1.md)        |
| `docker save`   | [`kpod save`](./docs/kpod-save.1.md)      |
| `docker stop`   | [`kpod stop`](./docs/kpod-stop.1.md)      |
| `docker tag`    | [`kpod tag`](./docs/kpod-tag.1.md)        |
| `docker unpause`| [`kpod unpause`](./docs/kpod-unpause.1.md)|
| `docker version`| [`kpod version`](./docs/kpod-version.1.md)|
| `docker wait`   | [`kpod wait`](./docs/kpod-wait.1.md)   |

*** Use `kpod exec` to enter a container and `kpod logs` to view the output of pid 1 of a container.
**** Use mount to take advantage of the entire linux tool chain rather then just cp.  Read [`here`](./docs/kpod-cp.1.md) for more information.
