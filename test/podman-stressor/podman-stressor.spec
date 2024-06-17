%if "%{?copr_username}" != "rhcontainerbot"
%bcond_with copr
%else
%bcond_without copr
%endif

Name: podman-stressor
# Set different Epochs for copr and koji
%if %{with copr}
Epoch: 101
%endif
# Keep Version in upstream specfile at 0. It will be automatically set
# to the correct value by Packit for copr and koji builds.
# IGNORE this comment if you're looking at it in dist-git.
Version: 0.1.0
%if %{defined autorelease}
Release: %autorelease
%else
Release: 1
%endif
License: Apache-2.0
URL: https://github.com/dougsland/podman-stressor
Summary: A stressor tool for podman
Source0: %{url}/archive/v%{version}.tar.gz
BuildArch: noarch
Requires: podman
Requires: sudo
Requires: stress-ng

# Requirements for cgroupv2 ?
BuildRequires:  systemd
Requires:       systemd >= 239
Requires:       kernel >= 5.0

%description
A collection of scripts wrapped with cgroupv2 namespaces to stress podman
and make sure containers are not escaping it's delimitations if there
are memory, CPU or others interferences in the system.

%prep
%autosetup -Sgit -n %{name}-%{version}

%build
%{__make} all

%install
%{__make} DESTDIR=%{buildroot} DATADIR=%{_datadir} install

#define license tag if not already defined
%{!?_licensedir:%global license %doc}

%files
%license LICENSE
%doc CODE-OF-CONDUCT.md NOTICE README.md SECURITY.md LICENSE
%{_bindir}/%{name}
%dir %{_datadir}/%{name}
%{_datadir}/podman-stressor/cgroup
%{_datadir}/podman-stressor/stress
%{_datadir}/podman-stressor/constants
%{_datadir}/podman-stressor/memory
%{_datadir}/podman-stressor/podman
%{_datadir}/podman-stressor/podman-operations
%{_datadir}/podman-stressor/processes
%{_datadir}/podman-stressor/network
%{_datadir}/podman-stressor/volume
%{_datadir}/podman-stressor/systemd
%{_datadir}/podman-stressor/system
%{_datadir}/podman-stressor/date
%{_datadir}/podman-stressor/rpm
%{_datadir}/podman-stressor/selinux

%changelog
%if %{defined autochangelog}
%autochangelog
%else
* Sun May 19 2024 RH Container Bot <rhcontainerbot@fedoraproject.org>
- Placeholder changelog for envs that are not autochangelog-ready
%endif
