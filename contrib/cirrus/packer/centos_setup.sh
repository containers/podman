#!/bin/bash

# This script is called by packer on the subject CentOS VM, to setup the podman
# build/test environment.  It's not intended to be used outside of this context.

set -e

# Load in library (copied by packer, before this script was run)
source /tmp/libpod/$SCRIPT_BASE/lib.sh

req_env_var "
SCRIPT_BASE $SCRIPT_BASE
CNI_COMMIT $CNI_COMMIT
CRIO_COMMIT $CRIO_COMMIT
"

install_ooe

export GOPATH="$(mktemp -d)"
trap "sudo rm -rf $GOPATH" EXIT

ooe.sh sudo yum -y update

ooe.sh sudo yum -y install centos-release-scl epel-release

ooe.sh sudo yum -y install \
          atomic-registries \
          btrfs-progs-devel \
          bzip2 \
          device-mapper-devel \
          findutils \
          glib2-devel \
          glibc-static \
          gnupg \
          golang \
          golang-github-cpuguy83-go-md2man \
          golang-github-cpuguy83-go-md2man \
          gpgme-devel \
          iptables \
          libassuan-devel \
          libseccomp-devel \
          libselinux-devel \
          lsof \
          make \
          nmap-ncat \
          ostree-devel \
          python \
          python3-dateutil \
          python3-psutil \
          python3-pytoml \
          runc \
          skopeo-containers \
          unzip \
          which \
          xz

install_scl_git

install_cni_plugins

install_buildah

install_conmon

install_packer_copied_files

rh_finalize

echo "SUCCESS!"
