#!/bin/bash

# This script is called by packer on a vanilla CentOS VM, to setup the image
# used for building images FROM base images. It's not intended to be used
# outside of this context.

set -e

[[ "$1" == "post" ]] || exit 0  # pre stage not needed

# Load in library (copied by packer, before this script was run)
source $GOSRC/$SCRIPT_BASE/lib.sh

req_env_var "
    TIMESTAMP $TIMESTAMP
    GOSRC $GOSRC
    SCRIPT_BASE $SCRIPT_BASE
    PACKER_BASE $PACKER_BASE
"

install_ooe

echo "Updating packages"
ooe.sh sudo yum -y update

echo "Configuring repositories"
ooe.sh sudo yum -y install centos-release-scl epel-release

echo "Installing packages"
ooe.sh sudo yum -y install \
    genisoimage \
    golang \
    google-cloud-sdk \
    libvirt \
    libvirt-admin \
    libvirt-client \
    libvirt-daemon \
    make \
    python34 \
    python34 \
    python34-PyYAML \
    python34-PyYAML \
    qemu-img \
    qemu-kvm \
    qemu-kvm-tools \
    qemu-user \
    rsync \
    unzip \
    util-linux \
    vim

sudo ln -s /usr/libexec/qemu-kvm /usr/bin/

sudo tee /etc/modprobe.d/kvm-nested.conf <<EOF
options kvm-intel nested=1
options kvm-intel enable_shadow_vmcs=1
options kvm-intel enable_apicv=1
options kvm-intel ept=1
EOF

echo "Installing packer"
sudo mkdir -p /root/$(basename $PACKER_BASE)
sudo cp $GOSRC/$PACKER_BASE/*packer* /root/$(basename $PACKER_BASE)
sudo mkdir -p /root/$(basename $SCRIPT_BASE)
sudo cp $GOSRC/$SCRIPT_BASE/*.sh /root/$(basename $SCRIPT_BASE)

install_scl_git

echo "Cleaning up"
cd /
rm -rf $GOSRC

rh_finalize

echo "SUCCESS!"
