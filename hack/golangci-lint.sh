#!/bin/bash

# Need to run linter twice to cover all the build tags code paths
set -e

# WARNING: This script executes on multiple operating systems that
# do not have the same version of Bash.  Specifically, Darwin uses
# a very old version, where modern features (like `declare -A`) are
# absent.

# Makefile calls script with the 'run' argument, but developers may not.
# Handle both cases transparently.
[[ $1 == run ]] && shift

BUILD_TAGS_DEFAULT="apparmor,seccomp,selinux"
BUILD_TAGS_ABI="$BUILD_TAGS_DEFAULT,systemd"
BUILD_TAGS_TUNNEL="$BUILD_TAGS_DEFAULT,remote"
BUILD_TAGS_REMOTE="remote,containers_image_openpgp"

SKIP_DIRS_ABI=""
SKIP_DIRS_TUNNEL=""
SKIP_DIRS_REMOTE="libpod/events,pkg/machine/qemu,pkg/machine/wsl,test"

declare -a to_lint
to_lint=(ABI TUNNEL)

# Special-case, for Darwin and Windows only "remote" linting is possible and required.
if [[ "$GOOS" == "windows" ]] || [[ "$GOOS" == "darwin" ]]; then
  to_lint=(REMOTE)
fi

for to_lint in "${to_lint[@]}"; do
  tags_var="BUILD_TAGS_${to_lint}"
  skip_var="SKIP_DIRS_${to_lint}"
  echo ""
  echo Running golangci-lint for "$to_lint"
  echo Build Tags          "$to_lint": ${!tags_var}
  echo Skipped directories "$to_lint": ${!skip_var}
  (
    # Make it really easy for a developer to copy-paste the command-line
    # to focus or debug a single, specific linting category.
    set -x
    ./bin/golangci-lint run --timeout=10m --build-tags="${!tags_var}" --exclude-dirs="${!skip_var}" "$@"
  )
done
