#!/bin/bash

set -e
source $HOME/.bash_profile

cd $GOSRC
source $(dirname $0)/lib.sh

req_env_var "
GOSRC $GOSRC
OS_RELEASE_ID $OS_RELEASE_ID
OS_RELEASE_VER $OS_RELEASE_VER
"

if ! run_rootless
then
    echo "Error: Expected rootless env. vars not set or empty"
    exit 1
fi

echo "."
echo "Hello, my name is $USER and I live in $PWD can I be your friend?"

record_timestamp "rootless test start"

cd "$GOSRC"
case "${OS_RELEASE_ID}-${OS_RELEASE_VER}" in
    ubuntu-18) ;&  # Continue to the next item
    fedora-29) ;&
    fedora-28)
        make
        ;;
    *) bad_os_id_ver ;;
esac

record_timestamp "rootless test end"
