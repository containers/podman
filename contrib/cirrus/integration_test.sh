#!/bin/bash

set -e
source $(dirname $0)/lib.sh

req_env_var "
GOSRC $GOSRC
OS_RELEASE_ID $OS_RELEASE_ID
OS_RELEASE_VER $OS_RELEASE_VER
"

record_timestamp "integration test start"

clean_env

set -x
cd "$GOSRC"
case "${OS_RELEASE_ID}-${OS_RELEASE_VER}" in
    ubuntu-18)
        make install PREFIX=/usr ETCDIR=/etc
        make test-binaries
        SKIP_USERNS=1 make localintegration
        ;;
    fedora-29) ;&  # Continue to the next item
    fedora-28) ;&
    centos-7) ;&
    rhel-7)
        make install PREFIX=/usr ETCDIR=/etc
        make podman-remote
        install bin/podman-remote /usr/bin
        make test-binaries
        make localintegration
        ;;
    *) bad_os_id_ver ;;
esac

record_timestamp "integration test end"
