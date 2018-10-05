#!/bin/bash

set -e
source $(dirname $0)/lib.sh

req_env_var "
GOSRC $GOSRC
OS_RELEASE_ID $OS_RELEASE_ID
OS_RELEASE_VER $OS_RELEASE_VER
"

show_env_vars

set -x
cd "$GOSRC"
case "${OS_RELEASE_ID}-${OS_RELEASE_VER}" in
    ubuntu-18)
        make install PREFIX=/usr ETCDIR=/etc "BUILDTAGS=$BUILDTAGS"
        make test-binaries "BUILDTAGS=$BUILDTAGS"
        SKIP_USERNS=1 make localintegration "BUILDTAGS=$BUILDTAGS"
        ;;
    fedora-28) ;&  # Continue to the next item
    centos-7) ;&
    rhel-7)
        stub 'integration testing not working on $OS_RELEASE_ID'
        ;;
    *) bad_os_id_ver ;;
esac
