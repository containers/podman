![PODMAN logo](https://cdn.rawgit.com/kubernetes-incubator/cri-o/master/logo/crio-logo.svg)
# libpod - library for running OCI-based containers in Pods

### Status: Active Development

## What is the scope of this project?

libpod provides a library for applications looking to use the Container Pod concept popularized by Kubernetes.
libpod also contains a tool podman, which allows you to manage Pods, Containers, and Container Images.

At a high level, we expect the scope of libpod/podman to be the following:

* Support multiple image formats including the existing Docker/OCI image formats.
* Support for multiple means to download images including trust & image verification.
* Container image management (managing image layers, overlay filesystems, etc).
* Container and POD process lifecycle management.
* Resource isolation of containers and PODS.

## What is not in scope for this project?

* Building container images. See [Buildah](https://github.com/projectatomic/buildah).
* Signing and pushing images to various image storages. See [Skopeo](https://github.com/projectatomic/skopeo/).
* Container Runtimes daemons for working with Kubernetes CRIs. See [CRI-O](https://github.com/kubernetes-incubator/cri-o).

## OCI Projects Plans

The plan is to use OCI projects and best of breed libraries for different aspects:
- Runtime: [runc](https://github.com/opencontainers/runc) (or any OCI runtime-spec implementation) and [oci runtime tools](https://github.com/opencontainers/runtime-tools)
- Images: Image management using [containers/image](https://github.com/containers/image)
- Storage: Storage and management of image layers using [containers/storage](https://github.com/containers/storage)
- Networking: Networking support through use of [CNI](https://github.com/containernetworking/cni)
- Conmon: [conmon](https://github.com/kubernetes-incubator/cri-o) Conmon is a tool for monitoring OCI runtimes.  Part of the CRI-O package

## Podman Information for Developers

**[Installation notes](/install.md)**
Information on how to install Podman in your environment.

**[OCI Hooks Support](/hooks.md)**
Information on how Podman configures OCI Hooks to run when launching a container.

**[Podman Commands](/commands.md)**
A list of the Podman commands with links to their man pages and in many cases videos
showing the commands in use.

**[Podman Usage Transfer](/transfer.md)**
Useful information for ops and dev transfer as it relates to infrastructure that utilizes Podman.  This page
includes tables showing Docker commands and their Podman equivalent commands.

**[Tutorials](docs/tutorials)**
Tutorials on the Podman utility.

**[Contributing](CONTRIBUTING.md)**
Information about contributing to this project.

### Current Roadmap

1. Basic pod/container lifecycle, basic image pull (done)
1. Support for tty handling and state management (done)
1. Basic integration with kubelet once client side changes are ready (done)
1. Support for log management, networking integration using CNI, pluggable image/storage management (done)
1. Support for exec/attach (done)
