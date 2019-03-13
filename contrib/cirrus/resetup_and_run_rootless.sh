#!/bin/bash

set -e

# N/B: This script is only intended to be used for the special-case of
#      setting up and executing the rootless tests AFTER normal tests complete
#      while testing a freshly built image.

source $(dirname $0)/lib.sh

# must be after source lib.sh b/c it loads $ENVLIB
export ROOTLESS_USER="pilferingpirate$RANDOM"

req_env_var "
CIRRUS_WORKING_DIR $CIRRUS_WORKING_DIR
GOSRC $GOSRC
SCRIPT_BASE $SCRIPT_BASE
ROOTLESS_USER $ROOTLESS_USER
"

if ! run_rootless
then
    die 86 "Error: Expected rootless env. var not set or empty"
fi

cd $GOSRC
make clean
setup_rootless

ssh $ROOTLESS_USER@localhost \
            -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -o CheckHostIP=no \
            $CIRRUS_WORKING_DIR/$SCRIPT_BASE/rootless_test.sh
