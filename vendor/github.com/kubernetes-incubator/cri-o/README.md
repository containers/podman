![CRI-O logo](https://cdn.rawgit.com/kubernetes-incubator/cri-o/master/logo/crio-logo.svg)
# CRI-O - OCI-based implementation of Kubernetes Container Runtime Interface

[![Build Status](https://img.shields.io/travis/kubernetes-incubator/cri-o.svg?maxAge=2592000&style=flat-square)](https://travis-ci.org/kubernetes-incubator/cri-o)
[![Go Report Card](https://goreportcard.com/badge/github.com/kubernetes-incubator/cri-o?style=flat-square)](https://goreportcard.com/report/github.com/kubernetes-incubator/cri-o)

### Status: Stable

## What is the scope of this project?

CRI-O is meant to provide an integration path between OCI conformant runtimes and the kubelet.
Specifically, it implements the Kubelet [Container Runtime Interface (CRI)](https://github.com/kubernetes/community/blob/master/contributors/devel/container-runtime-interface.md) using OCI conformant runtimes.
The scope of CRI-O is tied to the scope of the CRI.

At a high level, we expect the scope of CRI-O to be restricted to the following functionalities:

* Support multiple image formats including the existing Docker image format
* Support for multiple means to download images including trust & image verification
* Container image management (managing image layers, overlay filesystems, etc)
* Container process lifecycle management
* Monitoring and logging required to satisfy the CRI
* Resource isolation as required by the CRI

## What is not in scope for this project?

* Building, signing and pushing images to various image storages
* A CLI utility for interacting with CRI-O. Any CLIs built as part of this project are only meant for testing this project and there will be no guarantees on the backward compatibility with it.

This is an implementation of the Kubernetes Container Runtime Interface (CRI) that will allow Kubernetes to directly launch and manage Open Container Initiative (OCI) containers.

The plan is to use OCI projects and best of breed libraries for different aspects:
- Runtime: [runc](https://github.com/opencontainers/runc) (or any OCI runtime-spec implementation) and [oci runtime tools](https://github.com/opencontainers/runtime-tools)
- Images: Image management using [containers/image](https://github.com/containers/image)
- Storage: Storage and management of image layers using [containers/storage](https://github.com/containers/storage)
- Networking: Networking support through use of [CNI](https://github.com/containernetworking/cni)

It is currently in active development in the Kubernetes community through the [design proposal](https://github.com/kubernetes/kubernetes/pull/26788).  Questions and issues should be raised in the Kubernetes [sig-node Slack channel](https://kubernetes.slack.com/archives/sig-node).

## Commands
| Command                                              | Description                                                               | Demo|
| ---------------------------------------------------- | --------------------------------------------------------------------------|-----|
| [crio(8)](/docs/crio.8.md)                           | OCI Kubernetes Container Runtime daemon                                   ||
| [kpod(1)](/docs/kpod.1.md)                           | Simple management tool for pods and images                                ||
| [kpod-attach(1)](/docs/kpod-attach.1.md)             | Instead of providing a `kpod attach` command, the man page `kpod-attach` describes how to use the `kpod logs` and `kpod exec` commands to achieve the same goals as `kpod attach`.||
| [kpod-cp(1)](/docs/kpod-cp.1.md)                     | Instead of providing a `kpod cp` command, the man page `kpod-cp` describes how to use the `kpod mount` command to have even more flexibility and functionality.||
| [kpod-create(1)](/docs/kpod-create.1.md)             | Create a new container                                                    ||
| [kpod-diff(1)](/docs/kpod-diff.1.md)                 | Inspect changes on a container or image's filesystem                      ||
| [kpod-export(1)](/docs/kpod-export.1.md)             | Export container's filesystem contents as a tar archive                   |[![...](/docs/play.png)](https://asciinema.org/a/913lBIRAg5hK8asyIhhkQVLtV)|
| [kpod-history(1)](/docs/kpod-history.1.md)           | Shows the history of an image                                             |[![...](/docs/play.png)](https://asciinema.org/a/bCvUQJ6DkxInMELZdc5DinNSx)|
| [kpod-images(1)](/docs/kpod-images.1.md)             | List images in local storage                                              |[![...](/docs/play.png)](https://asciinema.org/a/133649)|
| [kpod-info(1)](/docs/kpod-info.1.md)                 | Display system information                                                ||
| [kpod-inspect(1)](/docs/kpod-inspect.1.md)           | Display the configuration of a container or image                         |[![...](/docs/play.png)](https://asciinema.org/a/133418)|
| [kpod-kill(1)](/docs/kpod-kill.1.md)                 | Kill the main process in one or more running containers                   |[![...](/docs/play.png)](https://asciinema.org/a/3jNos0A5yzO4hChu7ddKkUPw7)|
| [kpod-load(1)](/docs/kpod-load.1.md)                 | Load an image from docker archive or oci                                  |[![...](/docs/play.png)](https://asciinema.org/a/kp8kOaexEhEa20P1KLZ3L5X4g)|
| [kpod-login(1)](/docs/kpod-login.1.md)               | Login to a container registry	                                           |[![...](/docs/play.png)](https://asciinema.org/a/oNiPgmfo1FjV2YdesiLpvihtV)|
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
| [kpod-run(1)](/docs/kpod-run.1.md)                   | Run a command in a new container                                          ||
| [kpod-save(1)](/docs/kpod-save.1.md)                 | Saves an image to an archive                                              |[![...](/docs/play.png)](https://asciinema.org/a/kp8kOaexEhEa20P1KLZ3L5X4g)|
| [kpod-stats(1)](/docs/kpod-stats.1.md)               | Display a live stream of one or more containers' resource usage statistics||
| [kpod-stop(1)](/docs/kpod-stop.1.md)                 | Stops one or more running containers                                      ||
| [kpod-tag(1)](/docs/kpod-tag.1.md)                   | Add an additional name to a local image                                   |[![...](/docs/play.png)](https://asciinema.org/a/133803)|
| [kpod-umount(1)](/docs/kpod-umount.1.md)             | Unmount a working container's root filesystem                             ||
| [kpod-unpause(1)](/docs/kpod-unpause.1.md)           | Unpause one or more running containers                                    |[![...](/docs/play.png)](https://asciinema.org/a/141292)|
| [kpod-version(1)](/docs/kpod-version.1.md)           | Display the version information                                           |[![...](/docs/play.png)](https://asciinema.org/a/mfrn61pjZT9Fc8L4NbfdSqfgu)|
| [kpod-wait(1)](/docs/kpod-wait.1.md)                 | Wait on one or more containers to stop and print their exit codes||

## Configuration
| File                                       | Description                                                                                          |
| ---------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| [crio.conf(5)](/docs/crio.conf.5.md)       | CRI-O Configuation file |

## OCI Hooks Support

[CRI-O configures OCI Hooks to run when launching a container](./hooks.md)

## CRI-O Usage Transfer

[Useful information for ops and dev transfer as it relates to infrastructure that utilizes CRI-O](/transfer.md)

## Communication

For async communication and long running discussions please use issues and pull requests on the github repo. This will be the best place to discuss design and implementation.

For sync communication we have an IRC channel #CRI-O, on chat.freenode.net, that everyone is welcome to join and chat about development.

## Getting started

### Prerequisites

Latest version of `runc` is expected to be installed on the system. It is picked up as the default runtime by CRI-O.

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
  go-md2man \
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

As with other Go projects, CRI-O must be cloned into a directory structure like:

```
GOPATH
└── src
    └── github.com
        └── kubernetes-incubator
            └── cri-o
```

First, configure a `GOPATH` (if you are using go1.8 or later, this defaults to `~/go`).

```bash
export GOPATH=~/go
mkdir -p $GOPATH
```

Next, clone the source code using:

```bash
mkdir -p $GOPATH/src/github.com/kubernetes-incubator
cd $_ # or cd $GOPATH/src/github.com/kubernetes-incubator
git clone https://github.com/kubernetes-incubator/cri-o # or your fork
cd cri-o
```

### Build

```bash
make install.tools
make
sudo make install
```

Otherwise, if you do not want to build `CRI-O` with seccomp support you can add `BUILDTAGS=""` when running make.

```bash
make BUILDTAGS=""
sudo make install
```

#### Build Tags

`CRI-O` supports optional build tags for compiling support of various features.
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

Follow this [tutorial](tutorial.md) to get started with CRI-O.

### Setup CNI networking

A proper description of setting up CNI networking is given in the
[`contrib/cni` README](contrib/cni/README.md). But the gist is that you need to
have some basic network configurations enabled and CNI plugins installed on
your system.

### Running with kubernetes

You can run a local version of kubernetes with CRI-O using `local-up-cluster.sh`:

1. Clone the [kubernetes repository](https://github.com/kubernetes/kubernetes)
1. Start the CRI-O daemon (`crio`)
1. From the kubernetes project directory, run:
```shell
CGROUP_DRIVER=systemd \
CONTAINER_RUNTIME=remote \
CONTAINER_RUNTIME_ENDPOINT='/var/run/crio.sock  --runtime-request-timeout=15m' \
./hack/local-up-cluster.sh
```

To run a full cluster, see [the instructions](kubernetes.md).

### Current Roadmap

1. Basic pod/container lifecycle, basic image pull (done)
1. Support for tty handling and state management (done)
1. Basic integration with kubelet once client side changes are ready (done)
1. Support for log management, networking integration using CNI, pluggable image/storage management (done)
1. Support for exec/attach (done)
1. Target fully automated kubernetes testing without failures [e2e status](https://github.com/kubernetes-incubator/cri-o/issues/533)
1. Track upstream k8s releases
