#!/bin/bash

# N/B: This script is not intended to be run by humans.  It is used to configure the
# fedora base image for importing, so that it will boot in GCE

set -e

dnf -y update
dnf -y copr enable ngompa/gce-oslogin
dnf -y install rng-tools google-compute-engine google-compute-engine-oslogin
systemctl enable rngd
history -c
