#!/bin/bash

# This script is called from fedora_setup.sh and various Dockerfiles.
# It's not intended to be used outside of those contexts.  It assumes the lib.sh
# library has already been sourced, and that all "ground-up" package-related activity
# needs to be done, including repository setup and initial update.

set -e

echo "Updating/Installing repos and packages for $OS_REL_VER"

source $GOSRC/$SCRIPT_BASE/lib.sh

# Pre-req. to install automation tooing
$LILTO $SUDO dnf install -y git

# Install common automation tooling (i.e. ooe.sh)
curl --silent --show-error --location \
     --url "https://raw.githubusercontent.com/containers/automation/master/bin/install_automation.sh" | \
     $SUDO env INSTALL_PREFIX=/usr/share /bin/bash -s - "$INSTALL_AUTOMATION_VERSION"
# Reload installed environment right now (happens automatically in a new process)
source /usr/share/automation/environment

# Set this to 1 to NOT enable updates-testing repository
DISABLE_UPDATES_TESTING=${DISABLE_UPDATES_TESTING:0}

# Do not enable update-stesting on the previous Fedora release
if ((DISABLE_UPDATES_TESTING!=0)); then
    warn "Enabling updates-testing repository for image based on $FEDORA_BASE_IMAGE"
    $LILTO $SUDO ooe.sh dnf install -y 'dnf-command(config-manager)'
    $LILTO $SUDO ooe.sh dnf config-manager --set-enabled updates-testing
else
    warn "NOT enabling updates-testing repository for image based on $PRIOR_FEDORA_BASE_IMAGE"
fi

$BIGTO ooe.sh $SUDO dnf update -y

REMOVE_PACKAGES=()
INSTALL_PACKAGES=(\
    autoconf
    automake
    bash-completion
    bats
    bridge-utils
    btrfs-progs-devel
    buildah
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
    python
    python3-dateutil
    python3-psutil
    python3-pytoml
    rsync
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
    32)
        INSTALL_PACKAGES+=(crun)
        REMOVE_PACKAGES+=(runc)
        ;;
    *)
        bad_os_id_ver ;;
esac

echo "Installing general build/test dependencies for Fedora '$OS_RELEASE_VER'"
$BIGTO ooe.sh $SUDO dnf install -y ${INSTALL_PACKAGES[@]}

[[ ${#REMOVE_PACKAGES[@]} -eq 0 ]] || \
    $LILTO ooe.sh $SUDO dnf erase -y ${REMOVE_PACKAGES[@]}

export GOPATH="$(mktemp -d)"
trap "$SUDO rm -rf $GOPATH" EXIT
ooe.sh $SUDO $GOSRC/hack/install_catatonit.sh
