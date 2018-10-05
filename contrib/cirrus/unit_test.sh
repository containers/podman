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
        make localunit "BUILDTAGS=$BUILDTAGS"
        make "BUILDTAGS=$BUILDTAGS"
        ;;
    fedora-28)
        make localunit
        make
        ;;
    centos-7) ;&  # Continue to the next item
    rhel-7)
        stub 'unit testing not working on $OS_RELEASE_ID'
        ;;
    *) bad_os_id_ver ;;
esac
