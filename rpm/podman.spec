%global with_debug 1

%if 0%{?with_debug}
%global _find_debuginfo_dwz_opts %{nil}
%global _dwz_low_mem_die_limit 0
%else
%global debug_package %{nil}
%endif

# RHEL's default %%gobuild macro doesn't account for the BUILDTAGS variable, so we
# set it separately here and do not depend on RHEL's go-[s]rpm-macros package
# until that's fixed.
# c9s bz: https://bugzilla.redhat.com/show_bug.cgi?id=2227328
# c8s bz: https://bugzilla.redhat.com/show_bug.cgi?id=2227331
%if %{defined rhel} && !%{defined eln}
%define gobuild(o:) go build -buildmode pie -compiler gc -tags="rpm_crashtraceback libtrust_openssl ${BUILDTAGS:-}" -ldflags "-linkmode=external -compressdwarf=false ${LDFLAGS:-} -B 0x$(head -c20 /dev/urandom|od -An -tx1|tr -d ' \\n') -extldflags '%__global_ldflags'" -a -v -x %{?**};
# python3 dep conditional for rhel8
%if %{?rhel} == 8
%define rhel8py3 1
%endif
%endif

%global gomodulesmode GO111MODULE=on

%if %{defined rhel}
# _user_tmpfiles.d currently undefined on rhel
%global _user_tmpfilesdir %{_datadir}/user-tmpfiles.d
%endif

%if %{defined fedora}
%define build_with_btrfs 1
%endif

%if %{defined copr_username}
%define copr_build 1
%endif

%global container_base_path github.com/containers
%global container_base_url https://%{container_base_path}

# For LDFLAGS
%global ld_project %{container_base_path}/%{name}/v4
%global ld_libpod %{ld_project}/libpod

# %%{name}
%global git0 %{container_base_url}/%{name}

# dnsname
%global repo_plugins dnsname
%global git_plugins %{container_base_url}/%{repo_plugins}
%global commit_plugins 18822f9a4fb35d1349eb256f4cd2bfd372474d84
%global import_path_plugins %{container_base_path}/%{repo_plugins}

Name: podman
%if %{defined copr_build}
Epoch: 102
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
# The `AND` needs to be uppercase in the License for SPDX compatibility
License: Apache-2.0 AND BSD-2-Clause AND BSD-3-Clause AND ISC AND MIT AND MPL-2.0
Release: %autorelease
%if %{defined golang_arches_future}
ExclusiveArch: %{golang_arches_future}
%else
ExclusiveArch: aarch64 ppc64le s390x x86_64
%endif
Summary: Manage Pods, Containers and Container Images
URL: https://%{name}.io/
# All SourceN files fetched from upstream
Source0: %{git0}/archive/v%{version}.tar.gz
Source1: %{git_plugins}/archive/%{commit_plugins}/%{repo_plugins}-%{commit_plugins}.tar.gz
Provides: %{name}-manpages = %{epoch}:%{version}-%{release}
BuildRequires: %{_bindir}/envsubst
%if %{defined build_with_btrfs}
BuildRequires: btrfs-progs-devel
%endif
BuildRequires: gcc
BuildRequires: glib2-devel
BuildRequires: glibc-devel
BuildRequires: glibc-static
BuildRequires: golang
BuildRequires: git-core
%if !%{defined gobuild}
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
BuildRequires: man-db
BuildRequires: ostree-devel
BuildRequires: systemd
BuildRequires: systemd-devel
%if %{defined rhel8py3}
BuildRequires: python3
%endif
Requires: catatonit
Requires: conmon >= 2:2.1.7-2
Requires: containers-common-extra
%if %{defined rhel} && !%{defined eln}
Recommends: gvisor-tap-vsock-gvforwarder
%else
Requires: gvisor-tap-vsock-gvforwarder
%endif
Recommends: gvisor-tap-vsock
Provides: %{name}-quadlet
Obsoletes: %{name}-quadlet <= 5:4.4.0-1
Provides: %{name}-quadlet = %{epoch}:%{version}-%{release}

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
Recommends: gvisor-tap-vsock

%description plugins
This plugin sets up the use of dnsmasq on a given CNI network so
that Pods can resolve each other by name.  When configured,
the pod and its IP address are added to a network specific hosts file
that dnsmasq will read in.  Similarly, when a pod
is removed from the network, it will remove the entry from the hosts
file.  Each CNI network will have its own dnsmasq instance.

%package -n %{name}sh
Summary: Confined login and user shell using %{name}
Requires: %{name} = %{epoch}:%{version}-%{release}
Provides: %{name}-shell = %{epoch}:%{version}-%{release}
Provides: %{name}-%{name}sh = %{epoch}:%{version}-%{release}

%description -n %{name}sh
%{name}sh provides a confined login and user shell with access to volumes and
capabilities specified in user quadlets.

It is a symlink to %{_bindir}/%{name} and execs into the `%{name}sh` container
when `%{_bindir}/%{name}sh` is set as a login shell or set as os.Args[0].

%prep
%autosetup -Sgit -n %{name}-%{version}
sed -i 's;@@PODMAN@@\;$(BINDIR);@@PODMAN@@\;%{_bindir};' Makefile

# These changes are only meant for copr builds
%if %{defined copr_build}
# podman --version should show short sha
sed -i "s/^const RawVersion = .*/const RawVersion = \"##VERSION##-##SHORT_SHA##\"/" version/rawversion/version.go
# use ParseTolerant to allow short sha in version
sed -i "s/^var Version.*/var Version, err = semver.ParseTolerant(rawversion.RawVersion)/" version/version.go
%endif

# untar dnsname
tar zxf %{SOURCE1}

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

LDFLAGS="-X %{ld_libpod}/define.buildInfo=$(date +%s) \
         -X %{ld_libpod}/config._installPrefix=%{_prefix} \
         -X %{ld_libpod}/config._etcDir=%{_sysconfdir} \
         -X %{ld_project}/pkg/systemd/quadlet._binDir=%{_bindir}"

# build rootlessport first
%gobuild -o bin/rootlessport ./cmd/rootlessport

export BASEBUILDTAGS="seccomp exclude_graphdriver_devicemapper $(hack/systemd_tag.sh) $(hack/libsubid_tag.sh)"

# build %%{name}
export BUILDTAGS="$BASEBUILDTAGS $(hack/btrfs_installed_tag.sh) $(hack/btrfs_tag.sh) $(hack/libdm_tag.sh)"
%gobuild -o bin/%{name} ./cmd/%{name}

# build %%{name}-remote
export BUILDTAGS="$BASEBUILDTAGS exclude_graphdriver_btrfs btrfs_noversion remote"
%gobuild -o bin/%{name}-remote ./cmd/%{name}

# build quadlet
export BUILDTAGS="$BASEBUILDTAGS $(hack/btrfs_installed_tag.sh) $(hack/btrfs_tag.sh)"
%gobuild -o bin/quadlet ./cmd/quadlet

# reset LDFLAGS for plugins binaries
LDFLAGS=''

%{__make} docs docker-docs

# build dnsname the old way otherwise it fails on koji
cd %{repo_plugins}-%{commit_plugins}
mkdir _build
cd _build
mkdir -p src/%{container_base_path}
ln -s ../../../../ src/%{import_path_plugins}
cd ..
ln -s vendor src
export GOPATH=$(pwd)/_build:$(pwd)
%define gomodulesmode GO111MODULE=off
%gobuild -o bin/dnsname %{import_path_plugins}/plugins/meta/dnsname
cd ..

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
%if %{defined _modulesloaddir}
        install.modules-load
%endif

sed -i 's;%{buildroot};;g' %{buildroot}%{_bindir}/docker

# install dnsname plugin
cd %{repo_plugins}-%{commit_plugins}
%{__make} PREFIX=%{_prefix} DESTDIR=%{buildroot} install
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
%if %{defined _modulesloaddir}
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
%{_datadir}/%{name}/test

%files plugins
%license %{repo_plugins}-%{commit_plugins}/LICENSE
%doc %{repo_plugins}-%{commit_plugins}/{README.md,README_PODMAN.md}
%dir %{_libexecdir}/cni
%{_libexecdir}/cni/dnsname

%files -n %{name}sh
%{_bindir}/%{name}sh

%changelog
%if %{defined autochangelog}
%autochangelog
%else
* Mon May 01 2023 RH Container Bot <rhcontainerbot@fedoraproject.org>
- Placeholder changelog for envs that are not autochangelog-ready
%endif
