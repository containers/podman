#!/bin/bash
set -x

pkg_manager=`command -v dnf`
if [ -z "$pkg_manager" ]; then
    pkg_manager=`command -v yum`
fi

echo "Package manager binary: $pkg_manager"


if [ $pkg_manager == "/usr/bin/yum" ]; then
    echo "[virt7-container-common-candidate]
name=virt7-container-common-candidate
baseurl=https://cbs.centos.org/repos/virt7-container-common-candidate/x86_64/os/
enabled=1
gpgcheck=0" > /etc/yum.repos.d/container_virt.repo
fi

declare -a PKGS=(device-mapper-devel \
                git \
                glib2-devel \
                glibc-static \
                golang \
                golang-github-cpuguy83-go-md2man \
                gpgme-devel \
                libassuan-devel \
                libseccomp-devel \
                libselinux-devel \
                make \
                rpm-build \
                go-compilers-golang-compiler \
                )

if [ $pkg_manager == "/usr/bin/dnf" ]; then
    PKGS+=(python3-devel \
        python3-varlink \
        )
# btrfs-progs-devel is not available in CentOS/RHEL-8
    if ! grep -i -q 'Red Hat\|CentOS' /etc/redhat-release; then
        PKGS+=(btrfs-progs-devel)
    fi


fi

echo ${PKGS[*]}
$pkg_manager install -y ${PKGS[*]}

make -f .copr/Makefile
rpmbuild --rebuild podman-*.src.rpm

# Test to make sure the install of the binary works
$pkg_manager -y install ~/rpmbuild/RPMS/x86_64/podman-*.x86_64.rpm


# If we built python/varlink packages, we should test their installs too
if [ $pkg_manager == "/usr/bin/dnf" ]; then
    $pkg_manager -y install ~/rpmbuild/RPMS/noarch/python*
fi
