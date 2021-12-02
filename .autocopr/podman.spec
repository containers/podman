%global with_debug 0

%if 0%{?with_debug}
%global _find_debuginfo_dwz_opts %{nil}
%global _dwz_low_mem_die_limit 0
%else
%global debug_package %{nil}
%endif

%global provider github
%global provider_tld com
%global project containers
%global repo %{name}
# https://github.com/containers/%%{name}
%global import_path %{provider}.%{provider_tld}/%{project}/%{repo}
%global git0 https://%{import_path}

Name: podman
Epoch: 100
Version: 4
%define build_datestamp %{lua: print(os.date("%Y%m%d"))}
%define build_timestamp %{lua: print(os.date("%H%M%S"))}
Release: %{build_datestamp}.%{build_timestamp}
Summary: Manage Pods, Containers and Container Images
License: ASL 2.0
URL: https://%{name}.io/
Source0: %{git0}/archive/main.tar.gz
Provides: %{name}-manpages = %{epoch}:%{version}-%{release}
%if 0%{?fedora} && ! 0%{?rhel}
BuildRequires: btrfs-progs-devel
%endif
BuildRequires: gcc
BuildRequires: golang >= 1.16.6
BuildRequires: glib2-devel
BuildRequires: glibc-devel
BuildRequires: glibc-static
BuildRequires: git-core
BuildRequires: golang-github-cpuguy83-md2man
BuildRequires: go-rpm-macros
BuildRequires: gpgme-devel
BuildRequires: libassuan-devel
BuildRequires: libgpg-error-devel
BuildRequires: libseccomp-devel
BuildRequires: libselinux-devel
%if 0%{?fedora} >= 35
BuildRequires: shadow-utils-subid-devel
%endif
BuildRequires: pkgconfig
BuildRequires: make
BuildRequires: ostree-devel
BuildRequires: systemd
BuildRequires: systemd-devel
Requires: conmon >= 2:2.0.30-2
Requires: containers-common >= 4:1-30
Requires: containernetworking-plugins >= 1.0.0-15.1
Requires: iptables
Requires: nftables
Recommends: %{name}-plugins
Recommends: catatonit
Suggests: qemu-user-static

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

%prep
%autosetup -n %{name}-main

%build
make all docker-docs

%install
PODMAN_VERSION=%{version} %{__make} DESTDIR=%{buildroot} PREFIX=%{_prefix} ETCDIR=%{buildroot}%{_sysconfdir} \
        install.bin-nobuild \
        install.man-nobuild \
        install.systemd \
        install.completions \
        install.docker \
        install.docker-docs-nobuild \
        install.remote-nobuild \

mv pkg/hooks/README.md pkg/hooks/README-hooks.md


# do not include docker and podman-remote man pages in main package
for file in `find %{buildroot}%{_mandir}/man[15] -type f | sed "s,%{buildroot},," | grep -v -e remote -e docker`; do
    echo "$file*" >> podman.file-list
done

# install tests
install -d -p %{buildroot}/%{_datadir}/%{name}/test/system
cp -pav test/system %{buildroot}/%{_datadir}/%{name}/test/

%check

#define license tag if not already defined
%{!?_licensedir:%global license %doc}

%files -f %{name}.file-list
%license LICENSE
%doc README.md CONTRIBUTING.md pkg/hooks/README-hooks.md install.md transfer.md
%{_bindir}/%{name}
%dir %{_libexecdir}/%{name}
%{_libexecdir}/%{name}/rootlessport
%{_datadir}/bash-completion/completions/%{name}
# By "owning" the site-functions dir, we don't need to Require zsh
%dir %{_datadir}/zsh/site-functions
%{_datadir}/zsh/site-functions/_%{name}
%dir %{_datadir}/fish/vendor_completions.d
%{_datadir}/fish/vendor_completions.d/%{name}.fish
%{_unitdir}/%{name}-auto-update.service
%{_unitdir}/%{name}-auto-update.timer
%{_unitdir}/%{name}.service
%{_unitdir}/%{name}.socket
%{_unitdir}/%{name}-restart.service
%{_userunitdir}/%{name}-auto-update.service
%{_userunitdir}/%{name}-auto-update.timer
%{_userunitdir}/%{name}.service
%{_userunitdir}/%{name}.socket
%{_userunitdir}/%{name}-restart.service
%{_usr}/lib/tmpfiles.d/%{name}.conf

%files docker
%{_bindir}/docker
%{_mandir}/man1/docker*.1*
%{_mandir}/man5/docker*.5*
%{_usr}/lib/tmpfiles.d/%{name}-docker.conf

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

%triggerpostun -- %{name} <= 3.2
rm -f %{_sharedstatedir}/containers/storage/libpod/defaultCNINetExists
exit 0

%changelog
%autochangelog
