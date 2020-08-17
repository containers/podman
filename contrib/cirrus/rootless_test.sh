#!/usr/bin/env bash

set -e

echo "$(date --rfc-3339=seconds) $(basename $0) started with '$*'"

source $(dirname $0)/lib.sh

if [[ "$UID" == "0" ]]
then
    echo "$(basename $0): Error: Expected to be running as a regular user"
    exit 1
fi

TESTSUITE=${1?Missing TESTSUITE argument (arg1)}
LOCAL_OR_REMOTE=${2?Missing LOCAL_OR_REMOTE argument (arg2)}

# Ensure environment setup correctly
req_env_var GOSRC ROOTLESS_USER

echo "."
echo "Hello, my name is $USER and I live in $PWD can I be your friend?"
echo "."

show_env_vars

set -x
cd "$GOSRC"
make
make varlink_generate
make test-binaries
make ${LOCAL_OR_REMOTE}${TESTSUITE}
