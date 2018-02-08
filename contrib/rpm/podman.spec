# Generate devel rpm
%global with_devel 1
# Build with debug info rpm
%global with_debug 0
# Run tests in check section
%global with_check 0
# Generate unit-test rpm
%global with_unit_test 1

%if 0%{?with_debug}
%global _dwz_low_mem_die_limit 0
%else
%global debug_package   %{nil}
%endif

# %if ! 0% {?gobuild:1}
%define gobuild(o:) go build -tags="$BUILDTAGS selinux seccomp" -ldflags "${LDFLAGS:-} -B 0x$(head -c20 /dev/urandom|od -An -tx1|tr -d ' \\n')" -a -v -x %{?**};
#% endif

%global provider        github
%global provider_tld    com
%global project         projectatomic
%global repo            libpod
# https://github.com/projectatomic/libpod
%global provider_prefix %{provider}.%{provider_tld}/%{project}/%{repo}
%global import_path     %{provider_prefix}
%global commit         REPLACEWITHCOMMITID
%global shortcommit     %(c=%{commit}; echo ${c:0:7})

Name:           podman
Version:        0.1
Release:        1.git%{shortcommit}%{?dist}
Summary:        Podman allows you to manage Pods, Containers, and Container Images.
License:        ASL 2.0
URL:            https://%{provider_prefix}
Source0:        https://%{provider_prefix}/archive/%{commit}/%{repo}-%{shortcommit}.tar.gz

# e.g. el6 has ppc64 arch without gcc-go, so EA tag is required
ExclusiveArch:  %{?go_arches:%{go_arches}}%{!?go_arches:%{ix86} x86_64 aarch64 %{arm}}
# If go_compiler is not set to 1, there is no virtual provide. Use golang instead.
BuildRequires:  %{?go_compiler:compiler(go-compiler)}%{!?go_compiler:golang}


#dependencies written by hand, not by gofed:
BuildRequires:  btrfs-progs-devel, device-mapper-devel, glib2-devel, glibc-devel
BuildRequires:  glibc-static, golang-github-cpuguy83-go-md2man, gpgme-devel, libassuan-devel
BuildRequires:  libgpg-error-devel, libseccomp-devel, libselinux-devel, ostree-devel
BuildRequires:  pkgconfig, runc, skopeo-containers
BuildRequires:  git
Requires:       runc, skopeo-containers, conmon
Requires:	containernetworking-cni >= 0.6.0

%description
%{summary}
podman provides a simple management tool for container pods, containers and
images.

%if 0%{?with_devel}
%package -n libpod-devel
Summary:       libpod provides a library for applications looking to use the Container Pod concept popularized by Kubernetes
BuildArch:     noarch

Provides:      golang(%{import_path}/cmd/podman/docker) = %{version}-%{release}
Provides:      golang(%{import_path}/cmd/podman/formats) = %{version}-%{release}
Provides:      golang(%{import_path}/libpod) = %{version}-%{release}
Provides:      golang(%{import_path}/libpod/common) = %{version}-%{release}
Provides:      golang(%{import_path}/libpod/driver) = %{version}-%{release}
Provides:      golang(%{import_path}/libpod/layers) = %{version}-%{release}
Provides:      golang(%{import_path}/pkg/annotations) = %{version}-%{release}
Provides:      golang(%{import_path}/pkg/chrootuser) = %{version}-%{release}
Provides:      golang(%{import_path}/pkg/registrar) = %{version}-%{release}
Provides:      golang(%{import_path}/pkg/storage) = %{version}-%{release}
Provides:      golang(%{import_path}/utils) = %{version}-%{release}

%description -n  libpod-devel
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
Requires:        %{name}-devel = %{version}-%{release}

%description unit-test-devel
%{summary}
podman unit test development package

This package contains unit tests for project
providing packages with %{import_path} prefix.
%endif

%prep
%autosetup -Sgit -n %{repo}-%{commit}

%build
mkdir _build
pushd _build
mkdir -p src/%{provider}.%{provider_tld}/%{project}
ln -s ../../../../ src/%{import_path}
popd
ln -s vendor src
export GOPATH=$(pwd)/_build:$(pwd):$(pwd):%{gopath}
export BUILDTAGS="selinux seccomp $(hack/btrfs_installed_tag.sh) $(hack/btrfs_tag.sh) $(hack/libdm_tag.sh) containers_image_ostree_stub"

GOPATH=$GOPATH BUILDTAGS=$BUILDTAGS %gobuild -o bin/%{name} %{import_path}/cmd/%{name}

BUILDTAGS=$BUILDTAGS make all

%install
PREFIX=%{buildroot}/usr
%make_install PREFIX=${PREFIX}
%make_install PREFIX=${PREFIX} install.completions
%make_install PREFIX=${PREFIX} install.cni
%make_install PREFIX=${PREFIX} install.man

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
export GOPATH=%{buildroot}/%{gopath}:$(pwd)/vendor:%{gopath}
%endif

%if ! 0%{?gotest:1}
%global gotest go test
%endif

%gotest %{import_path}/cmd/podman
%gotest %{import_path}/libpod
%gotest %{import_path}/pkg/registrar

#define license tag if not already defined
%{!?_licensedir:%global license %doc}

%files
%license LICENSE
%doc README.md CONTRIBUTING.md hooks.md install.md code-of-conduct.md transfer.md
%{_bindir}/podman
%{_mandir}/man1/*.1*
%{_datadir}/bash-completion/completions/*
%{_sysconfdir}/cni/net.d/87-podman-bridge.conflist

%if 0%{?with_devel}
%files -n libpod-devel -f devel.file-list
%license LICENSE
%dir %{gopath}/src/%{provider}.%{provider_tld}/%{project}
%endif

%if 0%{?with_unit_test} && 0%{?with_devel}
%files unit-test-devel -f unit-test-devel.file-list
%license LICENSE
%endif

%changelog
* Wed Jan 10 2018 Frantisek Kluknavsky <fkluknav@redhat.com> - 0-0.1.gitc1b2278i
- First package for Fedora
