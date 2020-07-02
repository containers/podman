#!/bin/bash

# This script is called from ubuntu_setup.sh and various Dockerfiles.
# It's not intended to be used outside of those contexts.  It assumes the lib.sh
# library has already been sourced, and that all "ground-up" package-related activity
# needs to be done, including repository setup and initial update.

set -e

echo "Updating/Installing repos and packages for $OS_REL_VER"

source $GOSRC/$SCRIPT_BASE/lib.sh

req_env_var GOSRC SCRIPT_BASE BIGTO SUDOAPTGET INSTALL_AUTOMATION_VERSION

echo "Updating/configuring package repositories."
$BIGTO $SUDOAPTGET update

echo "Installing deps to add third-party repositories and automation tooling"
$LILTO $SUDOAPTGET install software-properties-common git curl

# Install common automation tooling (i.e. ooe.sh)
curl --silent --show-error --location \
     --url "https://raw.githubusercontent.com/containers/automation/master/bin/install_automation.sh" | \
     $SUDO env INSTALL_PREFIX=/usr/share /bin/bash -s - "$INSTALL_AUTOMATION_VERSION"
# Reload installed environment right now (happens automatically in a new process)
source /usr/share/automation/environment

$LILTO ooe.sh $SUDOAPTADD ppa:criu/ppa

echo "Configuring/Instaling deps from Open build server"
VERSION_ID=$(source /etc/os-release; echo $VERSION_ID)
echo "deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/xUbuntu_$VERSION_ID/ /" \
    | ooe.sh $SUDO tee /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list
ooe.sh curl -L -o /tmp/Release.key "https://download.opensuse.org/repositories/devel:kubic:libcontainers:stable/xUbuntu_${VERSION_ID}/Release.key"
ooe.sh $SUDO apt-key add - < /tmp/Release.key

INSTALL_PACKAGES=(\
    apparmor
    aufs-tools
    autoconf
    automake
    bash-completion
    bats
    bison
    btrfs-progs
    build-essential
    buildah
    bzip2
    conmon
    containernetworking-plugins
    containers-common
    coreutils
    cri-o-runc
    criu
    curl
    dnsmasq
    e2fslibs-dev
    emacs-nox
    file
    fuse3
    gawk
    gcc
    gettext
    git
    go-md2man
    golang
    iproute2
    iptables
    jq
    libaio-dev
    libapparmor-dev
    libbtrfs-dev
    libcap-dev
    libdevmapper-dev
    libdevmapper1.02.1
    libfuse-dev
    libfuse2
    libfuse3-dev
    libglib2.0-dev
    libgpgme11-dev
    liblzma-dev
    libnet1
    libnet1-dev
    libnl-3-dev
    libprotobuf-c-dev
    libprotobuf-dev
    libseccomp-dev
    libseccomp2
    libselinux-dev
    libsystemd-dev
    libtool
    libudev-dev
    libvarlink
    lsof
    make
    netcat
    openssl
    pkg-config
    podman
    protobuf-c-compiler
    protobuf-compiler
    python-protobuf
    python2
    python3-dateutil
    python3-pip
    python3-psutil
    python3-pytoml
    python3-setuptools
    rsync
    runc
    scons
    skopeo
    slirp4netns
    socat
    sudo
    unzip
    vim
    wget
    xz-utils
    zip
    zlib1g-dev
)
DOWNLOAD_PACKAGES=(\
    cri-o-$(get_kubernetes_version)
    cri-tools
    parallel
)

# These aren't resolvable on Ubuntu 20
if [[ "$OS_RELEASE_VER" -le 19 ]]; then
    INSTALL_PACKAGES+=(\
        python-future
        python-minimal
        yum-utils
    )
fi

# Do this at the last possible moment to avoid dpkg lock conflicts
echo "Upgrading all packages"
$BIGTO ooe.sh $SUDOAPTGET upgrade

echo "Installing general testing and system dependencies"
# Necessary to update cache of newly added repos
$LILTO ooe.sh $SUDOAPTGET update
$BIGTO ooe.sh $SUDOAPTGET install ${INSTALL_PACKAGES[@]}

if [[ ${#DOWNLOAD_PACKAGES[@]} -gt 0 ]]; then
    echo "Downloading packages for optional installation at runtime, as needed."
    $SUDO ln -s /var/cache/apt/archives "$PACKAGE_DOWNLOAD_DIR"
    $LILTO ooe.sh $SUDOAPTGET install --download-only ${DOWNLOAD_PACKAGES[@]}
    ls -la "$PACKAGE_DOWNLOAD_DIR/"
fi

echo "Installing runtime tooling"
cd $GOSRC
# Required since initially go was not installed
source $GOSRC/$SCRIPT_BASE/lib.sh
echo "Go environment has been setup:"
go env
$SUDO hack/install_catatonit.sh
$SUDO make install.libseccomp.sudo
$SUDO make install.tools
