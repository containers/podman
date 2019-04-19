![PODMAN logo](logo/podman-logo-source.svg)

# Library and tool for running OCI-based containers in Pods

Libpod provides a library for applications looking to use the Container Pod concept,
popularized by Kubernetes.  Libpod also contains the Pod Manager tool `(Podman)`. Podman manages pods, containers, container images, and container volumes.

* [Latest Version: 1.2.0](https://github.com/containers/libpod/releases/latest)
* [Continuous Integration:](contrib/cirrus/README.md) [![Build Status](https://api.cirrus-ci.com/github/containers/libpod.svg)](https://cirrus-ci.com/github/containers/libpod/master)

## Overview and scope

At a high level, the scope of libpod and podman is the following:

* Support multiple image formats including the OCI and Docker image formats.
* Support for multiple means to download images including trust & image verification.
* Container image management (managing image layers, overlay filesystems, etc).
* Full management of container lifecycle
* Support for pods to manage groups of containers together
* Resource isolation of containers and pods.
* Integration with CRI-O to share containers and backend code.

This project tests all builds against each supported version of Fedora, the latest released version of Red Hat Enterprise Linux, and the latest Ubuntu Long Term Support release. The community has also reported success with other Linux flavors.

## Roadmap

1. Allow the Podman CLI to use a Varlink backend to connect to remote Podman instances
1. Integrate libpod into CRI-O to replace its existing container management backend
1. Further work on the podman pod command
1. Further improvements on rootless containers

## [Shortcomings of Rootless Podman](https://github.com/containers/libpod/blob/master/rootless.md)

## Out of scope

* Specializing in signing and pushing images to various storage backends.
  See [Skopeo](https://github.com/containers/skopeo/) for those tasks.
* Container runtimes daemons for working with the Kubernetes CRI interface.
  [CRI-O](https://github.com/kubernetes-sigs/cri-o) specializes in that.
* Supporting `docker-compose`.  We believe that Kubernetes is the defacto
  standard for composing Pods and for orchestrating containers, making
  Kubernetes YAML a defacto standard file format. Hence, Podman allows the
  creation and execution of Pods from a Kubernetes YAML file (see
  [podman-play-kube](https://github.com/containers/libpod/blob/master/docs/podman-play-kube.1.md)).
  Podman can also generate Kubernetes YAML based on a container or Pod (see
  [podman-generate-kube](https://github.com/containers/libpod/blob/master/docs/podman-generate-kube.1.md)),
  which allows for an easy transition from a local development environment
  to a production Kubernetes cluster. Third-party tools might support `docker-compose` format
  like [kompose](https://github.com/kubernetes/kompose/)
  and [podman-compose](https://github.com/muayyad-alsadi/podman-compose).

## OCI Projects Plans

The plan is to use OCI projects and best of breed libraries for different aspects:
- Runtime: [runc](https://github.com/opencontainers/runc) (or any OCI compliant runtime) and [OCI runtime tools](https://github.com/opencontainers/runtime-tools) to generate the spec
- Images: Image management using [containers/image](https://github.com/containers/image)
- Storage: Container and image storage is managed by [containers/storage](https://github.com/containers/storage)
- Networking: Networking support through use of [CNI](https://github.com/containernetworking/cni)
- Builds: Builds are supported via [Buildah](https://github.com/containers/buildah).
- Conmon: [Conmon](https://github.com/kubernetes-sigs/cri-o) is a tool for monitoring OCI runtimes. It is part of the CRI-O package

## Podman Information for Developers

For blogs, release announcements and more, please checkout the [podman.io](https://podman.io) website!

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

**[Remote Client](remote_client.md)**
A brief how-to on using the Podman remote-client.

**[Release Notes](RELEASE_NOTES.md)**
Release notes for recent Podman versions

**[Contributing](CONTRIBUTING.md)**
Information about contributing to this project.

[spec-hooks]: https://github.com/opencontainers/runtime-spec/blob/v2.0.1/config.md#posix-platform-hooks

## Buildah and Podman relationship

Buildah and Podman are two complementary open-source projects that are
available on most Linux platforms and both projects reside at
[GitHub.com](https://github.com) with Buildah
[here](https://github.com/containers/buildah) and Podman
[here](https://github.com/containers/libpod).  Both, Buildah and Podman are
command line tools that work on Open Container Initiative (OCI) images and
containers.  The two projects differentiate in their specialization.

Buildah specializes in building OCI images.  Buildah's commands replicate all
of the commands that are found in a Dockerfile.  This allows building images
with and without Dockerfiles while not requiring any root privileges.
Buildah’s ultimate goal is to provide a lower-level coreutils interface to
build images.  The flexibility of building images without Dockerfiles allows
for the integration of other scripting languages into the build process.
Buildah follows a simple fork-exec model and does not run as a daemon
but it is based on a comprehensive API in golang, which can be vendored
into other tools.

Podman specializes in all of the commands and functions that help you to maintain and modify
OCI images, such as pulling and tagging.  It also allows you to create, run, and maintain those containers
created from those images.

A major difference between Podman and Buildah is their concept of a container.  Podman
allows users to create "traditional containers" where the intent of these containers is
to be long lived.  While Buildah containers are really just created to allow content
to be added back to the container image.  An easy way to think of it is the
`buildah run` command emulates the RUN command in a Dockerfile while the `podman run`
command emulates the `docker run` command in functionality.  Because of this and their underlying
storage differences, you can not see Podman containers from within Buildah or vice versa.

In short, Buildah is an efficient way to create OCI images while Podman allows
you to manage and maintain those images and containers in a production environment using
familiar container cli commands.  For more details, see the
[Container Tools Guide](https://github.com/containers/buildah/tree/master/docs/containertools).
