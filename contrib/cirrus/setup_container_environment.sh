#!/bin/bash
set -e

source $(dirname $0)/lib.sh

req_env_var GOSRC OS_RELEASE_ID CONTAINER_RUNTIME

DIST=$OS_RELEASE_ID
IMAGE=${DIST}podmanbuild

# Since CRIU 3.11 has been pushed to Fedora 28 the checkpoint/restore
# test cases are actually run. As CRIU uses iptables to lock and unlock
# the network during checkpoint and restore it needs the following two
# modules loaded.
modprobe ip6table_nat || :
modprobe iptable_nat || :

# Build the test image
${CONTAINER_RUNTIME} build -t ${IMAGE} -f Dockerfile.${DIST} .
