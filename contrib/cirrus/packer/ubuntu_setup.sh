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
$LILTO $SUDOAPTGET install software-properties-common
$LILTO $SUDOAPTADD ppa:longsleep/golang-backports
$LILTO $SUDOAPTADD ppa:projectatomic/ppa
$LILTO $SUDOAPTADD ppa:criu/ppa

echo "Upgrading all packages"
$LILTO $SUDOAPTGET update
$BIGTO $SUDOAPTGET upgrade

echo "Installing general testing and system dependencies"
$BIGTO $SUDOAPTGET install \
    apparmor \
    autoconf \
    automake \
    bats \
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
    libglib2.0-dev \
    libgpgme11-dev \
    liblzma-dev \
    libnet1 \
    libnet1-dev \
    libnl-3-dev \
    libostree-dev \
    libprotobuf-c0-dev \
    libprotobuf-dev \
    libseccomp-dev \
    libseccomp2 \
    libsystemd-dev \
    libtool \
    libudev-dev \
    lsof \
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
    slirp4netns \
    skopeo \
    socat \
    unzip \
    vim \
    xz-utils \
    zip

echo "Forced Ubuntu 18 kernel to enable cgroup swap accounting."
SEDCMD='s/^GRUB_CMDLINE_LINUX="(.*)"/GRUB_CMDLINE_LINUX="\1 cgroup_enable=memory swapaccount=1"/g'
ooe.sh sudo sed -re "$SEDCMD" -i /etc/default/grub.d/*
ooe.sh sudo sed -re "$SEDCMD" -i /etc/default/grub
ooe.sh sudo update-grub

sudo /tmp/libpod/hack/install_catatonit.sh
ooe.sh sudo make -C /tmp/libpod install.libseccomp.sudo

ubuntu_finalize

echo "SUCCESS!"
