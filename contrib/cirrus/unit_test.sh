#!/bin/bash

set -e
source $(dirname $0)/lib.sh

req_env_var GOSRC

set -x
cd "$GOSRC"
make install.tools
make localunit
make
