#!/bin/bash

set -e
source $(dirname $0)/lib.sh

req_env_var GOSRC OS_RELEASE_ID OS_RELEASE_VER

set -x
cd "$GOSRC"

case "${OS_RELEASE_ID}" in
    ubuntu) ;&  # Continue to the next item
    fedora)
        make install.tools
        make
        make test-binaries
        ;;
    *) bad_os_id_ver ;;
esac

make localsystem
