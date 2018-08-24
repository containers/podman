![PODMAN logo](logo/podman-logo-source.svg)
# libpod - library for running OCI-based containers in Pods

### Status: Active Development

## What is the scope of this project?

libpod provides a library for applications looking to use the Container Pod concept popularized by Kubernetes.
libpod also contains a tool called podman for managing Pods, Containers, and Container Images.

At a high level, the scope of libpod and podman is the following:

* Support multiple image formats including the existing Docker/OCI image formats.
* Support for multiple means to download images including trust & image verification.
* Container image management (managing image layers, overlay filesystems, etc).
* Full management of container lifecycle
* Support for pods to manage groups of containers together
* Resource isolation of containers and pods.

## What is not in scope for this project?

* Signing and pushing images to various image storages. See [Skopeo](https://github.com/containers/skopeo/).
* Container Runtimes daemons for working with Kubernetes CRIs. See [CRI-O](https://github.com/kubernetes-incubator/cri-o). We are working to integrate libpod into CRI-O to share containers and backend code with Podman.

## OCI Projects Plans

The plan is to use OCI projects and best of breed libraries for different aspects:
- Runtime: [runc](https://github.com/opencontainers/runc) (or any OCI compliant runtime) and [oci runtime tools](https://github.com/opencontainers/runtime-tools) to generate the spec
- Images: Image management using [containers/image](https://github.com/containers/image)
- Storage: Container and image storage is managed by [containers/storage](https://github.com/containers/storage)
- Networking: Networking support through use of [CNI](https://github.com/containernetworking/cni)
- Builds: Builds are supported via [Buildah](https://github.com/projectatomic/buildah).
- Conmon: [Conmon](https://github.com/kubernetes-incubator/cri-o) is a tool for monitoring OCI runtimes. It is part of the CRI-O package

## Podman Information for Developers

**[Installation notes](install.md)**
Information on how to install Podman in your environment.

**[OCI Hooks Support](pkg/hooks/README.md)**
Information on how Podman configures [OCI Hooks][spec-hooks] to run when launching a container.

**[Podman API](API.md)**
Documentation on the Podman API using [Varlink](https://www.varlink.org/).

**[Podman Commands](commands.md)**
A list of the Podman commands with links to their man pages and in many cases videos
showing the commands in use.

**[Podman Troubleshooting Guide](troubleshooting.md)**
A list of common issues and solutions for Podman.

**[Podman Usage Transfer](transfer.md)**
Useful information for ops and dev transfer as it relates to infrastructure that utilizes Podman.  This page
includes tables showing Docker commands and their Podman equivalent commands.

**[Tutorials](docs/tutorials)**
Tutorials on using Podman.

**[Release Notes](RELEASE_NOTES.md)**
Release notes for recent Podman versions

**[Contributing](CONTRIBUTING.md)**
Information about contributing to this project.

### Current Roadmap

1. Varlink API for Podman
1. Integrate libpod into CRI-O to replace its existing container management backend
1. Pod commands for Podman
1. Rootless containers
1. Support for cleaning up containers via post-run hooks

[spec-hooks]: https://github.com/opencontainers/runtime-spec/blob/v1.0.1/config.md#posix-platform-hooks
