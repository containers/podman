#!/usr/bin/env bash

# Need to run linter twice to cover all the build tags code paths
set -e

declare -A BUILD_TAGS
# TODO: add systemd tag
BUILD_TAGS[default]="apparmor,seccomp,selinux,linter,no_libsubid"
BUILD_TAGS[abi]="${BUILD_TAGS[default]},!remoteclient"
BUILD_TAGS[tunnel]="${BUILD_TAGS[default]},remote,remoteclient"

declare -A SKIP_DIRS
SKIP_DIRS[abi]=""
# TODO: add "remote" build tag to pkg/api
SKIP_DIRS[tunnel]="pkg/api"

[[ $1 == run ]] && shift

for i in tunnel abi; do
  echo ""
  echo Running golangci-lint for "$i"
  echo Build Tags          "$i": ${BUILD_TAGS[$i]}
  echo Skipped directories "$i": ${SKIP_DIRS[$i]}
  ./bin/golangci-lint run --build-tags=${BUILD_TAGS[$i]} --skip-dirs=${SKIP_DIRS[$i]} "$@"
done
