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
make install.tools
make localunit
make

record_timestamp "unit test end"
