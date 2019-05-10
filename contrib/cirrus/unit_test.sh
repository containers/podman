#!/bin/bash

set -e
source $(dirname $0)/lib.sh

req_env_var GOSRC

set -x
cd "$GOSRC"
make install.tools
make localunit

case "$SPECIALMODE" in
    in_podman) ;&
    rootless) ;&
    none)
        make
        ;;
    windows) ;&
    darwin)
        make podman-remote-$SPECIALMODE
        ;;
    *)
        die 109 "Unsupported \$SPECIAL_MODE: $SPECIALMODE"
esac
