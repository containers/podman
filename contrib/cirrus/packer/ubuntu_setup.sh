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

# Ensure there are no disruptive periodic services enabled by default in image
systemd_banish

echo "Updating/configuring package repositories."
$LILTO $SUDOAPTGET update

echo "Upgrading all packages"
$BIGTO $SUDOAPTGET upgrade

echo "Adding PPAs"
$LILTO $SUDOAPTGET install software-properties-common
$LILTO $SUDOAPTADD ppa:projectatomic/ppa
$LILTO $SUDOAPTADD ppa:criu/ppa
if [[ "$OS_RELEASE_VER" -eq "18" ]]
then
    $LILTO $SUDOAPTADD ppa:longsleep/golang-backports
fi
sudo apt-key adv --keyserver keyserver.ubuntu.com --recv-keys 4052245BD4284CDD
# The repository isn't available for ubnutu 19 but the binaries work perfectly
sudo echo "deb https://repo.iovisor.org/apt/bionic bionic main" | sudo tee /etc/apt/sources.list.d/iovisor.list

$LILTO $SUDOAPTGET update

echo "Installing general testing and system dependencies"
$BIGTO $SUDOAPTGET install \
    apparmor \
    aufs-tools \
    autoconf \
    automake \
    bats \
    bcc-tools \
    bison \
    btrfs-tools \
    build-essential \
    containernetworking-plugins \
    containers-common \
    cri-o-runc \
    criu \
    curl \
    e2fslibs-dev \
    emacs-nox \
    gawk \
    gcc \
    gettext \
    go-md2man \
    golang \
    iproute2 \
    iptables \
    jq \
    libaio-dev \
    libapparmor-dev \
    libcap-dev \
    libdevmapper-dev \
    libdevmapper1.02.1 \
    libfuse-dev \
    libfuse2 \
    libglib2.0-dev \
    libgpgme11-dev \
    liblzma-dev \
    libnet1 \
    libnet1-dev \
    libnl-3-dev \
    libostree-dev \
    libvarlink \
    libprotobuf-c-dev \
    libprotobuf-dev \
    libseccomp-dev \
    libseccomp2 \
    libsystemd-dev \
    libtool \
    libudev-dev \
    lsof \
    make \
    netcat \
    pkg-config \
    podman \
    protobuf-c-compiler \
    protobuf-compiler \
    python-future \
    python-minimal \
    python-protobuf \
    python3-dateutil \
    python3-pip \
    python3-psutil \
    python3-pytoml \
    python3-setuptools \
    skopeo \
    slirp4netns \
    socat \
    unzip \
    vim \
    xz-utils \
    zip \
    linux-headers-$(uname -r)

if [[ "$OS_RELEASE_VER" -ge "19" ]]
then
    echo "Installing Ubuntu > 18 packages"
    $LILTO $SUDOAPTGET install fuse3 libfuse3-dev libbtrfs-dev
fi

if [[ "$OS_RELEASE_VER" -eq "18" ]]
then
    echo "Forced Ubuntu 18 kernel to enable cgroup swap accounting."
    SEDCMD='s/^GRUB_CMDLINE_LINUX="(.*)"/GRUB_CMDLINE_LINUX="\1 cgroup_enable=memory swapaccount=1"/g'
    ooe.sh sudo sed -re "$SEDCMD" -i /etc/default/grub.d/*
    ooe.sh sudo sed -re "$SEDCMD" -i /etc/default/grub
    ooe.sh sudo update-grub
fi

sudo /tmp/libpod/hack/install_catatonit.sh
ooe.sh sudo make -C /tmp/libpod install.libseccomp.sudo

ubuntu_finalize

echo "SUCCESS!"
