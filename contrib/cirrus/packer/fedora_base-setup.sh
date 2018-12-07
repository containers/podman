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

echo "Installing necessary packages and  google services"
ooe.sh dnf -y copr enable ngompa/gce-oslogin
ooe.sh dnf -y install rng-tools google-compute-engine google-compute-engine-oslogin

echo "Enabling services"
ooe.sh systemctl enable rngd

rh_finalize

echo "SUCCESS!"
