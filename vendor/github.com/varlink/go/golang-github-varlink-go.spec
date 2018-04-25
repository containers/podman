%global goipath github.com/varlink/go
Version:        0
%gometa

Name:           %{goname}
Release:        1%{?dist}
Summary:        Go bindings for varlink
License:        ASL 2.0
URL:            %{gourl}
Source0:        %{gosource}

%description
Native Go bindings for the varlink protocol.

%package devel
Summary:       %{summary}
BuildArch:     noarch

%description devel
%{summary}

This package contains library source intended for
building other packages which use import path with
%{gobaseipath} prefix.

%prep
%forgesetup

%build
%gobuildroot

%install
gofiles=$(find . %{gofindfilter} -print)
%goinstall $gofiles

%check

%files devel -f devel.file-list
%license LICENSE
%doc README.md

%changelog
* Tue Mar 20 2018 <info@varlink.org> 0-1
- Version 0
