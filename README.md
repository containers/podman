![KPOD logo](https://cdn.rawgit.com/kubernetes-incubator/cri-o/master/logo/crio-logo.svg)
# libpod - library for running OCI-based containers in Pods

### Status: Development

## What is the scope of this project?

libpod provides a library for applications looking to use the Container Pod concept popularized by Kubernetes.
libpod also contains a tool kpod, which allows you to manage Pods, Containers, and Container Images.

At a high level, we expect the scope of libpod/kpod to the following functionalities:

* Support multiple image formats including the existing Docker/OCI image formats
* Support for multiple means to download images including trust & image verification
* Container image management (managing image layers, overlay filesystems, etc)
* Container and POD process lifecycle management
* Resource isolation of containers and PODS.

## What is not in scope for this project?

* Building container images. See Buildah
* Signing and pushing images to various image storages.  See Skopeo.
* Container Runtimes daemons for working with Kubernetes CRIs  See CRI-O.

The plan is to use OCI projects and best of breed libraries for different aspects:
- Runtime: [runc](https://github.com/opencontainers/runc) (or any OCI runtime-spec implementation) and [oci runtime tools](https://github.com/opencontainers/runtime-tools)
- Images: Image management using [containers/image](https://github.com/containers/image)
- Storage: Storage and management of image layers using [containers/storage](https://github.com/containers/storage)
- Networking: Networking support through use of [CNI](https://github.com/containernetworking/cni)

libpod is currently in active development.

## Commands
| Command                                              | Description                                                               | Demo|
| ---------------------------------------------------- | --------------------------------------------------------------------------|-----|
| [kpod(1)](/docs/kpod.1.md)                           | Simple management tool for pods and images                                ||
| [kpod-attach(1)](/docs/kpod-attach.1.md)             | Instead of providing a `kpod attach` command, the man page `kpod-attach` describes how to use the `kpod logs` and `kpod exec` commands to achieve the same goals as `kpod attach`.||
| [kpod-cp(1)](/docs/kpod-cp.1.md)                     | Instead of providing a `kpod cp` command, the man page `kpod-cp` describes how to use the `kpod mount` command to have even more flexibility and functionality.||
| [kpod-diff(1)](/docs/kpod-diff.1.md)                 | Inspect changes on a container or image's filesystem                      |[![...](/docs/play.png)](https://asciinema.org/a/FXfWB9CKYFwYM4EfqW3NSZy1G)|
| [kpod-export(1)](/docs/kpod-export.1.md)             | Export container's filesystem contents as a tar archive                   |[![...](/docs/play.png)](https://asciinema.org/a/913lBIRAg5hK8asyIhhkQVLtV)|
| [kpod-history(1)](/docs/kpod-history.1.md)           | Shows the history of an image                                             |[![...](/docs/play.png)](https://asciinema.org/a/bCvUQJ6DkxInMELZdc5DinNSx)|
| [kpod-images(1)](/docs/kpod-images.1.md)             | List images in local storage                                              |[![...](/docs/play.png)](https://asciinema.org/a/133649)|
| [kpod-info(1)](/docs/kpod-info.1.md)                 | Display system information                                                ||
| [kpod-inspect(1)](/docs/kpod-inspect.1.md)           | Display the configuration of a container or image                         |[![...](/docs/play.png)](https://asciinema.org/a/133418)|
| [kpod-kill(1)](/docs/kpod-kill.1.md)                 | Kill the main process in one or more running containers                   |[![...](/docs/play.png)](https://asciinema.org/a/3jNos0A5yzO4hChu7ddKkUPw7)|
| [kpod-load(1)](/docs/kpod-load.1.md)                 | Load an image from docker archive or oci                                  |[![...](/docs/play.png)](https://asciinema.org/a/kp8kOaexEhEa20P1KLZ3L5X4g)|
| [kpod-login(1)](/docs/kpod-login.1.md)               | Login to a container registry						   |[![...](/docs/play.png)](https://asciinema.org/a/oNiPgmfo1FjV2YdesiLpvihtV)|
| [kpod-logout(1)](/docs/kpod-logout.1.md)             | Logout of a container registry                                            |[![...](/docs/play.png)](https://asciinema.org/a/oNiPgmfo1FjV2YdesiLpvihtV)|
| [kpod-logs(1)](/docs/kpod-logs.1.md)                 | Display the logs of a container                                           ||
| [kpod-mount(1)](/docs/kpod-mount.1.md)               | Mount a working container's root filesystem                               ||
| [kpod-pause(1)](/docs/kpod-pause.1.md)               | Pause one or more running containers                                      |[![...](/docs/play.png)](https://asciinema.org/a/141292)|
| [kpod-ps(1)](/docs/kpod-ps.1.md)                     | Prints out information about containers                                   |[![...](/docs/play.png)](https://asciinema.org/a/bbT41kac6CwZ5giESmZLIaTLR)|
| [kpod-pull(1)](/docs/kpod-pull.1.md)                 | Pull an image from a registry                                             |[![...](/docs/play.png)](https://asciinema.org/a/lr4zfoynHJOUNu1KaXa1dwG2X)|
| [kpod-push(1)](/docs/kpod-push.1.md)                 | Push an image to a specified destination                                  |[![...](/docs/play.png)](https://asciinema.org/a/133276)|
| [kpod-rename(1)](/docs/kpod-rename.1.md)             | Rename a container                                                        ||
| [kpod-rm(1)](/docs/kpod-rm.1.md)                     | Removes one or more containers                                            |[![...](/docs/play.png)](https://asciinema.org/a/7EMk22WrfGtKWmgHJX9Nze1Qp)|
| [kpod-rmi(1)](/docs/kpod-rmi.1.md)                   | Removes one or more images                                                |[![...](/docs/play.png)](https://asciinema.org/a/133799)|
| [kpod-save(1)](/docs/kpod-save.1.md)                 | Saves an image to an archive                                              |[![...](/docs/play.png)](https://asciinema.org/a/kp8kOaexEhEa20P1KLZ3L5X4g)|
| [kpod-stats(1)](/docs/kpod-stats.1.md)               | Display a live stream of one or more containers' resource usage statistics||
| [kpod-stop(1)](/docs/kpod-stop.1.md)                 | Stops one or more running containers                                      ||
| [kpod-tag(1)](/docs/kpod-tag.1.md)                   | Add an additional name to a local image                                   |[![...](/docs/play.png)](https://asciinema.org/a/133803)|
| [kpod-umount(1)](/docs/kpod-umount.1.md)             | Unmount a working container's root filesystem                             ||
| [kpod-unpause(1)](/docs/kpod-unpause.1.md)           | Unpause one or more running containers                                    |[![...](/docs/play.png)](https://asciinema.org/a/141292)|
| [kpod-version(1)](/docs/kpod-version.1.md)           | Display the version information                                           |[![...](/docs/play.png)](https://asciinema.org/a/mfrn61pjZT9Fc8L4NbfdSqfgu)|
| [kpod-wait(1)](/docs/kpod-wait.1.md)                 | Wait on one or more containers to stop and print their exit codes||

## OCI Hooks Support

[KPOD configures OCI Hooks to run when launching a container](./hooks.md)

## KPOD Usage Transfer

[Useful information for ops and dev transfer as it relates to infrastructure that utilizes KPOD](/transfer.md)

## Communication

For async communication and long running discussions please use issues and pull requests on the github repo. This will be the best place to discuss design and implementation.

For sync communication we have an IRC channel #KPOD, on chat.freenode.net, that everyone is welcome to join and chat about development.

## Getting started

### Prerequisites

Latest version of `runc` is expected to be installed on the system. It is picked up as the default runtime by kpod.

### Build and Run Dependencies

**Required**

Fedora, CentOS, RHEL, and related distributions:

```bash
yum install -y \
  btrfs-progs-devel \
  device-mapper-devel \
  git \
  glib2-devel \
  glibc-devel \
  glibc-static \
  go \
  golang-github-cpuguy83-go-md2man \
  gpgme-devel \
  libassuan-devel \
  libgpg-error-devel \
  libseccomp-devel \
  libselinux-devel \
  ostree-devel \
  pkgconfig \
  runc \
  skopeo-containers
```

Debian, Ubuntu, and related distributions:

```bash
apt-get install -y \
  btrfs-tools \
  git \
  golang-go \
  libassuan-dev \
  libdevmapper-dev \
  libglib2.0-dev \
  libc6-dev \
  libgpgme11-dev \
  libgpg-error-dev \
  libseccomp-dev \
  libselinux1-dev \
  pkg-config \
  runc \
  skopeo-containers
```

Debian, Ubuntu, and related distributions will also need a copy of the development libraries for `ostree`, either in the form of the `libostree-dev` package from the [flatpak](https://launchpad.net/~alexlarsson/+archive/ubuntu/flatpak) PPA, or built [from source](https://github.com/ostreedev/ostree) (more on that [here](https://ostree.readthedocs.io/en/latest/#building)).

If using an older release or a long-term support release, be careful to double-check that the version of `runc` is new enough (running `runc --version` should produce `spec: 1.0.0`), or else build your own.

**NOTE**

Be careful to double-check that the version of golang is new enough, version 1.8.x or higher is required.  If needed, golang kits are avaliable at https://golang.org/dl/

**Optional**

Fedora, CentOS, RHEL, and related distributions:

(no optional packages)

Debian, Ubuntu, and related distributions:

```bash
apt-get install -y \
  libapparmor-dev
```

### Get Source Code

As with other Go projects, KPOD must be cloned into a directory structure like:

```
GOPATH
└── src
    └── github.com
	└── projectatomic
	    └── libpod
```

First, configure a `GOPATH` (if you are using go1.8 or later, this defaults to `~/go`).

```bash
export GOPATH=~/go
mkdir -p $GOPATH
```

Next, clone the source code using:

```bash
mkdir -p $GOPATH/src/github.com/projectatomic
cd $_ # or cd $GOPATH/src/github.com/projectatomic
git clone https://github.com/projectatomic/libpod # or your fork
cd libpod
```

### Build

```bash
make install.tools
make
sudo make install
```

Otherwise, if you do not want to build `kpod` with seccomp support you can add `BUILDTAGS=""` when running make.

```bash
make BUILDTAGS=""
sudo make install
```

#### Build Tags

`kpod` supports optional build tags for compiling support of various features.
To add build tags to the make option the `BUILDTAGS` variable must be set.

```bash
make BUILDTAGS='seccomp apparmor'
```

| Build Tag | Feature                            | Dependency  |
|-----------|------------------------------------|-------------|
| seccomp   | syscall filtering                  | libseccomp  |
| selinux   | selinux process and mount labeling | libselinux  |
| apparmor  | apparmor profile support           | libapparmor |

### Running pods and containers

Follow this [tutorial](tutorial.md) to get started with KPOD.

### Setup CNI networking

A proper description of setting up CNI networking is given in the
[`contrib/cni` README](contrib/cni/README.md). But the gist is that you need to
have some basic network configurations enabled and CNI plugins installed on
your system.

### Current Roadmap

1. Basic pod/container lifecycle, basic image pull (done)
1. Support for tty handling and state management (done)
1. Basic integration with kubelet once client side changes are ready (done)
1. Support for log management, networking integration using CNI, pluggable image/storage management (done)
1. Support for exec/attach (done)
