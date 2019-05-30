#!/bin/bash

set -e
source $(dirname $0)/lib.sh

req_env_var GOSRC SCRIPT_BASE OS_RELEASE_ID OS_RELEASE_VER CONTAINER_RUNTIME

cd "$GOSRC"

if [[ "$SPECIALMODE" == "in_podman" ]]
then
    ${CONTAINER_RUNTIME} run --rm --privileged --net=host \
        -v $GOSRC:$GOSRC:Z \
        --workdir $GOSRC \
        -e "CGROUP_MANAGER=cgroupfs" \
        -e "STORAGE_OPTIONS=--storage-driver=vfs" \
        -e "CRIO_ROOT=$GOSRC" \
        -e "PODMAN_BINARY=/usr/bin/podman" \
        -e "CONMON_BINARY=/usr/libexec/podman/conmon" \
        -e "DIST=$OS_RELEASE_ID" \
        -e "CONTAINER_RUNTIME=$CONTAINER_RUNTIME" \
        $IN_PODMAN_IMAGE bash $GOSRC/$SCRIPT_BASE/container_test.sh -b -i -t

    exit $?
elif [[ "$SPECIALMODE" == "rootless" ]]
then
    req_env_var ROOTLESS_USER

    if [[ "$USER" == "$ROOTLESS_USER" ]]
    then
        $GOSRC/$SCRIPT_BASE/rootless_test.sh
    else
        ssh $ROOTLESS_USER@localhost \
                -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -o CheckHostIP=no \
                $GOSRC/$SCRIPT_BASE/rootless_test.sh
    fi
else
    make
    make install PREFIX=/usr ETCDIR=/etc
    make test-binaries
    if [[ "$TEST_REMOTE_CLIENT" == "true" ]]
    then
        make remoteintegration
    else
        make localintegration
    fi
    exit $?
fi
