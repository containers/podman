#!/bin/bash

set -e
source $(dirname $0)/lib.sh

MAGIC_RE='\*\*\*\s*CIRRUS:\s*SYSTEM\s*TEST\s*\*\*\*'
if ! echo "$CIRRUS_CHANGE_MESSAGE" | egrep -q "$MAGIC_RE"
then
    echo "Skipping system-testing because PR title or description"
    echo "does not match regular expression: $MAGIC_RE"
    exit 0
fi

req_env_var "
GOSRC $GOSRC
OS_RELEASE_ID $OS_RELEASE_ID
OS_RELEASE_VER $OS_RELEASE_VER
"

show_env_vars

set -x
cd "$GOSRC"
make localsystem
