%global with_devel 0
%global with_bundled 1
%global with_check 0
%global with_unit_test 0

%if 0%{?fedora} || 0%{?centos} >= 8 || 0%{?rhel}
#### DO NOT REMOVE - NEEDED FOR CENTOS
%global with_debug 1
%else
%global with_debug 0
%endif

%if 0%{?with_debug}
%global _find_debuginfo_dwz_opts %{nil}
%global _dwz_low_mem_die_limit 0
%else
%global debug_package %{nil}
%endif

%if ! 0%{?gobuild:1}
%define gobuild(o:) GO111MODULE=off go build -buildmode pie -compiler gc -tags="rpm_crashtraceback ${BUILDTAGS:-}" -ldflags "${LDFLAGS:-} -B 0x$(head -c20 /dev/urandom|od -An -tx1|tr -d ' \\n') -extldflags '-Wl,-z,relro -Wl,-z,now -specs=/usr/lib/rpm/redhat/redhat-hardened-ld '" -a -v -x %{?**};
%endif

%global provider github
%global provider_tld com
%global project containers
%global repo %{name}
# https://github.com/containers/%%{name}
%global import_path %{provider}.%{provider_tld}/%{project}/%{repo}
%global git0 https://%{import_path}
# To build a random user's fork/commit, comment out above line,
# uncomment below line and replace the placeholders and commit0 below with the right info
#%%global git0 https://github.com/$GITHUB_USER/$GITHUB_USER_REPO
%global commit0 59dd35750931547c66e34e999ab960c90f18f510
%global shortcommit0 %(c=%{commit0}; echo ${c:0:7})

%global repo_plugins dnsname
# https://github.com/containers/dnsname
%global import_path_plugins %%{provider}.%{provider_tld}/%{project}/%{repo_plugins}
%global git_plugins https://%{import_path_plugins}
%global commit_plugins c654c95366ac5f309ca3e5727c9b858864247328
%global shortcommit_plugins %(c=%{commit_plugins}; echo ${c:0:7})

# Used for comparing with latest upstream tag
# to decide whether to autobuild and set download url (non-rawhide only)
%define built_tag v3.2.0-rc1
%define built_tag_strip %(b=%{built_tag}; echo ${b:1})
%define download_url %{git0}/archive/%{built_tag}.tar.gz

Name: podman
%if 0%{?fedora}
Epoch: 3
%else
Epoch: 0
%endif
Version: 3.2.0
# RELEASE TAG SHOULD ALWAYS BEGIN WITH A NUMBER
# N.foo if released, 0.N.foo if unreleased
# Rawhide almost always ships unreleased builds,
# so release tag should be of the form 0.N.foo
Release: 0.18.dev.git%{shortcommit0}%{?dist}
Summary: Manage Pods, Containers and Container Images
License: ASL 2.0
URL: https://%{name}.io/
Source0: %{git0}/archive/%{commit0}/%{name}-%{shortcommit0}.tar.gz
Source1: %{git_plugins}/archive/%{commit_plugins}/%{repo_plugins}-%{shortcommit_plugins}.tar.gz
Provides: %{name}-manpages = %{epoch}:%{version}-%{release}
# If go_compiler is not set to 1, there is no virtual provide. Use golang instead.
%if 0%{?fedora} && ! 0%{?rhel}
BuildRequires: btrfs-progs-devel
%endif
BuildRequires: gcc
BuildRequires: golang
BuildRequires: glib2-devel
BuildRequires: glibc-devel
BuildRequires: glibc-static
BuildRequires: git-core
BuildRequires: go-md2man
BuildRequires: gpgme-devel
BuildRequires: libassuan-devel
BuildRequires: libgpg-error-devel
BuildRequires: libseccomp-devel
BuildRequires: libselinux-devel
BuildRequires: pkgconfig
BuildRequires: make
BuildRequires: ostree-devel
BuildRequires: systemd
BuildRequires: systemd-devel
Requires: conmon >= 2:2.0.28-0.1
Requires: containers-common >= 4:1-17
Requires: containernetworking-plugins >= 0.9.1-1
Requires: iptables
Requires: nftables
Recommends: %{name}-plugins = %{epoch}:%{version}-%{release}
Recommends: catatonit

# vendored libraries
# awk '{print "Provides: bundled(golang("$1")) = "$2}' go.mod | sort
Provides: bundled(golang(github.com/BurntSushi/toml)) = v0.3.1
#Provides: bundled(golang(github.com/blang/semver)) = v3.5.1+incompatible
#Provides: bundled(golang(github.com/buger/goterm)) = v0.0.0-20181115115552-c206103e1f37
#Provides: bundled(golang(github.com/checkpoint-restore/go-criu)) = v0.0.0-20190109184317-bdb7599cd87b
#Provides: bundled(golang(github.com/codahale/hdrhistogram)) = v0.0.0-20161010025455-3a0bb77429bd
Provides: bundled(golang(github.com/containernetworking/cni)) = v0.8.0
Provides: bundled(golang(github.com/containernetworking/plugins)) = v0.8.7
#Provides: bundled(golang(github.com/containers/buildah)) = v1.15.1-0.20200813183340-0a8dc1f8064c
#Provides: bundled(golang(github.com/containers/common)) = v0.20.3-0.20200827091701-a550d6a98aa3
#Provides: bundled(golang(github.com/containers/conmon)) = v2.0.20+incompatible
Provides: bundled(golang(github.com/containers/image/v5)) = v5.5.2
Provides: bundled(golang(github.com/containers/psgo)) = v1.5.1
Provides: bundled(golang(github.com/containers/storage)) = v1.23.2
Provides: bundled(golang(github.com/coreos/go-systemd/v22)) = v22.1.0
Provides: bundled(golang(github.com/cri-o/ocicni)) = v0.2.0
Provides: bundled(golang(github.com/cyphar/filepath-securejoin)) = v0.2.2
Provides: bundled(golang(github.com/davecgh/go-spew)) = v1.1.1
Provides: bundled(golang(github.com/docker/distribution)) = v2.7.1+incompatible
#Provides: bundled(golang(github.com/docker/docker)) = v1.4.2-0.20191219165747-a9416c67da9f
Provides: bundled(golang(github.com/docker/go-connections)) = v0.4.0
Provides: bundled(golang(github.com/docker/go-units)) = v0.4.0
Provides: bundled(golang(github.com/fsnotify/fsnotify)) = v1.4.9
Provides: bundled(golang(github.com/ghodss/yaml)) = v1.0.0
Provides: bundled(golang(github.com/godbus/dbus/v5)) = v5.0.3
#Provides: bundled(golang(github.com/google/shlex)) = v0.0.0-20181106134648-c34317bd91bf
Provides: bundled(golang(github.com/google/uuid)) = v1.1.2
Provides: bundled(golang(github.com/gorilla/mux)) = v1.7.4
Provides: bundled(golang(github.com/gorilla/schema)) = v1.2.0
Provides: bundled(golang(github.com/hashicorp/go-multierror)) = v1.1.0
Provides: bundled(golang(github.com/hpcloud/tail)) = v1.0.0
Provides: bundled(golang(github.com/json-iterator/go)) = v1.1.10
#Provides: bundled(golang(github.com/mrunalp/fileutils)) = v0.0.0-20171103030105-7d4729fb3618
Provides: bundled(golang(github.com/onsi/ginkgo)) = v1.14.0
Provides: bundled(golang(github.com/onsi/gomega)) = v1.10.1
Provides: bundled(golang(github.com/opencontainers/go-digest)) = v1.0.0
#Provides: bundled(golang(github.com/opencontainers/image-spec)) = v1.0.2-0.20190823105129-775207bd45b6
#Provides: bundled(golang(github.com/opencontainers/runc)) = v1.0.0-rc91.0.20200708210054-ce54a9d4d79b
#Provides: bundled(golang(github.com/opencontainers/runtime-spec)) = v1.0.3-0.20200817204227-f9c09b4ea1df
Provides: bundled(golang(github.com/opencontainers/runtime-tools)) = v0.9.0
Provides: bundled(golang(github.com/opencontainers/selinux)) = v1.6.0
Provides: bundled(golang(github.com/opentracing/opentracing-go)) = v1.2.0
Provides: bundled(golang(github.com/pkg/errors)) = v0.9.1
Provides: bundled(golang(github.com/pmezard/go-difflib)) = v1.0.0
Provides: bundled(golang(github.com/rootless-containers/rootlesskit)) = v0.10.0
Provides: bundled(golang(github.com/sirupsen/logrus)) = v1.6.0
Provides: bundled(golang(github.com/spf13/cobra)) = v0.0.7
Provides: bundled(golang(github.com/spf13/pflag)) = v1.0.5
Provides: bundled(golang(github.com/stretchr/testify)) = v1.6.1
#Provides: bundled(golang(github.com/syndtr/gocapability)) = v0.0.0-20180916011248-d98352740cb2
Provides: bundled(golang(github.com/uber/jaeger-client-go)) = v2.25.0+incompatible
Provides: bundled(golang(github.com/uber/jaeger-lib)) = v2.2.0+incompatible
#Provides: bundled(golang(github.com/varlink/go)) = v0.0.0-20190502142041-0f1d566d194b
Provides: bundled(golang(github.com/vishvananda/netlink)) = v1.1.0
Provides: bundled(golang(go.etcd.io/bbolt)) = v1.3.5
#Provides: bundled(golang(golang.org/x/crypto)) = v0.0.0-20200622213623-75b288015ac9
#Provides: bundled(golang(golang.org/x/net)) = v0.0.0-20200707034311-ab3426394381
#Provides: bundled(golang(golang.org/x/sync)) = v0.0.0-20200317015054-43a5402ce75a
#Provides: bundled(golang(golang.org/x/sys)) = v0.0.0-20200728102440-3e129f6d46b1
Provides: bundled(golang(k8s.io/api)) = v0.18.8
Provides: bundled(golang(k8s.io/apimachinery)) = v0.19.0

%description
%{name} (Pod Manager) is a fully featured container engine that is a simple
daemonless tool.  %{name} provides a Docker-CLI comparable command line that
eases the transition from other container engines and allows the management of
pods, containers and images.  Simply put: alias docker=%{name}.
Most %{name} commands can be run as a regular user, without requiring
additional privileges.

%{name} uses Buildah(1) internally to create container images.
Both tools share image (not container) storage, hence each can use or
manipulate images (but not containers) created by the other.

%{summary}
%{repo} Simple management tool for pods, containers and images

%package docker
Summary: Emulate Docker CLI using %{name}
BuildArch: noarch
Requires: %{name} = %{epoch}:%{version}-%{release}
Conflicts: docker
Conflicts: docker-latest
Conflicts: docker-ce
Conflicts: docker-ee
Conflicts: moby-engine

%description docker
This package installs a script named docker that emulates the Docker CLI by
executes %{name} commands, it also creates links between all Docker CLI man
pages and %{name}.

%if 0%{?with_devel}
%package devel
Summary: Library for applications looking to use Container Pods
BuildArch: noarch
Provides: libpod-devel = %{epoch}:%{version}-%{release}

%if 0%{?with_check} && ! 0%{?with_bundled}
BuildRequires: golang(github.com/BurntSushi/toml)
BuildRequires: golang(github.com/containerd/cgroups)
BuildRequires: golang(github.com/containernetworking/plugins/pkg/ns)
BuildRequires: golang(github.com/containers/image/copy)
BuildRequires: golang(github.com/containers/image/directory)
BuildRequires: golang(github.com/containers/image/docker)
BuildRequires: golang(github.com/containers/image/docker/archive)
BuildRequires: golang(github.com/containers/image/docker/reference)
BuildRequires: golang(github.com/containers/image/docker/tarfile)
BuildRequires: golang(github.com/containers/image/image)
BuildRequires: golang(github.com/containers/image/oci/archive)
BuildRequires: golang(github.com/containers/image/pkg/strslice)
BuildRequires: golang(github.com/containers/image/pkg/sysregistries)
BuildRequires: golang(github.com/containers/image/signature)
BuildRequires: golang(github.com/containers/image/storage)
BuildRequires: golang(github.com/containers/image/tarball)
BuildRequires: golang(github.com/containers/image/transports/alltransports)
BuildRequires: golang(github.com/containers/image/types)
BuildRequires: golang(github.com/containers/storage)
BuildRequires: golang(github.com/containers/storage/pkg/archive)
BuildRequires: golang(github.com/containers/storage/pkg/idtools)
BuildRequires: golang(github.com/containers/storage/pkg/reexec)
BuildRequires: golang(github.com/coreos/go-systemd/dbus)
BuildRequires: golang(github.com/cri-o/ocicni/pkg/ocicni)
BuildRequires: golang(github.com/docker/distribution/reference)
BuildRequires: golang(github.com/docker/docker/daemon/caps)
BuildRequires: golang(github.com/docker/docker/pkg/mount)
BuildRequires: golang(github.com/docker/docker/pkg/namesgenerator)
BuildRequires: golang(github.com/docker/docker/pkg/stringid)
BuildRequires: golang(github.com/docker/docker/pkg/system)
BuildRequires: golang(github.com/docker/docker/pkg/term)
BuildRequires: golang(github.com/docker/docker/pkg/truncindex)
BuildRequires: golang(github.com/ghodss/yaml)
BuildRequires: golang(github.com/godbus/dbus)
BuildRequires: golang(github.com/mattn/go-sqlite3)
BuildRequires: golang(github.com/mrunalp/fileutils)
BuildRequires: golang(github.com/opencontainers/go-digest)
BuildRequires: golang(github.com/opencontainers/image-spec/specs-go/v1)
BuildRequires: golang(github.com/opencontainers/runc/libcontainer)
BuildRequires: golang(github.com/opencontainers/runtime-spec/specs-go)
BuildRequires: golang(github.com/opencontainers/runtime-tools/generate)
BuildRequires: golang(github.com/opencontainers/selinux/go-selinux)
BuildRequires: golang(github.com/opencontainers/selinux/go-selinux/label)
BuildRequires: golang(github.com/pkg/errors)
BuildRequires: golang(github.com/sirupsen/logrus)
BuildRequires: golang(github.com/ulule/deepcopier)
BuildRequires: golang(golang.org/x/crypto/ssh/terminal)
BuildRequires: golang(golang.org/x/sys/unix)
BuildRequires: golang(k8s.io/apimachinery/pkg/util/wait)
BuildRequires: golang(k8s.io/client-go/tools/remotecommand)
BuildRequires: golang(k8s.io/kubernetes/pkg/kubelet/container)
%endif

Requires: golang(github.com/BurntSushi/toml)
Requires: golang(github.com/containerd/cgroups)
Requires: golang(github.com/containernetworking/plugins/pkg/ns)
Requires: golang(github.com/containers/image/copy)
Requires: golang(github.com/containers/image/directory)
Requires: golang(github.com/containers/image/docker)
Requires: golang(github.com/containers/image/docker/archive)
Requires: golang(github.com/containers/image/docker/reference)
Requires: golang(github.com/containers/image/docker/tarfile)
Requires: golang(github.com/containers/image/image)
Requires: golang(github.com/containers/image/oci/archive)
Requires: golang(github.com/containers/image/pkg/strslice)
Requires: golang(github.com/containers/image/pkg/sysregistries)
Requires: golang(github.com/containers/image/signature)
Requires: golang(github.com/containers/image/storage)
Requires: golang(github.com/containers/image/tarball)
Requires: golang(github.com/containers/image/transports/alltransports)
Requires: golang(github.com/containers/image/types)
Requires: golang(github.com/containers/storage)
Requires: golang(github.com/containers/storage/pkg/archive)
Requires: golang(github.com/containers/storage/pkg/idtools)
Requires: golang(github.com/containers/storage/pkg/reexec)
Requires: golang(github.com/coreos/go-systemd/dbus)
Requires: golang(github.com/cri-o/ocicni/pkg/ocicni)
Requires: golang(github.com/docker/distribution/reference)
Requires: golang(github.com/docker/docker/daemon/caps)
Requires: golang(github.com/docker/docker/pkg/mount)
Requires: golang(github.com/docker/docker/pkg/namesgenerator)
Requires: golang(github.com/docker/docker/pkg/stringid)
Requires: golang(github.com/docker/docker/pkg/system)
Requires: golang(github.com/docker/docker/pkg/term)
Requires: golang(github.com/docker/docker/pkg/truncindex)
Requires: golang(github.com/ghodss/yaml)
Requires: golang(github.com/godbus/dbus)
Requires: golang(github.com/mattn/go-sqlite3)
Requires: golang(github.com/mrunalp/fileutils)
Requires: golang(github.com/opencontainers/go-digest)
Requires: golang(github.com/opencontainers/image-spec/specs-go/v1)
Requires: golang(github.com/opencontainers/runc/libcontainer)
Requires: golang(github.com/opencontainers/runtime-spec/specs-go)
Requires: golang(github.com/opencontainers/runtime-tools/generate)
Requires: golang(github.com/opencontainers/selinux/go-selinux)
Requires: golang(github.com/opencontainers/selinux/go-selinux/label)
Requires: golang(github.com/pkg/errors)
Requires: golang(github.com/sirupsen/logrus)
Requires: golang(github.com/ulule/deepcopier)
Requires: golang(golang.org/x/crypto/ssh/terminal)
Requires: golang(golang.org/x/sys/unix)
Requires: golang(k8s.io/apimachinery/pkg/util/wait)
Requires: golang(k8s.io/client-go/tools/remotecommand)
Requires: golang(k8s.io/kubernetes/pkg/kubelet/container)

Provides: golang(%{import_path}/cmd/%{name}/docker) = %{epoch}:%{version}-%{release}
Provides: golang(%{import_path}/cmd/%{name}/formats) = %{epoch}:%{version}-%{release}
Provides: golang(%{import_path}/libkpod) = %{epoch}:%{version}-%{release}
Provides: golang(%{import_path}/%{name}) = %{epoch}:%{version}-%{release}
Provides: golang(%{import_path}/%{name}/common) = %{epoch}:%{version}-%{release}
Provides: golang(%{import_path}/%{name}/driver) = %{epoch}:%{version}-%{release}
Provides: golang(%{import_path}/%{name}/layers) = %{epoch}:%{version}-%{release}
Provides: golang(%{import_path}/pkg/annotations) = %{epoch}:%{version}-%{release}
Provides: golang(%{import_path}/pkg/chrootuser) = %{epoch}:%{version}-%{release}
Provides: golang(%{import_path}/pkg/registrar) = %{epoch}:%{version}-%{release}
Provides: golang(%{import_path}/pkg/storage) = %{epoch}:%{version}-%{release}
Provides: golang(%{import_path}/utils) = %{epoch}:%{version}-%{release}

%description devel
%{summary}

This package contains library source intended for
building other packages which use import path with
%{import_path} prefix.
%endif

%if 0%{?with_unit_test} && 0%{?with_devel}
%package unit-test-devel
Summary:         Unit tests for %{name} package
%if 0%{?with_check}
#Here comes all BuildRequires: PACKAGE the unit tests
#in %%check section need for running
%endif

# test subpackage tests code from devel subpackage
Requires: %{name}-devel = %{epoch}:%{version}-%{release}

%if 0%{?with_check} && ! 0%{?with_bundled}
BuildRequires: golang(github.com/stretchr/testify/assert)
BuildRequires: golang(github.com/urfave/cli)
%endif

Requires: golang(github.com/stretchr/testify/assert)
Requires: golang(github.com/urfave/cli)

%description unit-test-devel
%{summary}
%{repo} provides a library for applications looking to use the
Container Pod concept popularized by Kubernetes.

This package contains unit tests for project
providing packages with %{import_path} prefix.
%endif

%if 0%{?fedora} || 0%{?rhel}
%package tests
Summary: Tests for %{name}

Requires: %{name} = %{epoch}:%{version}-%{release}
Requires: bats
Requires: jq
Requires: skopeo
Requires: nmap-ncat
Requires: httpd-tools
Requires: openssl
Requires: socat
Requires: buildah

%description tests
%{summary}

This package contains system tests for %{name}

%package remote
Summary: (Experimental) Remote client for managing %{name} containers

%description remote
Remote client for managing %{name} containers.

This experimental remote client is under heavy development. Please do not
run %{name}-remote in production.

%{name}-remote uses the version 2 API to connect to a %{name} client to
manage pods, containers and container images. %{name}-remote supports ssh
connections as well.
%endif

%package plugins
Summary: Plugins for %{name}
Requires: dnsmasq

%description plugins
This plugin sets up the use of dnsmasq on a given CNI network so
that Pods can resolve each other by name.  When configured,
the pod and its IP address are added to a network specific hosts file
that dnsmasq will read in.  Similarly, when a pod
is removed from the network, it will remove the entry from the hosts
file.  Each CNI network will have its own dnsmasq instance.

%prep
%autosetup -Sgit -n %{name}-%{commit0}
rm -f docs/source/markdown/containers-mounts.conf.5.md
sed -i 's/id128StringMax := C.ulong/id128StringMax := C.size_t/' vendor/github.com/coreos/go-systemd/v22/sdjournal/journal.go

# untar dnsname
tar zxf %{SOURCE1}

%build
export GO111MODULE=off
export GOPATH=$(pwd)/_build:$(pwd)
export CGO_CFLAGS='-O2 -g -grecord-gcc-switches -pipe -Wall -Werror=format-security -Wp,-D_FORTIFY_SOURCE=2 -specs=/usr/lib/rpm/redhat/redhat-hardened-cc1 -ffat-lto-objects -fexceptions -fasynchronous-unwind-tables -fstack-protector-strong -fstack-clash-protection -D_GNU_SOURCE -D_LARGEFILE_SOURCE -D_LARGEFILE64_SOURCE -D_FILE_OFFSET_BITS=64'
%ifarch x86_64
export CGO_CFLAGS="$CGO_CFLAGS -m64 -mtune=generic"
%if 0%{?fedora} || 0%{?centos} >= 8
export CGO_CFLAGS="$CGO_CFLAGS -fcf-protection"
%endif
%endif
# These extra flags present in %%{optflags} have been skipped for now as they break the build
#export CGO_CFLAGS="$CGO_CFLAGS -flto=auto -Wp,D_GLIBCXX_ASSERTIONS -specs=/usr/lib/rpm/redhat/redhat-annobin-cc1"

mkdir _build
pushd _build
mkdir -p src/%{provider}.%{provider_tld}/%{project}
ln -s ../../../../ src/%{import_path}
popd
ln -s vendor src

# build %%{name}
export BUILDTAGS="seccomp exclude_graphdriver_devicemapper $(hack/btrfs_installed_tag.sh) $(hack/btrfs_tag.sh) $(hack/libdm_tag.sh) $(hack/selinux_tag.sh) $(hack/systemd_tag.sh)"
%if 0%{?centos}
export BUILDTAGS+=" containers_image_ostree_stub"
%endif

# build date. FIXME: Makefile uses '/v2/libpod', that doesn't work here?
LDFLAGS="-X %{import_path}/libpod/define.buildInfo=$(date +%s)"

%gobuild -o bin/%{name} %{import_path}/cmd/%{name}

# build %%{name}-remote
export BUILDTAGS+=" exclude_graphdriver_btrfs btrfs_noversion remote"
%gobuild -o bin/%{name}-remote %{import_path}/cmd/%{name}

pushd dnsname-%{commit_plugins}
mkdir _build
pushd _build
mkdir -p src/%{provider}.%{provider_tld}/%{project}
ln -s ../../../../ src/%{import_path_plugins}
popd
ln -s vendor src
export GOPATH=$(pwd)/_build:$(pwd)
%gobuild -o bin/dnsname %{import_path_plugins}/plugins/meta/dnsname
popd

%{__make} docs docker-docs

%install
install -dp %{buildroot}%{_unitdir}
PODMAN_VERSION=%{version} %{__make} PREFIX=%{buildroot}%{_prefix} ETCDIR=%{buildroot}%{_sysconfdir} \
        install.bin-nobuild \
        install.man-nobuild \
        install.cni \
        install.systemd \
        install.completions \
        install.docker \
        install.docker-docs-nobuild \
%if 0%{?fedora} || 0%{?rhel}
        install.remote-nobuild \
%endif

mv pkg/hooks/README.md pkg/hooks/README-hooks.md

# install plugins
pushd dnsname-%{commit_plugins}
%{__make} PREFIX=%{_prefix} DESTDIR=%{buildroot} install
popd

# do not include docker and podman-remote man pages in main package
for file in `find %{buildroot}%{_mandir}/man[15] -type f | sed "s,%{buildroot},," | grep -v -e remote -e docker`; do
    echo "$file*" >> podman.file-list
done

# do not install remote manpages on centos7
%if 0%{?centos} && 0%{?centos} < 8
rm -rf %{buildroot}%{_mandir}/man1/docker-remote.1
rm -rf %{buildroot}%{_mandir}/man1/%{name}-remote.1
rm -rf %{buildroot}%{_mandir}/man5/%{name}-remote.conf.5
%endif

# source codes for building projects
%if 0%{?with_devel}
install -d -p %{buildroot}/%{gopath}/src/%{import_path}/

echo "%%dir %%{gopath}/src/%%{import_path}/." >> devel.file-list
# find all *.go but no *_test.go files and generate devel.file-list
for file in $(find . \( -iname "*.go" -or -iname "*.s" \) \! -iname "*_test.go" | grep -v "vendor") ; do
    dirprefix=$(dirname $file)
    install -d -p %{buildroot}/%{gopath}/src/%{import_path}/$dirprefix
    cp -pav $file %{buildroot}/%{gopath}/src/%{import_path}/$file
    echo "%%{gopath}/src/%%{import_path}/$file" >> devel.file-list

    while [ "$dirprefix" != "." ]; do
        echo "%%dir %%{gopath}/src/%%{import_path}/$dirprefix" >> devel.file-list
        dirprefix=$(dirname $dirprefix)
    done
done
%endif

# testing files for this project
%if 0%{?with_unit_test} && 0%{?with_devel}
install -d -p %{buildroot}/%{gopath}/src/%{import_path}/
# find all *_test.go files and generate unit-test-devel.file-list
for file in $(find . -iname "*_test.go" | grep -v "vendor") ; do
    dirprefix=$(dirname $file)
    install -d -p %{buildroot}/%{gopath}/src/%{import_path}/$dirprefix
    cp -pav $file %{buildroot}/%{gopath}/src/%{import_path}/$file
    echo "%%{gopath}/src/%%{import_path}/$file" >> unit-test-devel.file-list

    while [ "$dirprefix" != "." ]; do
        echo "%%dir %%{gopath}/src/%%{import_path}/$dirprefix" >> devel.file-list
        dirprefix=$(dirname $dirprefix)
    done
done
%endif

%if 0%{?with_devel}
sort -u -o devel.file-list devel.file-list
%endif

%check
%if 0%{?with_check} && 0%{?with_unit_test} && 0%{?with_devel}
%if ! 0%{?with_bundled}
export GOPATH=%{buildroot}/%{gopath}:%{gopath}
%else
# Since we aren't packaging up the vendor directory we need to link
# back to it somehow. Hack it up so that we can add the vendor
# directory from BUILD dir as a gopath to be searched when executing
# tests from the BUILDROOT dir.
ln -s ./ ./vendor/src # ./vendor/src -> ./vendor

export GOPATH=%{buildroot}/%{gopath}:$(pwd)/vendor:%{gopath}
%endif

%if ! 0%{?gotest:1}
%global gotest go test
%endif

%gotest %{import_path}/cmd/%{name}
%gotest %{import_path}/libkpod
%gotest %{import_path}/libpod
%gotest %{import_path}/pkg/registrar
%endif

install -d -p %{buildroot}/%{_datadir}/%{name}/test/system
cp -pav test/system %{buildroot}/%{_datadir}/%{name}/test/

%triggerpostun -- %{name} < 1.1
%{_bindir}/%{name} system renumber
exit 0

#define license tag if not already defined
%{!?_licensedir:%global license %doc}

%files -f %{name}.file-list
%license LICENSE
%doc README.md CONTRIBUTING.md pkg/hooks/README-hooks.md install.md transfer.md
%{_bindir}/%{name}
%{_datadir}/bash-completion/completions/%{name}
# By "owning" the site-functions dir, we don't need to Require zsh
%dir %{_datadir}/zsh/site-functions
%{_datadir}/zsh/site-functions/_%{name}
%dir %{_datadir}/fish/vendor_completions.d
%{_datadir}/fish/vendor_completions.d/%{name}.fish
%config(noreplace) %{_sysconfdir}/cni/net.d/87-%{name}-bridge.conflist
%{_unitdir}/%{name}-auto-update.service
%{_unitdir}/%{name}-auto-update.timer
%{_unitdir}/%{name}.service
%{_unitdir}/%{name}.socket
%{_userunitdir}/%{name}-auto-update.service
%{_userunitdir}/%{name}-auto-update.timer
%{_userunitdir}/%{name}.service
%{_userunitdir}/%{name}.socket
%{_usr}/lib/tmpfiles.d/%{name}.conf

%files docker
%{_bindir}/docker
%{_mandir}/man1/docker*.1*
%{_usr}/lib/tmpfiles.d/%{name}-docker.conf

%if 0%{?with_devel}
%files -n libpod-devel -f devel.file-list
%license LICENSE
%doc README.md CONTRIBUTING.md pkg/hooks/README-hooks.md install.md transfer.md
%dir %{gopath}/src/%{provider}.%{provider_tld}/%{project}
%endif

%if 0%{?with_unit_test} && 0%{?with_devel}
%files unit-test-devel -f unit-test-devel.file-list
%license LICENSE
%doc README.md CONTRIBUTING.md pkg/hooks/README-hooks.md install.md transfer.md
%endif

#### DO NOT REMOVE - NEEDED FOR CENTOS
%if 0%{?fedora} || 0%{?rhel}
%files remote
%license LICENSE
%{_bindir}/%{name}-remote
%{_mandir}/man1/%{name}-remote*.*
%{_datadir}/bash-completion/completions/%{name}-remote
%dir %{_datadir}/fish/vendor_completions.d
%{_datadir}/fish/vendor_completions.d/%{name}-remote.fish
%dir %{_datadir}/zsh/site-functions
%{_datadir}/zsh/site-functions/_%{name}-remote
#%%{_datadir}/man/man5/%%{name}-remote*.*

%files tests
%license LICENSE
%{_datadir}/%{name}/test
%endif

%files plugins
%license dnsname-%{commit_plugins}/LICENSE
%doc dnsname-%{commit_plugins}/{README.md,README_PODMAN.md}
%{_libexecdir}/cni/dnsname

# rhcontainerbot account currently managed by lsm5
%changelog
* Wed May 12 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 3:3.2.0-0.18.dev.git59dd357
- autobuilt 59dd357

* Tue May 11 2021 Lokesh Mandvekar <lsm5@fedoraproject.org> - 3:3.2.0-0.17.dev.git57b6425
- bump epoch to account for bad v3.2.0-rc1 in stable

* Tue May 11 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.2.0-0.16.dev.git57b6425
- autobuilt 57b6425

* Sun May 09 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.2.0-0.15.dev.git54bed10
- autobuilt 54bed10

* Sat May 08 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.2.0-0.14.dev.git141d3f1
- autobuilt 141d3f1

* Fri May 07 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.2.0-0.13.dev.git5616887
- autobuilt 5616887

* Fri May 07 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.2.0-0.12.dev.gitb533fcb
- autobuilt b533fcb

* Thu May 06 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.2.0-0.11.dev.git8cc96bd
- autobuilt 8cc96bd

* Thu May 06 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.2.0-0.10.dev.gitb6405c1
- autobuilt b6405c1

* Mon May 03 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.2.0-0.9.dev.git697ec8f
- autobuilt 697ec8f

* Wed Apr 28 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.2.0-0.8.dev.git4ca34fc
- autobuilt 4ca34fc

* Wed Apr 28 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.2.0-0.7.dev.git99e5a76
- autobuilt 99e5a76

* Tue Apr 27 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.2.0-0.6.dev.git3148e01
- autobuilt 3148e01

* Mon Apr 26 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.2.0-0.5.dev.git476c76f
- autobuilt 476c76f

* Tue Apr 20 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.2.0-0.4.dev.gitcf2c3a1
- autobuilt cf2c3a1

* Fri Apr 16 2021 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:3.2.0-0.3.dev.git35b62ef
- slirp4netns and fuse-overlayfs deps are in containers-common

* Thu Apr 15 2021 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:3.2.0-0.2.dev.git373f15f
- container-selinux and crun dependencies moved to containers-common

* Thu Apr 15 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.2.0-0.1.dev.git373f15f
- bump to 3.2.0
- autobuilt 373f15f

* Mon Apr 05 2021 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:3.1.0-0.102.dev.git259004f
- adjust dependencies

* Mon Mar 29 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.101.dev.git259004f
- autobuilt 259004f

* Thu Mar 25 2021 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:3.1.0-0.100.dev.gitdf1d561
- bump crun requirement

* Mon Mar 22 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.99.dev.gitdf1d561
- autobuilt df1d561

* Mon Mar 15 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.98.dev.gite7dc592
- autobuilt e7dc592

* Fri Mar 12 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.97.dev.gitfc02d16
- autobuilt fc02d16

* Fri Mar 12 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.96.dev.git5b22ddd
- autobuilt 5b22ddd

* Thu Mar 11 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.95.dev.git81737b3
- autobuilt 81737b3

* Thu Mar 11 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.94.dev.git8d33bfa
- autobuilt 8d33bfa

* Wed Mar 10 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.93.dev.gite2d35e5
- autobuilt e2d35e5

* Wed Mar 10 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.92.dev.git786757f
- autobuilt 786757f

* Wed Mar 10 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.91.dev.git5331096
- autobuilt 5331096

* Wed Mar 10 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.90.dev.git1ac2fb7
- autobuilt 1ac2fb7

* Tue Mar 09 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.89.dev.git09473d4
- autobuilt 09473d4

* Tue Mar 09 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.88.dev.git66ac942
- autobuilt 66ac942

* Tue Mar 09 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.87.dev.git36ec835
- autobuilt 36ec835

* Mon Mar 08 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.86.dev.git789d579
- autobuilt 789d579

* Mon Mar 08 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.85.dev.gitff46d13
- autobuilt ff46d13

* Mon Mar 08 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.84.dev.gitba36d79
- autobuilt ba36d79

* Mon Mar 08 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.83.dev.gitb386d23
- autobuilt b386d23

* Mon Mar 08 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.82.dev.git1e1035c
- autobuilt 1e1035c

* Mon Mar 08 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.81.dev.gitb6079bc
- autobuilt b6079bc

* Mon Mar 08 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.80.dev.git6fe634c
- autobuilt 6fe634c

* Mon Mar 08 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.79.dev.git7c09752
- autobuilt 7c09752

* Sun Mar 07 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.78.dev.gitb7c00f2
- autobuilt b7c00f2

* Sat Mar 06 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.77.dev.gita9fcd9d
- autobuilt a9fcd9d

* Sat Mar 06 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.76.dev.git77a597a
- autobuilt 77a597a

* Fri Mar 05 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.75.dev.git2a78157
- autobuilt 2a78157

* Fri Mar 05 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.74.dev.git44e6d20
- autobuilt 44e6d20

* Fri Mar 05 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.73.dev.git0bac30d
- autobuilt 0bac30d

* Fri Mar 05 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.72.dev.gitc6cefa5
- autobuilt c6cefa5

* Fri Mar 05 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.71.dev.git05080a1
- autobuilt 05080a1

* Thu Mar 04 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.70.dev.git4e5cc6a
- autobuilt 4e5cc6a

* Thu Mar 04 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.69.dev.gite65bcc1
- autobuilt e65bcc1

* Thu Mar 04 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.68.dev.git7a92de4
- autobuilt 7a92de4

* Thu Mar 04 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.67.dev.git87a78c0
- autobuilt 87a78c0

* Thu Mar 04 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.66.dev.git17cacea
- autobuilt 17cacea

* Wed Mar 03 2021 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:3.1.0-0.65.dev.git87e2056
- built 87e2056

* Tue Mar 02 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.64.dev.git426178a
- autobuilt 426178a

* Tue Mar 02 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.63.dev.gitc726732
- autobuilt c726732

* Tue Mar 02 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.62.dev.git7497dcb
- autobuilt 7497dcb

* Mon Mar 01 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.61.dev.git8af6680
- autobuilt 8af6680

* Mon Mar 01 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.60.dev.git73044b2
- autobuilt 73044b2

* Mon Mar 01 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.59.dev.git8daa014
- autobuilt 8daa014

* Mon Mar 01 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.58.dev.gitb5827d8
- autobuilt b5827d8

* Mon Mar 01 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.57.dev.gitb154c51
- autobuilt b154c51

* Sat Feb 27 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.56.dev.git9600ea6
- autobuilt 9600ea6

* Fri Feb 26 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.55.dev.git397aae3
- autobuilt 397aae3

* Fri Feb 26 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.54.dev.git05410e8
- autobuilt 05410e8

* Thu Feb 25 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.53.dev.gitbde1d3f
- autobuilt bde1d3f

* Thu Feb 25 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.52.dev.gitb220d6c
- autobuilt b220d6c

* Thu Feb 25 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.51.dev.git9ec8106
- autobuilt 9ec8106

* Thu Feb 25 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.50.dev.git79e8032
- autobuilt 79e8032

* Wed Feb 24 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.49.dev.git25d8195
- autobuilt 25d8195

* Wed Feb 24 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.48.dev.gitdec06b1
- autobuilt dec06b1

* Wed Feb 24 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.47.dev.git49fa19d
- autobuilt 49fa19d

* Tue Feb 23 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.46.dev.gitca0af71
- autobuilt ca0af71

* Tue Feb 23 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.45.dev.git4dfcd58
- autobuilt 4dfcd58

* Tue Feb 23 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.44.dev.git1702cbc
- autobuilt 1702cbc

* Tue Feb 23 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.43.dev.git96fc9d9
- autobuilt 96fc9d9

* Mon Feb 22 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.42.dev.gitd999328
- autobuilt d999328

* Mon Feb 22 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.41.dev.gitc69decc
- autobuilt c69decc

* Mon Feb 22 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.40.dev.gita6e7d19
- autobuilt a6e7d19

* Mon Feb 22 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.39.dev.gitf8ff172
- autobuilt f8ff172

* Mon Feb 22 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.38.dev.gitcb3af5b
- autobuilt cb3af5b

* Mon Feb 22 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.37.dev.git6fbf73e
- autobuilt 6fbf73e

* Mon Feb 22 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.36.dev.git10d52c0
- autobuilt 10d52c0

* Mon Feb 22 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.35.dev.gitd92b946
- autobuilt d92b946

* Sun Feb 21 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.34.dev.git4a6582b
- autobuilt 4a6582b

* Sun Feb 21 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.33.dev.git7b52654
- autobuilt 7b52654

* Fri Feb 19 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.32.dev.git4aaaa6c
- autobuilt 4aaaa6c

* Fri Feb 19 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.31.dev.gitb6db60e
- autobuilt b6db60e

* Fri Feb 19 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.30.dev.git6a9257a
- autobuilt 6a9257a

* Thu Feb 18 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.29.dev.git1c6c94d
- autobuilt 1c6c94d

* Thu Feb 18 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.28.dev.gitb2bb05d
- autobuilt b2bb05d

* Thu Feb 18 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.27.dev.gitc3419d2
- autobuilt c3419d2

* Wed Feb 17 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.26.dev.gitd48f4a0
- autobuilt d48f4a0

* Wed Feb 17 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.25.dev.git516dc6d
- autobuilt 516dc6d

* Wed Feb 17 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.24.dev.git2e522ff
- autobuilt 2e522ff

* Wed Feb 17 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.23.dev.gitd55d80a
- autobuilt d55d80a

* Tue Feb 16 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.22.dev.git5004212
- autobuilt 5004212

* Tue Feb 16 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.21.dev.git7fb347a
- autobuilt 7fb347a

* Tue Feb 16 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.20.dev.git58a4793
- autobuilt 58a4793

* Tue Feb 16 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.19.dev.gitaadb16d
- autobuilt aadb16d

* Tue Feb 16 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.18.dev.git8c444e6
- autobuilt 8c444e6

* Tue Feb 16 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.17.dev.gitac9a048
- autobuilt ac9a048

* Tue Feb 16 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.16.dev.gitdf8ba7f
- autobuilt df8ba7f

* Mon Feb 15 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.15.dev.git30607d7
- autobuilt 30607d7

* Sat Feb 13 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.14.dev.git3ba0afd
- autobuilt 3ba0afd

* Sat Feb 13 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.13.dev.git9d57aa7
- autobuilt 9d57aa7

* Fri Feb 12 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.12.dev.git87b2722
- autobuilt 87b2722

* Fri Feb 12 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.11.dev.git1d15ed7
- autobuilt 1d15ed7

* Fri Feb 12 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.10.dev.git73cf06a
- autobuilt 73cf06a

* Fri Feb 12 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.9.dev.git291f596
- autobuilt 291f596

* Thu Feb 11 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.8.dev.git1b284a2
- autobuilt 1b284a2

* Thu Feb 11 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.7.dev.gitb38b143
- autobuilt b38b143

* Thu Feb 11 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.6.dev.gita500d93
- autobuilt a500d93

* Thu Feb 11 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.5.dev.gitafe4ce6
- autobuilt afe4ce6

* Thu Feb 11 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.4.dev.gitca354f1
- autobuilt ca354f1

* Thu Feb 11 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.3.dev.gitdb64865
- autobuilt db64865

* Wed Feb 10 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.2.dev.git4d604c1
- autobuilt 4d604c1

* Wed Feb 10 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.1.0-0.1.dev.git88ab83d
- bump to 3.1.0
- autobuilt 88ab83d

* Wed Feb 10 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.213.dev.gitb4ca924
- autobuilt b4ca924

* Wed Feb 10 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.212.dev.git629a979
- autobuilt 629a979

* Wed Feb 10 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.211.dev.git055e2dd
- autobuilt 055e2dd

* Wed Feb 10 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.210.dev.git2d829ae
- autobuilt 2d829ae

* Tue Feb 09 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.209.dev.git8600c3b
- autobuilt 8600c3b

* Tue Feb 09 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.208.dev.git763d522
- autobuilt 763d522

* Tue Feb 09 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.207.dev.gitf98605e
- autobuilt f98605e

* Tue Feb 09 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.206.dev.git9da4169
- autobuilt 9da4169

* Mon Feb 08 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.205.dev.git19507d0
- autobuilt 19507d0

* Wed Feb 03 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.204.dev.gita086f60
- autobuilt a086f60

* Wed Feb 03 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.203.dev.git9742165
- autobuilt 9742165

* Tue Feb 02 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.202.dev.gitd1e0afd
- autobuilt d1e0afd

* Tue Feb 02 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.201.dev.gitaab8a93
- autobuilt aab8a93

* Tue Feb 02 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.200.dev.git628b0d7
- autobuilt 628b0d7

* Tue Feb 02 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.199.dev.gitd66a18c
- autobuilt d66a18c

* Tue Feb 02 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.198.dev.git828279d
- autobuilt 828279d

* Tue Feb 02 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.197.dev.git2314af7
- autobuilt 2314af7

* Tue Feb 02 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.196.dev.git52575db
- autobuilt 52575db

* Mon Feb 01 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.195.dev.git48a0e00
- autobuilt 48a0e00

* Mon Feb 01 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.194.dev.git182e841
- autobuilt 182e841

* Mon Feb 01 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.193.dev.git2018334
- autobuilt 2018334

* Mon Feb 01 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.192.dev.gitb045c17
- autobuilt b045c17

* Mon Feb 01 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.191.dev.git4ead806
- autobuilt 4ead806

* Sat Jan 30 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.190.dev.git735b16e
- autobuilt 735b16e

* Fri Jan 29 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.189.dev.git2686e40
- autobuilt 2686e40

* Fri Jan 29 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.188.dev.gitf3a7bc1
- autobuilt f3a7bc1

* Fri Jan 29 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.187.dev.git4ee66c2
- autobuilt 4ee66c2

* Thu Jan 28 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.186.dev.git0c6a889
- autobuilt 0c6a889

* Thu Jan 28 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.185.dev.git2ee034c
- autobuilt 2ee034c

* Thu Jan 28 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.184.dev.gitfb653c4
- autobuilt fb653c4

* Wed Jan 27 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.183.dev.git9d59daa
- autobuilt 9d59daa

* Wed Jan 27 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.182.dev.git14cc4aa
- autobuilt 14cc4aa

* Wed Jan 27 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.181.dev.git1814fa2
- autobuilt 1814fa2

* Wed Jan 27 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.180.dev.git2ff4da9
- autobuilt 2ff4da9

* Wed Jan 27 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.179.dev.git179b9d1
- autobuilt 179b9d1

* Wed Jan 27 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.178.dev.git2102e26
- autobuilt 2102e26

* Wed Jan 27 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.177.dev.gitc3b3984
- autobuilt c3b3984

* Tue Jan 26 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.176.dev.git5d44446
- autobuilt 5d44446

* Tue Jan 26 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.175.dev.gitad1e0bb
- autobuilt ad1e0bb

* Tue Jan 26 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.174.dev.gitf13385e
- autobuilt f13385e

* Tue Jan 26 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.173.dev.gitefcd48b
- autobuilt efcd48b

* Tue Jan 26 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.172.dev.gite5e447d
- autobuilt e5e447d

* Tue Jan 26 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.171.dev.git79565d1
- autobuilt 79565d1

* Mon Jan 25 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.170.dev.git6ba8819
- autobuilt 6ba8819

* Mon Jan 25 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.169.dev.git63cef43
- autobuilt 63cef43

* Mon Jan 25 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.168.dev.git23b879d
- autobuilt 23b879d

* Mon Jan 25 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.167.dev.gitf4e8572
- autobuilt f4e8572

* Mon Jan 25 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.166.dev.gitb4b7838
- autobuilt b4b7838

* Sun Jan 24 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.165.dev.git479fc22
- autobuilt 479fc22

* Sat Jan 23 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.164.dev.git3f5af4e
- autobuilt 3f5af4e

* Sat Jan 23 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.163.dev.git6cef7c7
- autobuilt 6cef7c7

* Fri Jan 22 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.162.dev.git474ba4c
- autobuilt 474ba4c

* Fri Jan 22 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.161.dev.git47616fe
- autobuilt 47616fe

* Thu Jan 21 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.160.dev.git6fd83de
- autobuilt 6fd83de

* Thu Jan 21 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.159.dev.git3ba1a8d
- autobuilt 3ba1a8d

* Thu Jan 21 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.158.dev.gitd102d02
- autobuilt d102d02

* Thu Jan 21 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.157.dev.git7d297dd
- autobuilt 7d297dd

* Thu Jan 21 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.156.dev.git5598229
- autobuilt 5598229

* Wed Jan 20 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.155.dev.git14443cc
- autobuilt 14443cc

* Wed Jan 20 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.154.dev.gitfe4f9ba
- autobuilt fe4f9ba

* Wed Jan 20 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.153.dev.git7d024a2
- autobuilt 7d024a2

* Wed Jan 20 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.152.dev.git54c465b
- autobuilt 54c465b

* Tue Jan 19 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.151.dev.git5e7262d
- autobuilt 5e7262d

* Tue Jan 19 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.150.dev.gitd99e475
- autobuilt d99e475

* Tue Jan 19 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.149.dev.git8c6df5e
- autobuilt 8c6df5e

* Tue Jan 19 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.148.dev.git9a10f20
- autobuilt 9a10f20

* Mon Jan 18 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.147.dev.git5f1a7a7
- autobuilt 5f1a7a7

* Sun Jan 17 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.146.dev.git5b3c7a5
- autobuilt 5b3c7a5

* Sun Jan 17 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.145.dev.git341c4b1
- autobuilt 341c4b1

* Sat Jan 16 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.144.dev.git73b036d
- autobuilt 73b036d

* Fri Jan 15 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.143.dev.git83ed464
- autobuilt 83ed464

* Fri Jan 15 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.142.dev.git0400dc0
- autobuilt 0400dc0

* Fri Jan 15 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.141.dev.git7d3a628
- autobuilt 7d3a628

* Fri Jan 15 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.140.dev.git5a166b2
- autobuilt 5a166b2

* Fri Jan 15 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.139.dev.git3ceef00
- autobuilt 3ceef00

* Fri Jan 15 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.138.dev.git3fcf346
- autobuilt 3fcf346

* Thu Jan 14 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.137.dev.git2b7793b
- autobuilt 2b7793b

* Thu Jan 14 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.136.dev.gita944f90
- autobuilt a944f90

* Thu Jan 14 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.135.dev.git9f50d48
- autobuilt 9f50d48

* Thu Jan 14 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.134.dev.git4e4477c
- autobuilt 4e4477c

* Wed Jan 13 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.133.dev.gitb2ac2a3
- autobuilt b2ac2a3

* Wed Jan 13 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.132.dev.gitbbff9c8
- autobuilt bbff9c8

* Wed Jan 13 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.131.dev.git9473dda
- autobuilt 9473dda

* Wed Jan 13 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.130.dev.git99c5746
- autobuilt 99c5746

* Wed Jan 13 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.129.dev.git183f443
- autobuilt 183f443

* Wed Jan 13 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.128.dev.gitf52a9ee
- autobuilt f52a9ee

* Tue Jan 12 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.127.dev.git265ec91
- autobuilt 265ec91

* Tue Jan 12 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.126.dev.git0ccc888
- autobuilt 0ccc888

* Tue Jan 12 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.125.dev.git0532fda
- autobuilt 0532fda

* Tue Jan 12 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.124.dev.git64b86d0
- autobuilt 64b86d0

* Tue Jan 12 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.123.dev.git5575c7b
- autobuilt 5575c7b

* Mon Jan 11 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.122.dev.git5681907
- autobuilt 5681907

* Mon Jan 11 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.121.dev.git20217f5
- autobuilt 20217f5

* Mon Jan 11 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.120.dev.gitd2503ae
- autobuilt d2503ae

* Mon Jan 11 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.119.dev.git3b987a7
- autobuilt 3b987a7

* Sun Jan 10 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.118.dev.git41613bd
- autobuilt 41613bd

* Sun Jan 10 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.117.dev.gitbc0fa65
- autobuilt bc0fa65

* Fri Jan  8 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.116.dev.git49db79e
- autobuilt 49db79e

* Fri Jan  8 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.115.dev.gita0b432d
- autobuilt a0b432d

* Thu Jan  7 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.114.dev.git78cda71
- autobuilt 78cda71

* Thu Jan  7 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.113.dev.git3cf41c4
- autobuilt 3cf41c4

* Thu Jan  7 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.112.dev.gita475150
- autobuilt a475150

* Thu Jan  7 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.111.dev.git355e387
- autobuilt 355e387

* Wed Jan  6 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.110.dev.gitbb82c37
- autobuilt bb82c37

* Tue Jan  5 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.109.dev.gitffe2b1e
- autobuilt ffe2b1e

* Tue Jan  5 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.108.dev.git1f59276
- autobuilt 1f59276

* Tue Jan  5 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.107.dev.gitb84b7c8
- autobuilt b84b7c8

* Tue Jan  5 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.106.dev.gitbc21fab
- autobuilt bc21fab

* Tue Jan  5 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.105.dev.git1b9366d
- autobuilt 1b9366d

* Tue Jan  5 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.104.dev.git618c355
- autobuilt 618c355

* Mon Jan  4 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.103.dev.gitced7c0a
- autobuilt ced7c0a

* Mon Jan  4 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.102.dev.gitb502854
- autobuilt b502854

* Mon Jan  4 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.101.dev.git6a1fbe7
- autobuilt 6a1fbe7

* Mon Jan  4 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.100.dev.gitf261bfc
- autobuilt f261bfc

* Mon Jan  4 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.99.dev.git8e4d19d
- autobuilt 8e4d19d

* Mon Jan  4 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.98.dev.git23f25b8
- autobuilt 23f25b8

* Sat Jan  2 2021 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.97.dev.git142b4ac
- autobuilt 142b4ac

* Thu Dec 31 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.96.dev.git39b1cb4
- autobuilt 39b1cb4

* Wed Dec 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.95.dev.gitc6c9b45
- autobuilt c6c9b45

* Wed Dec 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.94.dev.gitef12e36
- autobuilt ef12e36

* Wed Dec 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.93.dev.git7f0771f
- autobuilt 7f0771f

* Fri Dec 25 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.92.dev.git9c9f02a
- autobuilt 9c9f02a

* Thu Dec 24 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.91.dev.git8f75ed9
- autobuilt 8f75ed9

* Thu Dec 24 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.90.dev.gitb176c62
- autobuilt b176c62

* Thu Dec 24 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.89.dev.git231c528
- autobuilt 231c528

* Wed Dec 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.88.dev.git9ac5ed1
- autobuilt 9ac5ed1

* Wed Dec 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.87.dev.gitbbc0deb
- autobuilt bbc0deb

* Wed Dec 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.86.dev.git54b82a1
- autobuilt 54b82a1

* Wed Dec 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.85.dev.git0778c11
- autobuilt 0778c11

* Wed Dec 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.84.dev.git3728ca9
- autobuilt 3728ca9

* Wed Dec 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.83.dev.git06a6fd9
- autobuilt 06a6fd9

* Wed Dec 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.82.dev.git9b6324f
- autobuilt 9b6324f

* Tue Dec 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.81.dev.git07663f7
- autobuilt 07663f7

* Tue Dec 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.80.dev.gitcfdb8fb
- autobuilt cfdb8fb

* Tue Dec 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.79.dev.gitb4692f2
- autobuilt b4692f2

* Mon Dec 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.78.dev.git182646b
- autobuilt 182646b

* Mon Dec 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.77.dev.git076f77b
- autobuilt 076f77b

* Mon Dec 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.76.dev.gitd692518
- autobuilt d692518

* Fri Dec 18 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.75.dev.git5c6b5ef
- autobuilt 5c6b5ef

* Fri Dec 18 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.74.dev.gitf568658
- autobuilt f568658

* Thu Dec 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.73.dev.gita17afa9
- autobuilt a17afa9

* Thu Dec 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.72.dev.git0333366
- autobuilt 0333366

* Thu Dec 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.71.dev.git7592f8f
- autobuilt 7592f8f

* Thu Dec 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.70.dev.gitd291013
- autobuilt d291013

* Thu Dec 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.69.dev.gitc38ae47
- autobuilt c38ae47

* Wed Dec 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.68.dev.git915ae6d
- autobuilt 915ae6d

* Wed Dec 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.67.dev.git2a21dcd
- autobuilt 2a21dcd

* Wed Dec 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.66.dev.gitbacb2fc
- autobuilt bacb2fc

* Wed Dec 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.65.dev.git978c076
- autobuilt 978c076

* Wed Dec 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.64.dev.gitf1f7b8f
- autobuilt f1f7b8f

* Wed Dec 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.63.dev.git8333a9e
- autobuilt 8333a9e

* Tue Dec 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.62.dev.git66e979a
- autobuilt 66e979a

* Tue Dec 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.61.dev.gite689503
- autobuilt e689503

* Tue Dec 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.60.dev.git9379ee9
- autobuilt 9379ee9

* Mon Dec 14 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.59.dev.git999d40d
- autobuilt 999d40d

* Mon Dec 14 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.58.dev.git0fd31e2
- autobuilt 0fd31e2

* Mon Dec 14 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.57.dev.gitbdbf47f
- autobuilt bdbf47f

* Sat Dec 12 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.56.dev.gita226e6e
- autobuilt a226e6e

* Sat Dec 12 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.55.dev.git36bec38
- autobuilt 36bec38

* Sat Dec 12 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.54.dev.git1d50245
- autobuilt 1d50245

* Sat Dec 12 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.53.dev.gitfbcd445
- autobuilt fbcd445

* Fri Dec 11 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.52.dev.gitb0a287c
- autobuilt b0a287c

* Fri Dec 11 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.51.dev.git99ac30a
- autobuilt 99ac30a

* Fri Dec 11 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.50.dev.gitdd95478
- autobuilt dd95478

* Thu Dec 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.49.dev.git6823a5d
- autobuilt 6823a5d

* Thu Dec 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.48.dev.git2bb1490
- autobuilt 2bb1490

* Thu Dec 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.47.dev.gitdeb0042
- autobuilt deb0042

* Thu Dec 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.46.dev.giteaa19a1
- autobuilt eaa19a1

* Wed Dec  9 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.45.dev.git9216be2
- autobuilt 9216be2

* Wed Dec  9 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.44.dev.giteb053df
- autobuilt eb053df

* Wed Dec  9 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.43.dev.git43567c6
- autobuilt 43567c6

* Wed Dec  9 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.42.dev.git9abbe07
- autobuilt 9abbe07

* Wed Dec  9 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.41.dev.git3cd143f
- autobuilt 3cd143f

* Wed Dec  9 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.40.dev.gitb875c5c
- autobuilt b875c5c

* Wed Dec  9 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.39.dev.git2472600
- autobuilt 2472600

* Wed Dec  9 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.38.dev.gitdd295f2
- autobuilt dd295f2

* Tue Dec  8 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.37.dev.git7caef9c
- autobuilt 7caef9c

* Tue Dec  8 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.36.dev.git47d2a4b
- autobuilt 47d2a4b

* Tue Dec  8 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.35.dev.git0cccba8
- autobuilt 0cccba8

* Tue Dec  8 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.34.dev.git9b3a81a
- autobuilt 9b3a81a

* Mon Dec  7 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.33.dev.gite2f9120
- autobuilt e2f9120

* Mon Dec  7 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.32.dev.gitbfbeece
- autobuilt bfbeece

* Mon Dec  7 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.31.dev.gita5ca039
- autobuilt a5ca039

* Mon Dec  7 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.30.dev.git3569e24
- autobuilt 3569e24

* Mon Dec  7 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.29.dev.gite6f80fa
- autobuilt e6f80fa

* Mon Dec  7 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.28.dev.gite117ad3
- autobuilt e117ad3

* Mon Dec  7 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.27.dev.git0c96731
- autobuilt 0c96731

* Sat Dec  5 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.26.dev.git0c2a43b
- autobuilt 0c2a43b

* Fri Dec  4 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.25.dev.git8e83799
- autobuilt 8e83799

* Fri Dec  4 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.24.dev.gitb6536d2
- autobuilt b6536d2

* Fri Dec  4 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.23.dev.gitc55b831
- autobuilt c55b831

* Fri Dec  4 2020 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:3.0.0-0.22.dev.gitf01630a
- make both checksec and koji happy

* Fri Dec  4 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.21.dev.gitf01630a
- autobuilt f01630a

* Fri Dec  4 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.20.dev.gitec0411a
- autobuilt ec0411a

* Thu Dec  3 2020 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:3.0.0-0.19.dev.git85b412d
- Harden binaries
- Reported-by: Wade Mealing <wmealing@gmail.com>

* Thu Dec  3 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.18.dev.git70284b1
- autobuilt 70284b1

* Thu Dec  3 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.17.dev.gitc675d8a
- autobuilt c675d8a

* Thu Dec  3 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.16.dev.git85b412d
- autobuilt 85b412d

* Thu Dec  3 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.15.dev.git9180872
- autobuilt 9180872

* Thu Dec  3 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.14.dev.git5cf7aa6
- autobuilt 5cf7aa6

* Wed Dec  2 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.13.dev.git7984842
- autobuilt 7984842

* Wed Dec  2 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.12.dev.gitd456765
- autobuilt d456765

* Wed Dec  2 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.11.dev.gite82ec90
- autobuilt e82ec90

* Wed Dec  2 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.10.dev.git7210b86
- autobuilt 7210b86

* Wed Dec  2 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.9.dev.gitd28874b
- autobuilt d28874b

* Wed Dec  2 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.8.dev.git9c5fe95
- autobuilt 9c5fe95

* Tue Dec  1 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.7.dev.gitb2cd6e0
- autobuilt b2cd6e0

* Tue Dec  1 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.6.dev.gitc71ad9a
- autobuilt c71ad9a

* Tue Dec  1 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.5.dev.gitce45b71
- autobuilt ce45b71

* Tue Dec  1 2020 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:3.0.0-0.4.dev.git429d949
- use podman-plugins / dnsname upstream v1.1.1

* Tue Dec  1 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.3.dev.git429d949
- autobuilt 429d949

* Tue Dec  1 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.2.dev.git2438390
- autobuilt 2438390

* Tue Dec  1 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:3.0.0-0.1.dev.gitca612a3
- bump to 3.0.0
- autobuilt ca612a3

* Mon Nov 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.74.dev.gitc342583
- autobuilt c342583

* Mon Nov 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.73.dev.gitf6fb297
- autobuilt f6fb297

* Mon Nov 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.72.dev.git7ad1c9c
- autobuilt 7ad1c9c

* Mon Nov 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.71.dev.gitfc85ec9
- autobuilt fc85ec9

* Sat Nov 28 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.70.dev.git8b2c0a4
- autobuilt 8b2c0a4

* Sat Nov 28 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.69.dev.gitf0d48aa
- autobuilt f0d48aa

* Sat Nov 28 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.68.dev.git3110308
- autobuilt 3110308

* Thu Nov 26 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.67.dev.gitad24392
- autobuilt ad24392

* Wed Nov 25 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.66.dev.git397e9a9
- autobuilt 397e9a9

* Tue Nov 24 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.65.dev.gitd408395
- autobuilt d408395

* Tue Nov 24 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.64.dev.git850bdd2
- autobuilt 850bdd2

* Tue Nov 24 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.63.dev.git4ebd9d9
- autobuilt 4ebd9d9

* Mon Nov 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.62.dev.git4fe7c3f
- autobuilt 4fe7c3f

* Mon Nov 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.61.dev.gitcd6c4cb
- autobuilt cd6c4cb

* Mon Nov 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.60.dev.git5d55285
- autobuilt 5d55285

* Mon Nov 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.59.dev.gitdd34341
- autobuilt dd34341

* Sat Nov 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.58.dev.git5292d5a
- autobuilt 5292d5a

* Fri Nov 20 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.57.dev.git042d488
- autobuilt 042d488

* Thu Nov 19 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.56.dev.git70f91fb
- autobuilt 70f91fb

* Wed Nov 18 2020 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:2.2.0-0.55.dev.git286d356
- bump dnsname to v1.1.0, commit a9c2a10

* Wed Nov 18 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.54.dev.git286d356
- autobuilt 286d356

* Tue Nov 17 2020 Eduardo Santiago <santiago@redhat.com> - 2:2.2.0-0.53.dev.git42ec4cf
- slight correction to the path of the containers-mounts source file

* Tue Nov 17 2020 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:2.2.0-0.52.dev.git42ec4cf
- containers-mounts.conf.5 in containers-common

* Tue Nov 17 2020 Eduardo Santiago <santiago@redhat.com> - 2:2.2.0-0.51.dev.git42ec4cf
- completion files: be smarter, package -remote files only with -remote

* Tue Nov 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.50.dev.git42ec4cf
- autobuilt 42ec4cf

* Sun Nov 15 2020 Eduardo Santiago <santiago@redhat.com> - 2:2.2.0-0.49.dev.git3920756
- package new zsh and fish completion files

* Sun Nov 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.48.dev.git3920756
- autobuilt 3920756

* Sat Nov 14 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.47.dev.git4eb9c28
- autobuilt 4eb9c28

* Fri Nov 13 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.46.dev.git0b1a60e
- autobuilt 0b1a60e

* Thu Nov 12 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.45.dev.git6c2503c
- autobuilt 6c2503c

* Wed Nov 11 2020 Eduardo Santiago <santiago@redhat.com> - 2:2.2.0-0.44.dev.gite443c01
- distribute newly-added /usr/lib/tmpfiles.d/podman.conf

* Wed Nov 11 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.43.dev.gite443c01
- autobuilt e443c01

* Tue Nov 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.42.dev.gitda01191
- autobuilt da01191

* Sat Nov  7 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.41.dev.gite2b82e6
- autobuilt e2b82e6

* Fri Nov  6 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.40.dev.git07293bc
- autobuilt 07293bc

* Fri Oct 23 2020 ashley-cui <acui@redhat.com> - 2:2.2.0-0.39.dev.git287edd4
- ‘rebuild’

* Wed Oct 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.38.dev.git287edd4
- autobuilt 287edd4

* Tue Oct 20 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.37.dev.git35b4cb1
- autobuilt 35b4cb1

* Mon Oct 19 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.36.dev.git7ffcab0
- autobuilt 7ffcab0

* Sun Oct 18 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.35.dev.git6ec96dc
- autobuilt 6ec96dc

* Sat Oct 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.34.dev.git39f1bea
- autobuilt 39f1bea

* Fri Oct 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.33.dev.git9f98b34
- autobuilt 9f98b34

* Thu Oct 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.32.dev.gita82d60d
- autobuilt a82d60d

* Wed Oct 14 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.31.dev.gitd30b4b7
- autobuilt d30b4b7

* Tue Oct 13 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.30.dev.git7ad631b
- autobuilt 7ad631b

* Mon Oct 12 2020 Jindrich Novy <jnovy@redhat.com> - 2:2.2.0-0.29.dev.git212011f
- use %%rhel instead of %%eln, thanks to Adam Samalik for noticing

* Mon Oct 12 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.28.dev.git212011f
- autobuilt 212011f

* Sat Oct 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.27.dev.git7876dd5
- autobuilt 7876dd5

* Fri Oct  9 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.26.dev.git71d675a
- autobuilt 71d675a

* Thu Oct  8 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.25.dev.git59b5f0a
- autobuilt 59b5f0a

* Wed Oct  7 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.24.dev.gita7500e5
- autobuilt a7500e5

* Tue Oct  6 2020 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:2.2.0-0.23.dev.gitdefb754
- btrfs deps for fedora only

* Tue Oct  6 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.22.dev.gitdefb754
- autobuilt defb754

* Mon Oct  5 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.21.dev.gitcaace52
- autobuilt caace52

* Sat Oct  3 2020 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:2.2.0-0.20.dev.git7c12967
- rebuild

* Sat Oct  3 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.19.dev.git7c12967
- autobuilt 7c12967

* Fri Oct  2 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.18.dev.git14fd7b4
- autobuilt 14fd7b4

* Thu Oct  1 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.17.dev.git556117c
- autobuilt 556117c

* Wed Sep 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.16.dev.git4d57313
- autobuilt 4d57313

* Wed Sep 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.15.dev.git08d036c
- autobuilt 08d036c

* Wed Sep 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.14.dev.git19f080f
- autobuilt 19f080f

* Wed Sep 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.13.dev.gite9eddda
- autobuilt e9eddda

* Wed Sep 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.12.dev.gitb68b6f3
- autobuilt b68b6f3

* Tue Sep 29 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.11.dev.git453333a
- autobuilt 453333a

* Tue Sep 29 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.10.dev.git12f173f
- autobuilt 12f173f

* Tue Sep 29 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.9.dev.git2ee415b
- autobuilt 2ee415b

* Tue Sep 29 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.8.dev.gitbf10168
- autobuilt bf10168

* Tue Sep 29 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.7.dev.git84dede4
- autobuilt 84dede4

* Tue Sep 29 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.6.dev.git5cf8659
- autobuilt 5cf8659

* Tue Sep 29 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.5.dev.git4a7fb62
- autobuilt 4a7fb62

* Mon Sep 28 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.4.dev.gite7e466e
- autobuilt e7e466e

* Mon Sep 28 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.3.dev.gitb0e70a6
- autobuilt b0e70a6

* Fri Sep 25 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.2.dev.git03d01ab
- autobuilt 03d01ab

* Fri Sep 25 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.2.0-0.1.dev.gita1045ad
- bump to 2.2.0
- autobuilt a1045ad

* Tue Sep 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.287.dev.git141688c
- autobuilt 141688c

* Mon Sep 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.286.dev.git84c87fc
- autobuilt 84c87fc

* Mon Sep 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.285.dev.gitdd4dc4b
- autobuilt dd4dc4b

* Sat Sep 19 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.284.dev.git8529435
- autobuilt 8529435

* Fri Sep 18 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.283.dev.git5b7509c
- autobuilt 5b7509c

* Fri Sep 18 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.282.dev.gitfc3daae
- autobuilt fc3daae

* Fri Sep 18 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.281.dev.git273b954
- autobuilt 273b954

* Thu Sep 17 2020 Merlin Mathesius <mmathesi@redhat.com> - 2:2.1.0-0.280.dev.git1a929c7
- Minor conditional updates to enable building for ELN

* Thu Sep 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.279.dev.gitf84f441
- autobuilt f84f441

* Thu Sep 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.278.dev.git175d7b1
- autobuilt 175d7b1

* Thu Sep 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.277.dev.gitdc23ef1
- autobuilt dc23ef1

* Thu Sep 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.276.dev.git031ddf9
- autobuilt 031ddf9

* Thu Sep 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.275.dev.git9f745d5
- autobuilt 9f745d5

* Thu Sep 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.274.dev.git1a929c7
- autobuilt 1a929c7

* Wed Sep 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.273.dev.git8d7e795
- autobuilt 8d7e795

* Wed Sep 16 2020 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:2.1.0-0.272.dev.gitacf86ef
- fix dependencies for podman-plugins

* Wed Sep 16 2020 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:2.1.0-0.271.dev.gitacf86ef
- bump dnsname plugin commit and dependency on dnsmasq

* Wed Sep 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.270.dev.gitacf86ef
- autobuilt acf86ef

* Wed Sep 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.269.dev.git0d14d7b
- autobuilt 0d14d7b

* Wed Sep 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.268.dev.gitb9c47fa
- autobuilt b9c47fa

* Wed Sep 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.267.dev.git2604919
- autobuilt 2604919

* Tue Sep 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.266.dev.git2eb3339
- autobuilt 2eb3339

* Tue Sep 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.265.dev.gitbec96ab
- autobuilt bec96ab

* Tue Sep 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.264.dev.git0be5836
- autobuilt 0be5836

* Tue Sep 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.263.dev.git3f6045c
- autobuilt 3f6045c

* Tue Sep 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.262.dev.git5c47a33
- autobuilt 5c47a33

* Mon Sep 14 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.261.dev.gitfd7cdb2
- autobuilt fd7cdb2

* Mon Sep 14 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.260.dev.gitb7a7cf6
- autobuilt b7a7cf6

* Sun Sep 13 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.259.dev.git3f5f99b
- autobuilt 3f5f99b

* Fri Sep 11 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.258.dev.git25fb0c2
- autobuilt 25fb0c2

* Fri Sep 11 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.257.dev.git834c41d
- autobuilt 834c41d

* Fri Sep 11 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.256.dev.git4f04007
- autobuilt 4f04007

* Fri Sep 11 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.255.dev.git881f2df
- autobuilt 881f2df

* Fri Sep 11 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.254.dev.git37658c0
- autobuilt 37658c0

* Fri Sep 11 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.253.dev.git397de44
- autobuilt 397de44

* Thu Sep 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.252.dev.git10ba232
- autobuilt 10ba232

* Thu Sep 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.251.dev.git861451a
- autobuilt 861451a

* Thu Sep 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.250.dev.git96bc5eb
- autobuilt 96bc5eb

* Thu Sep 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.249.dev.git89a3483
- autobuilt 89a3483

* Thu Sep 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.248.dev.gitfc70360
- autobuilt fc70360

* Thu Sep 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.247.dev.gitaadf96a
- autobuilt aadf96a

* Thu Sep 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.246.dev.git3d33923
- autobuilt 3d33923

* Wed Sep  9 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.245.dev.gite1b4729
- autobuilt e1b4729

* Wed Sep  9 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.244.dev.git08b6020
- autobuilt 08b6020

* Wed Sep  9 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.243.dev.git68dace0
- autobuilt 68dace0

* Wed Sep  9 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.242.dev.git9c4c883
- autobuilt 9c4c883

* Wed Sep  9 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.241.dev.git5a09fd8
- autobuilt 5a09fd8

* Wed Sep  9 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.240.dev.git1b2b068
- autobuilt 1b2b068

* Wed Sep  9 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.239.dev.git6b1a1fc
- autobuilt 6b1a1fc

* Tue Sep  8 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.238.dev.git814784c
- autobuilt 814784c

* Tue Sep  8 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.237.dev.gite180de8
- autobuilt e180de8

* Tue Sep  8 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.236.dev.git54a61e3
- autobuilt 54a61e3

* Tue Sep  8 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.235.dev.git11679c2
- autobuilt 11679c2

* Mon Sep  7 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.234.dev.gitbe7778d
- autobuilt be7778d

* Mon Sep  7 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.233.dev.git21c6aae
- autobuilt 21c6aae

* Sun Sep  6 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.232.dev.gitba8d0bb
- autobuilt ba8d0bb

* Sat Sep  5 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.231.dev.gitf1323a9
- autobuilt f1323a9

* Wed Sep  2 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.230.dev.gitfa487a6
- autobuilt fa487a6

* Wed Sep  2 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.229.dev.git37791d7
- autobuilt 37791d7

* Wed Sep  2 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.228.dev.git1184cdf
- autobuilt 1184cdf

* Tue Sep  1 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.227.dev.gita867b16
- autobuilt a867b16

* Tue Sep  1 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.226.dev.git557cf94
- autobuilt 557cf94

* Mon Aug 31 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.225.dev.git138132e
- autobuilt 138132e

* Mon Aug 31 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.224.dev.git0c076db
- autobuilt 0c076db

* Mon Aug 31 2020 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:2.1.0-0.223.dev.git575b3a3
- bump

* Sat Aug 29 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.222.dev.git575b3a3
- autobuilt 575b3a3

* Fri Aug 28 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.221.dev.git4e3ea01
- autobuilt 4e3ea01

* Fri Aug 28 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.220.dev.git1f9b854
- autobuilt 1f9b854

* Fri Aug 28 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.219.dev.git522a32f
- autobuilt 522a32f

* Fri Aug 28 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.218.dev.git0640cc7
- autobuilt 0640cc7

* Fri Aug 28 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.217.dev.gitb1d6ea2
- autobuilt b1d6ea2

* Fri Aug 28 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.216.dev.git061c93f
- autobuilt 061c93f

* Thu Aug 27 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.215.dev.git72c5b35
- autobuilt 72c5b35

* Thu Aug 27 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.214.dev.git7d3cadc
- autobuilt 7d3cadc

* Thu Aug 27 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.213.dev.gitd6b0377
- autobuilt d6b0377

* Wed Aug 26 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.212.dev.gitf99954c
- autobuilt f99954c

* Wed Aug 26 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.211.dev.git3a9d524
- autobuilt 3a9d524

* Tue Aug 25 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.210.dev.git6a06944
- autobuilt 6a06944

* Mon Aug 24 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.209.dev.git8fdc116
- autobuilt 8fdc116

* Mon Aug 24 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.208.dev.git2c567dc
- autobuilt 2c567dc

* Sun Aug 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.207.dev.gite535f61
- autobuilt e535f61

* Sun Aug 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.206.dev.git80d2c01
- autobuilt 80d2c01

* Fri Aug 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.205.dev.git4828455
- autobuilt 4828455

* Fri Aug 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.204.dev.gita8619bb
- autobuilt a8619bb

* Thu Aug 20 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.203.dev.git516196f
- autobuilt 516196f

* Thu Aug 20 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.202.dev.git7ccd821
- autobuilt 7ccd821

* Thu Aug 20 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.201.dev.git7865db5
- autobuilt 7865db5

* Wed Aug 19 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.200.dev.git42690ff
- autobuilt 42690ff

* Wed Aug 19 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.199.dev.gitdcdb647
- autobuilt dcdb647

* Wed Aug 19 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.198.dev.git9babd21
- autobuilt 9babd21

* Wed Aug 19 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.197.dev.gitdd4e0da
- autobuilt dd4e0da

* Wed Aug 19 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.196.dev.git7e2a1b3
- autobuilt 7e2a1b3

* Tue Aug 18 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.195.dev.git9d096c1
- autobuilt 9d096c1

* Tue Aug 18 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.194.dev.gitff1f81b
- autobuilt ff1f81b

* Tue Aug 18 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.193.dev.git748e882
- autobuilt 748e882

* Tue Aug 18 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.192.dev.git49d6468
- autobuilt 49d6468

* Mon Aug 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.191.dev.git8caed30
- autobuilt 8caed30

* Mon Aug 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.190.dev.git47108e2
- autobuilt 47108e2

* Mon Aug 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.189.dev.gitfff66f1
- autobuilt fff66f1

* Sun Aug 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.188.dev.git96fb5dc
- autobuilt 96fb5dc

* Sat Aug 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.187.dev.gitca4423e
- autobuilt ca4423e

* Thu Aug 13 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.186.dev.git81499a5
- autobuilt 81499a5

* Thu Aug 13 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.185.dev.git9ede14e
- autobuilt 9ede14e

* Thu Aug 13 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.184.dev.git90831df
- autobuilt 90831df

* Wed Aug 12 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.183.dev.gitd777a7b
- autobuilt d777a7b

* Wed Aug 12 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.182.dev.git4ef4f52
- autobuilt 4ef4f52

* Wed Aug 12 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.181.dev.git8e4842a
- autobuilt 8e4842a

* Wed Aug 12 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.180.dev.gitac96112
- autobuilt ac96112

* Tue Aug 11 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.179.dev.git8eaacec
- autobuilt 8eaacec

* Tue Aug 11 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.178.dev.git43f2771
- autobuilt 43f2771

* Tue Aug 11 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.177.dev.gitaa66c06
- autobuilt aa66c06

* Tue Aug 11 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.176.dev.git6d3075a
- autobuilt 6d3075a

* Tue Aug 11 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.175.dev.git68c67d2
- autobuilt 68c67d2

* Tue Aug 11 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.174.dev.gita90ae00
- autobuilt a90ae00

* Tue Aug 11 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.173.dev.git92b088b
- autobuilt 92b088b

* Tue Aug 11 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.172.dev.gitdf0ad51
- autobuilt df0ad51

* Mon Aug 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.171.dev.git75d2fe6
- autobuilt 75d2fe6

* Mon Aug 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.170.dev.git68fd9aa
- autobuilt 68fd9aa

* Mon Aug 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.169.dev.git162625f
- autobuilt 162625f

* Mon Aug 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.168.dev.gitda00482
- autobuilt da00482

* Sun Aug 09 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.167.dev.git95e2e15
- autobuilt 95e2e15

* Sat Aug 08 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.166.dev.git3173a18
- autobuilt 3173a18

* Sat Aug 08 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.165.dev.git1298161
- autobuilt 1298161

* Fri Aug 07 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.164.dev.git51159e7
- autobuilt 51159e7

* Wed Aug 05 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.163.dev.git0d4a269
- autobuilt 0d4a269

* Wed Aug 05 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.162.dev.gita948635
- autobuilt a948635

* Wed Aug 05 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.161.dev.gitbae6d5d
- autobuilt bae6d5d

* Wed Aug 05 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.160.dev.gitd1aaf33
- autobuilt d1aaf33

* Wed Aug 05 2020 Eduardo Santiago <santiago@redhat.com> - 2:2.1.0-0.159.dev.git7a7c8e9
- add openssl, httpd-tools requirements to podman-tests

* Wed Aug 05 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.158.dev.git7a7c8e9
- autobuilt 7a7c8e9

* Wed Aug 05 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.157.dev.git4797190
- autobuilt 4797190

* Wed Aug 05 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.156.dev.git6260677
- autobuilt 6260677

* Wed Aug 05 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.155.dev.git0a3f3c9
- autobuilt 0a3f3c9

* Wed Aug 05 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.154.dev.git807efd6
- autobuilt 807efd6

* Tue Aug 04 2020 Peter Oliver <rpm@mavit.org.uk> - 2:2.1.0-0.153.dev.git1709335
- Include podman-auto-update systemd service and timer.

* Tue Aug 04 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.152.dev.gitd4cf3c5
- autobuilt d4cf3c5

* Tue Aug 04 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.151.dev.git93d6320
- autobuilt 93d6320

* Tue Aug 04 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.150.dev.gitf7440ff
- autobuilt f7440ff

* Tue Aug 04 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.149.dev.git1ed1e58
- autobuilt 1ed1e58

* Mon Aug 03 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.148.dev.git1709335
- autobuilt 1709335

* Mon Aug 03 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.147.dev.git2e3928e
- autobuilt 2e3928e

* Mon Aug 03 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.146.dev.git41358f5
- autobuilt 41358f5

* Mon Aug 03 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.145.dev.gitbfd3454
- autobuilt bfd3454

* Sun Aug 02 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.144.dev.gitf4d4bd2
- autobuilt f4d4bd2

* Sat Aug 01 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.143.dev.gitb425a4f
- autobuilt b425a4f

* Sat Aug 01 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.142.dev.git0d064a0
- autobuilt 0d064a0

* Fri Jul 31 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.141.dev.git4c75fe3
- autobuilt 4c75fe3

* Fri Jul 31 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.140.dev.git7a15be5
- autobuilt 7a15be5

* Fri Jul 31 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.139.dev.git3cf8237
- autobuilt 3cf8237

* Fri Jul 31 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.138.dev.gitbb96c89
- autobuilt bb96c89

* Fri Jul 31 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.137.dev.gite911875
- autobuilt e911875

* Fri Jul 31 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.136.dev.git0e009d5
- autobuilt 0e009d5

* Fri Jul 31 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.135.dev.git1b784b4
- autobuilt 1b784b4

* Thu Jul 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.134.dev.git4132b71
- autobuilt 4132b71

* Thu Jul 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.133.dev.gitca2bda6
- autobuilt ca2bda6

* Thu Jul 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.132.dev.git05b3e0e
- autobuilt 05b3e0e

* Thu Jul 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.131.dev.git1170430
- autobuilt 1170430

* Wed Jul 29 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.130.dev.gitc66ce8d
- autobuilt c66ce8d

* Wed Jul 29 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.129.dev.giteaa2f52
- autobuilt eaa2f52

* Wed Jul 29 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.128.dev.git044a7cb
- autobuilt 044a7cb

* Wed Jul 29 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.127.dev.git7f38774
- autobuilt 7f38774

* Wed Jul 29 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.126.dev.git83166a9
- autobuilt 83166a9

* Tue Jul 28 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.125.dev.git539bb4c
- autobuilt 539bb4c

* Tue Jul 28 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.124.dev.gitb0777ad
- autobuilt b0777ad

* Tue Jul 28 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.123.dev.git288ebec
- autobuilt 288ebec

* Tue Jul 28 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.122.dev.git6ed9868
- autobuilt 6ed9868

* Tue Jul 28 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.121.dev.gitec69497
- autobuilt ec69497

* Tue Jul 28 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.120.dev.git91c92d1
- autobuilt 91c92d1

* Tue Jul 28 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.119.dev.gitd463715
- autobuilt d463715

* Mon Jul 27 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.118.dev.git2b7bc9b
- autobuilt 2b7bc9b

* Mon Jul 27 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.117.dev.git956caf3
- autobuilt 956caf3

* Mon Jul 27 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.116.dev.gitbf92ec5
- autobuilt bf92ec5

* Mon Jul 27 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.115.dev.git5e9b54f
- autobuilt 5e9b54f

* Mon Jul 27 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.114.dev.git55a7faf
- autobuilt 55a7faf

* Mon Jul 27 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.113.dev.git71f7150
- autobuilt 71f7150

* Sun Jul 26 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.112.dev.git11e8e65
- autobuilt 11e8e65

* Fri Jul 24 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.111.dev.gitc2deeff
- autobuilt c2deeff

* Fri Jul 24 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.110.dev.gitd924476
- autobuilt d924476

* Thu Jul 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.109.dev.git197825d
- autobuilt 197825d

* Thu Jul 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.108.dev.git961fa6a
- autobuilt 961fa6a

* Wed Jul 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.107.dev.git1aac197
- autobuilt 1aac197

* Wed Jul 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.106.dev.git9223b72
- autobuilt 9223b72

* Wed Jul 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.105.dev.gitd493374
- autobuilt d493374

* Wed Jul 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.104.dev.git80add29
- autobuilt 80add29

* Wed Jul 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.103.dev.git9f5d146
- autobuilt 9f5d146

* Wed Jul 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.102.dev.gitef03815
- autobuilt ef03815

* Wed Jul 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.101.dev.git59bad8b
- autobuilt 59bad8b

* Tue Jul 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.100.dev.git344a791
- autobuilt 344a791

* Tue Jul 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.99.dev.gite5b3563
- autobuilt e5b3563

* Tue Jul 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.98.dev.gitbe5219a
- autobuilt be5219a

* Tue Jul 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.97.dev.gitf8e2a35
- autobuilt f8e2a35

* Tue Jul 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.96.dev.git644b5bc
- autobuilt 644b5bc

* Tue Jul 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.95.dev.gitdf6920a
- autobuilt df6920a

* Mon Jul 20 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.94.dev.git0d26a57
- autobuilt 0d26a57

* Mon Jul 20 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.93.dev.gite8de509
- autobuilt e8de509

* Mon Jul 20 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.92.dev.git17f9b80
- autobuilt 17f9b80

* Sun Jul 19 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.91.dev.gitb7b8fce
- autobuilt b7b8fce

* Sat Jul 18 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.90.dev.gitd087ade
- autobuilt d087ade

* Sat Jul 18 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.89.dev.gitdeff289
- autobuilt deff289

* Fri Jul 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.88.dev.git10c5f24
- autobuilt 10c5f24

* Fri Jul 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.87.dev.gitdfca83d
- autobuilt dfca83d

* Fri Jul 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.86.dev.gitd86bae2
- autobuilt d86bae2

* Thu Jul 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.85.dev.git0bd5181
- autobuilt 0bd5181

* Thu Jul 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.84.dev.gitf4766e0
- autobuilt f4766e0

* Thu Jul 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.83.dev.git984fffc
- autobuilt 984fffc

* Thu Jul 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.82.dev.git11fe857
- autobuilt 11fe857

* Wed Jul 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.81.dev.git9efeb1c
- autobuilt 9efeb1c

* Wed Jul 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.80.dev.git6dcff5c
- autobuilt 6dcff5c

* Wed Jul 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.79.dev.git9051546
- autobuilt 9051546

* Wed Jul 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.78.dev.git8704b78
- autobuilt 8704b78

* Wed Jul 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.77.dev.git60127cf
- autobuilt 60127cf

* Wed Jul 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.76.dev.git76f9f96
- autobuilt 76f9f96

* Wed Jul 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.75.dev.git4250d24
- autobuilt 4250d24

* Tue Jul 14 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.74.dev.gitc4843d4
- autobuilt c4843d4

* Tue Jul 14 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.73.dev.gita9a751f
- autobuilt a9a751f

* Tue Jul 14 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.72.dev.gitd83077b
- autobuilt d83077b

* Tue Jul 14 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.71.dev.git210f104
- autobuilt 210f104

* Mon Jul 13 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.70.dev.git3d33590
- autobuilt 3d33590

* Mon Jul 13 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.69.dev.gitd86acf2
- autobuilt d86acf2

* Mon Jul 13 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.68.dev.gite2a8e03
- autobuilt e2a8e03

* Sat Jul 11 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.67.dev.gite38001f
- autobuilt e38001f

* Fri Jul 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.66.dev.git1d71753
- autobuilt 1d71753

* Fri Jul 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.65.dev.git2ac8c69
- autobuilt 2ac8c69

* Thu Jul 09 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.64.dev.gitd9cd003
- autobuilt d9cd003

* Thu Jul 09 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.63.dev.gitbc3b3b3
- autobuilt bc3b3b3

* Wed Jul 08 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.62.dev.gitedf5fe8
- autobuilt edf5fe8

* Wed Jul 08 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.61.dev.gitb58d2b7
- autobuilt b58d2b7

* Wed Jul 08 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.60.dev.git85d71ae
- autobuilt 85d71ae

* Tue Jul 07 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.59.dev.git54d16f3
- autobuilt 54d16f3

* Tue Jul 07 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.58.dev.gitcd08485
- autobuilt cd08485

* Mon Jul 06 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.57.dev.git1a93857
- autobuilt 1a93857

* Mon Jul 06 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.56.dev.gitb1cc781
- autobuilt b1cc781

* Mon Jul 06 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.55.dev.gitf4708a5
- autobuilt f4708a5

* Mon Jul 06 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.54.dev.git9532509
- autobuilt 9532509

* Mon Jul 06 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.53.dev.git9eac75a
- autobuilt 9eac75a

* Mon Jul 06 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.52.dev.git4bdc119
- autobuilt 4bdc119

* Mon Jul 06 2020 Eduardo Santiago <santiago@redhat.com> - 2:2.1.0-0.51.dev.git4351e33
- bump crun dependency to 0.14, needed for --sdnotify option

* Mon Jul 06 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.50.dev.git4351e33
- autobuilt 4351e33

* Sun Jul 05 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.49.dev.git41ccc04
- autobuilt 41ccc04

* Fri Jul 03 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.48.dev.gitb9d48a9
- autobuilt b9d48a9

* Thu Jul 02 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.47.dev.gitbd2fca0
- autobuilt bd2fca0

* Thu Jul 02 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.46.dev.gitc131567
- autobuilt c131567

* Thu Jul 02 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.45.dev.git9fb0b56
- autobuilt 9fb0b56

* Wed Jul 01 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.44.dev.gite846952
- autobuilt e846952

* Wed Jul 01 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.43.dev.gitd8718fd
- autobuilt d8718fd

* Tue Jun 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.42.dev.git957e7a5
- autobuilt 957e7a5

* Tue Jun 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.41.dev.git1a1e3f4
- autobuilt 1a1e3f4

* Tue Jun 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.40.dev.git6fbd157
- autobuilt 6fbd157

* Tue Jun 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.39.dev.gitc2a0ccd
- autobuilt c2a0ccd

* Tue Jun 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.38.dev.git83bde3b
- autobuilt 83bde3b

* Mon Jun 29 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.37.dev.gitb163ec3
- autobuilt b163ec3

* Mon Jun 29 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.36.dev.gite0b93af
- autobuilt e0b93af

* Mon Jun 29 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.35.dev.gitc682ca3
- autobuilt c682ca3

* Mon Jun 29 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.34.dev.gitd90e8b6
- autobuilt d90e8b6

* Mon Jun 29 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.33.dev.git6ac009d
- autobuilt 6ac009d

* Mon Jun 29 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.32.dev.git771c887
- autobuilt 771c887

* Fri Jun 26 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.31.dev.git673116c
- autobuilt 673116c

* Fri Jun 26 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.30.dev.gitbb11b42
- autobuilt bb11b42

* Thu Jun 25 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.29.dev.git4db296f
- autobuilt 4db296f

* Thu Jun 25 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.28.dev.git358e69c
- autobuilt 358e69c

* Thu Jun 25 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.27.dev.git05e1df2
- autobuilt 05e1df2

* Thu Jun 25 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.26.dev.git7766192
- autobuilt 7766192

* Thu Jun 25 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.25.dev.gitc036eef
- autobuilt c036eef

* Thu Jun 25 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.24.dev.gitf8036c5
- autobuilt f8036c5

* Thu Jun 25 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.23.dev.gitcd36499
- autobuilt cd36499

* Thu Jun 25 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.22.dev.git35cca19
- autobuilt 35cca19

* Wed Jun 24 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.21.dev.git2df3faa
- autobuilt 2df3faa

* Wed Jun 24 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.20.dev.git4ee6659
- autobuilt 4ee6659

* Wed Jun 24 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.19.dev.gitb61e429
- autobuilt b61e429

* Wed Jun 24 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.18.dev.git6bc5dcc
- autobuilt 6bc5dcc

* Wed Jun 24 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.17.dev.git0d26b8f
- autobuilt 0d26b8f

* Tue Jun 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.16.dev.git5fe122b
- autobuilt 5fe122b

* Tue Jun 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.15.dev.git81f4204
- autobuilt 81f4204

* Tue Jun 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.14.dev.gitaa6881d
- autobuilt aa6881d

* Tue Jun 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.13.dev.git92af85f
- autobuilt 92af85f

* Tue Jun 23 2020 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:2.1.0-0.12.dev.git73514b1
- re-enable remote package

* Tue Jun 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.11.dev.git73514b1
- autobuilt 73514b1

* Tue Jun 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.10.dev.gitbbaba9f
- autobuilt bbaba9f

* Tue Jun 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.9.dev.git9e37fd4
- autobuilt 9e37fd4

* Mon Jun 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.8.dev.git22a7d60
- autobuilt 22a7d60

* Mon Jun 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.7.dev.git22942e3
- autobuilt 22942e3

* Mon Jun 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.6.dev.git78b205c
- autobuilt 78b205c

* Mon Jun 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.5.dev.git277732b
- autobuilt 277732b

* Mon Jun 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.4.dev.git4afdbcd
- autobuilt 4afdbcd

* Sun Jun 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.3.dev.git0e4b734
- autobuilt 0e4b734

* Sun Jun 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.2.dev.git4a1dd9f
- autobuilt 4a1dd9f

* Sat Jun 20 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.1.0-0.1.dev.gitbc256d9
- bump to 2.1.0
- autobuilt bc256d9

* Sat Jun 20 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.173.dev.gitf403aa3
- autobuilt f403aa3

* Fri Jun 19 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.172.dev.git89dbd1a
- autobuilt 89dbd1a

* Fri Jun 19 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.171.dev.git1a2eb3e
- autobuilt 1a2eb3e

* Fri Jun 19 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.170.dev.git33a6027
- autobuilt 33a6027

* Thu Jun 18 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.169.dev.gita2661b1
- autobuilt a2661b1

* Thu Jun 18 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.168.dev.gite6b9b3a
- autobuilt e6b9b3a

* Thu Jun 18 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.167.dev.git1099ad6
- autobuilt 1099ad6

* Thu Jun 18 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.166.dev.git3eb0ad0
- autobuilt 3eb0ad0

* Thu Jun 18 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.165.dev.gitb5f7afd
- autobuilt b5f7afd

* Thu Jun 18 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.164.dev.git6472b44
- autobuilt 6472b44

* Wed Jun 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.163.dev.git7b00e49
- autobuilt 7b00e49

* Wed Jun 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.162.dev.git7b5073b
- autobuilt 7b5073b

* Wed Jun 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.161.dev.gita76bf11
- autobuilt a76bf11

* Wed Jun 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.160.dev.git5694104
- autobuilt 5694104

* Wed Jun 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.159.dev.gitf293606
- autobuilt f293606

* Wed Jun 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.158.dev.git1acd2ad
- autobuilt 1acd2ad

* Wed Jun 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.157.dev.git38391ed
- autobuilt 38391ed

* Wed Jun 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.156.dev.git4fb0f56
- autobuilt 4fb0f56

* Wed Jun 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.155.dev.git4b2da3e
- autobuilt 4b2da3e

* Wed Jun 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.154.dev.gite4e10df
- autobuilt e4e10df

* Tue Jun 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.153.dev.git89630ad
- autobuilt 89630ad

* Tue Jun 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.152.dev.gitd6965da
- autobuilt d6965da

* Tue Jun 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.151.dev.git908bc3f
- autobuilt 908bc3f

* Tue Jun 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.150.dev.git0968f25
- autobuilt 0968f25

* Tue Jun 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.149.dev.gite0dd227
- autobuilt e0dd227

* Mon Jun 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.148.dev.git2c7b39d
- autobuilt 2c7b39d

* Mon Jun 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.147.dev.git8a42a32
- autobuilt 8a42a32

* Mon Jun 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.146.dev.git10c6c80
- autobuilt 10c6c80

* Mon Jun 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.145.dev.gitb897297
- autobuilt b897297

* Mon Jun 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.144.dev.git298d622
- autobuilt 298d622

* Mon Jun 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.143.dev.git230cd25
- autobuilt 230cd25

* Mon Jun 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.142.dev.gite94e3fd
- autobuilt e94e3fd

* Mon Jun 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.141.dev.gitc2690c2
- autobuilt c2690c2

* Fri Jun 12 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.140.dev.git3f026eb
- autobuilt 3f026eb

* Fri Jun 12 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.139.dev.git92dafdc
- autobuilt 92dafdc

* Fri Jun 12 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.138.dev.git8aa5cf3
- autobuilt 8aa5cf3

* Thu Jun 11 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.137.dev.git1f05606
- autobuilt 1f05606

* Thu Jun 11 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.136.dev.git39ad038
- autobuilt 39ad038

* Thu Jun 11 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.135.dev.git142e62c
- autobuilt 142e62c

* Thu Jun 11 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.134.dev.git5f3e64f
- autobuilt 5f3e64f

* Thu Jun 11 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.133.dev.git7b85d5c
- autobuilt 7b85d5c

* Wed Jun 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.132.dev.gitb2200db
- autobuilt b2200db

* Wed Jun 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.131.dev.git6c5bd15
- autobuilt 6c5bd15

* Wed Jun 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.130.dev.git37c9078
- autobuilt 37c9078

* Wed Jun 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.129.dev.git6346846
- autobuilt 6346846

* Wed Jun 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.128.dev.git9967f28
- autobuilt 9967f28

* Wed Jun 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.127.dev.gitfbe09d7
- autobuilt fbe09d7

* Tue Jun 09 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.126.dev.gitc831ae1
- autobuilt c831ae1

* Tue Jun 09 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.125.dev.git79f30af
- autobuilt 79f30af

* Mon Jun 08 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.124.dev.gita85e979
- autobuilt a85e979

* Mon Jun 08 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.123.dev.gitb8acc85
- autobuilt b8acc85

* Mon Jun 08 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.122.dev.git2869cce
- autobuilt 2869cce

* Sat Jun 06 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.121.dev.git1fcb678
- autobuilt 1fcb678

* Fri Jun 05 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.120.dev.git723e823
- autobuilt 723e823

* Fri Jun 05 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.119.dev.gitc448c03
- autobuilt c448c03

* Fri Jun 05 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.118.dev.gitc6da1a8
- autobuilt c6da1a8

* Fri Jun 05 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.117.dev.gitf243233
- autobuilt f243233

* Thu Jun 04 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.116.dev.gitb057896
- autobuilt b057896

* Thu Jun 04 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.115.dev.gitbf8337b
- autobuilt bf8337b

* Thu Jun 04 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.114.dev.gitceef4f6
- autobuilt ceef4f6

* Thu Jun 04 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.113.dev.git650ed43
- autobuilt 650ed43

* Thu Jun 04 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.112.dev.gitff99c3e
- autobuilt ff99c3e

* Thu Jun 04 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.111.dev.gitd6e70c6
- autobuilt d6e70c6

* Wed Jun 03 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.110.dev.git1f8c509
- autobuilt 1f8c509

* Wed Jun 03 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.109.dev.git986a277
- autobuilt 986a277

* Wed Jun 03 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.108.dev.gitbba0a8b
- autobuilt bba0a8b

* Wed Jun 03 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.107.dev.gitcbfb498
- autobuilt cbfb498

* Wed Jun 03 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.106.dev.git9bd48a6
- autobuilt 9bd48a6

* Wed Jun 03 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.105.dev.git428303c
- autobuilt 428303c

* Tue Jun 02 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.104.dev.git95ea39e
- autobuilt 95ea39e

* Tue Jun 02 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.103.dev.git4632a4b
- autobuilt 4632a4b

* Tue Jun 02 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.102.dev.gitc4ccd7c
- autobuilt c4ccd7c

* Tue Jun 02 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.101.dev.gitd10addc
- autobuilt d10addc

* Tue Jun 02 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.100.dev.git2937151
- autobuilt 2937151

* Tue Jun 02 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.99.dev.giteb488e7
- autobuilt eb488e7

* Tue Jun 02 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.98.dev.git92f5029
- autobuilt 92f5029

* Tue Jun 02 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.97.dev.gitcc02154
- autobuilt cc02154

* Mon Jun 01 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.96.dev.gitd6bf6b9
- autobuilt d6bf6b9

* Mon Jun 01 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.95.dev.git85d3641
- autobuilt 85d3641

* Mon Jun 01 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.94.dev.git5f1c23d
- autobuilt 5f1c23d

* Mon Jun 01 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.93.dev.gitf559cec
- autobuilt f559cec

* Mon Jun 01 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.92.dev.git2c6016f
- autobuilt 2c6016f

* Mon Jun 01 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.91.dev.git22713d6
- autobuilt 22713d6

* Sun May 31 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.90.dev.gitbb05337
- autobuilt bb05337

* Sat May 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.89.dev.git9037908
- autobuilt 9037908

* Sat May 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.88.dev.gitc479d63
- autobuilt c479d63

* Fri May 29 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.87.dev.git0eea051
- autobuilt 0eea051

* Fri May 29 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.86.dev.git0c750a9
- autobuilt 0c750a9

* Fri May 29 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.85.dev.git78c3846
- autobuilt 78c3846

* Fri May 29 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.84.dev.git6e3aec3
- autobuilt 6e3aec3

* Fri May 29 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.83.dev.gitcd1e25f
- autobuilt cd1e25f

* Thu May 28 2020 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:2.0.0-0.82.dev.gite8818ce
- use ABISupport buildtag for podman

* Thu May 28 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.81.dev.gite8818ce
- autobuilt e8818ce

* Thu May 28 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.80.dev.git4b2c980
- autobuilt 4b2c980

* Wed May 27 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.79.dev.gitadca437
- autobuilt adca437

* Wed May 27 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.78.dev.gitc64abd0
- autobuilt c64abd0

* Wed May 27 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.77.dev.gitab3a620
- autobuilt ab3a620

* Wed May 27 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.76.dev.git2a988a4
- autobuilt 2a988a4

* Wed May 27 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.75.dev.git89b4683
- autobuilt 89b4683

* Tue May 26 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.74.dev.git119e13d
- autobuilt 119e13d

* Tue May 26 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.73.dev.gite704da0
- autobuilt e704da0

* Tue May 26 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.72.dev.gitd32d588
- autobuilt d32d588

* Tue May 26 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.71.dev.git07ef44e
- autobuilt 07ef44e

* Mon May 25 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.70.dev.git1077d2d
- autobuilt 1077d2d

* Mon May 25 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.69.dev.git0b7b974
- autobuilt 0b7b974

* Sun May 24 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.68.dev.gitb4cd54a
- autobuilt b4cd54a

* Sat May 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.67.dev.git56a95b0
- autobuilt 56a95b0

* Sat May 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.66.dev.gite323d3e
- autobuilt e323d3e

* Sat May 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.65.dev.gitc166b21
- autobuilt c166b21

* Fri May 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.64.dev.gite1193c8
- autobuilt e1193c8

* Fri May 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.63.dev.gita6ee8bf
- autobuilt a6ee8bf

* Fri May 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.62.dev.gitc8d6426
- autobuilt c8d6426

* Fri May 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.61.dev.git6aa802d
- autobuilt 6aa802d

* Fri May 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.60.dev.gitcf5d338
- autobuilt cf5d338

* Fri May 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.59.dev.git05c0ae6
- autobuilt 05c0ae6

* Fri May 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.58.dev.git05a0612
- autobuilt 05a0612

* Fri May 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.57.dev.git3f2ab6b
- autobuilt 3f2ab6b

* Fri May 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.56.dev.git398d462
- autobuilt 398d462

* Fri May 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.55.dev.gitbe43536
- autobuilt be43536

* Thu May 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.54.dev.gitb023d6d
- autobuilt b023d6d

* Thu May 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.53.dev.gitd688851
- autobuilt d688851

* Thu May 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.52.dev.gitf6aa620
- autobuilt f6aa620

* Thu May 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.51.dev.git9d3b466
- autobuilt 9d3b466

* Thu May 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.50.dev.git6409ff6
- autobuilt 6409ff6

* Thu May 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.49.dev.gitfeb97bb
- autobuilt feb97bb

* Thu May 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.48.dev.git6668b13
- autobuilt 6668b13

* Thu May 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.47.dev.git72e8803
- autobuilt 72e8803

* Thu May 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.46.dev.git8db7b9e
- autobuilt 8db7b9e

* Wed May 20 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.45.dev.git02b29db
- autobuilt 02b29db

* Wed May 20 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.44.dev.gite8e5a5f
- autobuilt e8e5a5f

* Wed May 20 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.43.dev.git6a75dfa
- autobuilt 6a75dfa

* Wed May 20 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.42.dev.gitb0bfa0e
- autobuilt b0bfa0e

* Wed May 20 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.41.dev.git09f8f14
- autobuilt 09f8f14

* Wed May 20 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.40.dev.git70d89bf
- autobuilt 70d89bf

* Wed May 20 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.39.dev.gita670014
- autobuilt a670014

* Wed May 20 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.38.dev.git4eee0d8
- autobuilt 4eee0d8

* Mon May 18 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.37.dev.git0f8ad03
- autobuilt 0f8ad03

* Mon May 18 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.36.dev.git9fe4933
- autobuilt 9fe4933

* Mon May 18 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.35.dev.gitd6d4500
- autobuilt d6d4500

* Mon May 18 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.34.dev.gitd4587c6
- autobuilt d4587c6

* Sun May 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.33.dev.gitbfcec32
- autobuilt bfcec32

* Fri May 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.32.dev.git343ab99
- autobuilt 343ab99

* Fri May 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.31.dev.gitc61a45c
- autobuilt c61a45c

* Fri May 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.30.dev.git59dd341
- autobuilt 59dd341

* Fri May 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.29.dev.gitd5358e6
- autobuilt d5358e6

* Fri May 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.28.dev.gita88cd9a
- autobuilt a88cd9a

* Thu May 14 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.27.dev.git4611ff5
- autobuilt 4611ff5

* Thu May 14 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.26.dev.git0d96251
- autobuilt 0d96251

* Thu May 14 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.25.dev.git77dbfc7
- autobuilt 77dbfc7

* Thu May 14 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.24.dev.git7e9ed37
- autobuilt 7e9ed37

* Thu May 14 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.23.dev.gite35edb6
- autobuilt e35edb6

* Thu May 14 2020 Daniel J Walsh <dwalsh@redhat.com> - 2:2.0.0-0.22.dev.gitf2f0de4
- Add requires for oci-runtime
- Change crun to Recommends

* Thu May 14 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.21.dev.gitf2f0de4
- autobuilt f2f0de4

* Thu May 14 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.20.dev.git150679d
- autobuilt 150679d

* Wed May 13 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.19.dev.gitfa5b33e
- autobuilt fa5b33e

* Wed May 13 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.18.dev.git3c58e4f
- autobuilt 3c58e4f

* Wed May 13 2020 Eduardo Santiago <santiago@redhat.com> - 2:2.0.0-0.17.dev.gitd147b3e
- libpod.conf has been removed from the repo

* Wed May 13 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.16.dev.gitd147b3e
- autobuilt d147b3e

* Wed May 13 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.15.dev.gitb364420
- autobuilt b364420

* Tue May 12 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.14.dev.git486a117
- autobuilt 486a117

* Tue May 12 2020 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:2.0.0-0.13.dev.git5b4e91d
- disable remote package

* Mon May 11 2020 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:2.0.0-0.12.dev.git8857ba2
- gating test fix attempt by Ed Santiago <santiago@redhat.com>

* Mon May 11 2020 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:2.0.0-0.11.dev.git8857ba2
- do not modprobe br_netfilter

* Thu Apr 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.10.dev.git8857ba2
- autobuilt 8857ba2

* Thu Apr 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.9.dev.git155a7d6
- autobuilt 155a7d6

* Thu Apr 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.8.dev.git084cfb8
- autobuilt 084cfb8

* Thu Apr 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.7.dev.gitd6b3bc1
- autobuilt d6b3bc1

* Wed Apr 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.6.dev.gitc7d1761
- autobuilt c7d1761

* Wed Apr 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.5.dev.git3500a8b
- autobuilt 3500a8b

* Wed Apr 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.4.dev.git97bded8
- autobuilt 97bded8

* Wed Apr 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.3.dev.gitef297d4
- autobuilt ef297d4

* Wed Apr 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.2.dev.git9b78bf9
- autobuilt 9b78bf9

* Wed Apr 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:2.0.0-0.1.dev.gitcc9b78f
- bump to 2.0.0
- autobuilt cc9b78f

* Wed Apr 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.9.0-14.1.dev.git37ed662
- autobuilt 37ed662

* Wed Apr 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.9.0-13.1.dev.gita756161
- autobuilt a756161

* Wed Apr 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.9.0-12.1.dev.gitffcb99d
- autobuilt ffcb99d

* Wed Apr 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.9.0-11.1.dev.gitf0b6cde
- autobuilt f0b6cde

* Tue Apr 14 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.9.0-10.1.dev.git0d01f09
- autobuilt 0d01f09

* Tue Apr 14 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.9.0-9.1.dev.gita6caae0
- autobuilt a6caae0

* Tue Apr 14 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.9.0-8.1.dev.gite2a1373
- autobuilt e2a1373

* Tue Apr 14 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.9.0-7.1.dev.git26c1535
- autobuilt 26c1535

* Tue Apr 14 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.9.0-6.1.dev.gitd885342
- autobuilt d885342

* Mon Apr 13 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.9.0-5.1.dev.git5cf64ae
- autobuilt 5cf64ae

* Mon Apr 13 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.9.0-4.1.dev.git0b067b6
- autobuilt 0b067b6

* Mon Apr 13 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.9.0-3.1.dev.gitd7695dd
- autobuilt d7695dd

* Mon Apr 13 2020 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:1.9.0-2.1.dev.git465b4bc
- bump release tag to preserve clean upgrade path

* Mon Apr 13 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.9.0-0.2.dev.git465b4bc
- autobuilt 465b4bc

* Mon Apr 13 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.9.0-0.1.dev.git309a7f7
- bump to 1.9.0
- autobuilt 309a7f7

* Fri Apr 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.102.dev.git1593d4c
- autobuilt 1593d4c

* Fri Apr 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.101.dev.git2a8db9d
- autobuilt 2a8db9d

* Fri Apr 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.100.dev.git838b5e1
- autobuilt 838b5e1

* Thu Apr 09 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.99.dev.git3a4bd39
- autobuilt 3a4bd39

* Thu Apr 09 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.98.dev.git1662310
- autobuilt 1662310

* Thu Apr 09 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.97.dev.git555b30e
- autobuilt 555b30e

* Thu Apr 09 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.96.dev.git46227e0
- autobuilt 46227e0

* Thu Apr 09 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.95.dev.git3c94fa9
- autobuilt 3c94fa9

* Wed Apr 08 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.94.dev.gitf71e4d3
- autobuilt f71e4d3

* Wed Apr 08 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.93.dev.git1791fe6
- autobuilt 1791fe6

* Wed Apr 08 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.92.dev.git291ad7f
- autobuilt 291ad7f

* Wed Apr 08 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.91.dev.git522dcd6
- autobuilt 522dcd6

* Tue Apr 07 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.90.dev.gitb4840ec
- autobuilt b4840ec

* Tue Apr 07 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.89.dev.git11c8b01
- autobuilt 11c8b01

* Tue Apr 07 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.88.dev.git08fa3d5
- autobuilt 08fa3d5

* Tue Apr 07 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.87.dev.git9d0d9df
- autobuilt 9d0d9df

* Tue Apr 07 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.86.dev.git8289805
- autobuilt 8289805

* Tue Apr 07 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.85.dev.git44f910c
- autobuilt 44f910c

* Tue Apr 07 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.84.dev.gitc0e29b4
- autobuilt c0e29b4

* Mon Apr 06 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.83.dev.git64b6a19
- autobuilt 64b6a19

* Mon Apr 06 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.82.dev.git843fa25
- autobuilt 843fa25

* Mon Apr 06 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.81.dev.gita858b3a
- autobuilt a858b3a

* Mon Apr 06 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.80.dev.gite318b09
- autobuilt e318b09

* Mon Apr 06 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.79.dev.git09f553c
- autobuilt 09f553c

* Mon Apr 06 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.78.dev.git4b69cf0
- autobuilt 4b69cf0

* Mon Apr 06 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.77.dev.git5b853bb
- autobuilt 5b853bb

* Mon Apr 06 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.76.dev.git8dea3c3
- autobuilt 8dea3c3

* Fri Apr 03 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.75.dev.gitf7dffed
- autobuilt f7dffed

* Fri Apr 03 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.74.dev.git35f5867
- autobuilt 35f5867

* Fri Apr 03 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.73.dev.git64cade0
- autobuilt 64cade0

* Fri Apr 03 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.72.dev.git2d9b9e8
- autobuilt 2d9b9e8

* Fri Apr 03 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.71.dev.gita168dcc
- autobuilt a168dcc

* Fri Apr 03 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.70.dev.gitccb9e57
- autobuilt ccb9e57

* Thu Apr 02 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.69.dev.gitccf0e0d
- autobuilt ccf0e0d

* Thu Apr 02 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.68.dev.gitc3c6a7c
- autobuilt c3c6a7c

* Thu Apr 02 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.67.dev.gitffd2d78
- autobuilt ffd2d78

* Thu Apr 02 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.66.dev.git82610d6
- autobuilt 82610d6

* Thu Apr 02 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.65.dev.git88455fe
- autobuilt 88455fe

* Wed Apr 01 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.64.dev.gita8cde90
- autobuilt a8cde90

* Wed Apr 01 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.63.dev.git0f357be
- autobuilt 0f357be

* Wed Apr 01 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.62.dev.git0a16372
- autobuilt 0a16372

* Wed Apr 01 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.61.dev.gitd534e52
- autobuilt d534e52

* Wed Apr 01 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.60.dev.git82cbebc
- autobuilt 82cbebc

* Wed Apr 01 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.59.dev.git394f1c2
- autobuilt 394f1c2

* Tue Mar 31 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.58.dev.git6d36d05
- autobuilt 6d36d05

* Tue Mar 31 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.57.dev.git9f5fcc3
- autobuilt 9f5fcc3

* Tue Mar 31 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.56.dev.git6e8f6ca
- autobuilt 6e8f6ca

* Tue Mar 31 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.55.dev.git56ab9e4
- autobuilt 56ab9e4

* Tue Mar 31 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.54.dev.git4e3010d
- autobuilt 4e3010d

* Mon Mar 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.53.dev.git9c7410d
- autobuilt 9c7410d

* Mon Mar 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.52.dev.gitedd623c
- autobuilt edd623c

* Mon Mar 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.51.dev.git95d9a1e
- autobuilt 95d9a1e

* Mon Mar 30 2020 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:1.8.3-0.50.dev.git0fa01c8
- Resolves: gh#5316

* Mon Mar 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.49.dev.git0fa01c8
- autobuilt 0fa01c8

* Mon Mar 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.48.dev.gitcc22b94
- autobuilt cc22b94

* Mon Mar 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.47.dev.git8193751
- autobuilt 8193751

* Sun Mar 29 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.46.dev.git598bb53
- autobuilt 598bb53

* Sun Mar 29 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.45.dev.gitdabfa10
- autobuilt dabfa10

* Sat Mar 28 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.44.dev.git684b4bd
- autobuilt 684b4bd

* Sat Mar 28 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.43.dev.git21b67e6
- autobuilt 21b67e6

* Fri Mar 27 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.42.dev.git3336b10
- autobuilt 3336b10

* Fri Mar 27 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.41.dev.git1fe2fbb
- autobuilt 1fe2fbb

* Fri Mar 27 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.40.dev.git2c5c198
- autobuilt 2c5c198

* Fri Mar 27 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.39.dev.git4233250
- autobuilt 4233250

* Fri Mar 27 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.38.dev.git3ddb5b1
- autobuilt 3ddb5b1

* Fri Mar 27 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.37.dev.git7007680
- autobuilt 7007680

* Fri Mar 27 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.36.dev.git340312c
- autobuilt 340312c

* Thu Mar 26 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.35.dev.git1710eca
- autobuilt 1710eca

* Thu Mar 26 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.34.dev.git6a46a87
- autobuilt 6a46a87

* Thu Mar 26 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.33.dev.git913426c
- autobuilt 913426c

* Thu Mar 26 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.32.dev.git14ece7e
- autobuilt 14ece7e

* Thu Mar 26 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.31.dev.git8cccac5
- autobuilt 8cccac5

* Thu Mar 26 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.30.dev.gitc869b96
- autobuilt c869b96

* Thu Mar 26 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.29.dev.git18c1530
- autobuilt 18c1530

* Wed Mar 25 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.28.dev.gitff0124a
- autobuilt ff0124a

* Wed Mar 25 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.27.dev.git852dd7f
- autobuilt 852dd7f

* Wed Mar 25 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.26.dev.git69b011d
- autobuilt 69b011d

* Tue Mar 24 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.25.dev.gitb452064
- autobuilt b452064

* Tue Mar 24 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.24.dev.git0c084d9
- autobuilt 0c084d9

* Tue Mar 24 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.23.dev.gitc29a4c6
- autobuilt c29a4c6

* Tue Mar 24 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.22.dev.git14050c6
- autobuilt 14050c6

* Tue Mar 24 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.21.dev.git0334c8d
- autobuilt 0334c8d

* Tue Mar 24 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.20.dev.git0275eed
- autobuilt 0275eed

* Tue Mar 24 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.19.dev.gita2ffd5c
- autobuilt a2ffd5c

* Mon Mar 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.18.dev.git02de8d5
- autobuilt 02de8d5

* Mon Mar 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.17.dev.git48b3143
- autobuilt 48b3143

* Mon Mar 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.16.dev.gitb743f60
- autobuilt b743f60

* Mon Mar 23 2020 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:1.8.3-0.15.dev.gite34ec61
- do not use hack/ostree_tag.sh

* Mon Mar 23 2020 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:1.8.3-0.14.dev.gite34ec61
- Add APIv2 service files

* Mon Mar 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.13.dev.gite34ec61
- autobuilt e34ec61

* Mon Mar 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.12.dev.gitd6c9f3e
- autobuilt d6c9f3e

* Sun Mar 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.11.dev.git31d1445
- autobuilt 31d1445

* Sun Mar 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.10.dev.git98687ad
- autobuilt 98687ad

* Sat Mar 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.9.dev.git2ffff3c
- autobuilt 2ffff3c

* Sat Mar 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.8.dev.git89a3e59
- autobuilt 89a3e59

* Sat Mar 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.7.dev.gite1f2851
- autobuilt e1f2851

* Sat Mar 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.6.dev.git77187da
- autobuilt 77187da

* Fri Mar 20 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.5.dev.git7a095af
- autobuilt 7a095af

* Fri Mar 20 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.4.dev.git6df1d20
- autobuilt 6df1d20

* Fri Mar 20 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.3.dev.gitccc30c6
- autobuilt ccc30c6

* Thu Mar 19 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.2.dev.gitd927b43
- autobuilt d927b43

* Thu Mar 19 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.3-0.1.dev.gitaa6c8c2
- bump to 1.8.3
- autobuilt aa6c8c2

* Thu Mar 19 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.2-0.9.dev.gitcccd05c
- autobuilt cccd05c

* Thu Mar 19 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.2-0.8.dev.git1cb3e3a
- autobuilt 1cb3e3a

* Thu Mar 19 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.2-0.7.dev.gitedcc73e
- autobuilt edcc73e

* Thu Mar 19 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.2-0.6.dev.gite87fe4d
- autobuilt e87fe4d

* Thu Mar 19 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.2-0.5.dev.gitbd9386d
- autobuilt bd9386d

* Wed Mar 18 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.2-0.4.dev.git45e7cbf
- autobuilt 45e7cbf

* Wed Mar 18 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.2-0.3.dev.gitf3a28de
- autobuilt f3a28de

* Tue Mar 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.2-0.2.dev.git8f1ce4b
- autobuilt 8f1ce4b

* Mon Mar 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.2-0.1.dev.git412a114
- bump to 1.8.2
- autobuilt 412a114

* Mon Mar 02 2020 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:1.8.1-0.4.dev.git275e9b8
- bump release tag for smooth upgrade path

* Mon Mar 02 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.1-0.3.dev.git275e9b8
- autobuilt 275e9b8

* Sat Feb 22 2020 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:1.8.1-0.2.dev.git0bd29f8
- bump release tag

* Mon Feb 17 2020 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:1.8.1-0.1.dev.git0bd29f8
- bump to 1.8.1-dev
- built commit 0bd29f8

* Thu Feb 06 2020 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:1.8.0-0.4.dev.git5092c07
- bump crun dependency

* Wed Feb 05 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.0-0.3.dev.git5092c07
- autobuilt 5092c07

* Tue Feb 04 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.0-0.2.dev.gitc4f6d56
- autobuilt c4f6d56

* Sun Feb 02 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.8.0-0.1.dev.git4699d5e
- bump to 1.8.0
- autobuilt 4699d5e

* Fri Jan 31 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.66.dev.git36af283
- autobuilt 36af283

* Thu Jan 30 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.65.dev.giteb28365
- autobuilt eb28365

* Wed Jan 29 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.64.dev.gitb2ae45c
- autobuilt b2ae45c

* Tue Jan 28 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.63.dev.git326cdf9
- autobuilt 326cdf9

* Mon Jan 27 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.62.dev.gitc28af15
- autobuilt c28af15

* Sat Jan 25 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.61.dev.git975854a
- autobuilt 975854a

* Thu Jan 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.60.dev.git5bad873
- autobuilt 5bad873

* Thu Jan 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.59.dev.git8beeb06
- autobuilt 8beeb06

* Thu Jan 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.58.dev.git6518421
- autobuilt 6518421

* Thu Jan 23 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.57.dev.gite6cf0ec
- autobuilt e6cf0ec

* Wed Jan 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.56.dev.gitac3a6b8
- autobuilt ac3a6b8

* Wed Jan 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.55.dev.git8b377a7
- autobuilt 8b377a7

* Wed Jan 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.54.dev.gitc42383f
- autobuilt c42383f

* Wed Jan 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.53.dev.gitc40664d
- autobuilt c40664d

* Wed Jan 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.52.dev.git9f146b1
- autobuilt 9f146b1

* Wed Jan 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.51.dev.git7e1afe0
- autobuilt 7e1afe0

* Wed Jan 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.50.dev.git55abb6d
- autobuilt 55abb6d

* Wed Jan 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.49.dev.gitd52132b
- autobuilt d52132b

* Wed Jan 22 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.48.dev.gitaa13779
- autobuilt aa13779

* Tue Jan 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.47.dev.gitf63005e
- autobuilt f63005e

* Tue Jan 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.46.dev.gitf467bb2
- autobuilt f467bb2

* Tue Jan 21 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.45.dev.gitfb2bd26
- autobuilt fb2bd26

* Sat Jan 18 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.44.dev.git9be6430
- autobuilt 9be6430

* Fri Jan 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.43.dev.gitce4bf33
- autobuilt ce4bf33

* Fri Jan 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.42.dev.git3b6a843
- autobuilt 3b6a843

* Fri Jan 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.41.dev.gitab7e1a4
- autobuilt ab7e1a4

* Fri Jan 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.40.dev.gitf5e614b
- autobuilt f5e614b

* Fri Jan 17 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.39.dev.gitacbb6c0
- autobuilt acbb6c0

* Thu Jan 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.38.dev.git74b89da
- autobuilt 74b89da

* Thu Jan 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.37.dev.git79fbe72
- autobuilt 79fbe72

* Thu Jan 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.36.dev.git30245af
- autobuilt 30245af

* Thu Jan 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.35.dev.git1d7176b
- autobuilt 1d7176b

* Thu Jan 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.34.dev.gitdb00ee9
- autobuilt db00ee9

* Thu Jan 16 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.33.dev.git61fbce7
- autobuilt 61fbce7

* Wed Jan 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.32.dev.gite1e405b
- autobuilt e1e405b

* Wed Jan 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.31.dev.git978b891
- autobuilt 978b891

* Wed Jan 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.30.dev.git34429f3
- autobuilt 34429f3

* Wed Jan 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.29.dev.gite025b43
- autobuilt e025b43

* Wed Jan 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.28.dev.gitd914cc2
- autobuilt d914cc2

* Wed Jan 15 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.27.dev.git12aa9ca
- autobuilt 12aa9ca

* Tue Jan 14 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.26.dev.gitad5137b
- autobuilt ad5137b

* Tue Jan 14 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.25.dev.git564bd69
- autobuilt 564bd69

* Tue Jan 14 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.24.dev.git3961882
- autobuilt 3961882

* Mon Jan 13 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.23.dev.git79ec2a9
- autobuilt 79ec2a9

* Mon Jan 13 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.22.dev.git6c3d383
- autobuilt 6c3d383

* Mon Jan 13 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.21.dev.git796ae87
- autobuilt 796ae87

* Mon Jan 13 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.20.dev.gite83a1b8
- autobuilt e83a1b8

* Mon Jan 13 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.19.dev.git9e2e4d7
- autobuilt 9e2e4d7

* Sun Jan 12 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.18.dev.git55dd73c
- autobuilt 55dd73c

* Sat Jan 11 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.17.dev.git2d5fd7c
- autobuilt 2d5fd7c

* Fri Jan 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.16.dev.git0e9c208
- autobuilt 0e9c208

* Fri Jan 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.15.dev.gite1ffac6
- autobuilt e1ffac6

* Fri Jan 10 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.14.dev.git6ed88e0
- autobuilt 6ed88e0

* Thu Jan 09 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.13.dev.gitf57fdd0
- autobuilt f57fdd0

* Thu Jan 09 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.12.dev.git154b5ca
- autobuilt 154b5ca

* Thu Jan 09 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.11.dev.gitf3fc10f
- autobuilt f3fc10f

* Wed Jan 08 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.10.dev.gitc99b413
- autobuilt c99b413

* Wed Jan 08 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.9.dev.gitc6ad42a
- autobuilt c6ad42a

* Wed Jan 08 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.8.dev.git27caffb
- autobuilt 27caffb

* Wed Jan 08 2020 Jindrich Novy <jnovy@redhat.com> - 2:1.7.1-0.7.dev.git0b9dd1a
- require container-selinux only when selinux-policy is installed and
  move podman-remote man pages to dedicated package (#1765818)

* Wed Jan 08 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.6.dev.git0b9dd1a
- autobuilt 0b9dd1a

* Tue Jan 07 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.5.dev.gitc41fd09
- autobuilt c41fd09

* Tue Jan 07 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.4.dev.gitbd3d8f4
- autobuilt bd3d8f4

* Tue Jan 07 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.3.dev.gitf85b3a0
- autobuilt f85b3a0

* Tue Jan 07 2020 Jindrich Novy <jnovy@redhat.com> - 2:1.7.1-0.2.dev.gite362220
- always require container-selinux (#1765818)

* Mon Jan 06 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.1-0.1.dev.gite362220
- bump to 1.7.1
- autobuilt e362220

* Mon Jan 06 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.0-0.28.dev.git2d8f1c8
- autobuilt 2d8f1c8

* Mon Jan 06 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.0-0.27.dev.git2e0157a
- autobuilt 2e0157a

* Mon Jan 06 2020 Jindrich Novy <jnovy@redhat.com> - 2:1.7.0-0.26.dev.git9758a97
- also obsolete former podman-manpages package

* Mon Jan 06 2020 Jindrich Novy <jnovy@redhat.com> - 2:1.7.0-0.25.dev.git9758a97
- add podman-manpages provide to main podman package

* Mon Jan 06 2020 Jindrich Novy <jnovy@redhat.com> - 2:1.7.0-0.24.dev.git9758a97
- merge podman-manpages with podman package

* Fri Jan 03 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.0-0.23.dev.git9758a97
- autobuilt 9758a97

* Thu Jan 02 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.0-0.22.dev.git50b4446
- autobuilt 50b4446

* Thu Jan 02 2020 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.0-0.21.dev.git1faa5bb
- autobuilt 1faa5bb

* Tue Dec 31 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.0-0.20.dev.git6a370cb
- autobuilt 6a370cb

* Fri Dec 20 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.0-0.19.dev.gitfcd48db
- autobuilt fcd48db

* Fri Dec 20 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.0-0.18.dev.gite33d7e9
- autobuilt e33d7e9

* Thu Dec 19 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.0-0.17.dev.git1ba6d0f
- autobuilt 1ba6d0f

* Thu Dec 19 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.0-0.16.dev.gitc1a7911
- autobuilt c1a7911

* Tue Dec 17 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.0-0.15.dev.gite6b8433
- autobuilt e6b8433

* Tue Dec 17 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.0-0.14.dev.gitfab67f3
- autobuilt fab67f3

* Tue Dec 17 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.0-0.13.dev.git1e440a3
- autobuilt 1e440a3

* Tue Dec 17 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.0-0.12.dev.git4329204
- autobuilt 4329204

* Mon Dec 16 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.0-0.11.dev.git1162183
- autobuilt 1162183

* Mon Dec 16 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.0-0.10.dev.gitb2f05e0
- autobuilt b2f05e0

* Mon Dec 16 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.0-0.9.dev.git19064e5
- autobuilt 19064e5

* Sat Dec 14 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.0-0.8.dev.git6c7b6d9
- autobuilt 6c7b6d9

* Fri Dec 13 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.0-0.7.dev.git885967f
- autobuilt 885967f

* Fri Dec 13 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.0-0.6.dev.git22849ff
- autobuilt 22849ff

* Fri Dec 13 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.0-0.5.dev.git71a0c0f
- autobuilt 71a0c0f

* Fri Dec 13 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.0-0.4.dev.git123e7ea
- autobuilt 123e7ea

* Thu Dec 12 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.0-0.3.dev.git16de498
- autobuilt 16de498

* Wed Dec 11 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.0-0.2.dev.gitf81f15f
- autobuilt f81f15f

* Wed Dec 11 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.7.0-0.1.dev.git5941138
- bump to 1.7.0
- autobuilt 5941138

* Wed Dec 11 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.74.dev.git11541ae
- autobuilt 11541ae

* Wed Dec 11 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.73.dev.gitdd64038
- autobuilt dd64038

* Wed Dec 11 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.72.dev.gita18de10
- autobuilt a18de10

* Wed Dec 11 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.71.dev.git282787f
- autobuilt 282787f

* Mon Dec 09 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.70.dev.gitc2dab75
- autobuilt c2dab75

* Sat Dec 07 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.69.dev.git7287f69
- autobuilt 7287f69

* Fri Dec 06 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.68.dev.git82a83b9
- autobuilt 82a83b9

* Fri Dec 06 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.67.dev.git8924a30
- autobuilt 8924a30

* Fri Dec 06 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.66.dev.gite9c4820
- autobuilt e9c4820

* Thu Dec 05 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.65.dev.git465e142
- autobuilt 465e142

* Thu Dec 05 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.64.dev.git4fb724c
- autobuilt 4fb724c

* Thu Dec 05 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.63.dev.git813b00e
- autobuilt 813b00e

* Thu Dec 05 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.62.dev.gitbc40282
- autobuilt bc40282

* Wed Dec 04 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.61.dev.git4dbab37
- autobuilt 4dbab37

* Wed Dec 04 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.60.dev.gite47b7a6
- autobuilt e47b7a6

* Wed Dec 04 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.59.dev.git10f7334
- autobuilt 10f7334

* Tue Dec 03 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.58.dev.git06e2a20
- autobuilt 06e2a20

* Tue Dec 03 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.57.dev.git5c3af00
- autobuilt 5c3af00

* Tue Dec 03 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.56.dev.git748de3c
- autobuilt 748de3c

* Tue Dec 03 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.55.dev.gitd8bfd11
- autobuilt d8bfd11

* Tue Dec 03 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.54.dev.gitb88f2c4
- autobuilt b88f2c4

* Tue Dec 03 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.53.dev.git9e361fd
- autobuilt 9e361fd

* Tue Dec 03 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.52.dev.git309452d
- autobuilt 309452d

* Tue Dec 03 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.51.dev.git6458f96
- autobuilt 6458f96

* Tue Dec 03 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.50.dev.gitb905850
- autobuilt b905850

* Tue Dec 03 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.49.dev.gitc9696c4
- autobuilt c9696c4

* Mon Dec 02 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.48.dev.git7117286
- autobuilt 7117286

* Mon Dec 02 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.47.dev.git8d00c83
- autobuilt 8d00c83

* Mon Dec 02 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.46.dev.gite4275b3
- autobuilt e4275b3

* Fri Nov 29 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.45.dev.git39c705e
- autobuilt 39c705e

* Fri Nov 29 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.44.dev.git7f53178
- autobuilt 7f53178

* Fri Nov 29 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.43.dev.git1c0356e
- autobuilt 1c0356e

* Thu Nov 28 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.42.dev.gitaa95726
- autobuilt aa95726

* Wed Nov 27 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.41.dev.git2178875
- autobuilt 2178875

* Tue Nov 26 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.40.dev.git27a09f8
- autobuilt 27a09f8

* Tue Nov 26 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.39.dev.gitb29928f
- autobuilt b29928f

* Tue Nov 26 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.38.dev.gitf5ef3d5
- autobuilt f5ef3d5

* Tue Nov 26 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.37.dev.gitaef3858
- autobuilt aef3858

* Mon Nov 25 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.36.dev.git9fb0adf
- autobuilt 9fb0adf

* Fri Nov 22 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.35.dev.git6187e72
- autobuilt 6187e72

* Fri Nov 22 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.34.dev.git1284260
- autobuilt 1284260

* Fri Nov 22 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.33.dev.gitc2dfef5
- autobuilt c2dfef5

* Fri Nov 22 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.32.dev.gite4b8054
- autobuilt e4b8054

* Fri Nov 22 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.31.dev.git22e7d7d
- autobuilt 22e7d7d

* Thu Nov 21 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.30.dev.git6392477
- autobuilt 6392477

* Tue Nov 19 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.29.dev.gitc673ff8
- autobuilt c673ff8

* Tue Nov 19 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.28.dev.gitf3f219a
- autobuilt f3f219a

* Mon Nov 18 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.27.dev.git741b90c
- autobuilt 741b90c

* Sun Nov 17 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.26.dev.gitdb32ed1
- autobuilt db32ed1

* Sat Nov 16 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.25.dev.gitc6f2383
- autobuilt c6f2383

* Fri Nov 15 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.24.dev.git51c08f3
- autobuilt 51c08f3

* Thu Nov 14 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.23.dev.gitd7ed9fa
- autobuilt d7ed9fa

* Wed Nov 13 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.22.dev.git225f22b
- autobuilt 225f22b

* Wed Nov 13 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.21.dev.git15220af
- autobuilt 15220af

* Mon Nov 11 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.20.dev.gitde32b89
- autobuilt de32b89

* Fri Nov 08 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.19.dev.gitb713e53
- autobuilt b713e53

* Fri Nov 08 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.18.dev.gitf456ce9
- autobuilt f456ce9

* Fri Nov 08 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.17.dev.git4ed12f9
- autobuilt 4ed12f9

* Fri Nov 08 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.16.dev.git92af260
- autobuilt 92af260

* Fri Nov 08 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.15.dev.git3463a71
- autobuilt 3463a71

* Thu Nov 07 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.14.dev.git3ec9ee0
- autobuilt 3ec9ee0

* Thu Nov 07 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.13.dev.gitd919961
- autobuilt d919961

* Thu Nov 07 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.12.dev.git3474997
- autobuilt 3474997

* Thu Nov 07 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.11.dev.git24efb5e
- autobuilt 24efb5e

* Thu Nov 07 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.10.dev.gitb4a83bf
- autobuilt b4a83bf

* Thu Nov 07 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.9.dev.gitaad2904
- autobuilt aad2904

* Wed Nov 06 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.8.dev.git2e2d82c
- autobuilt 2e2d82c

* Wed Nov 06 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.7.dev.git581a7ec
- autobuilt 581a7ec

* Wed Nov 06 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.6.dev.git6f7c290
- autobuilt 6f7c290

* Tue Nov 05 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.5.dev.gitb4b7272
- autobuilt b4b7272

* Tue Nov 05 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.4.dev.git7eda1b0
- autobuilt 7eda1b0

* Tue Nov 05 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.3.dev.gita904e21
- autobuilt a904e21

* Tue Nov 05 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.2.dev.git08c5c54
- autobuilt 08c5c54

* Tue Nov 05 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.4-0.1.dev.gitcc19b09
- bump to 1.6.4
- autobuilt cc19b09

* Tue Nov 05 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.47.dev.git1db4556
- autobuilt 1db4556

* Mon Nov 04 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.46.dev.git17eadda
- autobuilt 17eadda

* Mon Nov 04 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.45.dev.git8e5aad9
- autobuilt 8e5aad9

* Mon Nov 04 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.44.dev.gitefc7f15
- autobuilt efc7f15

* Sun Nov 03 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.43.dev.gitca4c24c
- autobuilt ca4c24c

* Sat Nov 02 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.42.dev.git2bf4df4
- autobuilt 2bf4df4

* Sat Nov 02 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.41.dev.git10d67fc
- autobuilt 10d67fc

* Fri Nov 01 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.40.dev.git8238107
- autobuilt 8238107

* Fri Nov 01 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.39.dev.git04e8bf3
- autobuilt 04e8bf3

* Fri Nov 01 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.38.dev.git69165fa
- autobuilt 69165fa

* Fri Nov 01 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.37.dev.git7c7f000
- autobuilt 7c7f000

* Thu Oct 31 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.36.dev.git2dae257
- autobuilt 2dae257

* Thu Oct 31 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.35.dev.git0bfdeae
- autobuilt 0bfdeae

* Thu Oct 31 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.34.dev.git1e750f7
- autobuilt 1e750f7

* Thu Oct 31 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.33.dev.git5af166f
- autobuilt 5af166f

* Thu Oct 31 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.32.dev.git1b3e79d
- autobuilt 1b3e79d

* Wed Oct 30 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.31.dev.git381fa4d
- autobuilt 381fa4d

* Wed Oct 30 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.30.dev.git9ba8dae
- autobuilt 9ba8dae

* Wed Oct 30 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.29.dev.gita35d002
- autobuilt a35d002

* Wed Oct 30 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.28.dev.git63b57f5
- autobuilt 63b57f5

* Wed Oct 30 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.27.dev.git4762b63
- autobuilt 4762b63

* Tue Oct 29 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.26.dev.gite7540d0
- autobuilt e7540d0

* Tue Oct 29 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.25.dev.git6c6e783
- autobuilt 6c6e783

* Tue Oct 29 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.24.dev.git59582c5
- autobuilt 59582c5

* Tue Oct 29 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.23.dev.gita56131f
- autobuilt a56131f

* Tue Oct 29 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.22.dev.git8e264ca
- autobuilt 8e264ca

* Mon Oct 28 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.21.dev.git1b5c2d1
- autobuilt 1b5c2d1

* Mon Oct 28 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.20.dev.git94864ad
- autobuilt 94864ad

* Sun Oct 27 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.19.dev.gitac73fd3
- autobuilt ac73fd3

* Sat Oct 26 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.18.dev.gitea46937
- autobuilt ea46937

* Fri Oct 25 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.17.dev.gita01cb22
- autobuilt a01cb22

* Thu Oct 24 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.16.dev.git77c7a28
- autobuilt 77c7a28

* Thu Oct 24 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.15.dev.gitba4a808
- autobuilt ba4a808

* Thu Oct 24 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.14.dev.git43b1c2f
- autobuilt 43b1c2f

* Thu Oct 24 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.13.dev.git674dc2b
- autobuilt 674dc2b

* Wed Oct 23 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.12.dev.git299a430
- autobuilt 299a430

* Wed Oct 23 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.11.dev.git2e6c9aa
- autobuilt 2e6c9aa

* Wed Oct 23 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.10.dev.gitef556cf
- autobuilt ef556cf

* Tue Oct 22 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.9.dev.git46ad6bc
- autobuilt 46ad6bc

* Tue Oct 22 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.8.dev.gitd358840
- autobuilt d358840

* Tue Oct 22 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.7.dev.git5431ace
- autobuilt 5431ace

* Mon Oct 21 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.6.dev.gitefc54c3
- autobuilt efc54c3

* Mon Oct 21 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.5.dev.gitd2591a5
- autobuilt d2591a5

* Sun Oct 20 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.4.dev.gitd3520de
- autobuilt d3520de

* Fri Oct 18 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.3.dev.git02ab9c7
- autobuilt 02ab9c7

* Fri Oct 18 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.2.dev.gitf0da9cf
- autobuilt f0da9cf

* Thu Oct 17 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.3-0.1.dev.gitb6fdfa0
- bump to 1.6.3
- autobuilt b6fdfa0

* Thu Oct 17 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.41.dev.git2b0892e
- autobuilt 2b0892e

* Thu Oct 17 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.40.dev.gitf2d9a9d
- autobuilt f2d9a9d

* Thu Oct 17 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.39.dev.gitd7cbcfa
- autobuilt d7cbcfa

* Thu Oct 17 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.38.dev.git392846c
- autobuilt 392846c

* Wed Oct 16 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.37.dev.gite7d5ac0
- autobuilt e7d5ac0

* Wed Oct 16 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.36.dev.gitdc1f8b6
- autobuilt dc1f8b6

* Wed Oct 16 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.35.dev.git7825c58
- autobuilt 7825c58

* Wed Oct 16 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.34.dev.git8172460
- autobuilt 8172460

* Tue Oct 15 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.33.dev.git5f72e6e
- autobuilt 5f72e6e

* Mon Oct 14 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.32.dev.gita9190da
- autobuilt a9190da

* Mon Oct 14 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.31.dev.git3e45d07
- autobuilt 3e45d07

* Sat Oct 12 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.30.dev.gita8993ba
- autobuilt a8993ba

* Fri Oct 11 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.29.dev.gitb0b3506
- autobuilt b0b3506

* Fri Oct 11 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.28.dev.git79d05b9
- autobuilt 79d05b9

* Fri Oct 11 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.27.dev.gitcee6478
- autobuilt cee6478

* Fri Oct 11 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.26.dev.giteb6ca05
- autobuilt eb6ca05

* Fri Oct 11 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.25.dev.git50b1884
- autobuilt 50b1884

* Fri Oct 11 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.24.dev.git9f1f4ef
- autobuilt 9f1f4ef

* Fri Oct 11 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.23.dev.git495db28
- autobuilt 495db28

* Fri Oct 11 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.22.dev.git43dcc91
- autobuilt 43dcc91

* Thu Oct 10 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.21.dev.git6d35eac
- autobuilt 6d35eac

* Thu Oct 10 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.20.dev.gitf39e097
- autobuilt f39e097

* Thu Oct 10 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.19.dev.gita7f2668
- autobuilt a7f2668

* Wed Oct 09 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.18.dev.git12c9b53
- autobuilt 12c9b53

* Wed Oct 09 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.17.dev.gitf61e399
- autobuilt f61e399

* Wed Oct 09 2019 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:1.6.2-0.16.dev.gitc3c40f9
- remove polkit dependency for now

* Wed Oct 09 2019 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:1.6.2-0.15.dev.gitc3c40f9
- Requires: crun >= 0.10.2-1 and polkit

* Wed Oct 09 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.14.dev.gitc3c40f9
- autobuilt c3c40f9

* Tue Oct 08 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.13.dev.git10cbaad
- autobuilt 10cbaad

* Tue Oct 08 2019 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:1.6.2-0.12.dev.gitc817ea1
- add runc back

* Mon Oct 07 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.11.dev.gitc817ea1
- autobuilt c817ea1

* Mon Oct 07 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.10.dev.git589261f
- autobuilt 589261f

* Fri Oct 04 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.9.dev.git2c2782a
- autobuilt 2c2782a

* Fri Oct 04 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.8.dev.gitbd08fc0
- autobuilt bd08fc0

* Fri Oct 04 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.7.dev.git70d5b0a
- autobuilt 70d5b0a

* Fri Oct 04 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.6.dev.git1fe9556
- autobuilt 1fe9556

* Thu Oct 03 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.5.dev.git7af4074
- autobuilt 7af4074

* Thu Oct 03 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.4.dev.git86c8650
- autobuilt 86c8650

* Thu Oct 03 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.3.dev.gitf96fbfc
- autobuilt f96fbfc

* Thu Oct 03 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.2.dev.gitb32cb4b
- autobuilt b32cb4b

* Wed Oct 02 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.2-0.1.dev.gite67e9e1
- bump to 1.6.2
- autobuilt e67e9e1

* Wed Oct 02 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.1-0.12.dev.git960f07b
- autobuilt 960f07b

* Wed Oct 02 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.1-0.11.dev.git0046b01
- autobuilt 0046b01

* Wed Oct 02 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.1-0.10.dev.gitdac7889
- autobuilt dac7889

* Wed Oct 02 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.1-0.9.dev.git2648955
- autobuilt 2648955

* Wed Oct 02 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.1-0.8.dev.git257a985
- autobuilt 257a985

* Wed Oct 02 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.1-0.7.dev.git32a2ce8
- autobuilt 32a2ce8

* Wed Oct 02 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.1-0.6.dev.git74879c8
- autobuilt 74879c8

* Tue Oct 01 2019 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:1.6.1-0.5.dev.git7a56963
- Requires: crun >= 0.10-1

* Tue Oct 01 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.1-0.4.dev.git7a56963
- autobuilt 7a56963

* Tue Oct 01 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.1-0.3.dev.git8f2ec88
- autobuilt 8f2ec88

* Tue Oct 01 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.1-0.2.dev.git049aafa
- autobuilt 049aafa

* Tue Oct 01 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.1-0.1.dev.git5d344db
- bump to 1.6.1
- autobuilt 5d344db

* Mon Sep 30 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.0-0.42.dev.gitd7eba02
- autobuilt d7eba02

* Mon Sep 30 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.0-0.41.dev.git5702dd7
- autobuilt 5702dd7

* Mon Sep 30 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.0-0.40.dev.gitb063383
- autobuilt b063383

* Mon Sep 30 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.0-0.39.dev.git04b3a73
- autobuilt 04b3a73

* Mon Sep 30 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.0-0.38.dev.git2c23729
- autobuilt 2c23729

* Mon Sep 30 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.0-0.37.dev.git150ba5e
- autobuilt 150ba5e

* Sun Sep 29 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.0-0.36.dev.git01b7af8
- autobuilt 01b7af8

* Sat Sep 28 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.0-0.35.dev.git01a802e
- autobuilt 01a802e

* Fri Sep 27 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.0-0.34.dev.gite87012d
- autobuilt e87012d

* Fri Sep 27 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.0-0.33.dev.git0fb807d
- autobuilt 0fb807d

* Fri Sep 27 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.0-0.32.dev.gitd4399ee
- autobuilt d4399ee

* Fri Sep 27 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.0-0.31.dev.gita8c2b5d
- autobuilt a8c2b5d

* Thu Sep 26 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.0-0.30.dev.git851e377
- autobuilt 851e377

* Thu Sep 26 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.0-0.29.dev.gitd76b21e
- autobuilt d76b21e

* Wed Sep 25 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.0-0.28.dev.git3ed265c
- autobuilt 3ed265c

* Wed Sep 25 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.0-0.27.dev.git19075ca
- autobuilt 19075ca

* Wed Sep 25 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.0-0.26.dev.git8ab3c86
- autobuilt 8ab3c86

* Wed Sep 25 2019 RH Container Bot <rhcontainerbot@fedoraproject.org> - 2:1.6.0-0.25.dev.gitf197ebe
- autobuilt f197ebe

* Wed Sep 25 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.6.0-0.24.dev.git240095e
- autobuilt 240095e

* Wed Sep 25 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.6.0-0.23.dev.git525be7d
- autobuilt 525be7d

* Tue Sep 24 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.6.0-0.22.dev.git0000afc
- autobuilt 0000afc

* Tue Sep 24 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.6.0-0.21.dev.git1dfac0e
- autobuilt 1dfac0e

* Tue Sep 24 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.6.0-0.20.dev.gitb300b98
- autobuilt b300b98

* Tue Sep 24 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.6.0-0.19.dev.git83b2348
- autobuilt 83b2348

* Mon Sep 23 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.6.0-0.18.dev.git6ce8d05
- autobuilt 6ce8d05

* Mon Sep 23 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.6.0-0.17.dev.gitf5951c7
- autobuilt f5951c7

* Mon Sep 23 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.6.0-0.16.dev.gita74dfda
- autobuilt a74dfda

* Sun Sep 22 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.6.0-0.15.dev.gitc0eff1a
- autobuilt c0eff1a

* Sat Sep 21 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.6.0-0.14.dev.git0d95e3a
- autobuilt 0d95e3a

* Sat Sep 21 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.6.0-0.13.dev.gite947d63
- autobuilt e947d63

* Sat Sep 21 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.6.0-0.12.dev.git819b63c
- autobuilt 819b63c

* Fri Sep 20 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.6.0-0.11.dev.git66f4bc7
- autobuilt 66f4bc7

* Fri Sep 20 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.6.0-0.10.dev.git7ed1816
- autobuilt 7ed1816

* Fri Sep 20 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.6.0-0.9.dev.git9dc764c
- autobuilt 9dc764c

* Thu Sep 19 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.6.0-0.8.dev.gitc38844f
- autobuilt c38844f

* Thu Sep 19 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.6.0-0.7.dev.git408f278
- autobuilt 408f278

* Wed Sep 18 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.6.0-0.6.dev.gitfe48b9e
- autobuilt fe48b9e

* Wed Sep 18 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.6.0-0.5.dev.git8133aa1
- autobuilt 8133aa1

* Wed Sep 18 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.6.0-0.4.dev.git2c51d6f
- autobuilt 2c51d6f

* Tue Sep 17 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.6.0-0.3.dev.git143caa9
- autobuilt 143caa9

* Tue Sep 17 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.6.0-0.2.dev.git799aa70
- autobuilt 799aa70

* Mon Sep 16 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.6.0-0.1.dev.git2aa6771
- bump to 1.6.0
- autobuilt 2aa6771

* Mon Sep 16 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.92.dev.git2a4e062
- autobuilt 2a4e062

* Mon Sep 16 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.91.dev.git0014d6c
- autobuilt 0014d6c

* Mon Sep 16 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.90.dev.git1f5514e
- autobuilt 1f5514e

* Sat Sep 14 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.89.dev.gita1970e1
- autobuilt a1970e1

* Sat Sep 14 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.88.dev.git2366fd7
- autobuilt 2366fd7

* Fri Sep 13 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.87.dev.git0079c24
- autobuilt 0079c24

* Fri Sep 13 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.86.dev.gitd74cede
- autobuilt d74cede

* Fri Sep 13 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.85.dev.git7875e00
- autobuilt 7875e00

* Fri Sep 13 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.84.dev.git5c09c4d
- autobuilt 5c09c4d

* Fri Sep 13 2019 Daniel J Walsh <dwalsh@redhat.com> - 2:1.5.2-0.83.dev.gitb095d8a
- Grab specific version of crun or newer.

* Thu Sep 12 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.82.dev.gitb095d8a
- autobuilt b095d8a

* Thu Sep 12 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.81.dev.gitb43a36d
- autobuilt b43a36d

* Thu Sep 12 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.80.dev.git1ddfc11
- autobuilt 1ddfc11

* Thu Sep 12 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.79.dev.gitaf8fedc
- autobuilt af8fedc

* Thu Sep 12 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.78.dev.gitafa3d11
- autobuilt afa3d11

* Thu Sep 12 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.77.dev.git57e093b
- autobuilt 57e093b

* Thu Sep 12 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.76.dev.gitce31aa3
- autobuilt ce31aa3

* Wed Sep 11 2019 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:1.5.2-0.75.dev.git79ebb5f
- use conmon package as dependency

* Wed Sep 11 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.74.dev.git79ebb5f
- autobuilt 79ebb5f

* Wed Sep 11 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.73.dev.gitf73c3b8
- autobuilt f73c3b8

* Wed Sep 11 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.72.dev.git093013b
- autobuilt 093013b

* Wed Sep 11 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.71.dev.git9cf852c
- autobuilt 9cf852c

* Tue Sep 10 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.70.dev.git7ac6ed3
- autobuilt 7ac6ed3

* Tue Sep 10 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.69.dev.git997c4b5
- autobuilt 997c4b5

* Tue Sep 10 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.68.dev.gitc1761ba
- autobuilt c1761ba

* Tue Sep 10 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.67.dev.git095647c
- autobuilt 095647c

* Tue Sep 10 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.66.dev.git5233536
- autobuilt 5233536

* Mon Sep 09 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.65.dev.git9a55bce
- autobuilt 9a55bce

* Mon Sep 09 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.64.dev.git7042a3d
- autobuilt 7042a3d

* Mon Sep 09 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.63.dev.git511b071
- autobuilt 511b071

* Mon Sep 09 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.62.dev.git16a7049
- autobuilt 16a7049

* Mon Sep 09 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.61.dev.gitd78521d
- autobuilt d78521d

* Sun Sep 08 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.60.dev.gitf500feb
- autobuilt f500feb

* Sun Sep 08 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.59.dev.git7312811
- autobuilt 7312811

* Fri Sep 06 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.58.dev.git30cbb00
- autobuilt 30cbb00

* Fri Sep 06 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.57.dev.git290def5
- autobuilt 290def5

* Fri Sep 06 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.56.dev.git575ffee
- autobuilt 575ffee

* Fri Sep 06 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.55.dev.git8898085
- autobuilt 8898085

* Fri Sep 06 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.54.dev.git24171ae
- autobuilt 24171ae

* Thu Sep 05 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.53.dev.gita4572c4
- autobuilt a4572c4

* Thu Sep 05 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.52.dev.gitcef5bec
- autobuilt cef5bec

* Thu Sep 05 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.51.dev.git3f81f44
- autobuilt 3f81f44

* Thu Sep 05 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.50.dev.gitb962b1e
- autobuilt b962b1e

* Wed Sep 04 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.49.dev.gite74fcd7
- autobuilt e74fcd7

* Wed Sep 04 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.48.dev.git84140f5
- autobuilt 84140f5

* Wed Sep 04 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.47.dev.gitf1a3e02
- autobuilt f1a3e02

* Wed Sep 04 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.46.dev.git1d8a940
- autobuilt 1d8a940

* Tue Sep 03 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.45.dev.gita16f63e
- autobuilt a16f63e

* Tue Sep 03 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.44.dev.gitc039499
- autobuilt c039499

* Tue Sep 03 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.43.dev.git50a1910
- autobuilt 50a1910

* Mon Sep 02 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.42.dev.git099549b
- autobuilt 099549b

* Sun Sep 01 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.41.dev.gite5568d4
- autobuilt e5568d4

* Fri Aug 30 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.40.dev.git8ba21ac
- autobuilt 8ba21ac

* Fri Aug 30 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.39.dev.git3e0fdc7
- autobuilt 3e0fdc7

* Thu Aug 29 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.38.dev.gitd110998
- autobuilt d110998

* Thu Aug 29 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.37.dev.gitab5f52c
- autobuilt ab5f52c

* Wed Aug 28 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.36.dev.git1eb6b27
- autobuilt 1eb6b27

* Wed Aug 28 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.35.dev.gitbdf9e56
- autobuilt bdf9e56

* Wed Aug 28 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.34.dev.git4e209fc
- autobuilt 4e209fc

* Wed Aug 28 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.33.dev.git61dc63f
- autobuilt 61dc63f

* Wed Aug 28 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.32.dev.gite5c5a33
- autobuilt e5c5a33

* Wed Aug 28 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.31.dev.gita1a1342
- autobuilt a1a1342

* Tue Aug 27 2019 Daniel J Walsh <dwalsh@redhat.com> - 2:1.5.2-0.30.dev.gitf221c61
- Require crun rather then runc
- Switch to crun by default for cgroupsV2 support

* Tue Aug 27 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.29.dev.gitf221c61
- autobuilt f221c61

* Mon Aug 26 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.28.dev.gitcec354a
- autobuilt cec354a

* Mon Aug 26 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.27.dev.git112a3cc
- autobuilt 112a3cc

* Mon Aug 26 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.26.dev.git67926d8
- autobuilt 67926d8

* Sun Aug 25 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.25.dev.gitc0528c1
- autobuilt c0528c1

* Thu Aug 22 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.24.dev.git59261cf
- autobuilt 59261cf

* Thu Aug 22 2019 Daniel J Walsh <dwalsh@redhat.com> - 2:1.5.2-0.23.dev.gitb263dd9
- Move man5 man pages into podman-manpage package

* Thu Aug 22 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.22.dev.gitb263dd9
- autobuilt b263dd9

* Thu Aug 22 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.21.dev.git34002f9
- autobuilt 34002f9

* Thu Aug 22 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.20.dev.git18f2328
- autobuilt 18f2328

* Wed Aug 21 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.19.dev.gitecc5cc5
- autobuilt ecc5cc5

* Wed Aug 21 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.18.dev.git1ff984d
- autobuilt 1ff984d

* Tue Aug 20 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.17.dev.git1ad8fe5
- autobuilt 1ad8fe5

* Tue Aug 20 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.16.dev.gitf618bc3
- autobuilt f618bc3

* Tue Aug 20 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.15.dev.gita3c46fc
- autobuilt a3c46fc

* Tue Aug 20 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.14.dev.git230faa8
- autobuilt 230faa8

* Tue Aug 20 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.13.dev.git34fc1d0
- autobuilt 34fc1d0

* Mon Aug 19 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.12.dev.git890378e
- autobuilt 890378e

* Mon Aug 19 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.11.dev.gitd23639a
- autobuilt d23639a

* Mon Aug 19 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.10.dev.gitc137e8f
- autobuilt c137e8f

* Mon Aug 19 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.9.dev.gitb1acc43
- autobuilt b1acc43

* Mon Aug 19 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.8.dev.gitbd0b05f
- autobuilt bd0b05f

* Sun Aug 18 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.7.dev.git438cbf4
- autobuilt 438cbf4

* Sat Aug 17 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.6.dev.git76f327f
- autobuilt 76f327f

* Sat Aug 17 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.5.dev.git098ce2f
- autobuilt 098ce2f

* Fri Aug 16 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.4.dev.git8eab96e
- autobuilt 8eab96e

* Fri Aug 16 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.3.dev.git704cc58
- autobuilt 704cc58

* Fri Aug 16 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.2.dev.git2d47f1a
- autobuilt 2d47f1a

* Thu Aug 15 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.2-0.1.dev.git05149e6
- bump to 1.5.2
- autobuilt 05149e6

* Thu Aug 15 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.1-9.16.dev.gitb9a176b
- autobuilt b9a176b

* Thu Aug 15 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.1-8.16.dev.git74224d9
- autobuilt 74224d9

* Thu Aug 15 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.1-7.16.dev.git3f1657d
- autobuilt 3f1657d

* Thu Aug 15 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.1-6.16.dev.gitf9ddf91
- autobuilt f9ddf91

* Wed Aug 14 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.1-5.16.dev.gitbf9e801
- autobuilt bf9e801

* Wed Aug 14 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.1-4.16.dev.gitf5dcb80
- autobuilt f5dcb80

* Wed Aug 14 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.1-3.16.dev.git4823cf8
- autobuilt 4823cf8

* Wed Aug 14 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.1-2.16.dev.gita734b53
- autobuilt a734b53

* Tue Aug 13 2019 Dan Walsh <dwalsh@fedoraproject.org> - 2:1.5.1-1.16.dev.gitce64c14
- Add recommends libvarlink-util

* Tue Aug 13 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.1-0.16.dev.gitce64c14
- autobuilt ce64c14

* Tue Aug 13 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.1-0.15.dev.git7a859f0
- autobuilt 7a859f0

* Tue Aug 13 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.1-0.14.dev.git031437b
- autobuilt 031437b

* Tue Aug 13 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.1-0.13.dev.gitc48243e
- autobuilt c48243e

* Mon Aug 12 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.1-0.12.dev.gitf634fd3
- autobuilt f634fd3

* Mon Aug 12 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.1-0.11.dev.git3cf4567
- autobuilt 3cf4567

* Mon Aug 12 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.1-0.10.dev.git9bee690
- autobuilt 9bee690

* Mon Aug 12 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.1-0.9.dev.gitca7bae7
- autobuilt ca7bae7

* Mon Aug 12 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.1-0.8.dev.gitec93c9d
- autobuilt ec93c9d

* Mon Aug 12 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.1-0.7.dev.gitf18cfa4
- autobuilt f18cfa4

* Mon Aug 12 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.1-0.6.dev.git2348c28
- autobuilt 2348c28

* Sun Aug 11 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.1-0.5.dev.git1467197
- autobuilt 1467197

* Sun Aug 11 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.1-0.4.dev.git7bbaa36
- autobuilt 7bbaa36

* Sat Aug 10 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.1-0.3.dev.git3bc861c
- autobuilt 3bc861c

* Sat Aug 10 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.1-0.2.dev.git926901d
- autobuilt 926901d

* Fri Aug 09 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.5.1-0.1.dev.git2018faa
- bump to 1.5.1
- autobuilt 2018faa

* Fri Aug 09 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.99.dev.gitbb80586
- autobuilt bb80586

* Fri Aug 09 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.98.dev.gitd05798e
- autobuilt d05798e

* Fri Aug 09 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.97.dev.git4b91f60
- autobuilt 4b91f60

* Fri Aug 09 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.96.dev.gitdc38168
- autobuilt dc38168

* Fri Aug 09 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.95.dev.git00a20f7
- autobuilt 00a20f7

* Fri Aug 09 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.94.dev.git2a19036
- autobuilt 2a19036

* Fri Aug 09 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.93.dev.git76840f2
- autobuilt 76840f2

* Thu Aug 08 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.92.dev.git4349f42
- autobuilt 4349f42

* Thu Aug 08 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.91.dev.git202eade
- autobuilt 202eade

* Thu Aug 08 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.90.dev.git09cedd1
- autobuilt 09cedd1

* Thu Aug 08 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.89.dev.git3959a35
- autobuilt 3959a35

* Thu Aug 08 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.88.dev.git5701fe6
- autobuilt 5701fe6

* Thu Aug 08 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.87.dev.git31bfb12
- autobuilt 31bfb12

* Thu Aug 08 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.86.dev.git41de7b1
- autobuilt 41de7b1

* Wed Aug 07 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.85.dev.git35ecf49
- autobuilt 35ecf49

* Wed Aug 07 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.84.dev.git66ea32c
- autobuilt 66ea32c

* Tue Aug 06 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.83.dev.gitf0a5b7f
- autobuilt f0a5b7f

* Tue Aug 06 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.82.dev.gitb5618d9
- autobuilt b5618d9

* Mon Aug 05 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.81.dev.git3bffe77
- autobuilt 3bffe77

* Mon Aug 05 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.80.dev.git337358a
- autobuilt 337358a

* Mon Aug 05 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.79.dev.git626dfdb
- autobuilt 626dfdb

* Mon Aug 05 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.78.dev.gite2f38cd
- autobuilt e2f38cd

* Mon Aug 05 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.77.dev.gitb609de2
- autobuilt b609de2

* Sun Aug 04 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.76.dev.git389a7b7
- autobuilt 389a7b7

* Sun Aug 04 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.75.dev.gitd9ea4db
- autobuilt d9ea4db

* Fri Aug 02 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.74.dev.git140e08e
- autobuilt 140e08e

* Fri Aug 02 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.73.dev.git3cc9ab8
- autobuilt 3cc9ab8

* Fri Aug 02 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.72.dev.git5370c53
- autobuilt 5370c53

* Fri Aug 02 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.71.dev.git2cc5913
- autobuilt 2cc5913

* Fri Aug 02 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.70.dev.gite3240da
- autobuilt e3240da

* Fri Aug 02 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.69.dev.gite48dc50
- autobuilt e48dc50

* Thu Aug 01 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.68.dev.git1bbcb2f
- autobuilt 1bbcb2f

* Thu Aug 01 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.67.dev.gite1a099e
- autobuilt e1a099e

* Thu Aug 01 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.66.dev.gitafb493a
- autobuilt afb493a

* Thu Aug 01 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.65.dev.git6f62dac
- autobuilt 6f62dac

* Thu Aug 01 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.64.dev.gitee15e76
- autobuilt ee15e76

* Thu Aug 01 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.63.dev.git5056964
- autobuilt 5056964

* Thu Aug 01 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.62.dev.git3215ea6
- autobuilt 3215ea6

* Thu Aug 01 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.61.dev.gitccf4ec2
- autobuilt ccf4ec2

* Wed Jul 31 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.60.dev.gita622f8d
- autobuilt a622f8d

* Tue Jul 30 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.59.dev.git680a383
- autobuilt 680a383

* Tue Jul 30 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.58.dev.gite84ed3c
- autobuilt e84ed3c

* Tue Jul 30 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.57.dev.git1a00895
- autobuilt 1a00895

* Tue Jul 30 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.56.dev.git4196a59
- autobuilt 4196a59

* Tue Jul 30 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.55.dev.git040355d
- autobuilt 040355d

* Mon Jul 29 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.54.dev.git7d635ac
- autobuilt 7d635ac

* Mon Jul 29 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.53.dev.gitc3c45f3
- autobuilt c3c45f3

* Mon Jul 29 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.52.dev.git6665269
- autobuilt 6665269

* Mon Jul 29 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.51.dev.git2ca7861
- autobuilt 2ca7861

* Sun Jul 28 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.50.dev.git2c98bd5
- autobuilt 2c98bd5

* Fri Jul 26 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.49.dev.git0c4dfcf
- autobuilt 0c4dfcf

* Fri Jul 26 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.48.dev.giteca157f
- autobuilt eca157f

* Fri Jul 26 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.47.dev.git1910d68
- autobuilt 1910d68

* Fri Jul 26 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.46.dev.git4674d00
- autobuilt 4674d00

* Thu Jul 25 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.45.dev.gitdff82d9
- autobuilt dff82d9

* Thu Jul 25 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.44.dev.git5763618
- autobuilt 5763618

* Thu Jul 25 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.43.dev.git7c9095e
- autobuilt 7c9095e

* Wed Jul 24 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.42.dev.git2283471
- autobuilt 2283471

* Wed Jul 24 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.41.dev.git0917783
- autobuilt 0917783

* Wed Jul 24 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.40.dev.giteae9a00
- autobuilt eae9a00

* Wed Jul 24 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.39.dev.git3c6b111
- autobuilt 3c6b111

* Tue Jul 23 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.38.dev.git7dbc6d8
- autobuilt 7dbc6d8

* Tue Jul 23 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.37.dev.gitbb253af
- autobuilt bb253af

* Tue Jul 23 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.36.dev.gitce60c4d
- autobuilt ce60c4d

* Tue Jul 23 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.35.dev.git2674920
- autobuilt 2674920

* Mon Jul 22 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.34.dev.gita12a231
- autobuilt a12a231

* Mon Jul 22 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.33.dev.gitcf9efa9
- autobuilt cf9efa9

* Mon Jul 22 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.32.dev.git69f74f1
- autobuilt 69f74f1

* Mon Jul 22 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.31.dev.gitab7b47c
- autobuilt ab7b47c

* Mon Jul 22 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.30.dev.git3b52e4d
- autobuilt 3b52e4d

* Sun Jul 21 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.29.dev.gitd6b41eb
- autobuilt d6b41eb

* Sat Jul 20 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.28.dev.gita5aa44c
- autobuilt a5aa44c

* Sat Jul 20 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.27.dev.git8364552
- autobuilt 8364552

* Fri Jul 19 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.26.dev.git02140ea
- autobuilt 02140ea

* Fri Jul 19 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.25.dev.git398aeac
- autobuilt 398aeac

* Fri Jul 19 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.24.dev.gitdeb087d
- autobuilt deb087d

* Fri Jul 19 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.23.dev.gitb59abdc
- autobuilt b59abdc

* Thu Jul 18 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.22.dev.git2254a35
- autobuilt 2254a35

* Thu Jul 18 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.21.dev.git1065548
- autobuilt 1065548

* Thu Jul 18 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.20.dev.gitade0d87
- autobuilt ade0d87

* Thu Jul 18 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.19.dev.git22e62e8
- autobuilt 22e62e8

* Thu Jul 18 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.18.dev.gitadcde23
- autobuilt adcde23

* Thu Jul 18 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.17.dev.git456c045
- autobuilt 456c045

* Thu Jul 18 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.16.dev.git7488ed6
- autobuilt 7488ed6

* Thu Jul 18 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.15.dev.gitb2734ba
- autobuilt b2734ba

* Wed Jul 17 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.14.dev.git1c02905
- autobuilt 1c02905

* Wed Jul 17 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.13.dev.git04a9cb0
- autobuilt 04a9cb0

* Tue Jul 16 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.12.dev.gitfe83308
- autobuilt fe83308

* Tue Jul 16 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.11.dev.git400851a
- autobuilt 400851a

* Tue Jul 16 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.10.dev.gita449e9a
- autobuilt a449e9a

* Tue Jul 16 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.9.dev.git386ffd2
- autobuilt 386ffd2

* Tue Jul 16 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.8.dev.git7e4db44
- autobuilt 7e4db44

* Mon Jul 15 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.7.dev.gitd2291ec
- autobuilt d2291ec

* Sun Jul 14 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.6.dev.git456b6ab
- autobuilt 456b6ab

* Fri Jul 12 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.5.dev.gite2e8477
- built conmon 1de71ad

* Thu Jul 11 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.4.dev.gite2e8477
- autobuilt e2e8477

* Wed Jul 10 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.5-0.3.dev.gitdf3f5af
- autobuilt df3f5af

* Tue Jul 09 2019 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:1.4.5-0.2.dev.gitcea0e93
- Resolves: #1727933 - containers-monuts.conf.5 moved to containers-common

* Sun Jul 07 2019 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:1.4.5-0.1.dev.gitf7407f2
- bump to v1.4.5-dev
- use new name for go-md2man
- include centos conditionals

* Sun Jun 23 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.3-0.30.dev.git7c4e444
- autobuilt 7c4e444

* Sat Jun 22 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.3-0.29.dev.gitd9bdd3c
- autobuilt d9bdd3c

* Fri Jun 21 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.3-0.28.dev.git39fdf91
- autobuilt 39fdf91

* Thu Jun 20 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.3-0.27.dev.gitb4f9bc8
- autobuilt b4f9bc8

* Wed Jun 19 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.3-0.26.dev.git240b846
- bump to 1.4.3
- autobuilt 240b846

* Tue Jun 18 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.2-0.25.dev.git8bcfd24
- autobuilt 8bcfd24

* Sun Jun 16 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.2-0.24.dev.git670fc03
- autobuilt 670fc03

* Sat Jun 15 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.2-0.23.dev.git185b413
- bump to 1.4.2
- autobuilt 185b413

* Fri Jun 14 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.1-0.22.dev.git2784cf3
- autobuilt 2784cf3

* Thu Jun 13 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.1-0.21.dev.git77d1cf0
- autobuilt 77d1cf0

* Wed Jun 12 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.1-0.20.dev.gitf8a84fd
- autobuilt f8a84fd

* Tue Jun 11 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.1-0.19.dev.gitc93b8d6
- do not install /usr/libexec/crio - conflicts with crio

* Tue Jun 11 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.1-0.18.dev.gitc93b8d6
- autobuilt c93b8d6

* Mon Jun 10 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.1-0.17.dev.gitfcb7c14
- autobuilt fcb7c14

* Sun Jun 09 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.1-0.16.dev.git39f5ea4
- autobuilt 39f5ea4

* Sat Jun 08 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.4.1-0.15.dev.gitcae5af5
- bump to 1.4.1
- autobuilt cae5af5

* Fri Jun 07 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.2-0.14.dev.gitba36a5f
- autobuilt ba36a5f

* Fri Jun 07 2019 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:1.3.2-0.13.dev.git6d285b8
- Resolves: #1716809 - use conmon v0.2.0

* Thu Jun 06 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.2-0.12.dev.git6d285b8
- autobuilt 6d285b8

* Wed Jun 05 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.2-0.11.dev.git3fb9669
- autobuilt 3fb9669

* Tue Jun 04 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.2-0.10.dev.git0ede794
- autobuilt 0ede794

* Sun Jun 02 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.2-0.9.dev.git176a41c
- autobuilt 176a41c

* Sat Jun 01 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.2-0.8.dev.git2068919
- autobuilt 2068919

* Fri May 31 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.2-0.7.dev.git558ce8d
- autobuilt 558ce8d

* Thu May 30 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.2-0.6.dev.gitc871653
- autobuilt c871653

* Wed May 29 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.2-0.5.dev.git8649dbd
- autobuilt 8649dbd

* Mon May 27 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.2-0.4.dev.git25f8c21
- autobuilt 25f8c21

* Sun May 26 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.2-0.3.dev.gitb1d590b
- autobuilt b1d590b

* Fri May 24 2019 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:1.3.2-0.2.dev.git1ac06d8
- built commit 1ac06d8
- BR: systemd-devel
- correct build steps for %%{name}-remote

* Fri May 24 2019 Dan Walsh <dwalsh@fedoraproject.org> - 2:1.3.2-0.1.dev.git5296428
- Bump up to latest on master

* Fri May 10 2019 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:1.3.1-0.1.dev.git9ae3221
- bump to v1.3.1-dev
- built 9ae3221
- correct release tag format for unreleased versions

* Thu Apr 25 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.0-21.dev.gitb01fdcb
- autobuilt b01fdcb

* Tue Apr 23 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.0-20.dev.gitd652c86
- autobuilt d652c86

* Sat Apr 20 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.0-19.dev.git9f92b21
- autobuilt 9f92b21

* Fri Apr 19 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.0-18.dev.gite4947e5
- autobuilt e4947e5

* Thu Apr 18 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.0-17.dev.gitbf5ffda
- autobuilt bf5ffda

* Wed Apr 17 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.0-16.dev.gita87cf6f
- autobuilt a87cf6f

* Tue Apr 16 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.0-15.dev.gitc1e2b58
- autobuilt c1e2b58

* Mon Apr 15 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.0-14.dev.git167ce59
- autobuilt 167ce59

* Sun Apr 14 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.0-13.dev.gitb926005
- autobuilt b926005

* Sat Apr 13 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.0-12.dev.git1572367
- autobuilt 1572367

* Fri Apr 12 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.0-11.dev.git387d601
- autobuilt 387d601

* Thu Apr 11 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.0-10.dev.git6cd6eb6
- autobuilt 6cd6eb6

* Wed Apr 10 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.0-9.dev.git60ef8f8
- autobuilt 60ef8f8

* Tue Apr 09 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.0-8.dev.gitc94903a
- autobuilt c94903a

* Sat Apr 06 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.0-7.dev.gitbc320be
- autobuilt bc320be

* Fri Apr 05 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.0-6.dev.gitbda28c6
- autobuilt bda28c6

* Thu Apr 04 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.0-5.dev.git4bda537
- autobuilt 4bda537

* Wed Apr 03 2019 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:1.3.0-4.dev.gitad467ba
- Resolves: #1695492 - own /usr/libexec/podman

* Tue Apr 02 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.0-3.dev.gitad467ba
- autobuilt ad467ba

* Mon Apr 01 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.3.0-2.dev.gitcd35e20
- bump to 1.3.0
- autobuilt cd35e20

* Sun Mar 31 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.2.0-30.dev.git833204d
- autobuilt 833204d

* Sat Mar 30 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.2.0-29.dev.git7b73974
- autobuilt 7b73974

* Fri Mar 29 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.2.0-28.dev.gitfdf979a
- autobuilt fdf979a

* Thu Mar 28 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.2.0-27.dev.git850326c
- autobuilt 850326c

* Wed Mar 27 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.2.0-26.dev.gitfc546d4
- autobuilt fc546d4

* Mon Mar 25 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.2.0-25.dev.gitd0c6a35
- autobuilt d0c6a35

* Sat Mar 23 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.2.0-24.dev.git0458daf
- autobuilt 0458daf

* Fri Mar 22 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.2.0-23.dev.git68e3df3
- autobuilt 68e3df3

* Thu Mar 21 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.2.0-22.dev.gitc230f0c
- autobuilt c230f0c

* Wed Mar 20 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.2.0-21.dev.git537c382
- autobuilt 537c382

* Tue Mar 19 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.2.0-20.dev.gitac523cb
- autobuilt ac523cb

* Mon Mar 18 2019 Eduardo Santiago <santiago@redhat.com> - 2:1.2.0-19.dev.git6aa8078
- include zsh completion

* Fri Mar 15 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.2.0-18.dev.git31f11a8
- autobuilt 31f11a8

* Thu Mar 14 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.2.0-17.dev.git7426d4f
- autobuilt 7426d4f

* Wed Mar 13 2019 Eduardo Santiago <santiago@redhat.com> - 2:1.2.0-16.dev.git883566f
- new -tests subpackage

* Wed Mar 13 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.2.0-15.dev.git883566f
- autobuilt 883566f

* Tue Mar 12 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.2.0-14.dev.gitde0192a
- autobuilt de0192a

* Sun Mar 10 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.2.0-13.dev.gitd95f97a
- autobuilt d95f97a

* Sat Mar 09 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.2.0-12.dev.git9b21f14
- autobuilt 9b21f14

* Fri Mar 08 2019 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:1.2.0-11.dev.git1b2f867
- Resolves: #1686813 - conmon bundled inside podman rpm

* Fri Mar 08 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.2.0-10.dev.git1b2f867
- autobuilt 1b2f867

* Thu Mar 07 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.2.0-9.dev.git614409f
- autobuilt 614409f

* Wed Mar 06 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.2.0-8.dev.git40f7843
- autobuilt 40f7843

* Tue Mar 05 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.2.0-7.dev.git4b80517
- autobuilt 4b80517

* Mon Mar 04 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.2.0-6.dev.gitf3a3d8e
- autobuilt f3a3d8e

* Sat Mar 02 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.2.0-5.dev.git9adcda7
- autobuilt 9adcda7

* Fri Mar 01 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.2.0-4.dev.git9137315
- autobuilt 9137315

* Thu Feb 28 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.2.0-3.dev.git5afae0b
- autobuilt 5afae0b

* Wed Feb 27 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.2.0-2.dev.git623fcfa
- bump to 1.2.0
- autobuilt 623fcfa

* Tue Feb 26 2019 Dan Walsh <dwalsh@fedoraproject.org> - 2:1.0.1-39.dev.gitcf52144
* Tue Feb 26 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-38.dev.gitcf52144
- autobuilt cf52144

* Mon Feb 25 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-37.dev.git553ac80
- autobuilt 553ac80

* Sun Feb 24 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-36.dev.gitcc4addd
- autobuilt cc4addd

* Sat Feb 23 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-35.dev.gitb223d4e
- autobuilt b223d4e

* Fri Feb 22 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-34.dev.git1788add
- autobuilt 1788add

* Thu Feb 21 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-33.dev.git4934bf2
- autobuilt 4934bf2

* Wed Feb 20 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-32.dev.git3b88c73
- autobuilt 3b88c73

* Tue Feb 19 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-31.dev.git228d1cb
- autobuilt 228d1cb

* Mon Feb 18 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-30.dev.git3f32eae
- autobuilt 3f32eae

* Sun Feb 17 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-29.dev.git1cb16bd
- autobuilt 1cb16bd

* Sat Feb 16 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-28.dev.git0a521e1
- autobuilt 0a521e1

* Fri Feb 15 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-27.dev.git81ace5c
- autobuilt 81ace5c

* Thu Feb 14 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-26.dev.gitdfc64e1
- autobuilt dfc64e1

* Wed Feb 13 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-25.dev.gitee27c39
- autobuilt ee27c39

* Tue Feb 12 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-24.dev.git8923703
- autobuilt 8923703

* Sun Feb 10 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-23.dev.gitc86e8f1
- autobuilt c86e8f1

* Sat Feb 09 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-22.dev.gitafd4d5f
- autobuilt afd4d5f

* Fri Feb 08 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-21.dev.git962850c
- autobuilt 962850c

* Thu Feb 07 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-20.dev.gitf250745
- autobuilt f250745

* Wed Feb 06 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-19.dev.git650e242
- autobuilt 650e242

* Tue Feb 05 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-18.dev.git778f986
- autobuilt 778f986

* Sun Feb 03 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-17.dev.gitd5593b8
- autobuilt d5593b8

* Sat Feb 02 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-16.dev.gite6426af
- autobuilt e6426af

* Fri Feb 01 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-15.dev.gite97dc8e
- autobuilt e97dc8e

* Thu Jan 31 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-14.dev.git805c6d9
- autobuilt 805c6d9

* Wed Jan 30 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-13.dev.gitad5579e
- autobuilt ad5579e

* Tue Jan 29 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-12.dev.gitebe9297
- autobuilt ebe9297

* Thu Jan 24 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-11.dev.gitc9e1f36
- autobuilt c9e1f36

* Wed Jan 23 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-10.dev.git7838a13
- autobuilt 7838a13

* Tue Jan 22 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-9.dev.gitec96987
- autobuilt ec96987

* Mon Jan 21 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-8.dev.gitef2f6f9
- autobuilt ef2f6f9

* Sun Jan 20 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-7.dev.git579fc0f
- autobuilt 579fc0f

* Sat Jan 19 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-6.dev.git0d4bfb0
- autobuilt 0d4bfb0

* Fri Jan 18 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-5.dev.gite3dc660
- autobuilt e3dc660

* Thu Jan 17 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-4.dev.git0e3264a
- autobuilt 0e3264a

* Wed Jan 16 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-3.dev.git1b2f752
- autobuilt 1b2f752

* Tue Jan 15 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:1.0.1-2.dev.git6301f6a
- bump to 1.0.1
- autobuilt 6301f6a

* Mon Jan 14 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.12.2-3.dev.git140ae25
- autobuilt 140ae25

* Sat Jan 12 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.12.2-2.dev.git5c86efb
- bump to 0.12.2
- autobuilt 5c86efb

* Fri Jan 11 2019 bbaude <bbaude@redhat.com> - 1:1.0.0-1.dev.git82e8011
- Upstream 1.0.0 release

* Thu Jan 10 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.12.2-27.dev.git0f6535c
- autobuilt 0f6535c

* Wed Jan 09 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.12.2-26.dev.gitc9d63fe
- autobuilt c9d63fe

* Tue Jan 08 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.12.2-25.dev.gitfaa2462
- autobuilt faa2462

* Mon Jan 07 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.12.2-24.dev.gitb83b07c
- autobuilt b83b07c

* Sat Jan 05 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.12.2-23.dev.git4e0c0ec
- autobuilt 4e0c0ec

* Fri Jan 04 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.12.2-22.dev.git9ffd480
- autobuilt 9ffd480

* Thu Jan 03 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.12.2-21.dev.git098c134
- autobuilt 098c134

* Tue Jan 01 2019 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.12.2-20.dev.git7438b7b
- autobuilt 7438b7b

* Sat Dec 29 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.12.2-1.nightly.git5c86efb9.dev.git1aa55ed
- autobuilt 1aa55ed

* Thu Dec 27 2018 Igor Gnatenko <ignatenkobrain@fedoraproject.org> - 2:0.12.2-1.nightly.git5c86efb8.dev.gitc50332d
- Enable python dependency generator

* Tue Dec 25 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.12.2-1.nightly.git5c86efb7.dev.gitc50332d
- autobuilt c50332d

* Mon Dec 24 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.12.2-1.nightly.git5c86efb6.dev.git8fe3050
- autobuilt 8fe3050

* Sun Dec 23 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.12.2-1.nightly.git5c86efb5.dev.git792f109
- autobuilt 792f109

* Sat Dec 22 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.12.2-1.nightly.git5c86efb4.dev.gitfe186c6
- autobuilt fe186c6

* Fri Dec 21 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.12.2-1.nightly.git5c86efb3.dev.gitfa998f2
- autobuilt fa998f2

* Thu Dec 20 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.12.2-1.nightly.git5c86efb2.dev.git6b059a5
- autobuilt 6b059a5

* Wed Dec 19 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.12.2-1.nightly.git5c86efb1.dev.gitc8eaf59
- autobuilt c8eaf59

* Tue Dec 18 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.12.2-1.nightly.git5c86efb0.dev.git68414c5
- autobuilt 68414c5

* Mon Dec 17 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.12.2-9.dev.gitb21d474
- autobuilt b21d474

* Sat Dec 15 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.12.2-8.dev.gitc086118
- autobuilt c086118

* Fri Dec 14 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.12.2-7.dev.git93b5ccf
- autobuilt 93b5ccf

* Thu Dec 13 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.12.2-6.dev.git508388b
- autobuilt 508388b

* Wed Dec 12 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.12.2-5.dev.git8a3361f
- autobuilt 8a3361f

* Tue Dec 11 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.12.2-4.dev.git235a630
- autobuilt 235a630

* Sat Dec 08 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.12.2-3.dev.git1f547b2
- autobuilt 1f547b2

* Fri Dec 07 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.12.2-2.dev.gita387c72
- bump to 0.12.2
- autobuilt a387c72

* Thu Dec 06 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.11.2-15.dev.git75b19ca
- autobuilt 75b19ca

* Wed Dec 05 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.11.2-14.dev.git320085a
- autobuilt 320085a

* Tue Dec 04 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.11.2-13.dev.git5f6ad82
- autobuilt 5f6ad82

* Sun Dec 02 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.11.2-12.dev.git41f250c
- autobuilt 41f250c

* Sat Dec 01 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.11.2-11.dev.git6b8f89d
- autobuilt 6b8f89d

* Thu Nov 29 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.11.2-10.dev.git3af62f6
- autobuilt 3af62f6

* Tue Nov 27 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.11.2-9.dev.git3956050
- autobuilt 3956050

* Mon Nov 26 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.11.2-8.dev.gite3ece3b
- autobuilt e3ece3b

* Sat Nov 24 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.11.2-7.dev.git78604c3
- autobuilt 78604c3

* Thu Nov 22 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.11.2-6.dev.git1fdfeb8
- autobuilt 1fdfeb8

* Wed Nov 21 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.11.2-5.dev.git23feb0d
- autobuilt 23feb0d

* Tue Nov 20 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.11.2-4.dev.gitea928f2
- autobuilt ea928f2

* Sat Nov 17 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.11.2-3.dev.gitcd5742f
- autobuilt cd5742f

* Fri Nov 16 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 2:0.11.2-2.dev.git236408b
- autobuilt 236408b

* Wed Nov 14 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 2:0.11.2-1.dev.git97bded4
- bump epoch cause previous version was messed up
- built 97bded4

* Tue Nov 13 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 1:0.11.20.11.2-1.dev.git79657161
- bump to 0.11.2
- autobuilt 7965716

* Sat Nov 10 2018 Dan Walsh <dwalsh@redhat.com> - 1:0.11.20.11.2-2.dev.git78e6d8e1
- Remove dirty flag from podman version


* Sat Nov 10 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 1:0.11.20.11.2-1.dev.git7965716.dev.git78e6d8e1
- bump to 0.11.2
- autobuilt 78e6d8e

* Fri Nov 09 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 1:0.11.20.11.2-1.dev.git7965716.dev.git78e6d8e.dev.gitf5473c61
- bump to 0.11.2
- autobuilt f5473c6

* Thu Nov 08 2018 baude <bbaude@redhat.com> - 1:0.11.1-1.dev.gita4adfe5
- Upstream 0.11.1-1

* Thu Nov 08 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 1:0.10.2-3.dev.git672f572
- autobuilt 672f572

* Wed Nov 07 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 1:0.10.2-2.dev.gite9f8aed
- autobuilt e9f8aed

* Sun Oct 28 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 1:0.10.2-1.dev.git4955572
- Resolves: #1643744 - build podman with ostree support
- bump to v0.10.2
- built commit 4955572

* Fri Oct 19 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 1:0.10.1.3-3.dev.gitdb08685
- consistent epoch:version-release in changelog

* Thu Oct 18 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 1:0.10.1.3-2.dev.gitdb08685
- correct epoch mentions

* Thu Oct 18 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 1:0.10.1.3-1.dev.gitdb08685
- bump to v0.10.1.3

* Thu Oct 11 2018 baude <bbaude@redhat.com> - 1:0.10.1-1.gitda5c894
- Upstream v0.10.1 release

* Fri Sep 28 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 1:0.9.4-3.dev.gite7e81e6
- built libpod commit e7e81e6
- built conmon from cri-o commit 2cbe48b

* Tue Sep 25 2018 Dan Walsh <dwalsh@redhat.com> - 1:0.9.4-2.dev.gitaf791f3
- Fix required version of runc

* Mon Sep 24 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 1:0.9.4-1.dev.gitaf791f3
- bump to v0.9.4
- built af791f3

* Wed Sep 19 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 1:0.9.3-2.dev.gitc3a0874
- autobuilt c3a0874

* Mon Sep 17 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 1:0.9.3-1.dev.git28a2bf8
- bump to v0.9.3
- built commit 28a2bf82

* Tue Sep 11 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 1:0.9.1.1-1.dev.git95dbcad
- bump to v0.9.1.1
- built commit 95dbcad

* Tue Sep 11 2018 baude <bbaude@redhat.com> - 1:0.9.1-1.dev.git123de30
- Upstream release of 0.9.1
- Do not build with devicemapper

* Tue Sep 4 2018 Dan Walsh <dwalsh@redhat.com> - 1:0.8.5-5.git65c31d4
- Fix required version of runc

* Tue Sep 4 2018 Dan Walsh <dwalsh@redhat.com> - 1:0.8.5-4.dev.git65c31d4
- Fix rpm -qi podman to show the correct URL

* Tue Sep 4 2018 Dan Walsh <dwalsh@redhat.com> - 1:0.8.5-3.dev.git65c31d4
- Fix required version of runc

* Mon Sep 3 2018 Dan Walsh <dwalsh@redhat.com> - 1:0.8.5-2.dev.git65c31d4
- Add a specific version of runc or later to require

* Thu Aug 30 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 1:0.8.5-1.dev.git65c31d4
- bump to v0.8.5-dev
- built commit 65c31d4
- correct min dep on containernetworking-plugins for upgrade from
containernetworking-cni

* Mon Aug 20 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 1:0.8.3-4.dev.git3d55721f
- Resolves: #1619411 - python3-podman should require python3-psutil
- podman-docker should conflict with moby-engine
- require nftables
- recommend slirp4netns and fuse-overlayfs (latter only for kernel >= 4.18)

* Sun Aug 12 2018 Dan Walsh <dwalsh@redhat.com> - 1:0.8.3-3.dev.git3d55721f
- Add podman-docker support
- Force cgroupfs for non root podman

* Sun Aug 12 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 1:0.8.3-2.dev.git3d55721f
- Requires: conmon
- use default %%gobuild

* Sat Aug 11 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 1:0.8.3-1.dev.git3d55721f
- bump to v0.8.3-dev
- built commit 3d55721f
- bump Epoch to 1, cause my autobuilder messed up earlier

* Wed Aug 01 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.8.10.8.1-1.dev.git1a439f91
- bump to 0.8.1
- autobuilt 1a439f9

* Tue Jul 31 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.8.10.8.1-1.dev.git1a439f9.dev.git5a4e5901
- bump to 0.8.1
- autobuilt 5a4e590

* Sun Jul 29 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.8.10.8.1-1.dev.git1a439f9.dev.git5a4e590.dev.git433cbd51
- bump to 0.8.1
- autobuilt 433cbd5

* Sat Jul 28 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.8.10.8.1-1.dev.git1a439f9.dev.git5a4e590.dev.git433cbd5.dev.git87d8edb1
- bump to 0.8.1
- autobuilt 87d8edb

* Fri Jul 27 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 0.7.4-7.dev.git3dd577e
- fix python package version

* Fri Jul 27 2018 Igor Gnatenko <ignatenkobrain@fedoraproject.org> - 0.7.4-6.dev.git3dd577e
- Rebuild for new binutils

* Fri Jul 27 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.7.4-5.dev.git3dd577e
- autobuilt 3dd577e

* Thu Jul 26 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.7.4-4.dev.git9c806a4
- autobuilt 9c806a4

* Wed Jul 25 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.7.4-3.dev.gitc90b740
- autobuilt c90b740

* Tue Jul 24 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 0.7.4-2.dev.git9a18681
- pypodman package exists only if varlink

* Mon Jul 23 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 0.7.4-1.dev.git9a18681
- bump to v0.7.4-dev
- built commit 9a18681

* Mon Jul 23 2018 Dan Walsh <dwalsh@redhat.com> - 0.7.3-2.dev.git06c546e
- Add Reccommeds container-selinux

* Sun Jul 15 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 0.7.3-1.dev.git06c546e
- built commit 06c546e

* Sat Jul 14 2018 Dan Walsh <dwalsh@redhat.com> - 0.7.2-10.dev.git86154b6
- Add install of pypodman

* Fri Jul 13 2018 Fedora Release Engineering <releng@fedoraproject.org> - 0.7.2-9.dev.git86154b6
- Rebuilt for https://fedoraproject.org/wiki/Fedora_29_Mass_Rebuild

* Thu Jul 12 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.7.2-8.dev.git86154b6
- autobuilt 86154b6

* Wed Jul 11 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.7.2-7.dev.git84cfdb2
- autobuilt 84cfdb2

* Tue Jul 10 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.7.2-6.dev.git4f9b1ae
- autobuilt 4f9b1ae

* Mon Jul 09 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.7.2-5.gitc7424b6
- autobuilt c7424b6

* Mon Jul 09 2018 Dan Walsh <dwalsh@redhat.com> - 0.7.2-4.gitf661e1d
- Add ostree support

* Mon Jul 09 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.7.2-3.gitf661e1d
- autobuilt f661e1d

* Sun Jul 08 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.7.2-2.git0660108
- autobuilt 0660108

* Sat Jul 07 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.7.2-1.gitca6ffbc
- bump to 0.7.2
- autobuilt ca6ffbc

* Fri Jul 06 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.7.1-6.git99959e5
- autobuilt 99959e5

* Thu Jul 05 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.7.1-5.gitf2462ca
- autobuilt f2462ca

* Wed Jul 04 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.7.1-4.git6d8fac8
- autobuilt 6d8fac8

* Tue Jul 03 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.7.1-3.git767b3dd
- autobuilt 767b3dd

* Mon Jul 02 2018 Miro Hrončok <mhroncok@redhat.com> - 0.7.1-2.gitb96be3a
- Rebuilt for Python 3.7

* Sat Jun 30 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.7.1-1.gitb96be3a
- bump to 0.7.1
- autobuilt b96be3a

* Fri Jun 29 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.5-6.gitd61d8a3
- autobuilt d61d8a3

* Thu Jun 28 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.5-5.gitfd12c89
- autobuilt fd12c89

* Wed Jun 27 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.5-4.git56133f7
- autobuilt 56133f7

* Tue Jun 26 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.5-3.git208b9a6
- autobuilt 208b9a6

* Mon Jun 25 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.5-2.gite89bbd6
- autobuilt e89bbd6

* Sat Jun 23 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.5-1.git7182339
- bump to 0.6.5
- autobuilt 7182339

* Fri Jun 22 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.4-7.git4bd0f22
- autobuilt 4bd0f22

* Thu Jun 21 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.4-6.git6804fde
- autobuilt 6804fde

* Wed Jun 20 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.4-5.gitf228cf7
- autobuilt f228cf7

* Tue Jun 19 2018 Miro Hrončok <mhroncok@redhat.com> - 0.6.4-4.git5645789
- Rebuilt for Python 3.7

* Tue Jun 19 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.4-3.git5645789
- autobuilt 5645789

* Mon Jun 18 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.4-2.git9e13457
- autobuilt 9e13457

* Sat Jun 16 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.4-1.gitb43677c
- bump to 0.6.4
- autobuilt b43677c

* Fri Jun 15 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.3-6.git6bdf023
- autobuilt 6bdf023

* Thu Jun 14 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.3-5.git65033b5
- autobuilt 65033b5

* Wed Jun 13 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.3-4.git95ea3d4
- autobuilt 95ea3d4

* Tue Jun 12 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.3-3.gitab72130
- autobuilt ab72130

* Mon Jun 11 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.3-2.git1e9e530
- autobuilt 1e9e530

* Sat Jun 09 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.3-1.gitb78e7e4
- bump to 0.6.3
- autobuilt b78e7e4

* Fri Jun 08 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.2-7.git1cbce85
- autobuilt 1cbce85

* Thu Jun 07 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.2-6.gitb1ebad9
- autobuilt b1ebad9

* Wed Jun 06 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.2-5.git7b2b2bc
- autobuilt 7b2b2bc

* Tue Jun 05 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.2-4.git14cf6d2
- autobuilt 14cf6d2

* Mon Jun 04 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.2-3.gitcae49fc
- autobuilt cae49fc

* Sun Jun 03 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.2-2.git13f7450
- autobuilt 13f7450

* Sat Jun 02 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.2-1.git22e6f11
- bump to 0.6.2
- autobuilt 22e6f11

* Fri Jun 01 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.1-4.gita9e9fd4
- autobuilt a9e9fd4

* Thu May 31 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.1-3.gita127b4f
- autobuilt a127b4f

* Wed May 30 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.1-2.git8ee0f2b
- autobuilt 8ee0f2b

* Sat May 26 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.6.1-1.git44d1c1c
- bump to 0.6.1
- autobuilt 44d1c1c

* Fri May 18 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 0.5.3-7.gitc54b423
- make python3-podman the same version as the main package
- build python3-podman only for fedora >= 28

* Fri May 18 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.5.3-6.gitc54b423
- autobuilt c54b423

* Wed May 16 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 0.5.3-5.git624660c
- built commit 624660c
- New subapackage: python3-podman

* Wed May 16 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.5.3-4.git9fcc475
- autobuilt 9fcc475

* Wed May 16 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.5.3-3.git0613844
- autobuilt 0613844

* Tue May 15 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.5.3-2.git45838b9
- autobuilt 45838b9

* Fri May 11 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 0.5.3-1.git07253fc
- bump to v0.5.3
- built commit 07253fc

* Fri May 11 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.5.2-5.gitcc1bad8
- autobuilt cc1bad8

* Wed May 09 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.5.2-4.git2526355
- autobuilt 2526355

* Tue May 08 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.5.2-3.gitfaa8c3e
- autobuilt faa8c3e

* Sun May 06 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.5.2-2.gitfa4705c
- autobuilt fa4705c

* Sat May 05 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.5.2-1.gitbb0e754
- bump to 0.5.2
- autobuilt bb0e754

* Fri May 04 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.5.1-5.git5ae940a
- autobuilt 5ae940a

* Wed May 02 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.5.1-4.git64dc803
- autobuilt commit 64dc803

* Wed May 02 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.5.1-3.git970eaf0
- autobuilt commit 970eaf0

* Tue May 01 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.5.1-2.git7a0a855
- autobuilt commit 7a0a855

* Sun Apr 29 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 0.5.1-1.giteda0fd7
- reflect version number correctly
- my builder script error ended up picking the wrong version number previously

* Sun Apr 29 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.4.2-5.giteda0fd7
- autobuilt commit eda0fd7

* Sat Apr 28 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.4.2-4.git6774425
- autobuilt commit 6774425

* Fri Apr 27 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.4.2-3.git39a7a77
- autobuilt commit 39a7a77

* Thu Apr 26 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.4.2-2.git58cb8f7
- autobuilt commit 58cb8f7

* Wed Apr 25 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.4.2-1.gitbef93de
- bump to 0.4.2
- autobuilt commit bef93de

* Tue Apr 24 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 0.4.4-1.git398133e
- use correct version number

* Tue Apr 24 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.4.2-22.git398133e
- autobuilt commit 398133e

* Sun Apr 22 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.4.2-21.gitcf1d884
- autobuilt commit cf1d884

* Fri Apr 20 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.4.2-20.git9b457e3
- autobuilt commit 9b457e3

* Fri Apr 20 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.4.2-1.gitbef93de9.git228732d
- autobuilt commit 228732d

* Thu Apr 19 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.4.2-1.gitbef93de8.gitf2658ec
- autobuilt commit f2658ec

* Thu Apr 19 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.4.2-1.gitbef93de7.git6a9dbf3
- autobuilt commit 6a9dbf3

* Tue Apr 17 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.4.2-1.gitbef93de6.git96d1162
- autobuilt commit 96d1162

* Tue Apr 17 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.4.2-1.gitbef93de5.git96d1162
- autobuilt commit 96d1162

* Mon Apr 16 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.4.2-1.gitbef93de4.git6c5ebb0
- autobuilt commit 6c5ebb0

* Mon Apr 16 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.4.2-1.gitbef93de3.gitfa8442e
- autobuilt commit fa8442e

* Mon Apr 16 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.4.2-1.gitbef93de2.gitfa8442e
- autobuilt commit fa8442e

* Sun Apr 15 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.4.2-1.gitbef93de1.gitfa8442e
- autobuilt commit fa8442e

* Sat Apr 14 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.4.2-1.gitbef93de0.git62b59df
- autobuilt commit 62b59df

* Fri Apr 13 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.4.2-9.git191da31
- autobuilt commit 191da31

* Thu Apr 12 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.4.2-8.git6f51a5b
- autobuilt commit 6f51a5b

* Wed Apr 11 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.4.2-7.git77a1665
- autobuilt commit 77a1665

* Tue Apr 10 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.4.2-6.git864b9c0
- autobuilt commit 864b9c0

* Tue Apr 10 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.4.2-5.git864b9c0
- autobuilt commit 864b9c0

* Tue Apr 10 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.4.2-4.git998fd2e
- autobuilt commit 998fd2e

* Sun Apr 08 2018 Lokesh Mandvekar (Bot) <lsm5+bot@fedoraproject.org> - 0.4.2-3.git998fd2e
- autobuilt commit 998fd2e

* Sun Apr 08 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 0.4.2-2.git998fd2e
- autobuilt commit 998fd2e

* Sun Apr 08 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 0.4.2-1.gitbef93de.git998fd2e
- bump to 0.4.2
- autobuilt commit 998fd2e

* Thu Mar 29 2018 baude <bbaude@redhat.com> - 0.3.5-2.gitdb6bf9e3
- Upstream release 0.3.5

* Tue Mar 27 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 0.3.5-1.git304bf53
- built commit 304bf53

* Fri Mar 23 2018 baude <bbaude@redhat.com> - 0.3.4-1.git57b403e
- Upstream release 0.3.4

* Fri Mar 16 2018 baude <bbaude@redhat.com> - 0.3.3-2.dev.gitbc358eb
- Upstream release 0.3.3

* Wed Mar 14 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 0.3.3-1.dev.gitbc358eb
- built podman commit bc358eb
- built conmon from cri-o commit 712f3b8

* Fri Mar 09 2018 baude <bbaude@redhat.com> - 0.3.2-1.gitf79a39a
- Release 0.3.2-1

* Sun Mar 04 2018 baude <bbaude@redhat.com> - 0.3.1-2.git98b95ff
- Correct RPM version

* Fri Mar 02 2018 baude <bbaude@redhat.com> - 0.3.1-1-gitc187538
- Release 0.3.1-1

* Sun Feb 25 2018 Peter Robinson <pbrobinson@fedoraproject.org> 0.2.2-2.git525e3b1
- Build on ARMv7 too (Fedora supports containers on that arch too)

* Fri Feb 23 2018 baude <bbaude@redhat.com> - 0.2.2-1.git525e3b1
- Release 0.2.2

* Fri Feb 16 2018 baude <bbaude@redhat.com> - 0.2.1-1.git3d0100b
- Release 0.2.1

* Wed Feb 14 2018 baude <bbaude@redhat.com> - 0.2-3.git3d0100b
- Add dep for atomic-registries

* Tue Feb 13 2018 baude <bbaude@redhat.com> - 0.2-2.git3d0100b
- Add more 64bit arches
- Add containernetworking-cni dependancy
- Add iptables dependancy

* Mon Feb 12 2018 baude <bbaude@redhat.com> - 0-2.1.git3d0100
- Release 0.2

* Tue Feb 06 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 0-0.3.git367213a
- Resolves: #1541554 - first official build
- built commit 367213a

* Fri Feb 02 2018 Lokesh Mandvekar <lsm5@fedoraproject.org> - 0-0.2.git0387f69
- built commit 0387f69

* Wed Jan 10 2018 Frantisek Kluknavsky <fkluknav@redhat.com> - 0-0.1.gitc1b2278
- First package for Fedora
