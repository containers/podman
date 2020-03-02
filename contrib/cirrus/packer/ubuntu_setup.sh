#!/bin/bash

# This script is called by packer on the subject Ubuntu VM, to setup the podman
# build/test environment.  It's not intended to be used outside of this context.

set -e

# Load in library (copied by packer, before this script was run)
source $GOSRC/$SCRIPT_BASE/lib.sh

req_env_var SCRIPT_BASE

install_ooe

export GOPATH="$(mktemp -d)"
trap "sudo rm -rf $GOPATH" EXIT

# Stop disruption upon boot ASAP after booting
echo "Disabling all packaging activity on boot"
# Don't let sed process sed's temporary files
_FILEPATHS=$(sudo ls -1 /etc/apt/apt.conf.d)
for filename in $_FILEPATHS; do \
    echo "Checking/Patching $filename"
    sudo sed -i -r -e "s/$PERIODIC_APT_RE/"'\10"\;/' "/etc/apt/apt.conf.d/$filename"; done

echo "Updating/configuring package repositories."
$BIGTO $SUDOAPTGET update

echo "Upgrading all packages"
$BIGTO $SUDOAPTGET upgrade

echo "Adding third-party repositories and PPAs"
$LILTO $SUDOAPTGET install software-properties-common
$LILTO $SUDOAPTADD ppa:criu/ppa
if [[ "$OS_RELEASE_VER" -eq "18" ]]
then
    $LILTO $SUDOAPTADD ppa:longsleep/golang-backports
fi

echo "Configuring/Instaling deps from Open build server"
VERSION_ID=$(source /etc/os-release; echo $VERSION_ID)
echo "deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/xUbuntu_$VERSION_ID/ /" \
    > /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list
curl -L -o /tmp/Release.key "https://download.opensuse.org/repositories/devel:kubic:libcontainers:stable/xUbuntu_${VERSION_ID}/Release.key"
apt-key add - < /tmp/Release.key

INSTALL_PACKAGES=(\
    apparmor
    aufs-tools
    autoconf
    automake
    bash-completion
    bison
    btrfs-tools
    build-essential
    containernetworking-plugins
    containers-common
    cri-o-runc
    criu
    curl
    conmon
    dnsmasq
    e2fslibs-dev
    emacs-nox
    file
    gawk
    gcc
    gettext
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
    libvarlink
    libprotobuf-c-dev
    libprotobuf-dev
    libseccomp-dev
    libseccomp2
    libsystemd-dev
    libtool
    libudev-dev
    lsof
    make
    netcat
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
    skopeo
    slirp4netns
    socat
    unzip
    vim
    xz-utils
    zip
)

if [[ "$OS_RELEASE_VER" -ge "19" ]]
then
    INSTALL_PACKAGES+=(\
        bats
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
    INSTALL_PACKAGES+=(/tmp/$(basename $BATS_URL))

    echo "Forced Ubuntu 18 kernel to enable cgroup swap accounting."
    SEDCMD='s/^GRUB_CMDLINE_LINUX="(.*)"/GRUB_CMDLINE_LINUX="\1 cgroup_enable=memory swapaccount=1"/g'
    ooe.sh sudo sed -re "$SEDCMD" -i /etc/default/grub.d/*
    ooe.sh sudo sed -re "$SEDCMD" -i /etc/default/grub
    ooe.sh sudo update-grub
fi

echo "Installing general testing and system dependencies"
# Necessary to update cache of newly added repos
$LILTO $SUDOAPTGET update
$BIGTO $SUDOAPTGET install ${INSTALL_PACKAGES[@]}

echo "Installing cataonit and libseccomp.sudo"
ooe.sh sudo /tmp/libpod/hack/install_catatonit.sh
ooe.sh sudo make -C /tmp/libpod install.libseccomp.sudo

# Ensure there are no disruptive periodic services enabled by default in image
systemd_banish

ubuntu_finalize

echo "SUCCESS!"
