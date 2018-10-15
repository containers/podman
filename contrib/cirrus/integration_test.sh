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
make install.tools
make
make install PREFIX=/usr ETCDIR=/etc
make localintegration
