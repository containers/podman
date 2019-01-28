#!/bin/bash

set -e
source $(dirname $0)/lib.sh

req_env_var "
GOSRC $GOSRC
OS_RELEASE_ID $OS_RELEASE_ID
OS_RELEASE_VER $OS_RELEASE_VER
"

clean_env

set -x
cd "$GOSRC"

case "${OS_RELEASE_ID}-${OS_RELEASE_VER}" in
    ubuntu-18) ;&  # Continue to the next item
    fedora-28) ;&
    fedora-29) ;&
    centos-7) ;&
    rhel-7)
        make install.tools
        make
        make test-binaries
        ;;
    *) bad_os_id_ver ;;
esac

make localsystem
