#!/bin/bash

# This script is called by packer on the subject fedora VM, to setup the podman
# build/test environment.  It's not intended to be used outside of this context.

set -e

# Load in library (copied by packer, before this script was run)
source /tmp/libpod/$SCRIPT_BASE/lib.sh

req_env_var "
SCRIPT_BASE $SCRIPT_BASE
FEDORA_CNI_COMMIT $FEDORA_CNI_COMMIT
CNI_COMMIT $CNI_COMMIT
CRIO_COMMIT $CRIO_COMMIT
CRIU_COMMIT $CRIU_COMMIT
RUNC_COMMIT $RUNC_COMMIT
"

install_ooe

export GOPATH="$(mktemp -d)"
trap "sudo rm -rf $GOPATH" EXIT

ooe.sh sudo dnf update -y

ooe.sh sudo dnf install -y \
    atomic-registries \
    btrfs-progs-devel \
    bzip2 \
    device-mapper-devel \
    emacs-nox \
    findutils \
    git \
    glib2-devel \
    glibc-static \
    gnupg \
    golang \
    golang-github-cpuguy83-go-md2man \
    golang-github-cpuguy83-go-md2man \
    gpgme-devel \
    iptables \
    libassuan-devel \
    libcap-devel \
    libnet \
    libnet-devel \
    libnl3-devel \
    libseccomp-devel \
    libselinux-devel \
    lsof \
    make \
    nmap-ncat \
    ostree-devel \
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
    skopeo-containers \
    slirp4netns \
    unzip \
    vim \
    which \
    xz

install_varlink

CNI_COMMIT=$FEDORA_CNI_COMMIT
install_cni_plugins

install_buildah

install_conmon

install_criu

install_packer_copied_files

rh_finalize # N/B: Halts system!

echo "SUCCESS!"
