#!/bin/bash

# This script is called by packer on the subject Ubuntu VM, to setup the podman
# build/test environment.  It's not intended to be used outside of this context.

set -e

# Load in library (copied by packer, before this script was run)
source $GOSRC/$SCRIPT_BASE/lib.sh

req_env_var SCRIPT_BASE PACKER_BASE INSTALL_AUTOMATION_VERSION PACKER_BUILDER_NAME GOSRC UBUNTU_BASE_IMAGE OS_RELEASE_ID OS_RELEASE_VER

# Ensure there are no disruptive periodic services enabled by default in image
systemd_banish

# Stop disruption upon boot ASAP after booting
echo "Disabling all packaging activity on boot"
for filename in $(sudo ls -1 /etc/apt/apt.conf.d); do \
    echo "Checking/Patching $filename"
    sudo sed -i -r -e "s/$PERIODIC_APT_RE/"'\10"\;/' "/etc/apt/apt.conf.d/$filename"; done

bash $PACKER_BASE/ubuntu_packaging.sh

# Load installed environment right now (happens automatically in a new process)
source /usr/share/automation/environment

echo "Making Ubuntu kernel to enable cgroup swap accounting as it is not the default."
SEDCMD='s/^GRUB_CMDLINE_LINUX="(.*)"/GRUB_CMDLINE_LINUX="\1 cgroup_enable=memory swapaccount=1"/g'
ooe.sh sudo sed -re "$SEDCMD" -i /etc/default/grub.d/*
ooe.sh sudo sed -re "$SEDCMD" -i /etc/default/grub
ooe.sh sudo update-grub

ubuntu_finalize

echo "SUCCESS!"
