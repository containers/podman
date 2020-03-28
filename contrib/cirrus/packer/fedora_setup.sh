#!/bin/bash

# This script is called by packer on the subject fedora VM, to setup the podman
# build/test environment.  It's not intended to be used outside of this context.

set -e

# Load in library (copied by packer, before this script was run)
source /tmp/libpod/$SCRIPT_BASE/lib.sh

req_env_var SCRIPT_BASE PACKER_BUILDER_NAME GOSRC FEDORA_BASE_IMAGE OS_RELEASE_ID OS_RELEASE_VER

install_ooe

export GOPATH="$(mktemp -d)"
trap "sudo rm -rf $GOPATH" EXIT

$BIGTO ooe.sh sudo dnf update -y

# Do not enable update-stesting on the previous Fedora release
if [[ "$FEDORA_BASE_IMAGE" =~ "${OS_RELEASE_ID}-cloud-base-${OS_RELEASE_VER}" ]]; then
    warn "Enabling updates-testing repository for image based on $FEDORA_BASE_IMAGE"
    $LILTO ooe.sh sudo dnf install -y 'dnf-command(config-manager)'
    $LILTO ooe.sh sudo dnf config-manager --set-enabled updates-testing
else
    warn "NOT enabling updates-testing repository for image based on $PRIOR_FEDORA_BASE_IMAGE"
fi

REMOVE_PACKAGES=()
INSTALL_PACKAGES=(\
    autoconf
    automake
    bash-completion
    bats
    bridge-utils
    btrfs-progs-devel
    bzip2
    conmon
    container-selinux
    containernetworking-plugins
    containers-common
    criu
    device-mapper-devel
    dnsmasq
    emacs-nox
    file
    findutils
    fuse3
    fuse3-devel
    gcc
    git
    glib2-devel
    glibc-static
    gnupg
    go-md2man
    golang
    gpgme-devel
    iproute
    iptables
    jq
    libassuan-devel
    libcap-devel
    libmsi1
    libnet
    libnet-devel
    libnl3-devel
    libseccomp
    libseccomp-devel
    libselinux-devel
    libtool
    libvarlink-util
    lsof
    make
    msitools
    nmap-ncat
    ostree-devel
    pandoc
    podman
    procps-ng
    protobuf
    protobuf-c
    protobuf-c-devel
    protobuf-devel
    protobuf-python
    python
    python3-dateutil
    python3-psutil
    python3-pytoml
    rsync
    runc
    selinux-policy-devel
    skopeo
    skopeo-containers
    slirp4netns
    unzip
    vim
    wget
    which
    xz
    zip
)
case "$OS_RELEASE_VER" in
    30)
        INSTALL_PACKAGES+=(\
            atomic-registries
            golang-github-cpuguy83-go-md2man
            python2-future
            runc
        )
        REMOVE_PACKAGES+=(crun)
        ;;
    31)
        INSTALL_PACKAGES+=(crun)
        REMOVE_PACKAGES+=(runc)
        ;;
    *)
        bad_os_id_ver ;;
esac

echo "Installing general build/test dependencies for Fedora '$OS_RELEASE_VER'"
$BIGTO ooe.sh sudo dnf install -y ${INSTALL_PACKAGES[@]}

install_buildah_packages

[[ "${#REMOVE_PACKAGES[@]}" -eq "0" ]] || \
    $LILTO ooe.sh sudo dnf erase -y ${REMOVE_PACKAGES[@]}

echo "Enabling cgroup management from containers"
ooe.sh sudo setsebool container_manage_cgroup true

ooe.sh sudo /tmp/libpod/hack/install_catatonit.sh

# Ensure there are no disruptive periodic services enabled by default in image
systemd_banish

rh_finalize

echo "SUCCESS!"
