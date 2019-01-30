#!/bin/bash

set -e
source $(dirname $0)/lib.sh

req_env_var "
GOSRC $GOSRC
OS_RELEASE_ID $OS_RELEASE_ID
OS_RELEASE_VER $OS_RELEASE_VER
"

record_timestamp "unit test start"

clean_env

set -x
cd "$GOSRC"
case "${OS_RELEASE_ID}-${OS_RELEASE_VER}" in
    ubuntu-18) ;&  # Continue to the next item
    fedora-29) ;&
    fedora-28) ;&
    centos-7) ;&
    rhel-7)
        make install.tools
        make localunit
        make
        ;;
    *) bad_os_id_ver ;;
esac

record_timestamp "unit test end"
