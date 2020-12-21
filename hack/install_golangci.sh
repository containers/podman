#!/usr/bin/env bash

set -e

die() { echo "${1:-No error message given} (from $(basename $0))"; exit 1; }

[ -n "$VERSION" ] || die "\$VERSION is empty or undefined"
[ -n "$GOBIN" ] || die "\$GOBIN is empty or undefined"

BIN="$GOBIN/golangci-lint"
if [ ! -x "$BIN" ]; then
    echo "Installing golangci-lint v$VERSION into $GOBIN"
    curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $GOBIN v$VERSION
else
    # Prints its own file name as part of --version output
    echo "Using existing $(dirname $BIN)/$($BIN --version)"
fi
