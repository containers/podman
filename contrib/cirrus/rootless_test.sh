#!/bin/bash

set -e

source $(dirname $0)/lib.sh

req_env_var GOSRC ROOTLESS_USER

if [[ "$UID" == "0" ]]
then
    echo "Error: Expected to be running as a regular user"
    exit 1
fi

echo "."
echo "Hello, my name is $USER and I live in $PWD can I be your friend?"

export PODMAN_VARLINK_ADDRESS=unix:/tmp/podman-$(id -u)
show_env_vars

cd "$GOSRC"
make
make varlink_generate
make test-binaries
make ginkgo
make ginkgo-remote
