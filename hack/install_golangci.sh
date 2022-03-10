#!/usr/bin/env bash

die() { echo "${1:-No error message given} (from $(basename $0))"; exit 1; }

[ -n "$VERSION" ] || die "\$VERSION is empty or undefined"

function install() {
    echo "Installing golangci-lint v$VERSION into $BIN"
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v$VERSION
}

BIN="./bin/golangci-lint"
if [ ! -x "$BIN" ]; then
	install
else
    # Prints its own file name as part of --version output
    $BIN --version | grep "$VERSION"
    if [ $? -eq 0 ]; then
        echo "Using existing $(dirname $BIN)/$($BIN --version)"
    else
        install
    fi
fi
