#!/bin/bash

# Run golangci-lint with different sets of build tags.
set -e

# WARNING: This script executes on multiple operating systems that
# do not have the same version of Bash.  Specifically, Darwin uses
# a very old version, where modern features (like `declare -A`) are
# absent.

echo "Linting for GOOS=$GOOS"
# Special case: for Darwin and Windows only "remote" linting is possible and required.
if [[ "$GOOS" == "windows" || "$GOOS" == "darwin" ]]; then
  (
    set -x
    ./bin/golangci-lint run --build-tags="remote,containers_image_openpgp" "$@"
  )
  exit 0
fi

# Normal case (Linux): run linter for various sets of build tags.
TAGS="apparmor,seccomp,selinux"
for EXTRA_TAGS in "" ",systemd" ",remote"; do
  (
    # Make it really easy for a developer to copy-paste the command-line
    # to focus or debug a single, specific linting category.
    set -x
    ./bin/golangci-lint run --build-tags="${TAGS}${EXTRA_TAGS}" "$@"
  )
done
