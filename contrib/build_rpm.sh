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
                gpgme-devel \
                libassuan-devel \
                libseccomp-devel \
                libselinux-devel \
                make \
                rpm-build \
                go-compilers-golang-compiler \
                )

if [[ $pkg_manager == *dnf ]]; then
    # We need to enable PowerTools if we want to get
    # install all the pkgs we define in PKGS
    # PowerTools exists on centos-8 but not on fedora-30 and rhel-8
    if (dnf -v -C repolist all|grep "Repo-id      : PowerTools" >/dev/null); then
        sudo dnf config-manager --set-enabled PowerTools
    fi

    PKGS+=(python3-devel \
        python3-varlink \
        )
fi

# Package name on fedora 30 is golang-github-cpuguy83-go-md2man
if (grep -i 'Fedora' /etc/redhat-release | grep " 30" ) ; then
    PKGS+=(golang-github-cpuguy83-go-md2man \
        btrfs-progs-devel \
	)
fi

# disable doc until go-md2man rpm becomes available
# disable debug to avoid error: Empty %files file ~/rpmbuild/BUILD/libpod-.../debugsourcefiles.list
export extra_arg="--without doc --without debug"

echo ${PKGS[*]}
$pkg_manager install -y ${PKGS[*]}

make -f .copr/Makefile
if [ -d ~/rpmbuild/BUILD ]; then
    chmod -R +w ~/rpmbuild/BUILD
fi
rpmbuild --rebuild ${extra_arg:-} podman-*.src.rpm

# Test to make sure the install of the binary works
$pkg_manager -y install ~/rpmbuild/RPMS/x86_64/podman-*.x86_64.rpm


# If we built python/varlink packages, we should test their installs too
if [ $pkg_manager == "/usr/bin/dnf" ]; then
    $pkg_manager -y install ~/rpmbuild/RPMS/noarch/python*
fi
