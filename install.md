# libpod Installation Instructions

## Installing packaged versions of Podman

#### [Arch Linux](https://www.archlinux.org) & [Manjaro Linux](https://manjaro.org)

```bash
sudo pacman -S podman
```

If you have problems when running Podman in [rootless](README.md#rootless) mode follow the instructions [here](https://wiki.archlinux.org/index.php/Linux_Containers#Enable_support_to_run_unprivileged_containers_(optional))

#### [Debian](https://debian.org)

The libpod package is [being worked on](https://bugs.debian.org/cgi-bin/bugreport.cgi?bug=930440)
for inclusion in the default Debian repos. Relevant status updates can also be
found [here](https://github.com/containers/libpod/issues/1742).

Alternatively, the [Kubic project](https://build.opensuse.org/project/show/devel:kubic:libcontainers:stable)
provides packages for Debian 10, testing and unstable.

```bash
# Debian Unstable/Sid
echo 'deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/Debian_Unstable/ /' > /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list
wget -nv https://download.opensuse.org/repositories/devel:kubic:libcontainers:stable/Debian_Unstable/Release.key -O Release.key

# Debian Testing
echo 'deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/Debian_Testing/ /' > /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list
wget -nv https://download.opensuse.org/repositories/devel:kubic:libcontainers:stable/Debian_Testing/Release.key -O Release.key

# Debian 10
echo 'deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/Debian_10/ /' > /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list
wget -nv https://download.opensuse.org/repositories/devel:kubic:libcontainers:stable/Debian_10/Release.key -O Release.key

sudo apt-key add - < Release.key
sudo apt-get update -qq
sudo apt-get -qq -y install podman
```

There are many [packages](https://packages.debian.org/search?keywords=libpod&searchon=names&suite=stable&section=all)
with the libpod prefix available already on Debian. However, those are
unrelated to this project.


#### [Fedora](https://www.fedoraproject.org), [CentOS](https://www.centos.org)

```bash
sudo yum -y install podman
```

#### [Fedora-CoreOS](https://coreos.fedoraproject.org), [Fedora SilverBlue](https://silverblue.fedoraproject.org)

Built-in, no need to install

#### [Gentoo](https://www.gentoo.org)

```bash
sudo emerge app-emulation/libpod
```

#### [MacOS](https://www.apple.com/macos)

Using [Homebrew](https://brew.sh/):

```bash
brew cask install podman
```

#### [OpenEmbedded](https://www.openembedded.org)

Bitbake recipes for podman and its dependencies are available in the
[meta-virtualization layer](https://git.yoctoproject.org/cgit/cgit.cgi/meta-virtualization/).
Add the layer to your OpenEmbedded build environment and build podman using:

```bash
bitbake podman
```

#### [openSUSE](https://www.opensuse.org)

```bash
sudo zypper install podman
```

#### [openSUSE Kubic](https://kubic.opensuse.org)

Built-in, no need to install


#### [Raspbian](https://raspbian.org)

The Kubic project provides packages for Raspbian 10.

```bash
# Raspbian 10
echo 'deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/Raspbian_10/ /' > /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list
wget -nv https://download.opensuse.org/repositories/devel:kubic:libcontainers:stable/Raspbian_10/Release.key -O Release.key
sudo apt-key add - < Release.key
sudo apt-get update -qq
sudo apt-get -qq -y install podman
```


#### [RHEL7](https://www.redhat.com/en/technologies/linux-platforms/enterprise-linux)

Subscribe, then enable Extras channel and install Podman.

```bash
sudo subscription-manager repos --enable=rhel-7-server-extras-rpms
sudo yum -y install podman
```

#### [RHEL8 Beta](https://www.redhat.com/en/blog/powering-its-future-while-preserving-present-introducing-red-hat-enterprise-linux-8-beta?intcmp=701f2000001Cz6OAAS)

```bash
sudo yum module enable -y container-tools:1.0
sudo yum module install -y container-tools:1.0
```


#### [Ubuntu](https://www.ubuntu.com)

The Kubic project provides packages for Ubuntu 18.04, 19.04 and 19.10.

```bash
. /etc/os-release
sudo sh -c "echo 'deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/x${NAME}_${VERSION_ID}/ /' > /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list"
wget -nv https://download.opensuse.org/repositories/devel:kubic:libcontainers:stable/x${NAME}_${VERSION_ID}/Release.key -O Release.key
sudo apt-key add - < Release.key
sudo apt-get update -qq
sudo apt-get -qq -y install podman
sudo mkdir -p /etc/containers
echo -e "[registries.search]\nregistries = ['docker.io', 'quay.io']" | sudo tee /etc/containers/registries.conf
```

There are many [packages](https://packages.ubuntu.com/search?keywords=libpod&searchon=names&suite=eoan&section=all)
with the libpod prefix available already on Ubuntu. However, those are
unrelated to this project.


### Installing development versions of Podman

#### Debian

The Kubic project provides RC/testing packages for Debian 10, testing and
unstable.

```bash
# Debian Unstable/Sid
echo 'deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/testing/Debian_Unstable/ /' > /etc/apt/sources.list.d/devel:kubic:libcontainers:testing.list
wget -nv https://download.opensuse.org/repositories/devel:kubic:libcontainers:testing/Debian_Unstable/Release.key -O Release.key

# Debian Testing
echo 'deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/testing/Debian_Testing/ /' > /etc/apt/sources.list.d/devel:kubic:libcontainers:testing.list
wget -nv https://download.opensuse.org/repositories/devel:kubic:libcontainers:testing/Debian_Testing/Release.key -O Release.key

# Debian 10
echo 'deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/testing/Debian_10/ /' > /etc/apt/sources.list.d/devel:kubic:libcontainers:testing.list
wget -nv https://download.opensuse.org/repositories/devel:kubic:libcontainers:testing/Debian_10/Release.key -O Release.key

# Raspbian 10
echo 'deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/testing/Raspbian_10/ /' > /etc/apt/sources.list.d/devel:kubic:libcontainers:testing.list
wget -nv https://download.opensuse.org/repositories/devel:kubic:libcontainers:testing/Raspbian_10/Release.key -O Release.key

sudo apt-key add - < Release.key
sudo apt-get update -qq
sudo apt-get -qq -y install podman
```


#### Fedora

You can test the very latest Podman in Fedora's `updates-testing`
repository before it goes out to all Fedora users.

```console
sudo yum distro-sync --enablerepo=updates-testing podman
```

If you use a newer Podman package from Fedora's `updates-testing`, we would
appreciate your `+1` feedback in [Bodhi, Fedora's update management
system](https://bodhi.fedoraproject.org/updates/?packages=podman).

If you are running a non-rawhide Fedora distribution, you can also test the latest packages
with our [COPR repository](https://copr.fedorainfracloud.org/coprs/baude/Upstream_CRIO_Family/).

#### [Raspbian](https://raspbian.org)

The Kubic project provides RC/testing packages for Raspbian 10.

```bash
# Raspbian 10
echo 'deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/testing/Raspbian_10/ /' > /etc/apt/sources.list.d/devel:kubic:libcontainers:testing.list
wget -nv https://download.opensuse.org/repositories/devel:kubic:libcontainers:testing/Raspbian_10/Release.key -O Release.key
sudo apt-key add - < Release.key
sudo apt-get update -qq
sudo apt-get -qq -y install podman
```


#### Ubuntu

The Kubic project provides RC/testing packages for Ubuntu 18.04, 19.04 and 19.10.

```bash
. /etc/os-release
sudo sh -c "echo 'deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/testing/x${NAME}_${VERSION_ID}/ /' > /etc/apt/sources.list.d/devel:kubic:libcontainers:testing.list"
wget -nv https://download.opensuse.org/repositories/devel:kubic:libcontainers:testing/x${NAME}_${VERSION_ID}/Release.key -O Release.key
sudo apt-key add - < Release.key
sudo apt-get update -qq
sudo apt-get -qq -y install podman
```


## Building from scratch

### Build and Run Dependencies

**Required**

Fedora, CentOS, RHEL, and related distributions you should try to run
`make package-install` which will install dependencies, build the source,
produce rpms for the current platform and install them in the end.

```bash
sudo yum install -y \
  atomic-registries \
  btrfs-progs-devel \
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
  make \
  pkgconfig \
  runc \
  containers-common
```

Debian, Ubuntu, and related distributions:

```bash
sudo apt-get install \
  btrfs-tools \
  git \
  golang-go \
  go-md2man \
  iptables \
  libassuan-dev \
  libbtrfs-dev \
  libc6-dev \
  libdevmapper-dev \
  libglib2.0-dev \
  libgpgme-dev \
  libgpg-error-dev \
  libprotobuf-dev \
  libprotobuf-c0-dev \
  libseccomp-dev \
  libselinux1-dev \
  libsystemd-dev \
  pkg-config \
  runc \
  uidmap
```

On openSUSE Leap 15.x and Tumbleweed:

```bash
sudo zypper -n in libseccomp-devel libgpgme-devel
```

On Manjaro (and maybe other Linux distributions):

Make sure that the Linux kernel supports user namespaces:

```
> zgrep CONFIG_USER_NS /proc/config.gz
CONFIG_USER_NS=y

```

If not, please update the kernel.
For Manjaro Linux the instructions can be found here:
https://wiki.manjaro.org/index.php/Manjaro_Kernels

After that enable user namespaces:

```
sudo sysctl kernel.unprivileged_userns_clone=1
```

To enable the user namespaces permanently:

```
echo 'kernel.unprivileged_userns_clone=1' > /etc/sysctl.d/userns.conf
```

### Building missing dependencies

If any dependencies cannot be installed or are not sufficiently current, they have to be built from source.
This will mainly affect Debian, Ubuntu, and related distributions, or RHEL where no subscription is active (e.g. Cloud VMs).

#### golang

Be careful to double-check that the version of golang is new enough (i.e. `go version`), version 1.10.x or higher is required.
If needed, golang kits are available at https://golang.org/dl/. Alternatively, go can be built from source as follows
(it's helpful to leave the system-go installed, to avoid having to [bootstrap go](https://golang.org/doc/install/source):

```bash
export GOPATH=~/go
git clone https://go.googlesource.com/go $GOPATH
cd $GOPATH
git checkout tags/go1.10.8  # optional
cd src
./all.bash
export PATH=$GOPATH/bin:$PATH
```

#### conmon

The latest version of `conmon` is expected to be installed on the system. Conmon is used to monitor OCI Runtimes.
To build from source, use the following:

```bash
git clone https://github.com/containers/conmon
cd conmon
export GOCACHE="$(mktemp -d)"
make
sudo make podman
```

#### runc

The latest version of `runc` is expected to be installed on the system. It is picked up as the default runtime by Podman.
Version 1.0.0-rc4 is the minimal requirement, which is available in Ubuntu 18.04 already.
To double-check, `runc --version` should produce at least `spec: 1.0.1`, otherwise build your own:

```bash
git clone https://github.com/opencontainers/runc.git $GOPATH/src/github.com/opencontainers/runc
cd $GOPATH/src/github.com/opencontainers/runc
make BUILDTAGS="selinux seccomp"
sudo cp runc /usr/bin/runc
```

#### CNI plugins

#### Setup CNI networking

A proper description of setting up CNI networking is given in the [`cni` README](cni/README.md).

A basic setup for CNI networking is done by default during the installation or make processes and
no further configuration is needed to start using Podman.

#### Add configuration

```bash
sudo mkdir -p /etc/containers
sudo curl https://raw.githubusercontent.com/projectatomic/registries/master/registries.fedora -o /etc/containers/registries.conf
sudo curl https://raw.githubusercontent.com/containers/skopeo/master/default-policy.json -o /etc/containers/policy.json
```


#### Optional packages

Fedora, CentOS, RHEL, and related distributions:

(no optional packages)

Debian, Ubuntu, and related distributions:

```bash
apt-get install -y \
  libapparmor-dev
```

### Get Source Code

As with other Go projects, Podman must be cloned into a directory structure like:

```
GOPATH
└── src
    └── github.com
        └── containers
            └── libpod
```

First, ensure that the go version that is found first on the $PATH (in case you built your own; see [above](#golang)) is sufficiently recent -
`go version` must be higher than 1.10.x). Then we can finally build Podman (assuming we already have a `$GOPATH` and the corresponding folder,
`export GOPATH=~/go && mkdir -p $GOPATH`):

```bash
git clone https://github.com/containers/libpod/ $GOPATH/src/github.com/containers/libpod
cd $GOPATH/src/github.com/containers/libpod
make BUILDTAGS="selinux seccomp"
sudo make install PREFIX=/usr
```

#### Build Tags

Otherwise, if you do not want to build Podman with seccomp or selinux support you can add `BUILDTAGS=""` when running make.

```bash
make BUILDTAGS=""
sudo make install
```

Podman supports optional build tags for compiling support of various features.
To add build tags to the make option the `BUILDTAGS` variable must be set, for example:

```bash
make BUILDTAGS='seccomp apparmor'
```

| Build Tag                        | Feature                            | Dependency           |
|----------------------------------|------------------------------------|----------------------|
| apparmor                         | apparmor support                   | libapparmor          |
| exclude_graphdriver_btrfs        | exclude btrfs                      | libbtrfs             |
| exclude_graphdriver_devicemapper | exclude device-mapper              | libdm                |
| libdm_no_deferred_remove         | exclude deferred removal in libdm  | libdm                |
| seccomp                          | syscall filtering                  | libseccomp           |
| selinux                          | selinux process and mount labeling |                      |
| systemd                          | journald logging                   | libsystemd           |

Note that Podman does not officially support device-mapper. Thus, the `exclude_graphdriver_devicemapper` tag is mandatory.

### Vendoring - Dependency Management

This project is using [go modules](https://github.com/golang/go/wiki/Modules) for dependency management.  If the CI is complaining about a pull request leaving behind an unclean state, it is very likely right about it.  After changing dependencies, make sure to run `make vendor` to synchronize the code with the go module and repopulate the `./vendor` directory.

## Configuration files

### [registries.conf](https://src.fedoraproject.org/rpms/skopeo/blob/master/f/registries.conf)

#### Man Page: [registries.conf.5](https://github.com/containers/image/blob/master/docs/registries.conf.5.md)

`/etc/containers/registries.conf`

registries.conf is the configuration file which specifies which container registries should be consulted when completing image names which do not include a registry or domain portion.

#### Example from the Fedora `containers-common` package

```
cat /etc/containers/registries.conf
# This is a system-wide configuration file used to
# keep track of registries for various container backends.
# It adheres to TOML format and does not support recursive
# lists of registries.

# The default location for this configuration file is /etc/containers/registries.conf.

# The only valid categories are: 'registries.search', 'registries.insecure',
# and 'registries.block'.

[registries.search]
registries = ['docker.io', 'registry.fedoraproject.org', 'quay.io', 'registry.access.redhat.com', 'registry.centos.org']

# If you need to access insecure registries, add the registry's fully-qualified name.
# An insecure registry is one that does not have a valid SSL certificate or only does HTTP.
[registries.insecure]
registries = []


# If you need to block pull access from a registry, uncomment the section below
# and add the registries fully-qualified name.
#
[registries.block]
registries = []
```

### [mounts.conf](https://src.fedoraproject.org/rpms/skopeo/blob/master/f/mounts.conf)

`/usr/share/containers/mounts.conf` and optionally `/etc/containers/mounts.conf`

The mounts.conf files specify volume mount directories that are automatically mounted inside containers when executing the `podman run` or `podman build` commands.  Container process can then use this content.  The volume mount content does not get committed to the final image.

Usually these directories are used for passing secrets or credentials required by the package software to access remote package repositories.

For example, a mounts.conf with the line "`/usr/share/rhel/secrets:/run/secrets`", the content of `/usr/share/rhel/secrets` directory is mounted on `/run/secrets` inside the container.  This mountpoint allows Red Hat Enterprise Linux subscriptions from the host to be used within the container.

Note this is not a volume mount. The content of the volumes is copied into container storage, not bind mounted directly from the host.

#### Example from the Fedora `containers-common` package:

```
cat /usr/share/containers/mounts.conf
/usr/share/rhel/secrets:/run/secrets
```

### [seccomp.json](https://src.fedoraproject.org/rpms/skopeo/blob/master/f/seccomp.json)

`/usr/share/containers/seccomp.json`

seccomp.json contains the whitelist of seccomp rules to be allowed inside of
containers.  This file is usually provided by the containers-common package.

The link above takes you to the seccomp.json

### [policy.json](https://github.com/containers/skopeo/blob/master/default-policy.json)

`/etc/containers/policy.json`

#### Man Page: [policy.json.5](https://github.com/containers/image/blob/master/docs/policy.json.md)


#### Example from the Fedora `containers-common` package:

```
cat /etc/containers/policy.json
{
    "default": [
	{
	    "type": "insecureAcceptAnything"
	}
    ],
    "transports":
	{
	    "docker-daemon":
		{
		    "": [{"type":"insecureAcceptAnything"}]
		}
	}
}
```
