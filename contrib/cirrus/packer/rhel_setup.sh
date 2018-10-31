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
CRIU_COMMIT $CRIU_COMMIT
RHSM_COMMAND $RHSM_COMMAND
"

install_ooe

export GOPATH="$(mktemp -d)"
export RHSMCMD="$(mktemp)"

exit_handler() {
    set +ex
    cd /
    sudo rm -rf "$RHSMCMD"
    sudo rm -rf "$GOPATH"
    sudo subscription-manager remove --all
    sudo subscription-manager unregister
    sudo subscription-manager clean
}
trap "exit_handler" EXIT

# Avoid logging sensitive details
echo "$RHSM_COMMAND" > "$RHSMCMD"
ooe.sh sudo bash "$RHSMCMD"
sudo rm -rf "$RHSMCMD"

ooe.sh sudo yum -y erase "rh-amazon-rhui-client*"
ooe.sh sudo subscription-manager repos "--disable=*"
ooe.sh sudo subscription-manager repos \
    --enable=rhel-7-server-rpms \
    --enable=rhel-7-server-optional-rpms \
    --enable=rhel-7-server-extras-rpms \
    --enable=rhel-server-rhscl-7-rpms

ooe.sh sudo yum -y update

# Frequently needed
ooe.sh sudo yum -y install https://dl.fedoraproject.org/pub/epel/epel-release-latest-7.noarch.rpm

# Required for google to manage ssh keys
sudo tee -a /etc/yum.repos.d/google-cloud-sdk.repo << EOM
[google-cloud-compute]
name=google-cloud-compute
baseurl=https://packages.cloud.google.com/yum/repos/google-cloud-compute-el7-x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
       https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOM

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
    google-compute-engine \
    google-compute-engine-oslogin \
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
    protobuf \
    protobuf-c \
    protobuf-c-devel \
    protobuf-compiler \
    protobuf-devel \
    protobuf-python \
    python \
    python2-future \
    python34-dateutil \
    python34-psutil \
    python34-pytoml \
    runc \
    skopeo-containers \
    unzip \
    which \
    xz

install_scl_git

install_cni_plugins $CNI_COMMIT

install_buildah

install_conmon

install_criu

install_packer_copied_files

exit_handler  # release subscription!

rh_finalize

echo "SUCCESS!"
