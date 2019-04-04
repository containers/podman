#!/bin/bash

set -e
source $(dirname $0)/lib.sh

req_env_var GOSRC OS_RELEASE_ID OS_RELEASE_VER

clean_env

set -x
cd "$GOSRC"
make install.tools
make localunit
make
