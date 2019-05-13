#!/bin/bash

set -e
source $(dirname $0)/lib.sh

req_env_var GOSRC SCRIPT_BASE OS_RELEASE_ID OS_RELEASE_VER CONTAINER_RUNTIME

cd "$GOSRC"

if [[ "$SPECIALMODE" == "in_podman" ]]
then
    die 4 "NOT SUPPORTED"
elif [[ "$SPECIALMODE" == "rootless" ]]
then
    req_env_var ROOTLESS_USER
    set -x
    ssh $ROOTLESS_USER@localhost \
                -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -o CheckHostIP=no \
                $GOSRC/$SCRIPT_BASE/rootless_test.sh
    exit $?
else
    die 5 "NOT SUPPORTED"
fi
