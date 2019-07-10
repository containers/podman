#!/bin/bash

# N/B: This script is not intended to be run by humans.  It is used to configure the
# fedora base image for importing, so that it will boot in GCE

set -e

# Load in library (copied by packer, before this script was run)
source $GOSRC/$SCRIPT_BASE/lib.sh

[[ "$1" == "post" ]] || exit 0  # nothing to do

install_ooe

echo "Updating packages"
ooe.sh dnf -y update

echo "Installing necessary packages and google services"
ooe.sh dnf -y install rng-tools google-compute-engine-tools google-compute-engine-oslogin ethtool

echo "Enabling services"
ooe.sh systemctl enable rngd

# There is a race that can happen on boot between the GCE services configuring
# the VM, and cloud-init trying to do similar activities.  Use a customized
# unit file to make sure cloud-init starts after the google-compute-* services.
echo "Setting cloud-init service to start after google-network-daemon.service"
cp -v $GOSRC/$PACKER_BASE/cloud-init/fedora/cloud-init.service /etc/systemd/system/

# Ensure there are no disruptive periodic services enabled by default in image
systemd_banish

rh_finalize

echo "SUCCESS!"
