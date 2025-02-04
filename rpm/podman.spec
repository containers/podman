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
%if %{defined rhel} && 0%{?rhel} < 10
%define gobuild(o:) go build -buildmode pie -compiler gc -tags="rpm_crashtraceback libtrust_openssl ${BUILDTAGS:-}" -ldflags "-linkmode=external -compressdwarf=false ${LDFLAGS:-} -B 0x$(head -c20 /dev/urandom|od -An -tx1|tr -d ' \\n') -extldflags '%__global_ldflags'" -a -v -x %{?**};
%endif

%global gomodulesmode GO111MODULE=on

%if %{defined rhel}
# _user_tmpfiles.d currently undefined on rhel
%global _user_tmpfilesdir %{_datadir}/user-tmpfiles.d
%endif

%if %{defined fedora}
%define build_with_btrfs 1
# qemu-system* isn't packageed for CentOS Stream / RHEL
%define qemu 1
%endif

%if %{defined copr_username}
%define copr_build 1
%endif

%global container_base_path github.com/containers
%global container_base_url https://%{container_base_path}

# For LDFLAGS
%global ld_project %{container_base_path}/%{name}/v5
%global ld_libpod %{ld_project}/libpod

# %%{name}
%global git0 %{container_base_url}/%{name}

# podman-machine subpackage will be present only on these architectures
%global machine_arches x86_64 aarch64

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
Source0: %{git0}/archive/v%{version_no_tilde}.tar.gz
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
%if %{undefined rhel} || 0%{?rhel} >= 10
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
Requires: catatonit
Requires: conmon >= 2:2.1.7-2
%if %{defined fedora} && 0%{?fedora} >= 40
# TODO: Remove the f40 conditional after a few releases to keep conditionals to
# a minimum
# Ref: https://bugzilla.redhat.com/show_bug.cgi?id=2269148
Requires: containers-common-extra >= 5:0.58.0-1
%else
Requires: containers-common-extra
%endif
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
%if %{defined fedora}
Requires: bats
%endif
Requires: attr
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

%ifarch %{machine_arches}
%package machine
Summary: Metapackage for setting up %{name} machine
Requires: %{name} = %{epoch}:%{version}-%{release}
Requires: gvisor-tap-vsock
%if %{defined qemu}
%ifarch aarch64
Requires: qemu-system-aarch64-core
%endif
%ifarch x86_64
Requires: qemu-system-x86-core
%endif
%else
Requires: qemu-kvm
%endif
Requires: qemu-img
Requires: virtiofsd
ExclusiveArch: x86_64 aarch64

%description machine
This subpackage installs the dependencies for %{name} machine, for more see:
https://docs.podman.io/en/latest/markdown/podman-machine.1.html
%endif

%prep
%autosetup -Sgit -n %{name}-%{version_no_tilde}
sed -i 's;@@PODMAN@@\;$(BINDIR);@@PODMAN@@\;%{_bindir};' Makefile

# cgroups-v1 is supported on rhel9
%if 0%{?rhel} == 9
sed -i '/DELETE ON RHEL9/,/DELETE ON RHEL9/d' libpod/runtime.go
%endif

# These changes are only meant for copr builds
%if %{defined copr_build}
# podman --version should show short sha
sed -i "s/^const RawVersion = .*/const RawVersion = \"##VERSION##-##SHORT_SHA##\"/" version/rawversion/version.go
# use ParseTolerant to allow short sha in version
sed -i "s/^var Version.*/var Version, err = semver.ParseTolerant(rawversion.RawVersion)/" version/version.go
%endif

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

export GOPROXY=direct

LDFLAGS="-X %{ld_libpod}/define.buildInfo=${SOURCE_DATE_EPOCH:-$(date +%s)} \
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

# build %%{name}-testing
export BUILDTAGS="$BASEBUILDTAGS $(hack/btrfs_installed_tag.sh) $(hack/btrfs_tag.sh)"
%gobuild -o bin/podman-testing ./cmd/podman-testing

# reset LDFLAGS for plugins binaries
LDFLAGS=''

%{__make} docs docker-docs

%install
install -dp %{buildroot}%{_unitdir}
PODMAN_VERSION=%{version} %{__make} DESTDIR=%{buildroot} PREFIX=%{_prefix} ETCDIR=%{_sysconfdir} \
       install.bin \
       install.man \
       install.systemd \
       install.completions \
       install.docker \
       install.docker-docs \
       install.remote \
       install.testing

# See above for the iptables.conf declaration
%if %{defined fedora} && 0%{?fedora} < 41
%{__make} DESTDIR=%{buildroot} MODULESLOADDIR=%{_modulesloaddir} install.modules-load
%endif

sed -i 's;%{buildroot};;g' %{buildroot}%{_bindir}/docker

# do not include docker and podman-remote man pages in main package
for file in `find %{buildroot}%{_mandir}/man[157] -type f | sed "s,%{buildroot},," | grep -v -e %{name}sh.1 -e remote -e docker`; do
    echo "$file*" >> %{name}.file-list
done

rm -f %{buildroot}%{_mandir}/man5/docker*.5

install -d -p %{buildroot}%{_datadir}/%{name}/test/system
cp -pav test/system %{buildroot}%{_datadir}/%{name}/test/

%ifarch %{machine_arches}
# symlink virtiofsd in %%{name} libexecdir for machine subpackage
ln -s ../virtiofsd %{buildroot}%{_libexecdir}/%{name}
%endif

#define license tag if not already defined
%{!?_licensedir:%global license %doc}

# Include empty check to silence rpmlint warning
%check

%files -f %{name}.file-list
%license LICENSE vendor/modules.txt
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
# iptables modules are only needed with iptables-legacy,
# as of f41 netavark will default to nftables so do not load unessary modules
# https://fedoraproject.org/wiki/Changes/NetavarkNftablesDefault
%if %{defined fedora} && 0%{?fedora} < 41
%{_modulesloaddir}/%{name}-iptables.conf
%endif

%files docker
%{_bindir}/docker
%{_mandir}/man1/docker*.1*
%{_sysconfdir}/profile.d/%{name}-docker.*
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
%{_bindir}/%{name}-testing
%{_datadir}/%{name}/test

%files -n %{name}sh
%{_bindir}/%{name}sh
%{_mandir}/man1/%{name}sh.1*

%ifarch %{machine_arches}
%files machine
%dir %{_libexecdir}/%{name}
%{_libexecdir}/%{name}/virtiofsd
%endif

%changelog
%autochangelog
