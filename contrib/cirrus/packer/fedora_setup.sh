#!/bin/bash

# This script is called by packer on the subject fedora VM, to setup the podman
# build/test environment.  It's not intended to be used outside of this context.

set -e

# Load in library (copied by packer, before this script was run)
source /tmp/libpod/$SCRIPT_BASE/lib.sh

req_env_var SCRIPT_BASE PACKER_BUILDER_NAME GOSRC

install_ooe

export GOPATH="$(mktemp -d)"
trap "sudo rm -rf $GOPATH" EXIT

ooe.sh sudo dnf update -y

echo "Enabling updates-testing repository"
ooe.sh sudo dnf install -y 'dnf-command(config-manager)'
ooe.sh sudo dnf config-manager --set-enabled updates-testing

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
    libvarlink-util \
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

# Ensure there are no disruptive periodic services enabled by default in image
systemd_banish

sudo /tmp/libpod/hack/install_catatonit.sh

# Same script is used for several related contexts
case "$PACKER_BUILDER_NAME" in
    xfedora*)
        echo "Configuring CGroups v2 enabled on next boot"
        sudo grubby --update-kernel=ALL --args="systemd.unified_cgroup_hierarchy=1"
        ;&  # continue to next matching item
    *)
        echo "Finalizing $PACKER_BUILDER_NAME VM image"
        ;;
esac

rh_finalize

echo "SUCCESS!"
