#!/bin/bash
set -euxo pipefail

# returned path can vary: /usr/bin/dnf /bin/dnf ...
pkg_manager=$(command -v dnf yum | head -n1)
echo "Package manager binary: $pkg_manager"


if [[ $pkg_manager == *yum ]]; then
    echo "[virt7-container-common-candidate]
name=virt7-container-common-candidate
baseurl=https://cbs.centos.org/repos/virt7-container-common-candidate/x86_64/os/
enabled=1
gpgcheck=0" > /etc/yum.repos.d/container_virt.repo
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
    PKGS+=(python3-devel \
        python3-varlink \
        )
    # btrfs-progs-devel is not available in CentOS/RHEL-8
    if ! grep -i -q 'Red Hat\|CentOS' /etc/redhat-release; then
        PKGS+=(btrfs-progs-devel)
    fi
    # disable doc until go-md2man rpm becomes available
    # disable debug to avoid error: Empty %files file ~/rpmbuild/BUILD/libpod-.../debugsourcefiles.list
    export extra_arg="--without doc --without debug"
else
    if ! grep -i -q 'Red Hat\|CentOS' /etc/redhat-release; then
        PKGS+=(golang-github-cpuguy83-go-md2man)
    fi
fi

echo ${PKGS[*]}
sudo $pkg_manager install --disablerepo podman -y ${PKGS[*]}

make -f .copr/Makefile
# workaround for https://github.com/containers/libpod/issues/4627
if [ -d ~/rpmbuild/BUILD ]; then
    chmod -R +w ~/rpmbuild/BUILD
fi

rpmbuild --rebuild ${extra_arg:-} podman-*.src.rpm

# build repository
mkdir -p build/buildset
pushd build/buildset
cp -l ~/rpmbuild/RPMS/*/*.rpm .
createrepo .

cat <<EOF >../podman.repo
[podman]
priority=1
name=Podman Override
baseurl=file://${PWD}
enabled=1
gpgcheck=0
metadata_expire=1s
cost=0
EOF
popd
