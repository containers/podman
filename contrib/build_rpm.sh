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
    # We need to enable PowerTools if we want to get
    # install all the pkgs we define in PKGS
    # PowerTools exists on centos-8(-stream) but not on fedora-30 and rhel-8
    dnf -C repolist all | sed -rn 's/(.*PowerTools).*/\1/p' | \
        xargs sudo dnf config-manager --set-enabled

    PKGS+=(python3-devel \
        python3-varlink \
        )
fi

# btrfs-progs-devel is not available in CentOS/RHEL-8
if ! (grep -i 'Red Hat\|CentOS' /etc/redhat-release | grep " 8" ); then
    PKGS+=(golang-github-cpuguy83-go-md2man \
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
# workaround for https://github.com/containers/libpod/issues/4627
if [ -d ~/rpmbuild/BUILD ]; then
    chmod -R +w ~/rpmbuild/BUILD
fi

rpmbuild --rebuild ${extra_arg:-} podman-*.src.rpm
createrepo -v ~/rpmbuild/RPMS/x86_64

if [ "${1:-}" == "install" ]; then
  	sudo ${pkg_manager} -y --repofrom podman,file://${HOME}/rpmbuild/RPMS/x86_64/ install podman podman-remote
    # Remember that rpms install exec to /usr/bin/podman while a `make install`
    # installs them to /usr/local/bin/podman which is likely before. Always use
    # a full path to test installed podman or you risk to call another executable.
	/usr/bin/podman version
	# TODO(ssbarnea): implement a test that verifies the version to match what
	# we build. The problem is that Version seems to be generic, maybe use
	# of Git Commit
	sudo /usr/bin/podman info  # will catch a broken conmon
fi