#!/bin/bash

# N/B: This script is not intended to be run by humans.  It is used to configure the
# rhel base image for importing, so that it will boot in GCE

set -e

[[ "$1" == "post" ]] || exit 0  # pre stage is not needed

# Load in library (copied by packer, before this script was run)
source $GOSRC/$SCRIPT_BASE/lib.sh

req_env_var "
    RHSM_COMMAND $RHSM_COMMAND
"

install_ooe

echo "Setting up repos"
# Frequently needed
ooe.sh sudo yum -y install https://dl.fedoraproject.org/pub/epel/epel-release-latest-7.noarch.rpm

# Required for google to manage ssh keys
ooe.sh sudo tee /etc/yum.repos.d/google-cloud-sdk.repo << EOM
[google-cloud-compute]
name=google-cloud-compute
baseurl=https://packages.cloud.google.com/yum/repos/google-cloud-compute-el7-x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
       https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOM

rhsm_enable

echo "Installing/removing packages"
ooe.sh sudo yum -y install google-compute-engine google-compute-engine-oslogin
ooe.sh sudo yum -y erase "cloud-init" "rh-amazon-rhui-client*" || true
ooe.sh sudo systemctl enable \
    google-accounts-daemon \
    google-clock-skew-daemon \
    google-instance-setup \
    google-network-daemon \
    google-shutdown-scripts \
    google-startup-scripts

rhel_exit_handler  # release subscription!

rh_finalize

echo "SUCCESS!"
