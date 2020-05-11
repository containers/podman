![PODMAN logo](logo/podman-logo-source.svg)

# Library and tool for running OCI-based containers in Pods

Libpod provides a library for applications looking to use the Container Pod concept,
popularized by Kubernetes.  Libpod also contains the Pod Manager tool `(Podman)`. Podman manages pods, containers, container images, and container volumes.

* [Latest Version: 1.9.1](https://github.com/containers/libpod/releases/latest)
* [Continuous Integration:](contrib/cirrus/README.md) [![Build Status](https://api.cirrus-ci.com/github/containers/libpod.svg)](https://cirrus-ci.com/github/containers/libpod/master)
* [GoDoc: ![GoDoc](https://godoc.org/github.com/containers/libpod/libpod?status.svg)](https://godoc.org/github.com/containers/libpod/libpod)
* Automated continuous release downloads (including remote-client):
  * [Latest remote client for Windows](https://storage.googleapis.com/libpod-master-releases/podman-remote-latest-master-windows-amd64.msi)
  * [Latest remote client for MacOS](https://storage.googleapis.com/libpod-master-releases/podman-remote-latest-master-darwin-amd64.zip)
  * [Latest Snap package](https://snapcraft.io/podman)

## Overview and scope

At a high level, the scope of libpod and Podman is the following:

* Support multiple image formats including the OCI and Docker image formats.
* Support for multiple means to download images including trust & image verification.
* Container image management (managing image layers, overlay filesystems, etc).
* Full management of container lifecycle.
* Support for pods to manage groups of containers together.
* Resource isolation of containers and pods.
* Support for a Docker-compatible CLI interface through Podman.
* Support for a REST API providing both a Docker-compatible interface and an improved interface exposing advanced Podman functionality.
* Integration with CRI-O to share containers and backend code.

Podman presently only supports running containers on Linux. However, we are building a remote client which can run on Windows and OS X and manage Podman containers on a Linux system via the REST API using SSH tunneling.

## Roadmap

1. Complete the Podman REST API and Podman v2, which will be able to connect to remote Podman instances via this API
1. Integrate libpod into CRI-O to replace its existing container management backend
1. Further work on the podman pod command
1. Further improvements on rootless containers

## Communications

If you think you've identified a security issue in the project, please *DO NOT* report the issue publicly via the Github issue tracker, mailing list, or IRC.
Instead, send an email with as many details as possible to `security@lists.podman.io`. This is a private mailing list for the core maintainers.

For general questions and discussion, please use the
IRC `#podman` channel on `irc.freenode.net`.

For discussions around issues/bugs and features, you can use the GitHub
[issues](https://github.com/containers/libpod/issues)
and
[PRs](https://github.com/containers/libpod/pulls)
tracking system.

There is also a [mailing list](https://lists.podman.io/archives/) at `lists.podman.io`.
You can subscribe by sending a message to `podman-join@lists.podman.io` with the subject `subscribe`.

## Rootless
Podman can be easily run as a normal user, without requiring a setuid binary.
When run without root, Podman containers use user namespaces to set root in the container to the user running Podman.
Rootless Podman runs locked-down containers with no privileges that the user running the container does not have.
Some of these restrictions can be lifted (via `--privileged`, for example), but rootless containers will never have more privileges than the user that launched them.
If you run Podman as your user and mount in `/etc/passwd` from the host, you still won't be able to change it, since your user doesn't have permission to do so.

Almost all normal Podman functionality is available, though there are some [shortcomings](https://github.com/containers/libpod/blob/master/rootless.md).
Any recent Podman release should be able to run rootless without any additional configuration, though your operating system may require some additional configuration detailed in the [install guide](https://github.com/containers/libpod/blob/master/install.md).

A little configuration by an administrator is required before rootless Podman can be used, the necessary setup is documented [here](https://github.com/containers/libpod/blob/master/docs/tutorials/rootless_tutorial.md).

## Out of scope

* Specializing in signing and pushing images to various storage backends.
  See [Skopeo](https://github.com/containers/skopeo/) for those tasks.
* Container runtimes daemons for working with the Kubernetes CRI interface.
  [CRI-O](https://github.com/cri-o/cri-o) specializes in that.
* Supporting `docker-compose`.  We believe that Kubernetes is the defacto
  standard for composing Pods and for orchestrating containers, making
  Kubernetes YAML a defacto standard file format. Hence, Podman allows the
  creation and execution of Pods from a Kubernetes YAML file (see
  [podman-play-kube](https://github.com/containers/libpod/blob/master/docs/source/markdown/podman-play-kube.1.md)).
  Podman can also generate Kubernetes YAML based on a container or Pod (see
  [podman-generate-kube](https://github.com/containers/libpod/blob/master/docs/source/markdown/podman-generate-kube.1.md)),
  which allows for an easy transition from a local development environment
  to a production Kubernetes cluster. If Kubernetes does not fit your requirements,
  there are other third-party tools that support the docker-compose format such as
  [kompose](https://github.com/kubernetes/kompose/) and
  [podman-compose](https://github.com/muayyad-alsadi/podman-compose)
  that might be appropriate for your environment. This situation may change with
  the addition of the REST API.

## OCI Projects Plans

The plan is to use OCI projects and best of breed libraries for different aspects:
- Runtime: [runc](https://github.com/opencontainers/runc) (or any OCI compliant runtime) and [OCI runtime tools](https://github.com/opencontainers/runtime-tools) to generate the spec
- Images: Image management using [containers/image](https://github.com/containers/image)
- Storage: Container and image storage is managed by [containers/storage](https://github.com/containers/storage)
- Networking: Networking support through use of [CNI](https://github.com/containernetworking/cni)
- Builds: Builds are supported via [Buildah](https://github.com/containers/buildah).
- Conmon: [Conmon](https://github.com/containers/conmon) is a tool for monitoring OCI runtimes.

## Podman Information for Developers

For blogs, release announcements and more, please checkout the [podman.io](https://podman.io) website!

**[Installation notes](install.md)**
Information on how to install Podman in your environment.

**[OCI Hooks Support](pkg/hooks/README.md)**
Information on how Podman configures [OCI Hooks][spec-hooks] to run when launching a container.

**[Podman API](http://docs.podman.io/en/latest/_static/api.html)**
Documentation on the Podman REST API. Please note that the API is still in its early stages and not yet stable.

**[Podman Commands](https://podman.readthedocs.io/en/latest/Commands.html)**
A list of the Podman commands with links to their man pages and in many cases videos
showing the commands in use.

**[Podman Troubleshooting Guide](troubleshooting.md)**
A list of common issues and solutions for Podman.

**[Podman Usage Transfer](transfer.md)**
Useful information for ops and dev transfer as it relates to infrastructure that utilizes Podman.  This page
includes tables showing Docker commands and their Podman equivalent commands.

**[Tutorials](docs/tutorials)**
Tutorials on using Podman.

**[Remote Client](https://github.com/containers/libpod/blob/master/docs/tutorials/remote_client.md)**
A brief how-to on using the Podman remote-client.

**[Basic Setup and Use of Podman in a Rootless environment](https://github.com/containers/libpod/blob/master/docs/tutorials/rootless_tutorial.md)**
A tutorial showing the setup and configuration necessary to run Rootless Podman.

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
created from those images.  For building container images via Dockerfiles, Podman uses Buildah's
golang API and can be installed independently from Buildah.

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

## Podman Legacy API (Varlink)
Podman offers a Varlink-based API for remote management of containers.
However, this API has been deprecated by the REST API.
Varlink support is in maintenance mode, and will be removed in a future release.
For more details, you can see [this blog](https://podman.io/blogs/2020/01/17/podman-new-api.html).

## Static Binary Builds
The Cirrus CI integration within this repository contains a `static_build` job
which produces a static Podman binary for testing purposes. Please note that
this binary is not officially supported with respect to feature-completeness
and functionality and should be only used for testing.
