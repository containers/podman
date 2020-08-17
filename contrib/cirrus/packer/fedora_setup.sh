#!/usr/bin/env bash

# This script is called by packer on the subject fedora VM, to setup the podman
# build/test environment.  It's not intended to be used outside of this context.

set -e

# Load in library (copied by packer, before this script was run)
source $GOSRC/$SCRIPT_BASE/lib.sh

req_env_var SCRIPT_BASE PACKER_BASE INSTALL_AUTOMATION_VERSION PACKER_BUILDER_NAME GOSRC FEDORA_BASE_IMAGE OS_RELEASE_ID OS_RELEASE_VER

workaround_bfq_bug

# Do not enable updates-testing on the previous Fedora release
if [[ "$PRIOR_FEDORA_BASE_IMAGE" =~ "${OS_RELEASE_ID}-cloud-base-${OS_RELEASE_VER}" ]]; then
    DISABLE_UPDATES_TESTING=1
else
    DISABLE_UPDATES_TESTING=0
fi

bash $PACKER_BASE/fedora_packaging.sh
# Load installed environment right now (happens automatically in a new process)
source /usr/share/automation/environment

echo "Enabling cgroup management from containers"
ooe.sh sudo setsebool container_manage_cgroup true

# Ensure there are no disruptive periodic services enabled by default in image
systemd_banish

rh_finalize

echo "SUCCESS!"
