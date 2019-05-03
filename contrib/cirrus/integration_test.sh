#!/bin/bash

set -e
source $(dirname $0)/lib.sh

req_env_var GOSRC SCRIPT_BASE OS_RELEASE_ID OS_RELEASE_VER CONTAINER_RUNTIME

cd "$GOSRC"

if [[ "$SPECIALMODE" == "in_podman" ]]
then
    set -x
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
        ${OS_RELEASE_ID}podmanbuild bash $GOSRC/$SCRIPT_BASE/container_test.sh -b -i -t -n

    exit $?
elif [[ "$SPECIALMODE" == "rootless" ]]
then
    req_env_var ROOTLESS_USER
    set -x
    ssh $ROOTLESS_USER@localhost \
                -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -o CheckHostIP=no \
                $GOSRC/$SCRIPT_BASE/rootless_test.sh
    exit $?
else
    set -x
    make
    make install PREFIX=/usr ETCDIR=/etc
    make test-binaries
    clean_env

    case "${OS_RELEASE_ID}-${OS_RELEASE_VER}" in
        ubuntu-18) ;;
        fedora-29) ;&  # Continue to the next item
        fedora-28) ;&
        centos-7) ;&
        rhel-7)
            make podman-remote
            install bin/podman-remote /usr/bin
            ;;
        *) bad_os_id_ver ;;
    esac
    make localintegration
    exit $?
fi
