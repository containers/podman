#!/bin/bash

# This script is called by packer on the subject fah VM, to setup the podman
# build/test environment.  It's not intended to be used outside of this context.

set -e

# Load in library (copied by packer, before this script was run)
source /tmp/libpod/$SCRIPT_BASE/lib.sh

req_env_var "
SCRIPT_BASE $SCRIPT_BASE
"

install_ooe

ooe.sh sudo atomic host upgrade

ooe.sh sudo rpm-ostree uninstall cloud-init

rh_finalize

echo "SUCCESS!"
