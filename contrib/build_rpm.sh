#!/usr/bin/env bash
set -euxo pipefail

# returned path can vary: /usr/bin/dnf /bin/dnf ...
pkg_manager=$(command -v dnf yum | head -n1)
echo "Package manager binary: $pkg_manager"


if [[ $pkg_manager == *yum ]]; then
    echo "[virt7-container]
name=virt7-container
baseurl=https://buildlogs.centos.org/centos/7/virt/x86_64/container/
enabled=1
gpgcheck=0" > /etc/yum.repos.d/container_virt.repo
    echo "[paas_openshift_origin311]
name=paas_openshift_origin311
# Use this if you need the newest *-testing packages
# baseurl=https://buildlogs.centos.org/centos/7/paas/x86_64/openshift-origin311/
baseurl=http://mirror.centos.org/centos/7/paas/x86_64/openshift-origin311/
enabled=1
gpgcheck=0" > /etc/yum.repos.d/paas_openshift_origin311.repo
fi

declare -a PKGS=(\
                createrepo \
                device-mapper-devel \
                git \
                glib2-devel \
                glibc-static \
                go-compilers-golang-compiler \
                golang \
                gpgme-devel \
                libassuan-devel \
                libseccomp-devel \
                libselinux-devel \
                make \
                redhat-rpm-config \
                rpm-build \
                rpmdevtools \
                systemd-devel \
                )

if [[ $pkg_manager == *dnf ]]; then
    # We need to enable PowerTools if we want to get
    # install all the pkgs we define in PKGS
    # PowerTools exists on centos-8 but not on fedora-30 and rhel-8
    if (dnf -v -C repolist all|grep "Repo-id      : PowerTools" >/dev/null); then
        sudo dnf config-manager --set-enabled PowerTools
    fi

    PKGS+=(python3-devel \
        )
fi

# Package name on fedora 30 is golang-github-cpuguy83-go-md2man
if (grep -i 'Fedora' /etc/redhat-release | grep " 30" ) ; then
    PKGS+=(golang-github-cpuguy83-go-md2man \
        btrfs-progs-devel \
	)
# btrfs-progs-devel is not available in CentOS/RHEL-8
elif ! (grep -i 'Red Hat\|CentOS' /etc/redhat-release | grep " 8" ) ; then
    PKGS+=(golang-github-cpuguy83-md2man \
        btrfs-progs-devel \
        )
fi

# disable doc until go-md2man rpm becomes available
# disable debug to avoid error: Empty %files file ~/rpmbuild/BUILD/libpod-.../debugsourcefiles.list
export extra_arg="--without doc --without debug"

echo ${PKGS[*]}
sudo $pkg_manager install -y ${PKGS[*]}

# clean up src.rpm as it's been built
sudo rm -f podman-*.src.rpm

make -f .copr/Makefile
# workaround for https://github.com/containers/podman/issues/4627
if [ -d ~/rpmbuild/BUILD ]; then
    chmod -R +w ~/rpmbuild/BUILD
fi

rpmbuild --rebuild ${extra_arg:-} podman-*.src.rpm
