#!/bin/bash

# This script is called from fedora_setup.sh and various Dockerfiles.
# It's not intended to be used outside of those contexts.  It assumes the lib.sh
# library has already been sourced, and that all "ground-up" package-related activity
# needs to be done, including repository setup and initial update.

set -e

echo "Updating/Installing repos and packages for $OS_REL_VER"

source $GOSRC/$SCRIPT_BASE/lib.sh

req_env_var GOSRC SCRIPT_BASE BIGTO INSTALL_AUTOMATION_VERSION FEDORA_BASE_IMAGE PRIOR_FEDORA_BASE_IMAGE

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

# Do not enable updates-testing on the previous Fedora release
if ((DISABLE_UPDATES_TESTING!=0)); then
    warn "Enabling updates-testing repository for image based on $FEDORA_BASE_IMAGE"
    $LILTO $SUDO ooe.sh dnf install -y 'dnf-command(config-manager)'
    $LILTO $SUDO ooe.sh dnf config-manager --set-enabled updates-testing
else
    warn "NOT enabling updates-testing repository for image based on $PRIOR_FEDORA_BASE_IMAGE"
fi

$BIGTO ooe.sh $SUDO dnf update -y

# Fedora, as of 31, uses cgroups v2 by default. runc does not support
# cgroups v2, only crun does. (As of 2020-07-30 runc support is
# forthcoming but not even close to ready yet). To ensure a reliable
# runtime environment, force-remove runc if it is present.
# However, because a few other repos. which use these images still need
# it, ensure the runc package is cached in $PACKAGE_DOWNLOAD_DIR so
# it may be swap it in when required.
REMOVE_PACKAGES=(runc)

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
    crun
    curl
    device-mapper-devel
    dnsmasq
    e2fsprogs-devel
    emacs-nox
    file
    findutils
    fuse3
    fuse3-devel
    gcc
    git
    glib2-devel
    glibc-devel
    glibc-static
    gnupg
    go-md2man
    golang
    gpgme
    gpgme-devel
    grubby
    hostname
    iproute
    iptables
    jq
    krb5-workstation
    libassuan
    libassuan-devel
    libblkid-devel
    libcap-devel
    libffi-devel
    libgpg-error-devel
    libguestfs-tools
    libmsi1
    libnet
    libnet-devel
    libnl3-devel
    libseccomp
    libseccomp-devel
    libselinux-devel
    libtool
    libvarlink-util
    libxml2-devel
    libxslt-devel
    lsof
    make
    mlocate
    msitools
    nfs-utils
    nmap-ncat
    openssl
    openssl-devel
    ostree-devel
    pandoc
    pkgconfig
    podman
    policycoreutils
    procps-ng
    protobuf
    protobuf-c
    protobuf-c-devel
    protobuf-devel
    python2
    python3-PyYAML
    python3-dateutil
    python3-libselinux
    python3-libsemanage
    python3-libvirt
    python3-psutil
    python3-pytoml
    python3-requests
    redhat-rpm-config
    rpcbind
    rsync
    sed
    selinux-policy-devel
    skopeo
    skopeo-containers
    slirp4netns
    socat
    tar
    unzip
    vim
    wget
    which
    xz
    zip
    zlib-devel
)
DOWNLOAD_PACKAGES=(\
    "cri-o-$(get_kubernetes_version)*"
    cri-tools
    "kubernetes-$(get_kubernetes_version)*"
    runc
    oci-umount
    parallel
)

echo "Installing general build/test dependencies for Fedora '$OS_RELEASE_VER'"
$BIGTO ooe.sh $SUDO dnf install -y ${INSTALL_PACKAGES[@]}

[[ ${#REMOVE_PACKAGES[@]} -eq 0 ]] || \
    $LILTO ooe.sh $SUDO dnf erase -y "${REMOVE_PACKAGES[@]}"

if [[ ${#DOWNLOAD_PACKAGES[@]} -gt 0 ]]; then
    echo "Downloading packages for optional installation at runtime, as needed."
    # Required for cri-o
    ooe.sh $SUDO dnf -y module enable cri-o:$(get_kubernetes_version)
    $SUDO mkdir -p "$PACKAGE_DOWNLOAD_DIR"
    cd "$PACKAGE_DOWNLOAD_DIR"
    $LILTO ooe.sh $SUDO dnf download -y --resolve "${DOWNLOAD_PACKAGES[@]}"
fi

# HACK: Need Conmon 2.0.17, currently in updates-testing on F31.
$SUDO dnf update -y --enablerepo=updates-testing conmon

echo "Installing runtime tooling"
# Save some runtime by having these already available
cd $GOSRC
$SUDO make install.tools
$SUDO $GOSRC/hack/install_catatonit.sh
