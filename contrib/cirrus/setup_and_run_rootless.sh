#!/bin/bash

set -ex

source $(dirname $0)/lib.sh

req_env_var "
CIRRUS_WORKING_DIR $CIRRUS_WORKING_DIR
GOSRC $GOSRC
SCRIPT_BASE $SCRIPT_BASE
ROOTLESS_USER $ROOTLESS_USER
ROOTLESS_UID $ROOTLESS_UID
ROOTLESS_GID $ROOTLESS_GID
"

if run_rootless
then
    die 86 "Error: Expected rootless env. vars not set or empty"
fi

cd $GOSRC
setup_rootless

ssh $ROOTLESS_USER@localhost \
            -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -o CheckHostIP=no \
            $CIRRUS_WORKING_DIR/$SCRIPT_BASE/rootless_test.sh
