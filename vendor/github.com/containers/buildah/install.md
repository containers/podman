![buildah logo](https://cdn.rawgit.com/containers/buildah/master/logos/buildah-logo_large.png)

# Installation Instructions

## Installing packaged versions of buildah

### [Arch Linux](https://www.archlinux.org)

```bash
sudo pacman -S buildah
```

### [Fedora](https://www.fedoraproject.org), [CentOS](https://www.centos.org)

```bash
sudo yum -y install buildah
```

### [Fedora SilverBlue](https://silverblue.fedoraproject.org)

Installed by default

### [Fedora CoreOS](https://coreos.fedoraproject.org)

Not Available.  Must be installed via package layering.

rpm-ostree install buildah

Note: `[podman](https://podman.io) build` is available by default.

### [Gentoo](https://www.gentoo.org)

```bash
sudo emerge app-emulation/libpod
```

### [openSUSE](https://www.opensuse.org)

```bash
sudo zypper install buildah
```

### [openSUSE Kubic](https://kubic.opensuse.org)

transactional-update pkg in buildah

### [RHEL7](https://www.redhat.com/en/technologies/linux-platforms/enterprise-linux)

Subscribe, then enable Extras channel and install buildah.

```bash
sudo subscription-manager repos --enable=rhel-7-server-extras-rpms
sudo yum -y install buildah
```

### [RHEL8 Beta](https://www.redhat.com/en/blog/powering-its-future-while-preserving-present-introducing-red-hat-enterprise-linux-8-beta?intcmp=701f2000001Cz6OAAS)

```bash
sudo yum module enable -y container-tools:1.0
sudo yum module install -y buildah
```

### [Ubuntu](https://www.ubuntu.com)

```bash
sudo apt-get update -qq
sudo apt-get install -qq -y software-properties-common
sudo add-apt-repository -y ppa:projectatomic/ppa
sudo apt-get update -qq
sudo apt-get -qq -y install buildah
```

# Building from scratch

## System Requirements

### Kernel Version Requirements
To run Buildah on Red Hat Enterprise Linux or CentOS, version 7.4 or higher is required.
On other Linux distributions Buildah requires a kernel version of 4.0 or
higher in order to support the OverlayFS filesystem.  The kernel version can be checked
with the 'uname -a' command.

### runc Requirement

Buildah uses `runc` to run commands when `buildah run` is used, or when `buildah build-using-dockerfile`
encounters a `RUN` instruction, so you'll also need to build and install a compatible version of
[runc](https://github.com/opencontainers/runc) for Buildah to call for those cases.  If Buildah is installed
via a package manager such as yum, dnf or apt-get, runc will be installed as part of that process.

### CNI Requirement

When Buildah uses `runc` to run commands, it defaults to running those commands
in the host's network namespace.  If the command is being run in a separate
user namespace, though, for example when ID mapping is used, then the command
will also be run in a separate network namespace.

A newly-created network namespace starts with no network interfaces, so
commands which are run in that namespace are effectively disconnected from the
network unless additional setup is done.  Buildah relies on the CNI
[library](https://github.com/containernetworking/cni) and
[plugins](https://github.com/containernetworking/plugins) to set up interfaces
and routing for network namespaces.

If Buildah is installed via a package manager such as yum, dnf or apt-get, a
package containing CNI plugins may be available (in Fedora, the package is
named `containernetworking-cni`).  If not, they will need to be installed,
for example using:
```
  git clone https://github.com/containernetworking/plugins
  ( cd ./plugins; ./build.sh )
  mkdir -p /opt/cni/bin
  install -v ./plugins/bin/* /opt/cni/bin
```

The CNI library needs to be configured so that it will know which plugins to
call to set up namespaces.  Usually, this configuration takes the form of one
or more configuration files in the `/etc/cni/net.d` directory.  A set of example
configuration files is included in the
[`docs/cni-examples`](https://github.com/containers/buildah/tree/master/docs/cni-examples)
directory of this source tree.

## Package Installation

Buildah is available on several software repositories and can be installed via a package manager such
as yum, dnf or apt-get on a number of Linux distributions.

## Installation from GitHub

Prior to installing Buildah, install the following packages on your Linux distro:
* make
* golang (Requires version 1.10 or higher.)
* bats
* btrfs-progs-devel
* bzip2
* device-mapper-devel
* git
* go-md2man
* gpgme-devel
* glib2-devel
* libassuan-devel
* libseccomp-devel
* ostree-devel
* runc (Requires version 1.0 RC4 or higher.)
* containers-common

### Fedora

In Fedora, you can use this command:

```
 dnf -y install \
    make \
    golang \
    bats \
    btrfs-progs-devel \
    device-mapper-devel \
    glib2-devel \
    gpgme-devel \
    libassuan-devel \
    libseccomp-devel \
    ostree-devel \
    git \
    bzip2 \
    go-md2man \
    runc \
    containers-common
```

Then to install Buildah on Fedora follow the steps in this example:

```
  mkdir ~/buildah
  cd ~/buildah
  export GOPATH=`pwd`
  git clone https://github.com/containers/buildah ./src/github.com/containers/buildah
  cd ./src/github.com/containers/buildah
  make
  sudo make install
  buildah --help
```

### RHEL, CentOS

In RHEL and CentOS 7, ensure that you are subscribed to the `rhel-7-server-rpms`,
`rhel-7-server-extras-rpms`, `rhel-7-server-optional-rpms` and `EPEL` repositories, then
run this command:

```
 yum -y install \
    make \
    golang \
    bats \
    btrfs-progs-devel \
    device-mapper-devel \
    glib2-devel \
    gpgme-devel \
    libassuan-devel \
    libseccomp-devel \
    ostree-devel \
    git \
    bzip2 \
    go-md2man \
    runc \
    skopeo-containers
```

The build steps for Buildah on RHEL or CentOS are the same as for Fedora, above.

*NOTE:* Buildah on RHEL or CentOS version 7.* is not supported running as non-root due to
these systems not having newuidmap or newgidmap installed.  It is possible to pull
the shadow-utils source RPM from Fedora 29 and build and install from that in order to
run Buildah as non-root on these systems.

### openSUSE

On openSUSE Tumbleweed, install go via `zypper in go`, then run this command:

```
 zypper in make \
    git \
    golang \
    runc \
    bzip2 \
    libgpgme-devel \
    libseccomp-devel \
    device-mapper-devel \
    libbtrfs-devel \
    go-md2man
```

The build steps for Buildah on SUSE / openSUSE are the same as for Fedora, above.


### Ubuntu

In Ubuntu zesty and xenial, you can use these commands:

```
  apt-get -y install software-properties-common
  add-apt-repository -y ppa:alexlarsson/flatpak
  add-apt-repository -y ppa:gophers/archive
  apt-add-repository -y ppa:projectatomic/ppa
  apt-get -y -qq update
  apt-get -y install bats btrfs-tools git libapparmor-dev libdevmapper-dev libglib2.0-dev libgpgme11-dev libostree-dev libseccomp-dev libselinux1-dev skopeo-containers go-md2man
  apt-get -y install golang-1.10
```
Then to install Buildah on Ubuntu follow the steps in this example:

```
  mkdir ~/buildah
  cd ~/buildah
  export GOPATH=`pwd`
  git clone https://github.com/containers/buildah ./src/github.com/containers/buildah
  cd ./src/github.com/containers/buildah
  PATH=/usr/lib/go-1.10/bin:$PATH make runc all SECURITYTAGS="apparmor seccomp"
  sudo make install install.runc
  buildah --help
```

### Debian

To install the required dependencies, you can use those commands, tested under Debian GNU/Linux amd64 9.3 (stretch):

```
gpg --recv-keys 0x018BA5AD9DF57A4448F0E6CF8BECF1637AD8C79D
gpg --export 0x018BA5AD9DF57A4448F0E6CF8BECF1637AD8C79D >> /usr/share/keyrings/projectatomic-ppa.gpg
echo 'deb [signed-by=/usr/share/keyrings/projectatomic-ppa.gpg] http://ppa.launchpad.net/projectatomic/ppa/ubuntu zesty main' > /etc/apt/sources.list.d/projectatomic-ppa.list
apt update
apt -y install -t stretch-backports libostree-dev golang
apt -y install bats btrfs-tools git libapparmor-dev libdevmapper-dev libglib2.0-dev libgpgme11-dev libseccomp-dev libselinux1-dev skopeo-containers go-md2man
```

The build steps on Debian are otherwise the same as Ubuntu, above.

## Vendoring - Dependency Management

This project is using [go modules](https://github.com/golang/go/wiki/Modules) for dependency management.  If the CI is complaining about a pull request leaving behind an unclean state, it is very likely right about it.  After changing dependencies, make sure to run `make vendor` to synchronize the code with the go module and repopulate the `./vendor` directory.

## Configuration files

The following configuration files are required in order for Buildah to run appropriately.  The
majority of these files are commonly contained in the `containers-common` package.

### [registries.conf](https://github.com/containers/buildah/blob/master/docs/samples/registries.conf)

#### Man Page: [registries.conf.5](https://github.com/containers/image/blob/master/docs/containers-registries.conf.5.md)

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
# Docker only
[registries.block]
registries = []
```

### [mounts.conf](https://src.fedoraproject.org/rpms/skopeo/blob/master/f/mounts.conf)

`/usr/share/containers/mounts.conf` and optionally `/etc/containers/mounts.conf`

The mounts.conf files specify volume mount directories that are automatically mounted inside containers when executing the `buildah run` or `buildah build-using-dockerfile` commands.  Container process can then use this content.  The volume mount content does not get committed to the final image.  This file is usually provided by the containers-common package.

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

## Vendoring

Buildah uses Go Modules for vendoring purposes.  If you need to update or add a vendored package into Buildah, please follow this proceedure:
 * Enter into your sandbox `src/github.com/containers/buildah` and ensure that he GOPATH variable is set to the directory prior as noted above.
 * `export GO111MODULE=on`
 * Assuming you want to 'bump' the `github.com/containers/storage` package to version 1.12.13, use this command: `go get github.com/containers/storage@v1.12.13`
 * `make vendor`
 * `make`
 * `make install`
 * Then add any updated or added files with `git add` then do a `git commit` and create a PR.
