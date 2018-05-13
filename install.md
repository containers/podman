# libpod Installation Instructions

### Prerequisites

#### runc installed

The latest version of `runc` is expected to be installed on the system. It is picked up as the default runtime by podman.

#### conmon installed

The latest version of `conmon` is expected to be installed on the system. Conmon is used to monitor OCI Runtimes.

#### Setup CNI networking

A proper description of setting up CNI networking is given in the [`cni` README](cni/README.md).
But the gist is that you need to have some basic network configurations enabled and
CNI plugins installed on your system.

### Build and Run Dependencies

**Required**

Fedora, CentOS, RHEL, and related distributions:

```bash
yum install -y \
  atomic-registries \
  btrfs-progs-devel \
  conmon \
  containernetworking-cni \
  device-mapper-devel \
  git \
  glib2-devel \
  glibc-devel \
  glibc-static \
  go \
  golang-github-cpuguy83-go-md2man \
  gpgme-devel \
  iptables \
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
  go-md2man \
  iptables \
  libassuan-dev \
  libdevmapper-dev \
  libglib2.0-dev \
  libc6-dev \
  libgpgme11-dev \
  libgpg-error-dev \
  libprotobuf-dev \
  libprotobuf-c0-dev \
  libseccomp-dev \
  libselinux1-dev \
  pkg-config
```

Debian, Ubuntu, and related distributions will also need to do the following setup:

 * A copy of the development libraries for `ostree`, either in the form of the `libostree-dev` package from the [flatpak](https://launchpad.net/~alexlarsson/+archive/ubuntu/flatpak) PPA, or built [from source](https://github.com/ostreedev/ostree) (more on that [here](https://ostree.readthedocs.io/en/latest/#building)).
 * [Add required configuration files](https://github.com/projectatomic/libpod/blob/master/docs/tutorials/podman_tutorial.md#adding-required-configuration-files)
 * Install conman, CNI plugins and runc
   * [Install conman](https://github.com/projectatomic/libpod/blob/master/docs/tutorials/podman_tutorial.md#building-and-installing-conmon)
   * [Install CNI plugins](https://github.com/projectatomic/libpod/blob/master/docs/tutorials/podman_tutorial.md#installing-cni-plugins)
   * [runc Installation](https://github.com/projectatomic/libpod/blob/master/docs/tutorials/podman_tutorial.md#installing-runc) - Although installable, the latest runc is not available in the Ubuntu repos. Version 1.0.0-rc4 is the minimal requirement.

**NOTE**

If using an older release or a long-term support release, be careful to double-check that the version of `runc` is new enough (running `runc --version` should produce `spec: 1.0.0`), or else [build](https://github.com/projectatomic/libpod/blob/master/docs/tutorials/podman_tutorial.md#installing-runc) your own.

Be careful to double-check that the version of golang is new enough, version 1.8.x or higher is required.  If needed, golang kits are available at https://golang.org/dl/

**Optional**

Fedora, CentOS, RHEL, and related distributions:

(no optional packages)

Debian, Ubuntu, and related distributions:

```bash
apt-get install -y \
  libapparmor-dev
```

### Get Source Code

As with other Go projects, PODMAN must be cloned into a directory structure like:

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

Otherwise, if you do not want to build `podman` with seccomp support you can add `BUILDTAGS=""` when running make.

```bash
make BUILDTAGS=""
sudo make install
```

#### Build Tags

`podman` supports optional build tags for compiling support of various features.
To add build tags to the make option the `BUILDTAGS` variable must be set.

```bash
make BUILDTAGS='seccomp apparmor'
```

| Build Tag | Feature                            | Dependency  |
|-----------|------------------------------------|-------------|
| seccomp   | syscall filtering                  | libseccomp  |
| selinux   | selinux process and mount labeling | libselinux  |
| apparmor  | apparmor profile support           | libapparmor |
