#!/bin/bash

set -e

remote=0

# The TEST_REMOTE_CLIENT environment variable decides whether
# to test varlinke
if [[ "$TEST_REMOTE_CLIENT" == "true" ]]; then
    remote=1
fi

source $(dirname $0)/lib.sh

if [[ "$UID" == "0" ]]
then
    echo "Error: Expected to be running as a regular user"
    exit 1
fi

# Which set of tests to run; possible alternative is "system"
TESTSUITE=integration
if [[ -n "$*" ]]; then
    TESTSUITE="$1"
fi

# Ensure environment setup correctly
req_env_var GOSRC ROOTLESS_USER

echo "."
echo "Hello, my name is $USER and I live in $PWD can I be your friend?"
echo "."

export PODMAN_VARLINK_ADDRESS=unix:/tmp/podman-$(id -u)
show_env_vars

set -x
cd "$GOSRC"
make
make varlink_generate
make test-binaries
if [ $remote -eq 0 ]; then
    make local${TESTSUITE}
else
    make remote${TESTSUITE}
fi
