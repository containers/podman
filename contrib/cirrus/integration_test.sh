#!/bin/bash

set -e
source $(dirname $0)/lib.sh

req_env_var "
GOSRC $GOSRC
SCRIPT_BASE $SCRIPT_BASE
OS_RELEASE_ID $OS_RELEASE_ID
OS_RELEASE_VER $OS_RELEASE_VER
CONTAINER_RUNTIME $CONTAINER_RUNTIME
"

exit_handler() {
    set +ex
    record_timestamp "integration test end"
}
trap exit_handler EXIT

record_timestamp "integration test start"

cd "$GOSRC"
go get github.com/onsi/ginkgo/ginkgo
go get github.com/onsi/gomega/...

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
    req_env_var "ROOTLESS_USER $ROOTLESS_USER"
    set -x
    ssh $ROOTLESS_USER@localhost \
                -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -o CheckHostIP=no \
                $GOSRC/$SCRIPT_BASE/rootless_test.sh
    exit $?
elif [[ "$SPECIALMODE" == "remote_client" ]]
then
    set -x
    make
    make podman-remote
    make install install.remote PREFIX=/usr ETCDIR=/etc
    clean_env
    make remoteintegration
    exit $?
else
    set -x
    make
    make install PREFIX=/usr ETCDIR=/etc
    clean_env
    make localintegration
    exit $?
fi
