#!/bin/bash

set -e

source $(dirname $0)/lib.sh

req_env_var GOSRC
req_env_var CODECOV_TOKEN

cd "$GOSRC"
make codecov
