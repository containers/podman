#!/bin/bash -e

# Need to run linter twice to cover all the build tags code paths

declare -A BUILD_TAGS
BUILD_TAGS[default]="apparmor,seccomp,selinux"
BUILD_TAGS[abi]="${BUILD_TAGS[default]},ABISupport,varlink,!remoteclient"
BUILD_TAGS[tunnel]="${BUILD_TAGS[default]},!ABISupport,!varlink,remoteclient"

[[ $1 == run ]] && shift

for i in tunnel abi; do
  echo Build Tags: ${BUILD_TAGS[$i]}
  golangci-lint run --build-tags=${BUILD_TAGS[$i]} "$@"
done
