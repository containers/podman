#!/bin/bash

set -e

source $(dirname $0)/lib.sh

req_env_var GOSRC SCRIPT_BASE OS_RELEASE_ID OS_RELEASE_VER CONTAINER_RUNTIME

# Our name must be of the form xxxx_test or xxxx_test.sh, where xxxx is
# the test suite to run; currently (2019-05) the only option is 'integration'
# but pr2947 intends to add 'system'.
TESTSUITE=$(expr $(basename $0) : '\(.*\)_test')
if [[ -z $TESTSUITE ]]; then
    die 1 "Script name is not of the form xxxx_test.sh"
fi

cd "$GOSRC"

case "$SPECIALMODE" in
    in_podman)
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
        ;;
    rootless)
        req_env_var ROOTLESS_USER
        ssh $ROOTLESS_USER@localhost \
                -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no \
                -o CheckHostIP=no $GOSRC/$SCRIPT_BASE/rootless_test.sh ${TESTSUITE}
        ;;
    udica)
        make
        make install PREFIX=/usr ETCDIR=/etc
        cd /tmp
        git clone https://github.com/containers/udica
        cd udica && python3 ./setup.py install
        podman run -d -v /home:/home:ro -v /var/spool:/var/spool:rw -p 21:21 fedora sleep 1h
        podman inspect -l > container.json
        udica -j container.json my_container || \
            echo "FIXME: Notify someone it broke"
        ;;
    none)
        make
        make install PREFIX=/usr ETCDIR=/etc
        make test-binaries
        if [[ "$TEST_REMOTE_CLIENT" == "true" ]]
        then
            make remote${TESTSUITE}
        else
            make local${TESTSUITE}
        fi
        ;;
    windows) ;&  # for podman-remote building only
    darwin)
        warn '' "No $SPECIALMODE remote client integration tests configured"
        ;;
    *)
        die 110 "Unsupported \$SPECIAL_MODE: $SPECIALMODE"
esac
