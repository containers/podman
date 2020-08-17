#!/usr/bin/env bash

set -e

die() {
    echo "${2:-FATAL ERROR (but no message given!)} (gate container entrypoint)"
    exit ${1:-1}
}

[[ -n "$SRCPATH" ]] || die 1 "ERROR: \$SRCPATH must be non-empty"
[[ -n "$GOPATH" ]] || die 2 "ERROR: \$GOPATH must be non-empty"
[[ -n "$GOSRC" ]] || die 3 "ERROR: \$GOSRC must be non-empty"
[[ -r "${SRCPATH}/contrib/gate/Dockerfile" ]] || \
    die 4 "ERROR: Expecting libpod repository root at $SRCPATH"

# Working from a copy avoids needing to perturb the actual source files
# if/when developers use gate container for local testing
echo "Copying $SRCPATH to $GOSRC"
mkdir -vp "$GOSRC"
/usr/bin/rsync --recursive --links --quiet --safe-links \
               --perms --times --delete "${SRCPATH}/" "${GOSRC}/"
cd "$GOSRC"
exec make "$@"
