#!/usr/bin/env bash

# This script is intended to be a convenience, to be called from the
# Makefile `.install.swagger` target.  Any other usage is not recommended.

BIN="$BINDIR/swagger"

die() { echo "${1:-No error message given} (from $(basename $0))"; exit 1; }

function install() {
    echo "Installing swagger v$VERSION into $BIN"
    curl -sS --retry 5 --location -o $BIN \
        https://github.com/go-swagger/go-swagger/releases/download/v$VERSION/swagger_${GOOS}_${GOARCH}
    chmod +x $BIN
    $BIN version
}

for req_var in VERSION BINDIR GOOS GOARCH; do
    [[ -n "${!req_var}" ]] || die "\$$req_var is empty or undefined"
done

if [ ! -x "$BIN" ]; then
    install
else
    $BIN version | grep "$VERSION"
    if [[ "$?" -eq 0 ]]; then
        echo "Using existing $BIN"
    else
        install
    fi
fi
