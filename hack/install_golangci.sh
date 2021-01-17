#!/usr/bin/env bash

set -e

die() { echo "${1:-No error message given} (from $(basename $0))"; exit 1; }

[ -n "$VERSION" ] || die "\$VERSION is empty or undefined"
[ -n "$GOBIN" ] || die "\$GOBIN is empty or undefined"

BIN="./bin/golangci-lint"
if [ ! -x "$BIN" ]; then
    echo "Installing golangci-lint v$VERSION into $BIN"
    curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b ./bin v$VERSION
else
    # Prints its own file name as part of --version output
    echo "Using existing $(dirname $BIN)/$($BIN --version)"
fi
