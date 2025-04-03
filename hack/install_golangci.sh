#!/usr/bin/env bash

# This script is intended to be a convenience, to be called from the
# Makefile `.install.golangci-lint` target.  Any other usage is not recommended.

die() { echo "${1:-No error message given} (from $(basename "$0"))"; exit 1; }

[ -n "$VERSION" ] || die "\$VERSION is empty or undefined"

# Strip the leading v, if found.
VERSION=${VERSION#v}

function install() {
    local retry=$1

    local msg="Installing golangci-lint v$VERSION into $BIN"
    if [[ $retry -ne 0 ]]; then
        msg+=" - retry #$retry"
    fi
    echo "$msg"

    curl -sSfL --retry 5 https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $BINDIR "v$VERSION"
}

BINDIR="./bin"
BIN="$BINDIR/golangci-lint"
if [ -x "$BIN" ] && $BIN --version | grep "$VERSION"; then
    echo "Using existing $BIN"
    exit 0
fi

# This flakes much too frequently with "crit unable to find v1.51.1"
for retry in $(seq 0 5); do
    install "$retry" && exit 0
    sleep 5
done
