#!/bin/bash

# N/B: This script is not intended to be run by humans.  It is used to configure the
# fedora base image for importing, so that it will boot in GCE

set -e

# Load in library (copied by packer, before this script was run)
source $GOSRC/$SCRIPT_BASE/lib.sh

echo "Updating packages"
dnf -y update

echo "Installing necessary packages and google services"
dnf -y install rng-tools google-compute-engine-tools google-compute-engine-oslogin ethtool

echo "Enabling services"
systemctl enable rngd

# There is a race that can happen on boot between the GCE services configuring
# the VM, and cloud-init trying to do similar activities.  Use a customized
# unit file to make sure cloud-init starts after the google-compute-* services.
echo "Setting cloud-init service to start after google-network-daemon.service"
cp -v $GOSRC/$PACKER_BASE/cloud-init/fedora/cloud-init.service /etc/systemd/system/

# ref: https://cloud.google.com/compute/docs/startupscript
# The mechanism used by Cirrus-CI to execute tasks on the system is through an
# "agent" process launched as a GCP startup-script (from the metadata service).
# This agent is responsible for cloning the repository and executing all task
# scripts and other operations.  Therefor, on SELinux-enforcing systems, the
# service must be labeled properly to ensure it's child processes can
# run with the proper contexts.
METADATA_SERVICE_CTX=unconfined_u:unconfined_r:unconfined_t:s0
METADATA_SERVICE_PATH=systemd/system/google-startup-scripts.service
sed -r -e \
    "s/Type=oneshot/Type=oneshot\nSELinuxContext=$METADATA_SERVICE_CTX/" \
    /lib/$METADATA_SERVICE_PATH > /etc/$METADATA_SERVICE_PATH

# Ensure there are no disruptive periodic services enabled by default in image
systemd_banish

rh_finalize

echo "SUCCESS!"
