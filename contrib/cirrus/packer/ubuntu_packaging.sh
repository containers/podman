#!/bin/bash

# This script is called from ubuntu_setup.sh and various Dockerfiles.
# It's not intended to be used outside of those contexts.  It assumes the lib.sh
# library has already been sourced, and that all "ground-up" package-related activity
# needs to be done, including repository setup and initial update.

set -e

echo "Updating/Installing repos and packages for $OS_REL_VER"

source $GOSRC/$SCRIPT_BASE/lib.sh

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

# Install newer version of golang
if [[ "$OS_RELEASE_VER" -eq "18" ]]
then
    $LILTO ooe.sh $SUDOAPTADD ppa:longsleep/golang-backports
fi

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
    bison
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
    libcap-dev
    libdevmapper-dev
    libdevmapper1.02.1
    libfuse-dev
    libfuse2
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
    python-future
    python-minimal
    python-protobuf
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
    yum-utils
    zip
    zlib1g-dev
)

if [[ $OS_RELEASE_VER -ge 19 ]]
then
    INSTALL_PACKAGES+=(\
        bats
        btrfs-progs
        fuse3
        libbtrfs-dev
        libfuse3-dev
    )
else
    echo "Downloading version of bats with fix for a \$IFS related bug in 'run' command"
    cd /tmp
    BATS_URL='http://launchpadlibrarian.net/438140887/bats_1.1.0+git104-g1c83a1b-1_all.deb'
    curl -L -O "$BATS_URL"
    cd -
    INSTALL_PACKAGES+=(\
        /tmp/$(basename $BATS_URL)
        btrfs-tools
    )
fi

# Do this at the last possible moment to avoid dpkg lock conflicts
echo "Upgrading all packages"
$BIGTO ooe.sh $SUDOAPTGET upgrade

echo "Installing general testing and system dependencies"
# Necessary to update cache of newly added repos
$LILTO ooe.sh $SUDOAPTGET update
$BIGTO ooe.sh $SUDOAPTGET install ${INSTALL_PACKAGES[@]}

export GOPATH="$(mktemp -d)"
trap "$SUDO rm -rf $GOPATH" EXIT
echo "Installing cataonit and libseccomp.sudo"
cd $GOSRC
ooe.sh $SUDO hack/install_catatonit.sh
ooe.sh $SUDO make install.libseccomp.sudo

CRIO_RUNC_PATH="/usr/lib/cri-o-runc/sbin/runc"
if $SUDO dpkg -L cri-o-runc | grep -m 1 -q "$CRIO_RUNC_PATH"
then
    echo "Linking $CRIO_RUNC_PATH to /usr/bin/runc for ease of testing."
    $SUDO ln -f "$CRIO_RUNC_PATH" "/usr/bin/runc"
fi
