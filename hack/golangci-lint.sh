#!/usr/bin/env bash

# Need to run linter twice to cover all the build tags code paths
set -e

# Dedicated block for Darwin: OS doesn't support the rest of this
# script, only needs to check 'remote', and its golangci-lint needs
# specialized arguments.
if [[ $(uname -s) == "Darwin" ]] || [[ "$GOOS" == "darwin" ]]; then
  declare -a DARWIN_SKIP_DIRS
  DARWIN_SKIP_DIRS=(
    libpod/events
    pkg/api
    pkg/domain/infra/abi
    pkg/machine/qemu
    pkg/trust
    test
  )
  echo ""
  echo Running golangci-lint for "remote"
  echo Build Tags          "remote": remote
  echo Skipped directories "remote": ${DARWIN_SKIP_DIRS[*]}
  ./bin/golangci-lint run --build-tags="remote" \
    --skip-dirs=$(tr ' ' ',' <<<"${DARWIN_SKIP_DIRS[@]}")
  exit 0  # All done, don't execute anything below, it will break on Darwin
fi

declare -A BUILD_TAGS
BUILD_TAGS[default]="apparmor,seccomp,selinux"
BUILD_TAGS[abi]="${BUILD_TAGS[default]},systemd"
BUILD_TAGS[tunnel]="${BUILD_TAGS[default]},remote"

declare -A SKIP_DIRS
SKIP_DIRS[abi]=""
# TODO: add "remote" build tag to pkg/api
SKIP_DIRS[tunnel]="pkg/api,pkg/domain/infra/abi"

[[ $1 == run ]] && shift

for i in tunnel abi; do
  echo ""
  echo Running golangci-lint for "$i"
  echo Build Tags          "$i": ${BUILD_TAGS[$i]}
  echo Skipped directories "$i": ${SKIP_DIRS[$i]}
  ./bin/golangci-lint run --build-tags=${BUILD_TAGS[$i]} --skip-dirs=${SKIP_DIRS[$i]} "$@"
done
