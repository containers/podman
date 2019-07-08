#!/bin/bash

# This script is called by packer on the subject fedora VM, to setup the podman
# build/test environment.  It's not intended to be used outside of this context.

set -e

# Load in library (copied by packer, before this script was run)
source /tmp/libpod/$SCRIPT_BASE/lib.sh

req_env_var SCRIPT_BASE

install_ooe

export GOPATH="$(mktemp -d)"
trap "sudo rm -rf $GOPATH" EXIT

ooe.sh sudo dnf update -y

echo "Installing general build/test dependencies"
ooe.sh sudo dnf install -y \
    atomic-registries \
    bats \
    bridge-utils \
    btrfs-progs-devel \
    bzip2 \
    container-selinux \
    containernetworking-plugins \
    containers-common \
    criu \
    device-mapper-devel \
    emacs-nox \
    findutils \
    git \
    glib2-devel \
    glibc-static \
    gnupg \
    golang \
    golang-github-cpuguy83-go-md2man \
    gpgme-devel \
    iproute \
    iptables \
    jq \
    libassuan-devel \
    libcap-devel \
    libnet \
    libnet-devel \
    libnl3-devel \
    libseccomp \
    libseccomp-devel \
    libselinux-devel \
    lsof \
    make \
    nmap-ncat \
    ostree \
    ostree-devel \
    podman \
    procps-ng \
    protobuf \
    protobuf-c \
    protobuf-c-devel \
    protobuf-compiler \
    protobuf-devel \
    protobuf-python \
    python \
    python2-future \
    python3-dateutil \
    python3-psutil \
    python3-pytoml \
    runc \
    selinux-policy-devel \
    slirp4netns \
    unzip \
    vim \
    which \
    xz \
    zip

sudo /tmp/libpod/hack/install_catatonit.sh

rh_finalize

echo "SUCCESS!"
