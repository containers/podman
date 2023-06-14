%global with_debug 1

%if 0%{?with_debug}
%global _find_debuginfo_dwz_opts %{nil}
%global _dwz_low_mem_die_limit 0
%else
%global debug_package %{nil}
%endif

# RHEL 8's default %%gobuild macro doesn't account for the BUILDTAGS variable, so we
# set it separately here and do not depend on RHEL 8's go-srpm-macros package.
%if !0%{?fedora} && 0%{?rhel} <= 8
%define gobuild(o:) go build -buildmode pie -compiler gc -tags="rpm_crashtraceback libtrust_openssl ${BUILDTAGS:-}" -ldflags "-linkmode=external -compressdwarf=false ${LDFLAGS:-} -B 0x$(head -c20 /dev/urandom|od -An -tx1|tr -d ' \\n') -extldflags '%__global_ldflags'" -a -v -x %{?**};
%endif

%if 0%{?rhel}
%bcond_with btrfs
# _user_tmpfiles.d currently undefined on rhel
%global _user_tmpfilesdir %{_datadir}/user-tmpfiles.d
%else
%bcond_without btrfs
%endif

# RHEL 8 needs /usr/bin/python3 to build docs
%if 0%{?rhel} == 8
%bcond_without python3
%else
%bcond_with python3
%endif

%if 0%{?fedora} || 0%{?rhel} >= 10
%bcond_without modules_load
%else
%bcond_with modules_load
%endif

%if 0%{?fedora} || 0%{?rhel} >= 9
%bcond_without go_rpm_macros
%else
%bcond_with go_rpm_macros
%endif

# copr_username is only set on copr environments, not on others like koji
%if "%{?copr_username}" != "rhcontainerbot"
%bcond_with copr
%else
%bcond_without copr
%endif

%if 0%{?centos} <= 8
%bcond_without changelog
%else
%bcond_with changelog
%endif

%if 0%{?fedora}
%bcond_without golang_arches_future
%else
%bcond_with golang_arches_future
%endif

%global provider github
%global provider_tld com
%global project containers
%global repo %{name}
# https://github.com/containers/%%{name}
%global import_path %{provider}.%{provider_tld}/%{project}/%{repo}
%global git0 https://%{import_path}

# dnsname
%global repo_plugins dnsname
# https://github.com/containers/dnsname
%global import_path_plugins %{provider}.%{provider_tld}/%{project}/%{repo_plugins}
%global git_plugins https://%{import_path_plugins}
%global commit_plugins 18822f9a4fb35d1349eb256f4cd2bfd372474d84

# gvproxy
%global repo_gvproxy gvisor-tap-vsock
# https://github.com/containers/gvisor-tap-vsock
%global import_path_gvproxy %{provider}.%{provider_tld}/%{project}/%{repo_gvproxy}
%global git_gvproxy https://%{import_path_gvproxy}
%global commit_gvproxy 407efb5dcdb0f4445935f7360535800b60447544

# podman
%global git0 https://github.com/containers/%{name}

Name: podman
%if %{with copr}
Epoch: 101
%else
Epoch: 5
%endif
# DO NOT TOUCH the Version string!
# The TRUE source of this specfile is:
# https://github.com/containers/podman/blob/main/rpm/podman.spec
# If that's what you're reading, Version must be 0, and will be updated by Packit for
# copr and koji builds.
# If you're reading this on dist-git, the version is automatically filled in by Packit.
Version: 0
License: Apache-2.0 and BSD-2-Clause and BSD-3-Clause and ISC and MIT and MPL-2.0
Release: %autorelease
%if %{with golang_arches_future}
ExclusiveArch: %{golang_arches_future}
%else
ExclusiveArch: aarch64 ppc64le s390x x86_64
%endif
Summary: Manage Pods, Containers and Container Images
URL: https://%{name}.io/
# All SourceN files fetched from upstream
Source0: %{git0}/archive/v%{version}.tar.gz
Source1: %{git_plugins}/archive/%{commit_plugins}/%{repo_plugins}-%{commit_plugins}.tar.gz
Source2: %{git_gvproxy}/archive/%{commit_gvproxy}/%{repo_gvproxy}-%{commit_gvproxy}.tar.gz
Provides: %{name}-manpages = %{epoch}:%{version}-%{release}
BuildRequires: %{_bindir}/envsubst
BuildRequires: go-md2man
%if %{with btrfs}
BuildRequires: btrfs-progs-devel
%endif
BuildRequires: gcc
BuildRequires: glib2-devel
BuildRequires: glibc-devel
BuildRequires: glibc-static
BuildRequires: golang
BuildRequires: git-core
%if %{with go_rpm_macros}
BuildRequires: go-rpm-macros
%endif
BuildRequires: gpgme-devel
BuildRequires: libassuan-devel
BuildRequires: libgpg-error-devel
BuildRequires: libseccomp-devel
BuildRequires: libselinux-devel
BuildRequires: shadow-utils-subid-devel
BuildRequires: pkgconfig
BuildRequires: make
BuildRequires: ostree-devel
BuildRequires: systemd
BuildRequires: systemd-devel
%if %{with python3}
BuildRequires: python3
%endif
Requires: catatonit
Requires: conmon >= 2:2.1.7-2
%if 0%{?fedora} > 38
Requires: containers-common-extra >= 4:1-90
%else
%if 0%{?fedora} == 38
Requires: containers-common-extra >= 4:1-89
%else
Requires: containers-common-extra >= 4:1-82
%endif
%endif
Recommends: %{name}-gvproxy = %{epoch}:%{version}-%{release}
Provides: %{name}-quadlet
Obsoletes: %{name}-quadlet <= 5:4.4.0-1
Provides: %{name}-quadlet = %{epoch}:%{version}-%{release}
# DO NOT DELETE BELOW LINE - used for updating downstream goimports
# vendored libraries

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
Requires: gnupg

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

%package plugins
Summary: Plugins for %{name}
Requires: dnsmasq
Recommends: %{name}-gvproxy = %{epoch}:%{version}-%{release}

%description plugins
This plugin sets up the use of dnsmasq on a given CNI network so
that Pods can resolve each other by name.  When configured,
the pod and its IP address are added to a network specific hosts file
that dnsmasq will read in.  Similarly, when a pod
is removed from the network, it will remove the entry from the hosts
file.  Each CNI network will have its own dnsmasq instance.

%package gvproxy
Summary: Go replacement for libslirp and VPNKit

%description gvproxy
A replacement for libslirp and VPNKit, written in pure Go.
It is based on the network stack of gVisor. Compared to libslirp,
gvisor-tap-vsock brings a configurable DNS server and
dynamic port forwarding.

%package -n %{name}sh
Summary: Confined login and user shell using %{name}
Requires: %{name} = %{epoch}:%{version}-%{release}
Provides: %{name}-shell = %{epoch}:%{version}-%{release}
Provides: %{name}-%{name}sh = %{epoch}:%{version}-%{release}

%description -n %{name}sh
%{name}sh provides a confined login and user shell with access to volumes and
capabilities specified in user quadlets.

It is a symlink to %{_bindir}/%{name} and execs into the `%{name}sh` container
when `%{_bindir}/%{name}sh is set as a login shell or set as os.Args[0].

%prep
%autosetup -Sgit -n %{name}-%{version}
sed -i 's;@@PODMAN@@\;$(BINDIR);@@PODMAN@@\;%{_bindir};' Makefile

# untar dnsname
tar zxf %{SOURCE1}

# untar %%{name}-gvproxy
tar zxf %{SOURCE2}

%build
%set_build_flags
export CGO_CFLAGS=$CFLAGS
# These extra flags present in $CFLAGS have been skipped for now as they break the build
CGO_CFLAGS=$(echo $CGO_CFLAGS | sed 's/-flto=auto//g')
CGO_CFLAGS=$(echo $CGO_CFLAGS | sed 's/-Wp,D_GLIBCXX_ASSERTIONS//g')
CGO_CFLAGS=$(echo $CGO_CFLAGS | sed 's/-specs=\/usr\/lib\/rpm\/redhat\/redhat-annobin-cc1//g')

%ifarch x86_64
export CGO_CFLAGS+=" -m64 -mtune=generic -fcf-protection=full"
%endif

export GO111MODULE=off
export GOPATH=$(pwd)/_build:$(pwd)

mkdir _build
cd _build
mkdir -p src/%{provider}.%{provider_tld}/%{project}
ln -s ../../../../ src/%{import_path}
cd ..
ln -s vendor src

# build date. FIXME: Makefile uses '/v2/libpod', that doesn't work here?
LDFLAGS="-X %{import_path}/libpod/define.buildInfo=$(date +%s)"

# build rootlessport first
%gobuild -o bin/rootlessport %{import_path}/cmd/rootlessport

export BASEBUILDTAGS="seccomp exclude_graphdriver_devicemapper $(hack/systemd_tag.sh) $(hack/libsubid_tag.sh)"

# build %%{name}
export BUILDTAGS="$BASEBUILDTAGS $(hack/btrfs_installed_tag.sh) $(hack/btrfs_tag.sh) $(hack/libdm_tag.sh)"
%gobuild -o bin/%{name} %{import_path}/cmd/%{name}

# build %%{name}-remote
export BUILDTAGS="$BASEBUILDTAGS exclude_graphdriver_btrfs btrfs_noversion remote"
%gobuild -o bin/%{name}-remote %{import_path}/cmd/%{name}

# build quadlet
export BUILDTAGS="$BASEBUILDTAGS $(hack/btrfs_installed_tag.sh) $(hack/btrfs_tag.sh)"
%gobuild -o bin/quadlet %{import_path}/cmd/quadlet

cd %{repo_plugins}-%{commit_plugins}
mkdir _build
cd _build
mkdir -p src/%{provider}.%{provider_tld}/%{project}
ln -s ../../../../ src/%{import_path_plugins}
cd ..
ln -s vendor src
export GOPATH=$(pwd)/_build:$(pwd)
%gobuild -o bin/dnsname %{import_path_plugins}/plugins/meta/dnsname
cd ..

cd %{repo_gvproxy}-%{commit_gvproxy}
mkdir _build
cd _build
mkdir -p src/%{provider}.%{provider_tld}/%{project}
ln -s ../../../../ src/%{import_path_gvproxy}
cd ..
ln -s vendor src
export GOPATH=$(pwd)/_build:$(pwd)
%gobuild -o bin/gvproxy %{import_path_gvproxy}/cmd/gvproxy
%gobuild -o bin/gvforwarder %{import_path_gvproxy}/cmd/vm
cd ..

%{__make} docs docker-docs

%install
install -dp %{buildroot}%{_unitdir}
PODMAN_VERSION=%{version} %{__make} PREFIX=%{buildroot}%{_prefix} ETCDIR=%{_sysconfdir} \
       install.bin \
       install.man \
       install.systemd \
       install.completions \
       install.docker \
       install.docker-docs \
       install.remote \
%if %{with modules_load}
        install.modules-load
%endif

sed -i 's;%{buildroot};;g' %{buildroot}%{_bindir}/docker

# install dnsname plugin
cd %{repo_plugins}-%{commit_plugins}
%{__make} PREFIX=%{_prefix} DESTDIR=%{buildroot} install
cd ..

# install gvproxy
cd %{repo_gvproxy}-%{commit_gvproxy}
install -dp %{buildroot}%{_libexecdir}/%{name}
install -p -m0755 bin/gvproxy %{buildroot}%{_libexecdir}/%{name}
install -p -m0755 bin/gvforwarder %{buildroot}%{_libexecdir}/%{name}
cd ..

# do not include docker and podman-remote man pages in main package
for file in `find %{buildroot}%{_mandir}/man[15] -type f | sed "s,%{buildroot},," | grep -v -e remote -e docker`; do
    echo "$file*" >> podman.file-list
done

rm -f %{buildroot}%{_mandir}/man5/docker*.5

install -d -p %{buildroot}/%{_datadir}/%{name}/test/system
cp -pav test/system %{buildroot}/%{_datadir}/%{name}/test/

#define license tag if not already defined
%{!?_licensedir:%global license %doc}

%files -f %{name}.file-list
%license LICENSE
%doc README.md CONTRIBUTING.md install.md transfer.md
%{_bindir}/%{name}
%dir %{_libexecdir}/%{name}
%{_libexecdir}/%{name}/rootlessport
%{_libexecdir}/%{name}/quadlet
%{_datadir}/bash-completion/completions/%{name}
# By "owning" the site-functions dir, we don't need to Require zsh
%dir %{_datadir}/zsh/site-functions
%{_datadir}/zsh/site-functions/_%{name}
%dir %{_datadir}/fish/vendor_completions.d
%{_datadir}/fish/vendor_completions.d/%{name}.fish
%{_unitdir}/%{name}*
%{_userunitdir}/%{name}*
%{_tmpfilesdir}/%{name}.conf
%{_systemdgeneratordir}/%{name}-system-generator
%{_systemdusergeneratordir}/%{name}-user-generator
%if %{with modules_load}
%{_modulesloaddir}/%{name}-iptables.conf
%endif

%files docker
%{_bindir}/docker
%{_mandir}/man1/docker*.1*
%{_tmpfilesdir}/%{name}-docker.conf
%{_user_tmpfilesdir}/%{name}-docker.conf

%files remote
%license LICENSE
%{_bindir}/%{name}-remote
%{_mandir}/man1/%{name}-remote*.*
%{_datadir}/bash-completion/completions/%{name}-remote
%dir %{_datadir}/fish/vendor_completions.d
%{_datadir}/fish/vendor_completions.d/%{name}-remote.fish
%dir %{_datadir}/zsh/site-functions
%{_datadir}/zsh/site-functions/_%{name}-remote

%files tests
%license LICENSE
%{_datadir}/%{name}/test

%files plugins
%license %{repo_plugins}-%{commit_plugins}/LICENSE
%doc %{repo_plugins}-%{commit_plugins}/{README.md,README_PODMAN.md}
%dir %{_libexecdir}/cni
%{_libexecdir}/cni/dnsname

%files gvproxy
%license %{repo_gvproxy}-%{commit_gvproxy}/LICENSE
%doc %{repo_gvproxy}-%{commit_gvproxy}/README.md
%dir %{_libexecdir}/%{name}
%{_libexecdir}/%{name}/gvproxy
%{_libexecdir}/%{name}/gvforwarder

%files -n %{name}sh
%license LICENSE
%doc README.md CONTRIBUTING.md install.md transfer.md
%{_bindir}/%{name}sh

%changelog
%if %{with changelog}
* Mon May 01 2023 RH Container Bot <rhcontainerbot@fedoraproject.org>
- Placeholder changelog for envs that are not autochangelog-ready
%else
%autochangelog
%endif
