#!/bin/bash

set -e
source $(dirname $0)/lib.sh

req_env_var "
OS_RELEASE_ID $OS_RELEASE_ID
OS_RELEASE_VER $OS_RELEASE_VER
"

show_env_vars

set -x
cd "$GOSRC"

case "${OS_RELEASE_ID}-${OS_RELEASE_VER}" in
    ubuntu-18)
        make install.tools "BUILDTAGS=$BUILDTAGS"
        make validate "BUILDTAGS=$BUILDTAGS"
        # make lint "BUILDTAGS=$BUILDTAGS"
        ;;
    fedora-28) ;&
    centos-7) ;&
    rhel-7)
        make install.tools
        make validate
        # make lint
        ;;
    *) bad_os_id_ver ;;
esac
