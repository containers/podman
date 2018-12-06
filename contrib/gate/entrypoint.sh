#!/bin/bash

[[ -n "$SRCPATH" ]] || \
    ( echo "ERROR: \$SRCPATH must be non-empty" && exit 1 )
[[ -n "$GOSRC" ]] || \
    ( echo "ERROR: \$GOSRC must be non-empty" && exit 2 )
[[ -r "${SRCPATH}/contrib/gate/Dockerfile" ]] || \
    ( echo "ERROR: Expecting libpod repository root at $SRCPATH" && exit 3 )

# Working from a copy avoids needing to perturb the actual source files
mkdir -p "$GOSRC"
/usr/bin/rsync --recursive --links --quiet --safe-links \
               --perms --times "${SRCPATH}/" "${GOSRC}/"
cd "$GOSRC"
make "$@"
